//go:build windows

package kernel

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type platformPathIdentity struct {
	VolumeSerialNumber uint32
	FileIndexHigh      uint32
	FileIndexLow       uint32

	IsDirectory bool
}

type preparedRestoreTemp struct {
	Name     string
	File     *os.File
	Identity platformPathIdentity
}

const (
	windowsFileAddFile         = 0x0002
	windowsFileAddSubdirectory = 0x0004
)

type platformRestoreDirectory struct {
	handle       windows.Handle
	pathIdentity platformPathIdentity
	closed       bool
}

func captureWindowsHandleIdentity(handle windows.Handle, requireDirectory bool) (platformPathIdentity, error) {
	var information windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &information); err != nil {
		return platformPathIdentity{}, err
	}
	if err := validateWindowsPathInformation("<handle>", information, requireDirectory); err != nil {
		return platformPathIdentity{}, err
	}
	return platformPathIdentity{VolumeSerialNumber: information.VolumeSerialNumber, FileIndexHigh: information.FileIndexHigh, FileIndexLow: information.FileIndexLow, IsDirectory: requireDirectory}, nil
}

func ntCreateRelative(parent windows.Handle, name string, access uint32, attributes uint32, share uint32, disposition uint32, options uint32) (windows.Handle, error) {
	if parent == windows.InvalidHandle {
		return windows.InvalidHandle, fmt.Errorf("kernel: invalid Windows parent handle")
	}
	if err := validatePlatformPathComponent(name); err != nil {
		return windows.InvalidHandle, err
	}
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return windows.InvalidHandle, err
	}
	oa := windows.OBJECT_ATTRIBUTES{Length: uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})), RootDirectory: parent, ObjectName: objectName, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE}
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	if err := windows.NtCreateFile(&handle, access, &oa, &status, nil, attributes, share, disposition, options, 0, 0); err != nil {
		return windows.InvalidHandle, err
	}
	return handle, nil
}

func openPlatformRestoreRoot(absolutePath string) (*platformRestoreDirectory, error) {
	pointer, err := windows.UTF16PtrFromString(absolutePath)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(pointer, windows.FILE_LIST_DIRECTORY|windowsFileAddFile|windowsFileAddSubdirectory|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, err
	}
	identity, err := captureWindowsHandleIdentity(handle, true)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	return &platformRestoreDirectory{handle: handle, pathIdentity: identity}, nil
}

func (directory *platformRestoreDirectory) openExistingChildDirectory(name string) (*platformRestoreDirectory, error) {
	if directory == nil || directory.closed {
		return nil, fmt.Errorf("kernel: restore directory handle unavailable")
	}
	handle, err := ntCreateRelative(
		directory.handle,
		name,
		windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		windows.FILE_ATTRIBUTE_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
	)
	if err != nil {
		if isWindowsPathNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	identity, err := captureWindowsHandleIdentity(handle, true)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	return &platformRestoreDirectory{handle: handle, pathIdentity: identity}, nil
}

func (directory *platformRestoreDirectory) createChildDirectoryExclusive(name string) (*platformRestoreDirectory, error) {
	if directory == nil || directory.closed {
		return nil, fmt.Errorf("kernel: restore directory handle unavailable")
	}
	handle, err := ntCreateRelative(
		directory.handle,
		name,
		windows.FILE_LIST_DIRECTORY|windowsFileAddFile|windowsFileAddSubdirectory|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		windows.FILE_ATTRIBUTE_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_CREATE,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_EXISTS) || errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			return nil, NewPackageError(
				PackageErrCodeResourceRestorePathRace,
				409,
				fmt.Errorf("kernel: missing restore directory %s appeared concurrently", name),
			)
		}
		return nil, err
	}
	identity, err := captureWindowsHandleIdentity(handle, true)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	return &platformRestoreDirectory{handle: handle, pathIdentity: identity}, nil
}

func (directory *platformRestoreDirectory) openRegularFile(name string) (*os.File, platformPathIdentity, error) {
	if directory == nil || directory.closed {
		return nil, platformPathIdentity{}, fmt.Errorf("kernel: restore directory handle unavailable")
	}
	handle, err := ntCreateRelative(directory.handle, name, windows.GENERIC_READ|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN, windows.FILE_NON_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT)
	if err != nil {
		if isWindowsPathNotExist(err) {
			return nil, platformPathIdentity{}, os.ErrNotExist
		}
		return nil, platformPathIdentity{}, err
	}
	identity, err := captureWindowsHandleIdentity(handle, false)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, platformPathIdentity{}, err
	}
	return os.NewFile(uintptr(handle), name), identity, nil
}

func safeCreateFilePlatform(parent *platformRestoreDirectory, name string) (*os.File, platformPathIdentity, error) {
	if parent == nil || parent.closed {
		return nil, platformPathIdentity{}, fmt.Errorf("kernel: restore directory handle unavailable")
	}
	handle, err := ntCreateRelative(parent.handle, name, windows.GENERIC_READ|windows.GENERIC_WRITE|windows.FILE_READ_ATTRIBUTES|windows.DELETE|windows.SYNCHRONIZE, windows.FILE_ATTRIBUTE_TEMPORARY, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_CREATE, windows.FILE_NON_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT)
	if err != nil {
		return nil, platformPathIdentity{}, err
	}
	identity, err := captureWindowsHandleIdentity(handle, false)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, platformPathIdentity{}, err
	}
	return os.NewFile(uintptr(handle), name), identity, nil
}

