package encoder

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"h265conv/internal/i18n"
)

type JobStatus int

const (
	StatusPending   JobStatus = iota // 待機中
	StatusEncoding                   // 処理中
	StatusDone                       // 完了
	StatusError                      // エラー
	StatusSkipped                    // スキップ
	StatusCancelled                  // キャンセル
)

func (s JobStatus) String() string {
	switch s {
	case StatusPending:
		return i18n.T("status.pending")
	case StatusEncoding:
		return i18n.T("status.encoding")
	case StatusDone:
		return i18n.T("status.done")
	case StatusError:
		return i18n.T("status.error")
	case StatusSkipped:
		return i18n.T("status.skipped")
	case StatusCancelled:
		return i18n.T("status.cancelled")
	default:
		return "?"
	}
}

type Job struct {
	Mu sync.Mutex

	InputPath       string
	OutputPath      string
	FileName        string
	OriginalSize    int64   // bytes
	OriginalBitrate int64   // kbps (video only)
	Duration        float64 // seconds

	Status     JobStatus
	Progress   float64 // 0.0 ~ 100.0
	OutputSize int64   // bytes (after encode)
	ErrorMsg   string

	// Per-job cancel support
	cancelFunc context.CancelFunc
}

func NewJob(inputPath string) *Job {
	return &Job{
		InputPath: inputPath,
		FileName:  filepath.Base(inputPath),
		Status:    StatusPending,
	}
}

func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.0f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// SavedText returns the reduction string (locks Mu).
func (j *Job) SavedText() string {
	j.Mu.Lock()
	defer j.Mu.Unlock()
	return j.SavedTextUnlocked()
}

// SavedTextUnlocked returns the reduction string (caller must hold Mu).
func (j *Job) SavedTextUnlocked() string {
	if j.Status != StatusDone || j.OriginalSize == 0 {
		return ""
	}
	saved := j.OriginalSize - j.OutputSize
	pct := float64(saved) / float64(j.OriginalSize) * 100
	if saved <= 0 {
		return fmt.Sprintf(i18n.T("size.increase"), FormatSize(-saved))
	}
	return fmt.Sprintf(i18n.T("size.decrease"), FormatSize(saved), pct)
}

// EstimatedSavedText returns the estimated reduction (locks Mu).
func (j *Job) EstimatedSavedText(decayRate int) string {
	j.Mu.Lock()
	defer j.Mu.Unlock()
	return j.EstimatedSavedTextUnlocked(decayRate)
}

// EstimatedSavedTextUnlocked (caller must hold Mu).
func (j *Job) EstimatedSavedTextUnlocked(decayRate int) string {
	if j.Status != StatusPending || j.OriginalSize == 0 {
		return ""
	}
	estimated := float64(j.OriginalSize) * float64(decayRate) / 100.0
	saved := j.OriginalSize - int64(estimated)
	if saved <= 0 {
		return ""
	}
	return fmt.Sprintf(i18n.T("size.estimate"), FormatSize(saved))
}

var SupportedExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".mov":  true,
	".avi":  true,
	".wmv":  true,
	".flv":  true,
	".m4v":  true,
	".ts":   true,
	".mts":  true,
	".m2ts": true,
	".webm": true,
}

func IsSupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return SupportedExtensions[ext]
}
