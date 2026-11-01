//go:build linux

package cli

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// readPassword reads a line with terminal echo disabled. If stdin is not a
// terminal (for example a pipe), it falls back to a plain read.
func readPassword(f *os.File) (string, error) {
	fd := f.Fd()

	var original syscall.Termios
	if err := ioctlTermios(fd, syscall.TCGETS, &original); err != nil {
		return readLinePlain(f)
	}

	noEcho := original
	noEcho.Lflag &^= syscall.ECHO
	if err := ioctlTermios(fd, syscall.TCSETS, &noEcho); err != nil {
		return readLinePlain(f)
	}
	defer func() {
		_ = ioctlTermios(fd, syscall.TCSETS, &original)
	}()

	line, err := readLinePlain(f)
	fmt.Fprintln(os.Stderr)
	return line, err
}

func ioctlTermios(fd, request uintptr, state *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		request,
		uintptr(unsafe.Pointer(state)),
		0, 0, 0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}
