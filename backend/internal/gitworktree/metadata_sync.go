package gitworktree

import (
	"os"
	"runtime"
)

func syncManagedMetadataDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func metadataFilePermissionsUnsafe(permissions os.FileMode) bool {
	return runtime.GOOS != "windows" && permissions&0077 != 0
}
