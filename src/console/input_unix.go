// +build darwin dragonfly freebsd linux netbsd openbsd

package console

import (
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

var oldTerminalState syscall.Termios
var terminalModified bool

func init() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		if terminalModified {
			syscall.Syscall6(syscall.SYS_IOCTL, uintptr(0), ioctlWriteTermios, uintptr(unsafe.Pointer(&oldTerminalState)), 0, 0, 0)
		}
		os.Exit(130)
	}()
}

func ReadPassword() string {
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(0), ioctlReadTermios, uintptr(unsafe.Pointer(&oldTerminalState)), 0, 0, 0); err != 0 {
		return ""
	}

	newState := oldTerminalState
	newState.Lflag &^= syscall.ECHO
	newState.Lflag |= syscall.ICANON | syscall.ISIG
	newState.Iflag |= syscall.ICRNL
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(0), ioctlWriteTermios, uintptr(unsafe.Pointer(&newState)), 0, 0, 0); err != 0 {
		return ""
	}

	terminalModified = true
	defer func() {
		syscall.Syscall6(syscall.SYS_IOCTL, uintptr(0), ioctlWriteTermios, uintptr(unsafe.Pointer(&oldTerminalState)), 0, 0, 0)
		terminalModified = false
	}()

	var buf [16]byte
	var ret []byte
	for {
		n, err := syscall.Read(0, buf[:])
		if err != nil {
			return ""
		}
		if n == 0 {
			if len(ret) == 0 {
				return ""
			}
			break
		}
		if buf[n-1] == '\n' {
			n--
		}
		ret = append(ret, buf[:n]...)
		if n < len(buf) {
			break
		}
	}

	return string(ret)
}
