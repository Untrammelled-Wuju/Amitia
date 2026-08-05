//go:build windows

package kernel

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

type platformPathIdentity struct {
	VolumeSerialNumber uint32
	FileIndexHigh      uint32
	FileIndexLow       uint32

	IsDirectory bool
}

func (
	left platformPathIdentity,
) same(
	right platformPathIdentity,
) bool {
	return left.VolumeSerialNumber ==
		right.VolumeSerialNumber &&
		left.FileIndexHigh ==
			right.FileIndexHigh &&
		left.FileIndexLow ==
			right.FileIndexLow &&
		left.IsDirectory ==
			right.IsDirectory
}

func syscallNofollow() int {
	return 0
}

func validatePlatformPathComponent(
	component string,
) error {
	component =
		strings.TrimSpace(
			component,
		)

	if component == "" {
		return fmt.Errorf(
			"kernel: empty Windows path component",
		)
	}

	if component == "." ||
		component == ".." {
		return fmt.Errorf(
			"kernel: path component %q is forbidden",
			component,
		)
	}

	for _, character :=
	range component {
		if character == 0 {
			return fmt.Errorf(
				"kernel: path component contains null byte",
			)
		}

		if character == '/' ||
			character == '\\' {
			return fmt.Errorf(
				"kernel: path component %q contains separator character",
				component,
			)
		}
	}

	if strings.HasSuffix(
		component,
		" ",
	) ||
		strings.HasSuffix(
			component,
			".",
		) {
		return fmt.Errorf(
			"kernel: Windows path component %q has forbidden trailing space or dot",
			component,
		)
	}

	return nil
}

func isWindowsPathNotExist(
	err error,
) bool {
	return errors.Is(
		err,
		windows.ERROR_FILE_NOT_FOUND,
	) ||
		errors.Is(
			err,
			windows.ERROR_PATH_NOT_FOUND,
		)
}

func openWindowsPathMetadataNoFollow(
	path string,
) (
	windows.Handle,
	windows.ByHandleFileInformation,
	error,
) {
	var empty windows.ByHandleFileInformation

	path =
		strings.TrimSpace(
			path,
		)

	if path == "" {
		return windows.InvalidHandle,
			empty,
			fmt.Errorf(
				"kernel: empty Windows metadata path",
			)
	}

	pathPointer,
		err :=
		windows.UTF16PtrFromString(
			path,
		)

	if err != nil {
		return windows.InvalidHandle,
			empty,
			fmt.Errorf(
				"kernel: convert path %s to UTF-16: %w",
				path,
				err,
			)
	}

	handle,
		err :=
		windows.CreateFile(
			pathPointer,
			windows.FILE_READ_ATTRIBUTES,
			windows.FILE_SHARE_READ|
				windows.FILE_SHARE_WRITE|
				windows.FILE_SHARE_DELETE,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_FLAG_OPEN_REPARSE_POINT|
				windows.FILE_FLAG_BACKUP_SEMANTICS,
			0,
		)

	if err != nil {
		return windows.InvalidHandle,
			empty,
			fmt.Errorf(
				"kernel: open Windows path without following reparse point %s: %w",
				path,
				err,
			)
	}

	var information windows.ByHandleFileInformation

	if err :=
		windows.GetFileInformationByHandle(
			handle,
			&information,
		); err != nil {
		_ =
			windows.CloseHandle(
				handle,
			)

		return windows.InvalidHandle,
			empty,
			fmt.Errorf(
				"kernel: GetFileInformationByHandle %s: %w",
				path,
				err,
			)
	}

	return handle,
		information,
		nil
}

func validateWindowsPathInformation(
	path string,
	information windows.ByHandleFileInformation,
	requireDirectory bool,
) error {
	if information.FileAttributes&
		windows.FILE_ATTRIBUTE_REPARSE_POINT !=
		0 {
		return NewPackageError(
			PackageErrCodeResourceRestoreReparsePointForbidden,
			400,
			fmt.Errorf(
				"kernel: Windows path %s is a reparse point",
				path,
			),
		)
	}

	if information.FileAttributes&
		windows.FILE_ATTRIBUTE_DEVICE !=
		0 {
		return NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf(
				"kernel: Windows path %s is a device",
				path,
			),
		)
	}

	isDirectory :=
		information.FileAttributes&
			windows.FILE_ATTRIBUTE_DIRECTORY !=
			0

	if requireDirectory &&
		!isDirectory {
		return NewPackageError(
			PackageErrCodeResourceSnapshotPathInvalid,
			400,
			fmt.Errorf(
				"kernel: Windows path %s is not a directory",
				path,
			),
		)
	}

	if !requireDirectory &&
		isDirectory {
		return NewPackageError(
			PackageErrCodeResourceRestoreTargetChanged,
			409,
			fmt.Errorf(
				"kernel: Windows path %s is a directory, expected regular file",
				path,
			),
		)
	}

	return nil
}

