//go:build windows

package clipboard

import (
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procGlobalSize                 = kernel32.NewProc("GlobalSize")
)

// It replaces the real clipboard of whoever runs it, so it is opt-in.
func TestSetSecret(t *testing.T) {
	if os.Getenv("PICKROLE_CLIPBOARD_TEST") != "1" {
		t.Skip("set PICKROLE_CLIPBOARD_TEST=1 to run (it overwrites the clipboard)")
	}
	const text = "export AWS_SECRET_ACCESS_KEY=çãé-teste"
	if err := SetSecret(text); err != nil {
		t.Fatal(err)
	}

	if err := open(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		procEmptyClipboard.Call()
		procCloseClipboard.Call()
	}()

	for _, name := range secretFormats {
		f, err := register(name)
		if err != nil {
			t.Fatal(err)
		}
		if r, _, _ := procIsClipboardFormatAvailable.Call(f); r == 0 {
			t.Errorf("format %s missing", name)
		}
	}

	h, _, err := procGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		t.Fatalf("GetClipboardData: %v", err)
	}
	size, _, _ := procGlobalSize.Call(h)
	buf := make([]uint16, size/2)
	p, _, _ := procGlobalLock.Call(h)
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&buf[0])), p, size)
	procGlobalUnlock.Call(h)
	if got := windows.UTF16ToString(buf); got != text {
		t.Errorf("clipboard = %q, want %q", got, text)
	}
}
