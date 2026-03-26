//go:build !windows

package gui

import "fmt"

// Synchronize is a no-op on non-Windows.
var Synchronize func(autoStart bool, files []string)

func Run(initialFiles []string, autoStart bool) error {
	return fmt.Errorf("GUI is only supported on Windows")
}
