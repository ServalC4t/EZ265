//go:build windows

package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"h265conv/internal/encoder"
	"h265conv/internal/i18n"
	"h265conv/internal/registry"
)

type jobModel struct {
	walk.TableModelBase
	items    []*encoder.Job
	settings *encoder.Settings
}

func (m *jobModel) RowCount() int {
	return len(m.items)
}

func (m *jobModel) Value(row, col int) interface{} {
	if row >= len(m.items) {
		return ""
	}
	j := m.items[row]
	j.Mu.Lock()
	defer j.Mu.Unlock()

	switch col {
	case 0:
		return j.FileName
	case 1:
		if j.OriginalSize > 0 {
			return encoder.FormatSize(j.OriginalSize)
		}
		return ""
	case 2:
		if j.Status == encoder.StatusEncoding {
			return fmt.Sprintf(i18n.T("status.encoding_pct"), j.Progress)
		}
		if j.Status == encoder.StatusError && j.ErrorMsg != "" {
			msg := j.ErrorMsg
			if len(msg) > 120 {
				msg = msg[:120]
			}
			return "✗ " + msg
		}
		return j.Status.String()
	case 3:
		switch j.Status {
		case encoder.StatusDone:
			return j.SavedTextUnlocked()
		case encoder.StatusPending:
			return j.EstimatedSavedTextUnlocked(m.settings.Load().DecayRate)
		default:
			return ""
		}
	}
	return ""
}

// App holds the main window state.
type App struct {
	mw       *walk.MainWindow
	queue    *encoder.Queue
	ffmpeg   *encoder.FFmpeg
	settings *encoder.Settings

	model      *jobModel
	tv         *walk.TableView
	addBtn     *walk.PushButton
	removeBtn  *walk.PushButton
	currentPB  *walk.ProgressBar
	totalPB    *walk.ProgressBar
	currentLbl *walk.Label
	totalLbl   *walk.Label
	statusBar  *walk.StatusBarItem
	startBtn   *walk.PushButton
	regBtn     *walk.PushButton
	decaySlider  *walk.Slider
	decayLbl     *walk.Label
	trashCB      *walk.CheckBox
	lowPriCB     *walk.CheckBox
	shutdownCB   *walk.CheckBox
	appendH265CB *walk.CheckBox
	appendRateCB *walk.CheckBox

	started bool
}

// Run creates the GUI and starts the event loop.
func Run(initialFiles []string, autoStart bool) error {
	settings := encoder.NewSettings()

	exePath, _ := os.Executable()
	binDir := filepath.Join(filepath.Dir(exePath), "bin")

	ffmpeg, err := encoder.NewFFmpeg(binDir)
	if err != nil {
		walk.MsgBox(nil, i18n.T("dlg.error"), err.Error(), walk.MsgBoxIconError)
		return err
	}

	app := &App{
		settings: settings,
		ffmpeg:   ffmpeg,
		model: &jobModel{
			settings: settings,
		},
	}

	return app.create(initialFiles, autoStart)
}

// Synchronize provides a way for IPC to add files on the GUI thread.
var Synchronize func(autoStart bool, files []string)