func capturePlatformPathIdentity(
	path string,
	requireDirectory bool,
) (
	platformPathIdentity,
	error,
) {
	var empty platformPathIdentity

	handle,
		information,
		err :=
		openWindowsPathMetadataNoFollow(
			path,
		)

	if err != nil {
		return empty, err
	}

	defer windows.CloseHandle(
		handle,
	)

	if err :=
		validateWindowsPathInformation(
			path,
			information,
			requireDirectory,
		); err != nil {
		return empty, err
	}

	return platformPathIdentity{
		VolumeSerialNumber:
		information.VolumeSerialNumber,

		FileIndexHigh:
		information.FileIndexHigh,

		FileIndexLow:
		information.FileIndexLow,

		IsDirectory:
		information.FileAttributes&
			windows.FILE_ATTRIBUTE_DIRECTORY !=
			0,
	}, nil
}

func validateExistingPathNoReparse(
	path string,
	requireDirectory bool,
) error {
	_,
		err :=
		capturePlatformPathIdentity(
			path,
			requireDirectory,
		)

	return err
}

func validatePlatformPathIdentity(
	path string,
	expected platformPathIdentity,
	requireDirectory bool,
) error {
	actual,
		err :=
		capturePlatformPathIdentity(
			path,
			requireDirectory,
		)

	if err != nil {
		return err
	}

	if !expected.same(
		actual,
	) {
		return NewPackageError(
			PackageErrCodeResourceRestorePathRace,
			409,
			fmt.Errorf(
				"kernel: Windows filesystem identity changed for %s",
				path,
			),
		)
	}

	return nil
}

func validateReparsePoint(
	path string,
) error {
	_,
		err :=
		capturePlatformPathIdentity(
			path,
			false,
		)

	if err == nil {
		return nil
	}

	if isWindowsPathNotExist(
		err,
	) {
		return nil
	}

	return err
}

func safeCreateFilePlatform(
	path string,
) (
	*os.File,
	error,
) {
	pathPointer,
		err :=
		windows.UTF16PtrFromString(
			path,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"kernel: convert path %s to UTF-16: %w",
				path,
				err,
			)
	}

	handle,
		err :=
		windows.CreateFile(
			pathPointer,
			windows.GENERIC_WRITE|
				windows.FILE_READ_ATTRIBUTES,
			0,
			nil,
			windows.CREATE_NEW,
			windows.FILE_ATTRIBUTE_NORMAL|
				windows.FILE_FLAG_OPEN_REPARSE_POINT,
			0,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"kernel: safely create Windows file %s: %w",
				path,
				err,
			)
	}

	var information windows.ByHandleFileInformation

	if err :=
		windows.GetFileInformationByHandle(
			handle,
			&information,
		); err != nil {
		_ =
			windows.CloseHandle(
				handle,
			)

		_ =
			os.Remove(
				path,
			)

		return nil,
			fmt.Errorf(
				"kernel: inspect newly created Windows file %s: %w",
				path,
				err,
			)
	}

	if err :=
		validateWindowsPathInformation(
			path,
			information,
			false,
		); err != nil {
		_ =
			windows.CloseHandle(
				handle,
			)

		_ =
			os.Remove(
				path,
			)

		return nil, err
	}

	return os.NewFile(
			uintptr(
				handle,
			),
			path,
		),
		nil
}

func createHardLinkNoReplacePlatform(
	newName string,
	existingName string,
) error {
	newPointer,
		err :=
		windows.UTF16PtrFromString(
			newName,
		)

	if err != nil {
		return fmt.Errorf(
			"kernel: convert new hard-link path %s to UTF-16: %w",
			newName,
			err,
		)
	}

	existingPointer,
		err :=
		windows.UTF16PtrFromString(
			existingName,
		)

	if err != nil {
		return fmt.Errorf(
			"kernel: convert existing hard-link path %s to UTF-16: %w",
			existingName,
			err,
		)
	}

	return windows.CreateHardLink(
		newPointer,
		existingPointer,
		0,
	)
}
