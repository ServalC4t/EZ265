//go:build !windows

package encoder

import "os/exec"

func hideWindow(cmd *exec.Cmd) {
	// No-op on non-Windows
}

func setLowPriority(cmd *exec.Cmd) {
	// No-op on non-Windows
}
