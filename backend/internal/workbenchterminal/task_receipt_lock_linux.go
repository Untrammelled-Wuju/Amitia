package workbenchterminal

import (
	"os"
	"syscall"
	"time"
)

func lockReceiptJournal(path string, timeout time.Duration) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		_ = file.Close()
		return nil, ErrInvalid
	}
	deadline := time.Now().Add(timeout)
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN && err != syscall.EINTR {
			_ = file.Close()
			return nil, err
		}
		if time.Now().After(deadline) {
			_ = file.Close()
			return nil, ErrReceiptUnavailable
		}
		time.Sleep(15 * time.Millisecond)
	}
	return func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN); _ = file.Close() }, nil
}
