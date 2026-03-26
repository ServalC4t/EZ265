//go:build windows

package encoder

import (
	"syscall"
	"unsafe"
)

var (
	shell32          = syscall.NewLazyDLL("shell32.dll")
	shFileOperationW = shell32.NewProc("SHFileOperationW")
)

const (
	foDelete        = 0x0003
	fofAllowUndo    = 0x0040
	fofNoConfirmation = 0x0010
	fofSilent       = 0x0004
)

// SHFILEOPSTRUCTW
type shFileOpStruct struct {
	Hwnd                  uintptr
	Func                  uint32
	From                  *uint16
	To                    *uint16
	Flags                 uint16
	AnyOperationsAborted  int32
	NameMappings          uintptr
	ProgressTitle         *uint16
}

// MoveToTrash sends a file to the Windows recycle bin using SHFileOperationW.
func MoveToTrash(path string) error {
	// SHFileOperationW requires double-null terminated string
	pathUTF16, err := syscall.UTF16FromString(path)
	if err != nil {
		return err
	}
	// Append extra null terminator
	pathUTF16 = append(pathUTF16, 0)

	op := shFileOpStruct{
		Func:  foDelete,
		From:  &pathUTF16[0],
		Flags: fofAllowUndo | fofNoConfirmation | fofSilent,
	}

	ret, _, _ := shFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return syscall.Errno(ret)
	}
	return nil
}
