//go:build windows

package system

import "golang.org/x/sys/windows"

func diskCapacity(path string) (int64, int64, error) {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var available uint64
	var total uint64
	var totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(pointer, &available, &total, &totalFree); err != nil {
		return 0, 0, err
	}
	return int64(total), int64(available), nil
}
