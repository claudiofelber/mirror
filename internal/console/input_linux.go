// +build linux

package console

import "syscall"

const ioctlReadTermios = syscall.TCGETS
const ioctlWriteTermios = syscall.TCSETS
