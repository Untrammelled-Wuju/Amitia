//go:build !windows

package kernel

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

type platformPathIdentity struct {
	Device uint64
	Inode  uint64

	IsDirectory bool
}

type preparedRestoreTemp struct {
	Name     string
	File     *os.File
	Identity platformPathIdentity
}

type platformRestoreDirectory struct {
	file         *os.File
	pathIdentity platformPathIdentity
	closed       bool
}

func captureUnixHandleIdentity(file *os.File, requireDirectory bool) (platformPathIdentity, error) {
	var stat unix.Stat_t
	if file == nil {
		return platformPathIdentity{}, fmt.Errorf("kernel: file handle missing")
	}
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return platformPathIdentity{}, err
	}
	fileType := stat.Mode & syscall.S_IFMT
	if requireDirectory && fileType != syscall.S_IFDIR {
		return platformPathIdentity{}, fmt.Errorf("kernel: handle is not a directory")
	}
	if !requireDirectory && fileType != syscall.S_IFREG {
		return platformPathIdentity{}, fmt.Errorf("kernel: handle is not a regular file")
	}
	return platformPathIdentity{Device: uint64(stat.Dev), Inode: stat.Ino, IsDirectory: fileType == syscall.S_IFDIR}, nil
}

func openPlatformRestoreRoot(absolutePath string) (*platformRestoreDirectory, error) {
	fd, err := unix.Open(absolutePath, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), absolutePath)
	identity, err := captureUnixHandleIdentity(file, true)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return &platformRestoreDirectory{file: file, pathIdentity: identity}, nil
}

func (directory *platformRestoreDirectory) openOrCreateChildDirectory(name string) (*platformRestoreDirectory, bool, error) {
	if directory == nil || directory.file == nil || directory.closed {
		return nil, false, fmt.Errorf("kernel: restore directory handle unavailable")
	}
	if err := validatePlatformPathComponent(name); err != nil {
		return nil, false, err
	}
	fd, err := unix.Openat(int(directory.file.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	created := false
	if err != nil && err == unix.ENOENT {
		if err = unix.Mkdirat(int(directory.file.Fd()), name, 0o700); err != nil && err != unix.EEXIST {
			return nil, false, err
		}
		created = err == nil
		fd, err = unix.Openat(int(directory.file.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	}
	if err != nil {
		return nil, false, err
	}
	file := os.NewFile(uintptr(fd), name)
	identity, err := captureUnixHandleIdentity(file, true)
	if err != nil {
		_ = file.Close()
		return nil, false, err
	}
	return &platformRestoreDirectory{file: file, pathIdentity: identity}, created, nil
}

func (directory *platformRestoreDirectory) openRegularFile(name string) (*os.File, platformPathIdentity, error) {
	if directory == nil || directory.file == nil || directory.closed {
		return nil, platformPathIdentity{}, fmt.Errorf("kernel: restore directory handle unavailable")
	}
	if err := validatePlatformPathComponent(name); err != nil {
		return nil, platformPathIdentity{}, err
	}
	fd, err := unix.Openat(int(directory.file.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, platformPathIdentity{}, err
	}
	file := os.NewFile(uintptr(fd), name)
	identity, err := captureUnixHandleIdentity(file, false)
	if err != nil {
		_ = file.Close()
		return nil, platformPathIdentity{}, err
	}
	return file, identity, nil
}

func (directory *platformRestoreDirectory) createTempFile(prefix string) (*preparedRestoreTemp, error) {
	if directory == nil || directory.file == nil || directory.closed {
		return nil, fmt.Errorf("kernel: restore directory handle unavailable")
	}
	for attempt := 0; attempt < 32; attempt++ {
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}
		name := fmt.Sprintf("%s%x.tmp", prefix, bytes)
		fd, err := unix.Openat(int(directory.file.Fd()), name, unix.O_CREAT|unix.O_EXCL|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
		if err == unix.EEXIST {
			continue
		}
		if err != nil {
			return nil, err
		}
		file := os.NewFile(uintptr(fd), name)
		identity, err := captureUnixHandleIdentity(file, false)
		if err != nil {
			_ = file.Close()
			_ = unix.Unlinkat(int(directory.file.Fd()), name, 0)
			return nil, err
		}
		return &preparedRestoreTemp{Name: name, File: file, Identity: identity}, nil
	}
	return nil, fmt.Errorf("kernel: create restore temp exhausted retries")
}

func (directory *platformRestoreDirectory) publishTempNoReplace(temp *preparedRestoreTemp, targetName string) error {
	if directory == nil || directory.file == nil || temp == nil || temp.File == nil {
		return fmt.Errorf("kernel: restore publish handles missing")
	}
	if err := validatePlatformPathComponent(targetName); err != nil {
		return err
	}
	return unix.Linkat(int(directory.file.Fd()), temp.Name, int(directory.file.Fd()), targetName, 0)
}

func (directory *platformRestoreDirectory) removeChild(name string) error {
	if directory == nil || directory.file == nil || directory.closed {
		return fmt.Errorf("kernel: restore directory handle unavailable")
	}
	if err := validatePlatformPathComponent(name); err != nil {
		return err
	}
	return unix.Unlinkat(int(directory.file.Fd()), name, 0)
}

func (directory *platformRestoreDirectory) sync() error {
	if directory == nil || directory.file == nil || directory.closed {
		return fmt.Errorf("kernel: restore directory handle unavailable")
	}
	return unix.Fsync(int(directory.file.Fd()))
}
func (directory *platformRestoreDirectory) close() error {
	if directory == nil || directory.closed {
		return nil
	}
	directory.closed = true
	return directory.file.Close()
}
func (directory *platformRestoreDirectory) identity() platformPathIdentity {
	if directory == nil {
		return platformPathIdentity{}
	}
	return directory.pathIdentity
}

func openPlatformFileParent(absoluteRoot string, absoluteFilePath string) (*platformRestoreDirectory, string, error) {
	parent, err := openPlatformRestoreRoot(filepath.Dir(absoluteFilePath))
	if err != nil {
		return nil, "", err
	}
	name := filepath.Base(absoluteFilePath)
	if err := validatePlatformPathComponent(name); err != nil {
		_ = parent.close()
		return nil, "", err
	}
	return parent, name, nil
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

	for _, character := range component {
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
			Device: uint64(
				stat.Dev,
			),

			Inode: stat.Ino,

			IsDirectory: isDirectory,
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