func (a *App) create(initialFiles []string, autoStart bool) error {
	sd := a.settings.Load()

	filterStr := fmt.Sprintf("%s (*.mp4;*.mkv;*.mov;*.avi;*.wmv;*.flv;*.m4v;*.ts;*.mts;*.m2ts;*.webm)|*.mp4;*.mkv;*.mov;*.avi;*.wmv;*.flv;*.m4v;*.ts;*.mts;*.m2ts;*.webm",
		i18n.T("dlg.filter"))

	if err := (MainWindow{
		AssignTo: &a.mw,
		Title:    i18n.T("app.title"),
		MinSize:  Size{Width: 700, Height: 500},
		Size:     Size{Width: 750, Height: 550},
		Layout:   VBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}},
		MenuItems: []MenuItem{
			Menu{
				Text: i18n.T("menu.settings"),
				Items: []MenuItem{
					Menu{
						Text: i18n.T("menu.language"),
						Items: []MenuItem{
							Action{
								Text: i18n.T("menu.lang_ja"),
								OnTriggered: func() {
									a.switchLanguage("ja")
								},
							},
							Action{
								Text: i18n.T("menu.lang_en"),
								OnTriggered: func() {
									a.switchLanguage("en")
								},
							},
						},
					},
				},
			},
			Menu{
				Text: i18n.T("menu.help"),
				Items: []MenuItem{
					Action{
						Text: i18n.T("menu.about"),
						OnTriggered: func() {
							a.showAbout()
						},
					},
				},
			},
		},
		OnDropFiles: func(files []string) {
			for _, f := range files {
				a.queue.AddFile(f)
			}
			a.updateStartButton()
		},
		Children: []Widget{
			TableView{
				AssignTo: &a.tv,
				Columns: []TableViewColumn{
					{Title: i18n.T("col.filename"), Width: 220},
					{Title: i18n.T("col.origsize"), Width: 80},
					{Title: i18n.T("col.status"), Width: 120},
					{Title: i18n.T("col.saved"), Width: 130},
				},
				Model:              a.model,
				StretchFactor:      3,
				LastColumnStretched: true,
				MultiSelection:     true,
				OnItemActivated: func() {
					a.onTableDoubleClick()
				},
			},

			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					PushButton{
						AssignTo: &a.addBtn,
						Text:     i18n.T("btn.add_files"),
						OnClicked: func() {
							a.openFileDialog(filterStr)
						},
					},
					PushButton{
						AssignTo: &a.removeBtn,
						Text:     i18n.T("btn.remove"),
						OnClicked: func() {
							a.removeSelected()
						},
					},
					HSpacer{},
				},
			},

			Composite{
				Layout: VBox{MarginsZero: true},
				Children: []Widget{
					Label{AssignTo: &a.currentLbl, Text: i18n.T("lbl.current_idle")},
					ProgressBar{AssignTo: &a.currentPB, MaxValue: 100},
					Label{AssignTo: &a.totalLbl, Text: i18n.Tf("lbl.total", 0, 0)},
					ProgressBar{AssignTo: &a.totalPB, MaxValue: 100},
				},
			},

			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					Label{AssignTo: &a.decayLbl, Text: i18n.Tf("lbl.rate", sd.DecayRate), MinSize: Size{Width: 80}},
					Slider{
						AssignTo:      &a.decaySlider,
						MinValue:      10,
						MaxValue:      90,
						Value:         sd.DecayRate,
						MinSize:       Size{Width: 150},
						StretchFactor: 2,
						ToolTipText:   i18n.T("slider.tooltip"),
						OnValueChanged: func() {
							val := a.decaySlider.Value()
							a.decayLbl.SetText(i18n.Tf("lbl.rate", val))
							a.settings.Update(func(s *encoder.SettingsData) {
								s.DecayRate = val
							})
							sel := a.tv.SelectedIndexes()
							for idx := range a.model.items {
								a.model.PublishRowChanged(idx)
							}
							a.tv.SetSelectedIndexes(sel)
							a.settings.Save()
						},
					},
					HSpacer{},
					PushButton{
						AssignTo:  &a.startBtn,
						Text:      i18n.T("btn.start"),
						MinSize:   Size{Width: 100},
						OnClicked: func() {
							a.onStartButtonClick()
						},
					},
				},
			},

			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					CheckBox{
						AssignTo: &a.appendH265CB,
						Text:     i18n.T("cb.append_h265"),
						Checked:  sd.AppendH265,
						OnCheckedChanged: func() {
							a.settings.Update(func(s *encoder.SettingsData) {
								s.AppendH265 = a.appendH265CB.Checked()
							})
						},
					},
					CheckBox{
						AssignTo: &a.appendRateCB,
						Text:     i18n.T("cb.append_rate"),
						Checked:  sd.AppendRate,
						OnCheckedChanged: func() {
							a.settings.Update(func(s *encoder.SettingsData) {
								s.AppendRate = a.appendRateCB.Checked()
							})
						},
					},
					CheckBox{
						AssignTo: &a.trashCB,
						Text:     i18n.T("cb.trash"),
						Checked:  sd.MoveToTrash,
						OnCheckedChanged: func() {
							a.settings.Update(func(s *encoder.SettingsData) {
								s.MoveToTrash = a.trashCB.Checked()
							})
						},
					},
					CheckBox{
						AssignTo: &a.lowPriCB,
						Text:     i18n.T("cb.low_priority"),
						Checked:  sd.LowPriority,
						OnCheckedChanged: func() {
							a.settings.Update(func(s *encoder.SettingsData) {
								s.LowPriority = a.lowPriCB.Checked()
							})
						},
					},
					CheckBox{
						AssignTo: &a.shutdownCB,
						Text:     i18n.T("cb.shutdown"),
						Checked:  sd.ShutdownOnDone,
						OnCheckedChanged: func() {
							a.settings.Update(func(s *encoder.SettingsData) {
								s.ShutdownOnDone = a.shutdownCB.Checked()
							})
						},
					},
					HSpacer{},
					PushButton{
						AssignTo: &a.regBtn,
						Text:     a.contextMenuBtnLabel(),
						OnClicked: func() {
							a.toggleContextMenu()
						},
					},
				},
			},
		},
		StatusBarItems: []StatusBarItem{
			{
				AssignTo: &a.statusBar,
				Text:     a.ffmpeg.StatusText(),
				Width:    400,
			},
		},
	}).Create(); err != nil {
		return err
	}

	// Set up queue with synchronized callback (starts paused)
	a.queue = encoder.NewQueue(a.ffmpeg, a.settings, func(job *encoder.Job) {
		a.mw.Synchronize(func() {
			a.onJobUpdate(job)
		})
	})

	// Set up IPC synchronize hook
	Synchronize = func(start bool, files []string) {
		a.mw.Synchronize(func() {
			for _, f := range files {
				a.queue.AddFile(f)
			}
			if start && !a.started && a.queue.HasPendingJobs() {
				a.started = true
				a.queue.Resume()
			}
			a.updateStartButton()
		})
	}

	// Add initial files
	for _, f := range initialFiles {
		a.queue.AddFile(f)
	}

	// Auto-start only if launched with --encode
	if autoStart && a.queue.HasPendingJobs() {
		a.started = true
		a.queue.Resume()
	}

	a.updateStartButton()

	// On close
	a.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if a.started && a.queue.HasPendingJobs() {
			ret := walk.MsgBox(a.mw, i18n.T("dlg.confirm"),
				i18n.T("dlg.force_quit"),
				walk.MsgBoxYesNo|walk.MsgBoxIconWarning)
			if ret != walk.DlgCmdYes {
				*canceled = true
				return
			}
		}
		a.settings.Save()
		a.queue.Stop()
	})

	a.mw.Run()
	return nil
}

