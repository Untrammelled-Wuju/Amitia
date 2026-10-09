package ioshostbridge

import (
	"errors"
	"io"
	"os"
	"syscall"
)

func openDescriptors(read, write int) (io.ReadCloser, io.WriteCloser, error) {
	syscall.CloseOnExec(read)
	syscall.CloseOnExec(write)
	r := os.NewFile(uintptr(read), "ios-host-read")
	w := os.NewFile(uintptr(write), "ios-host-write")
	if r == nil || w == nil {
		return nil, nil, errors.New("iOS 宿主通道句柄无效")
	}
	if _, err := r.Stat(); err != nil {
		r.Close()
		w.Close()
		return nil, nil, errors.New("iOS 宿主通道不可用")
	}
	if _, err := w.Stat(); err != nil {
		r.Close()
		w.Close()
		return nil, nil, errors.New("iOS 宿主通道不可用")
	}
	return r, w, nil
}
