//go:build !windows

package encoder

import "fmt"

// MoveToTrash is a stub for non-Windows platforms.
func MoveToTrash(path string) error {
	return fmt.Errorf("trash operation not supported on this platform")
}
