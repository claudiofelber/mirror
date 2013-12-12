package console

import (
	"syscall"
	"unsafe"
)

func init() {
	initSyscall()
}

func ReadPassword() string {
	password := make([]rune, 0, 10)

	for {
		char := readChar()
		if char == 13 {
			return string(password)
		} else if char == 8 && len(password) > 0 {
			password = password[:len(password)-1]
		} else if char > 0 {
			password = append(password, char)
		}
	}
}

func readChar() rune {
	var buffer _INPUT_RECORD
	var events uint32

	for {
		readConsoleInput.Call(uintptr(syscall.Stdin), uintptr(unsafe.Pointer(&buffer)), 1, uintptr(unsafe.Pointer(&events)))
		if buffer.bKeyDown != 0 && buffer.uChar != 0 {
			return rune(buffer.uChar)
		}
	}

	return 0
}
