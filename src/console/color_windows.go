package console

import (
	"syscall"
	"unsafe"
)

var defaultAttributes uint16

func init() {
	initSyscall()
	info := new(_CONSOLE_SCREEN_BUFFER_INFO)
	getConsoleScreenBufferInfo.Call(uintptr(syscall.Stdout), uintptr(unsafe.Pointer(info)))
	defaultAttributes = info.wAttributes
}

func SetTextColor(color ColorValue) {
	setConsoleTextAttribute.Call(uintptr(syscall.Stdout), uintptr((defaultAttributes&0xF0)|uint16(color)))
}

func SetBackColor(color ColorValue) {
	setConsoleTextAttribute.Call(uintptr(syscall.Stdout), uintptr(uint16(color)<<4|(defaultAttributes&0x0F)))
}

func SetColor(foreground, background ColorValue) {
	setConsoleTextAttribute.Call(uintptr(syscall.Stdout), uintptr(background<<4|foreground))
}

func ResetColor() {
	setConsoleTextAttribute.Call(uintptr(syscall.Stdout), uintptr(defaultAttributes))
}