func (a *App) onJobUpdate(job *encoder.Job) {
	a.model.items = a.queue.Jobs()
	a.model.PublishRowsReset()

	job.Mu.Lock()
	status := job.Status
	progress := job.Progress
	name := job.FileName
	job.Mu.Unlock()

	if status == encoder.StatusEncoding {
		a.currentLbl.SetText(fmt.Sprintf(i18n.T("lbl.current_enc"), name, progress))
		a.currentPB.SetValue(int(progress))
	} else if status == encoder.StatusDone || status == encoder.StatusError || status == encoder.StatusCancelled {
		a.currentPB.SetValue(0)
		a.currentLbl.SetText(i18n.T("lbl.current_idle"))
	}

	done, total := a.queue.CompletedCount()
	a.totalLbl.SetText(i18n.Tf("lbl.total", done, total))
	if total > 0 {
		currentProgress := 0.0
		if status == encoder.StatusEncoding {
			currentProgress = progress / 100.0
		}
		totalPct := (float64(done) + currentProgress) / float64(total) * 100.0
		a.totalPB.SetValue(int(totalPct))
	}

	if a.queue.AllDone() {
		a.started = false
		a.queue.Pause()
		a.currentLbl.SetText(i18n.T("lbl.current_done"))
		a.totalLbl.SetText(i18n.Tf("lbl.total_done", done, total))
		a.totalPB.SetValue(100)
		a.updateStartButton()

		if a.settings.Load().ShutdownOnDone {
			exec.Command("shutdown", "/s", "/t", "10").Start()
		}
	}

	a.updateStartButton()
}

