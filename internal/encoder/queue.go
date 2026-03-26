package encoder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// JobUpdateCallback is called when a job's state changes.
// It is called from the worker goroutine; GUI code must synchronize.
type JobUpdateCallback func(job *Job)

type Queue struct {
	mu       sync.Mutex
	jobs     []*Job
	ffmpeg   *FFmpeg
	settings *Settings

	paused   bool
	pauseCh  chan struct{}
	onUpdate JobUpdateCallback

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	addCh chan *Job
}

func NewQueue(ffmpeg *FFmpeg, settings *Settings, onUpdate JobUpdateCallback) *Queue {
	ctx, cancel := context.WithCancel(context.Background())
	q := &Queue{
		ffmpeg:   ffmpeg,
		settings: settings,
		onUpdate: onUpdate,
		paused:   true, // Start paused — user must press Start
		pauseCh:  make(chan struct{}, 1),
		ctx:      ctx,
		cancel:   cancel,
		addCh:    make(chan *Job, 64),
	}

	q.wg.Add(1)
	go q.worker()

	return q
}

// HasPendingJobs returns true if there are jobs waiting to be processed.
func (q *Queue) HasPendingJobs() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range q.jobs {
		j.Mu.Lock()
		s := j.Status
		j.Mu.Unlock()
		if s == StatusPending {
			return true
		}
	}
	return false
}

// IsRunning returns true if the queue is not paused (actively processing).
func (q *Queue) IsRunning() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return !q.paused
}

func (q *Queue) AddFile(path string) {
	if !IsSupportedFile(path) {
		return
	}

	q.mu.Lock()
	// Check for duplicate
	for _, j := range q.jobs {
		j.Mu.Lock()
		dup := j.InputPath == path && (j.Status == StatusPending || j.Status == StatusEncoding)
		j.Mu.Unlock()
		if dup {
			q.mu.Unlock()
			return
		}
	}
	q.mu.Unlock()

	job := NewJob(path)

	// Probe the file for metadata
	probe, err := q.ffmpeg.Probe(path)
	if err == nil {
		job.OriginalBitrate = probe.VideoBitrateKbps
		job.Duration = probe.DurationSec
		job.OriginalSize = probe.FileSize
	} else {
		// Fallback: get file size from OS
		if info, ferr := os.Stat(path); ferr == nil {
			job.OriginalSize = info.Size()
		}
	}

	q.mu.Lock()
	q.jobs = append(q.jobs, job)
	q.mu.Unlock()

	q.onUpdate(job)

	select {
	case q.addCh <- job:
	default:
	}
}

func (q *Queue) Jobs() []*Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make([]*Job, len(q.jobs))
	copy(result, q.jobs)
	return result
}

// RemoveJobs removes jobs at the given indices.
// Encoding jobs are cancelled first; completed jobs are just removed from the list.
func (q *Queue) RemoveJobs(indices []int) {
	// Sort descending so removal doesn't shift indices
	// Simple approach: build a set, then filter
	removeSet := make(map[int]bool, len(indices))
	for _, idx := range indices {
		removeSet[idx] = true
	}

	q.mu.Lock()
	// Cancel encoding/pending jobs first
	for idx := range removeSet {
		if idx < 0 || idx >= len(q.jobs) {
			continue
		}
		job := q.jobs[idx]
		job.Mu.Lock()
		switch job.Status {
		case StatusEncoding:
			if job.cancelFunc != nil {
				job.cancelFunc()
			}
		case StatusPending:
			job.Status = StatusCancelled
		}
		job.Mu.Unlock()
	}

	// Remove from list
	var kept []*Job
	for i, j := range q.jobs {
		if !removeSet[i] {
			kept = append(kept, j)
		}
	}
	q.jobs = kept
	q.mu.Unlock()
}

func (q *Queue) Pause() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.paused = true
}

func (q *Queue) Resume() {
	q.mu.Lock()
	q.paused = false
	q.mu.Unlock()

	select {
	case q.pauseCh <- struct{}{}:
	default:
	}
}

func (q *Queue) IsPaused() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.paused
}

func (q *Queue) Stop() {
	q.cancel()
	q.wg.Wait()
}

// CancelJob cancels the job at the given index.
// Encoding jobs are killed via context cancel; pending jobs are marked cancelled.
func (q *Queue) CancelJob(index int) {
	q.mu.Lock()
	if index < 0 || index >= len(q.jobs) {
		q.mu.Unlock()
		return
	}
	job := q.jobs[index]
	q.mu.Unlock()

	job.Mu.Lock()
	switch job.Status {
	case StatusEncoding:
		// Cancel the running ffmpeg process
		if job.cancelFunc != nil {
			job.cancelFunc()
		}
		// Status will be set to StatusCancelled in processJob when context error is detected
	case StatusPending:
		job.Status = StatusCancelled
		job.Mu.Unlock()
		q.onUpdate(job)
		return
	default:
		// Already done/error/cancelled — nothing to do
	}
	job.Mu.Unlock()
}

// CompletedCount returns (completed, total).
func (q *Queue) CompletedCount() (int, int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	done := 0
	for _, j := range q.jobs {
		j.Mu.Lock()
		if j.Status == StatusDone || j.Status == StatusError || j.Status == StatusSkipped || j.Status == StatusCancelled {
			done++
		}
		j.Mu.Unlock()
	}
	return done, len(q.jobs)
}

