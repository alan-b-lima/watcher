//go:build windows

package ansi

import (
	"syscall"

	"golang.org/x/sys/windows"
)

func EnableVirtualTerminal(fd uintptr) error {
	var mode uint32
	err := syscall.GetConsoleMode(syscall.Handle(fd), &mode)
	if err != nil {
		return err
	}

	mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING

	err = windows.SetConsoleMode(windows.Handle(fd), mode)
	return err
}

func DisableVirtualTerminal(fd uintptr) error {
	var mode uint32
	err := syscall.GetConsoleMode(syscall.Handle(fd), &mode)
	if err != nil {
		return err
	}

	mode &^= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING

	err = windows.SetConsoleMode(windows.Handle(fd), mode)
	return err
}
