package console

import (
	"syscall"
)

type _INPUT_RECORD struct {
	EventType         uint16
	bKeyDown          uint32
	wRepeatCount      uint16
	wVirtualKeyCode   uint16
	wVirtualScanCode  uint16
	uChar             uint16
	dwControlKeyState uint32
	dummy             [9]byte
}

type _CONSOLE_SCREEN_BUFFER_INFO struct {
	dwSize struct {
		x, y int16
	}
	dwCursorPosition struct {
		x, y int16
	}
	wAttributes uint16
	srWindow    struct {
		left, top, right, bottom int16
	}
	dwMaximumWindowSize struct {
		x, y int16
	}
}

var getConsoleScreenBufferInfo *syscall.LazyProc
var setConsoleTextAttribute *syscall.LazyProc
var setConsoleMode *syscall.LazyProc
var readConsoleInput *syscall.LazyProc

func initSyscall() {
	if getConsoleScreenBufferInfo == nil {
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		getConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
		setConsoleTextAttribute = kernel32.NewProc("SetConsoleTextAttribute")
		setConsoleMode = kernel32.NewProc("SetConsoleMode")
		readConsoleInput = kernel32.NewProc("ReadConsoleInputW")
	}
}