func (directory *platformRestoreDirectory) createTempFile(prefix string) (*preparedRestoreTemp, error) {
	for attempt := 0; attempt < 32; attempt++ {
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}
		name := fmt.Sprintf("%s%x.tmp", prefix, bytes)
		file, identity, err := safeCreateFilePlatform(directory, name)
		if err != nil {
			if errors.Is(err, windows.ERROR_FILE_EXISTS) {
				continue
			}
			return nil, err
		}
		return &preparedRestoreTemp{Name: name, File: file, Identity: identity}, nil
	}
	return nil, fmt.Errorf("kernel: create restore temp exhausted retries")
}

type windowsFileLinkInformationLayout struct {
	ReplaceIfExists uint8
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

func buildWindowsFileLinkInformation(root windows.Handle, name string) ([]byte, error) {
	utf16Name, err := windows.UTF16FromString(name)
	if err != nil {
		return nil, err
	}
	utf16Name = utf16Name[:len(utf16Name)-1]
	var layout windowsFileLinkInformationLayout
	offset := unsafe.Offsetof(layout.FileName)
	buffer := make([]byte, int(offset)+len(utf16Name)*2)
	header := (*windowsFileLinkInformationLayout)(unsafe.Pointer(&buffer[0]))
	header.RootDirectory = root
	header.FileNameLength = uint32(len(utf16Name) * 2)
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(&buffer[offset])), len(utf16Name)), utf16Name)
	return buffer, nil
}
func (directory *platformRestoreDirectory) publishTempNoReplace(temp *preparedRestoreTemp, targetName string) error {
	if directory == nil || temp == nil || temp.File == nil {
		return fmt.Errorf("kernel: restore publish handles missing")
	}
	buffer, err := buildWindowsFileLinkInformation(directory.handle, targetName)
	if err != nil {
		return err
	}
	var status windows.IO_STATUS_BLOCK
	return windows.NtSetInformationFile(windows.Handle(temp.File.Fd()), &status, &buffer[0], uint32(len(buffer)), windows.FileLinkInformation)
}
func (directory *platformRestoreDirectory) removeChild(name string) error {
	handle, err := ntCreateRelative(directory.handle, name, windows.DELETE|windows.SYNCHRONIZE, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN, windows.FILE_NON_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	var status windows.IO_STATUS_BLOCK
	disposition := byte(1)
	return windows.NtSetInformationFile(handle, &status, &disposition, 1, windows.FileDispositionInformation)
}
func (directory *platformRestoreDirectory) sync() error {
	if directory == nil || directory.closed || directory.handle == windows.InvalidHandle {
		return fmt.Errorf("kernel: invalid Windows directory handle")
	}
	return nil
}
func (directory *platformRestoreDirectory) close() error {
	if directory == nil || directory.closed {
		return nil
	}
	directory.closed = true
	return windows.CloseHandle(directory.handle)
}
func (directory *platformRestoreDirectory) identity() platformPathIdentity {
	if directory == nil {
		return platformPathIdentity{}
	}
	return directory.pathIdentity
}
func openPlatformFileParent(absoluteRoot string, absoluteFilePath string) (*platformRestoreDirectory, string, error) {
	absoluteRoot = filepath.Clean(absoluteRoot)
	absoluteFilePath = filepath.Clean(absoluteFilePath)
	if !pathIsWithinRoot(absoluteRoot, absoluteFilePath) {
		return nil, "", fmt.Errorf("kernel: file path %s escapes root %s", absoluteFilePath, absoluteRoot)
	}
	name := filepath.Base(absoluteFilePath)
	if err := validatePlatformPathComponent(name); err != nil {
		return nil, "", err
	}
	current, err := openPlatformRestoreRoot(absoluteRoot)
	if err != nil {
		return nil, "", err
	}
	relativeParent, err := filepath.Rel(absoluteRoot, filepath.Dir(absoluteFilePath))
	if err != nil {
		_ = current.close()
		return nil, "", err
	}
	if relativeParent == "." {
		return current, name, nil
	}
	for _, component := range strings.Split(filepath.Clean(relativeParent), string(filepath.Separator)) {
		child, err := current.openExistingChildDirectory(component)
		if err != nil {
			_ = current.close()
			return nil, "", err
		}
		_ = current.close()
		current = child
	}
	return current, name, nil
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

	for _, character := range component {
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
			windows.STATUS_OBJECT_NAME_NOT_FOUND,
		) ||
		errors.Is(
			err,
			windows.STATUS_OBJECT_PATH_NOT_FOUND,
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
		VolumeSerialNumber: information.VolumeSerialNumber,

		FileIndexHigh: information.FileIndexHigh,

		FileIndexLow: information.FileIndexLow,

		IsDirectory: information.FileAttributes&
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

func safeCreateFilePlatformPathLegacy(
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
