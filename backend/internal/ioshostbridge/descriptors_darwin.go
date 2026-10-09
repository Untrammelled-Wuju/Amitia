package ioshostbridge

import (
	"errors"
	"io"
)

func openDescriptors(read, write int) (io.ReadCloser, io.WriteCloser, error) {
	return nil, nil, errors.New("iOS 宿主安全通道须由移动 Runtime 使用")
}