func (a *App) onStartButtonClick() {
	if !a.started {
		if !a.queue.HasPendingJobs() {
			return
		}
		a.started = true
		a.queue.Resume()
		a.updateStartButton()
		return
	}

	if a.queue.IsPaused() {
		a.queue.Resume()
	} else {
		a.queue.Pause()
	}
	a.updateStartButton()
}

func (a *App) updateStartButton() {
	if !a.started {
		a.startBtn.SetText(i18n.T("btn.start"))
		return
	}
	if a.queue.IsPaused() {
		a.startBtn.SetText(i18n.T("btn.resume"))
	} else {
		a.startBtn.SetText(i18n.T("btn.pause"))
	}
}

func (a *App) onTableDoubleClick() {
	idx := a.tv.CurrentIndex()
	if idx < 0 {
		return
	}

	jobs := a.queue.Jobs()
	if idx >= len(jobs) {
		return
	}
	job := jobs[idx]

	job.Mu.Lock()
	status := job.Status
	name := job.FileName
	job.Mu.Unlock()

	switch status {
	case encoder.StatusEncoding:
		ret := walk.MsgBox(a.mw, i18n.T("dlg.confirm"),
			i18n.Tf("dlg.cancel_encode", name),
			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)
		if ret == walk.DlgCmdYes {
			a.queue.CancelJob(idx)
		}
	case encoder.StatusPending:
		ret := walk.MsgBox(a.mw, i18n.T("dlg.confirm"),
			i18n.Tf("dlg.remove_pending", name),
			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)
		if ret == walk.DlgCmdYes {
			a.queue.CancelJob(idx)
		}
	}
}

func (a *App) removeSelected() {
	indices := a.tv.SelectedIndexes()
	if len(indices) == 0 {
		return
	}
	a.queue.RemoveJobs(indices)
	a.model.items = a.queue.Jobs()
	a.model.PublishRowsReset()
	a.updateStartButton()
}

func (a *App) openFileDialog(filterStr string) {
	dlg := new(walk.FileDialog)
	dlg.Title = i18n.T("dlg.title")
	dlg.Filter = filterStr

	if ok, _ := dlg.ShowOpenMultiple(a.mw); ok {
		for _, f := range dlg.FilePaths {
			a.queue.AddFile(f)
		}
		a.updateStartButton()
	}
}

func (a *App) toggleContextMenu() {
	if registry.IsRegistered() {
		ret := walk.MsgBox(a.mw, i18n.T("dlg.confirm"),
			i18n.T("dlg.reg_confirm_del"),
			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)
		if ret == walk.DlgCmdYes {
			if err := registry.Unregister(); err != nil {
				walk.MsgBox(a.mw, i18n.T("dlg.error"), err.Error(), walk.MsgBoxIconError)
			}
		}
	} else {
		if err := registry.Register(); err != nil {
			walk.MsgBox(a.mw, i18n.T("dlg.error"),
				i18n.Tf("dlg.reg_failed", err.Error()), walk.MsgBoxIconError)
		} else {
			walk.MsgBox(a.mw, i18n.T("dlg.complete"),
				i18n.T("dlg.reg_done"), walk.MsgBoxIconInformation)
		}
	}
	a.regBtn.SetText(a.contextMenuBtnLabel())
}

func (a *App) contextMenuBtnLabel() string {
	if registry.IsRegistered() {
		return i18n.T("btn.reg_del")
	}
	return i18n.T("btn.reg_add")
}

func (a *App) switchLanguage(lang string) {
	a.settings.Update(func(s *encoder.SettingsData) {
		s.Language = lang
	})
	a.settings.Save()
	walk.MsgBox(a.mw, i18n.T("dlg.complete"),
		i18n.T("dlg.lang_restart"), walk.MsgBoxIconInformation)
}

func (a *App) showAbout() {
	walk.MsgBox(a.mw, i18n.T("about.title"),
		i18n.T("about.text"), walk.MsgBoxIconInformation)
}
