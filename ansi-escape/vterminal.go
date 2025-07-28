//go:build !windows

package ansi

func EnableVirtualTerminal(_ uintptr) error {
	return nil
}

func DisableVirtualTerminal(_ uintptr) error {
	return nil
}
