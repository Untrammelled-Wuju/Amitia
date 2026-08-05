//go:build !windows

package kernel

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

type platformPathIdentity struct {
	Device uint64
	Inode  uint64

	IsDirectory bool
}

func (
	left platformPathIdentity,
) same(
	right platformPathIdentity,
) bool {
	return left.Device ==
		right.Device &&
		left.Inode ==
			right.Inode &&
		left.IsDirectory ==
			right.IsDirectory
}

func syscallNofollow() int {
	return unix.O_NOFOLLOW
}

func validatePlatformPathComponent(
	component string,
) error {
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
	var stat unix.Stat_t

	if err :=
		unix.Lstat(
			path,
			&stat,
		); err != nil {
		return platformPathIdentity{},
			fmt.Errorf(
				"kernel: lstat %s: %w",
				path,
				err,
			)
	}

	fileType :=
		stat.Mode &
			syscall.S_IFMT

	if fileType ==
		syscall.S_IFLNK {
		return platformPathIdentity{},
			NewPackageError(
				PackageErrCodeResourceSnapshotSymlinkForbidden,
				400,
				fmt.Errorf(
					"kernel: path %s is a symbolic link",
					path,
				),
			)
	}

	isDirectory :=
		fileType ==
			syscall.S_IFDIR

	if requireDirectory &&
		!isDirectory {
		return platformPathIdentity{},
			NewPackageError(
				PackageErrCodeResourceSnapshotPathInvalid,
				400,
				fmt.Errorf(
					"kernel: path %s is not a directory",
					path,
				),
			)
	}

	if !requireDirectory &&
		fileType !=
			syscall.S_IFREG {
		return platformPathIdentity{},
			NewPackageError(
				PackageErrCodeResourceRestoreTargetChanged,
				409,
				fmt.Errorf(
					"kernel: path %s is not a regular file",
					path,
				),
			)
	}

	return platformPathIdentity{
			Device:
				uint64(
					stat.Dev,
				),

			Inode:
				stat.Ino,

			IsDirectory:
				isDirectory,
		},
		nil
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
				"kernel: filesystem identity changed for %s",
				path,
			),
		)
	}

	return nil
}

func safeCreateFilePlatform(
	path string,
) (
	*os.File,
	error,
) {
	fileDescriptor,
		err :=
		unix.Open(
			path,
			unix.O_CREAT|
				unix.O_WRONLY|
				unix.O_NOFOLLOW|
				unix.O_EXCL,
			0o600,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"kernel: safely create file %s: %w",
				path,
				err,
			)
	}

	return os.NewFile(
			uintptr(
				fileDescriptor,
			),
			path,
		),
		nil
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

	if os.IsNotExist(
		err,
	) {
		return nil
	}

	return err
}

func createHardLinkNoReplacePlatform(
	newName string,
	existingName string,
) error {
	return os.Link(
		existingName,
		newName,
	)
}
