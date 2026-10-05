package secretstore

import (
	"bytes"
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

func init() {
	protect = windowsProtect
	unprotect = windowsUnprotect
}

func windowsBlob(data []byte) windows.DataBlob {
	if len(data) == 0 {
		return windows.DataBlob{}
	}
	return windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
}

func windowsProtect(data []byte) ([]byte, error) {
	input := windowsBlob(data)
	entropy := windowsBlob([]byte("amitia-device-secret-v1"))
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, &entropy, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	if output.Data == nil || output.Size == 0 {
		return nil, errors.New("设备安全存储加密失败")
	}
	return append(append([]byte(nil), envelope...), unsafe.Slice(output.Data, int(output.Size))...), nil
}

func windowsUnprotect(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, envelope) {
		return append([]byte(nil), data...), nil
	}
	input := windowsBlob(data[len(envelope):])
	entropy := windowsBlob([]byte("amitia-device-secret-v1"))
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, &entropy, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	if output.Data == nil || output.Size == 0 {
		return nil, errors.New("设备安全存储解密失败")
	}
	return append([]byte(nil), unsafe.Slice(output.Data, int(output.Size))...), nil
}