// AllDone returns true if all jobs are finished.
func (q *Queue) AllDone() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) == 0 {
		return false
	}
	for _, j := range q.jobs {
		j.Mu.Lock()
		s := j.Status
		j.Mu.Unlock()
		if s == StatusPending || s == StatusEncoding {
			return false
		}

	}
	return true
}

func (q *Queue) worker() {
	defer q.wg.Done()

	for {
		// Find next pending job
		var job *Job
		q.mu.Lock()
		for _, j := range q.jobs {
			j.Mu.Lock()
			s := j.Status
			j.Mu.Unlock()
			if s == StatusPending {
				job = j
				break
			}
		}
		q.mu.Unlock()

		if job == nil {
			// Wait for new jobs
			select {
			case <-q.ctx.Done():
				return
			case <-q.addCh:
				continue
			}
		}

		// Check pause
		q.mu.Lock()
		paused := q.paused
		q.mu.Unlock()
		if paused {
			select {
			case <-q.ctx.Done():
				return
			case <-q.pauseCh:
				continue
			}
		}

		q.processJob(job)
	}
}

func (q *Queue) processJob(job *Job) {
	settings := q.settings.Load()

	// Create per-job context (child of queue context)
	jobCtx, jobCancel := context.WithCancel(q.ctx)
	defer jobCancel()

	job.Mu.Lock()
	job.Status = StatusEncoding
	job.cancelFunc = jobCancel
	job.Mu.Unlock()
	q.onUpdate(job)

	// Determine output path
	outputPath := q.outputPath(job.InputPath, settings)

	// Ensure output directory exists
	outDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		errMsg := fmt.Sprintf("出力フォルダ作成失敗: %v", err)
		fmt.Fprintf(os.Stderr, "[ERROR] %s: %s\n", job.FileName, errMsg)
		job.Mu.Lock()
		job.Status = StatusError
		job.ErrorMsg = errMsg
		job.cancelFunc = nil
		job.Mu.Unlock()
		q.onUpdate(job)
		return
	}

	// Calculate target bitrate
	targetKbps := CalculateTargetBitrate(job.OriginalBitrate, settings.DecayRate)

	opts := EncodeOptions{
		InputPath:   job.InputPath,
		OutputPath:  outputPath,
		TargetKbps:  targetKbps,
		LowPriority: settings.LowPriority,
	}

	err := q.ffmpeg.Encode(jobCtx, opts, func(pct float64) {
		job.Mu.Lock()
		job.Progress = pct
		job.Mu.Unlock()
		q.onUpdate(job)
	}, job.Duration)

	if err != nil {
		// Remove partial output file
		os.Remove(outputPath)

		// Check if this was a user-initiated cancel (jobCtx cancelled but queue still alive)
		if jobCtx.Err() != nil && q.ctx.Err() == nil {
			job.Mu.Lock()
			job.Status = StatusCancelled
			job.cancelFunc = nil
			job.Mu.Unlock()
			q.onUpdate(job)
			return
		}

		errMsg := err.Error()
		fmt.Fprintf(os.Stderr, "[ERROR] %s: %s\n", job.FileName, errMsg)

		job.Mu.Lock()
		job.Status = StatusError
		job.ErrorMsg = errMsg
		job.cancelFunc = nil
		job.Mu.Unlock()
		q.onUpdate(job)
		return
	}

	// Get output file size
	if info, err := os.Stat(outputPath); err == nil {
		job.Mu.Lock()
		job.OutputSize = info.Size()
		job.Mu.Unlock()
	}

	// If move-to-trash is enabled, move original and rename output to original location
	if settings.MoveToTrash {
		if err := MoveToTrash(job.InputPath); err != nil {
			// Trash move failure is non-fatal (spec: log only)
			_ = err
		} else {
			// Rename output to original filename at original location
			finalPath := job.InputPath
			if outputPath != finalPath {
				os.Rename(outputPath, finalPath)
			}
		}
	}

	job.Mu.Lock()
	job.OutputPath = outputPath
	job.Status = StatusDone
	job.Progress = 100
	job.cancelFunc = nil
	job.Mu.Unlock()
	q.onUpdate(job)
}

// outputPath builds the output file path based on settings.
// Default: saves to "EZ265" subfolder alongside the source file.
// If MoveToTrash: uses a temp name in the subfolder, later renamed to original path.
func (q *Queue) outputPath(inputPath string, settings SettingsData) string {
	dir := filepath.Dir(inputPath)
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)

	// Build suffix from settings
	suffix := ""
	if settings.AppendH265 {
		suffix += "_h265"
	}
	if settings.AppendRate {
		suffix += fmt.Sprintf("_%d%%", settings.DecayRate)
	}

	if settings.MoveToTrash {
		// Temp file: will be renamed to original path after trash move
		tmpName := fmt.Sprintf("%s_h265_tmp%s", base, ext)
		return filepath.Join(dir, "EZ265", tmpName)
	}

	// Normal: output to subfolder with suffix
	outName := fmt.Sprintf("%s%s%s", base, suffix, ext)
	return filepath.Join(dir, "EZ265", outName)
}
