//go:build windows

package encoder

import (
	"os/exec"
	"syscall"
)

const (
	createNoWindow    = 0x08000000
	idlePriorityClass = 0x00000040
)

// hideWindow ensures the ffmpeg/ffprobe process has no visible console window.
func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}

func setLowPriority(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= idlePriorityClass
}
