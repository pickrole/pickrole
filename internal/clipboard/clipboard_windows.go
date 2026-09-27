//go:build windows

package clipboard

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procOpenClipboard            = user32.NewProc("OpenClipboard")
	procCloseClipboard           = user32.NewProc("CloseClipboard")
	procEmptyClipboard           = user32.NewProc("EmptyClipboard")
	procSetClipboardData         = user32.NewProc("SetClipboardData")
	procRegisterClipboardFormatW = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc              = kernel32.NewProc("GlobalAlloc")
	procGlobalLock               = kernel32.NewProc("GlobalLock")
	procGlobalUnlock             = kernel32.NewProc("GlobalUnlock")
	procGlobalFree               = kernel32.NewProc("GlobalFree")
	procRtlMoveMemory            = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// Formats that Windows (and well-behaved clipboard tools) honour: keep the
// content out of the clipboard history (Win+V), out of the cloud clipboard
// sync, and away from clipboard monitors. Password managers use the same.
var secretFormats = []string{
	"ExcludeClipboardContentFromMonitorProcessing",
	"CanIncludeInClipboardHistory",
	"CanUploadToCloudClipboard",
}

// SetSecret puts text on the clipboard marked as sensitive.
func SetSecret(text string) error {
	utf16, err := windows.UTF16FromString(text)
	if err != nil {
		return err
	}
	if err := open(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()

	if r, _, err := procEmptyClipboard.Call(); r == 0 {
		return fmt.Errorf("EmptyClipboard: %w", err)
	}
	if err := setData(cfUnicodeText, unsafe.Pointer(&utf16[0]), uintptr(len(utf16)*2)); err != nil { // #nosec G103 -- Win32 clipboard API takes raw memory
		return err
	}
	// A DWORD 0 means "no" for the two Can* formats; for the Exclude format
	// only its presence matters.
	var no uint32
	for _, name := range secretFormats {
		f, err := register(name)
		if err == nil {
			err = setData(f, unsafe.Pointer(&no), unsafe.Sizeof(no)) // #nosec G103 -- Win32 clipboard API takes raw memory
		}
		if err != nil {
			// Never leave the secret on the clipboard without the markers.
			procEmptyClipboard.Call() // #nosec G104 -- best effort; the error being returned matters more
			return err
		}
	}
	return nil
}

// open retries briefly: another program may be holding the clipboard.
func open() error {
	var err error
	for range 10 {
		var r uintptr
		if r, _, err = procOpenClipboard.Call(0); r != 0 {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("OpenClipboard: %w", err)
}

func register(name string) (uintptr, error) {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	f, _, err := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(p))) // #nosec G103 -- Win32 string argument
	if f == 0 {
		return 0, fmt.Errorf("RegisterClipboardFormat(%s): %w", name, err)
	}
	return f, nil
}

// setData copies size bytes at src into global memory and hands it to the
// clipboard, which then owns it.
func setData(format uintptr, src unsafe.Pointer, size uintptr) error {
	h, _, err := procGlobalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return fmt.Errorf("GlobalAlloc: %w", err)
	}
	dst, _, err := procGlobalLock.Call(h)
	if dst == 0 {
		procGlobalFree.Call(h) // #nosec G104 -- cleanup on an error path
		return fmt.Errorf("GlobalLock: %w", err)
	}
	procRtlMoveMemory.Call(dst, uintptr(src), size) // #nosec G104 -- RtlMoveMemory returns nothing
	procGlobalUnlock.Call(h)                        // #nosec G104 -- unlock of memory we just locked
	if r, _, err := procSetClipboardData.Call(format, h); r == 0 {
		procGlobalFree.Call(h) // #nosec G104 -- cleanup on an error path
		return fmt.Errorf("SetClipboardData: %w", err)
	}
	return nil
}
