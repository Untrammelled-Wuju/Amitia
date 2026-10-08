package kernel

import (
	"context"
	"fmt"
	"os"
	"sync"
)

func (s *PackageGenerationStore) AcquireCurrentExecution(ctx context.Context, extensionID, generationID, treeHash string) (string, func(), error) {
	if err := validatePathSegment("extension ID", extensionID, true); err != nil {
		return "", nil, err
	}
	if err := validatePathSegment("generation ID", generationID, false); err != nil || treeHash == "" {
		return "", nil, fmt.Errorf("%w: execution generation identity incomplete", ErrPackageGenerationUnsafe)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	current, err := s.readCurrentLocked(extensionID)
	if err != nil {
		return "", nil, err
	}
	if current.GenerationID != generationID || !equalTreeHash(current.TreeHash, treeHash) {
		return "", nil, ErrPackageGenerationCAS
	}
	_, path, err := s.paths(current.ExtensionID, current.GenerationID, current.OperationID)
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", nil, ErrPackageGenerationNotFound
	}
	key := extensionID + "\x00" + generationID
	if s.readers[key] >= 128 {
		return "", nil, fmt.Errorf("插件安装代次的执行读取租约超过限制")
	}
	s.readers[key]++
	var once sync.Once
	release := func() {
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.readers[key]--
			if s.readers[key] == 0 {
				delete(s.readers, key)
			}
		})
	}
	return path, release, nil
}
