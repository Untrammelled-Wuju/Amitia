package migration

import "context"

type PhysicalSnapshotPort interface {
	CheckpointWAL(ctx context.Context) error
	RunIntegrityCheck(ctx context.Context) error
	RunForeignKeyCheck(ctx context.Context) error
	GetSQLiteFiles(ctx context.Context) ([]SQLiteFile, error)
	BackupTo(ctx context.Context, destPath string) error
	Migrate(ctx context.Context) error
}

type SQLiteFile struct {
	Name string
	Path string
}

type PhysicalSnapshotDependencies struct {
	Port PhysicalSnapshotPort
}

type physicalSnapshotSafety struct {
	deps PhysicalSnapshotDependencies
}

func NewPhysicalSnapshotSafety(deps PhysicalSnapshotDependencies) *physicalSnapshotSafety {
	return &physicalSnapshotSafety{deps: deps}
}

func (s *physicalSnapshotSafety) Execute(ctx context.Context, destPath string) error {
	if s.deps.Port == nil {
		return nil
	}
	if err := s.deps.Port.CheckpointWAL(ctx); err != nil {
		return err
	}
	if err := s.deps.Port.RunIntegrityCheck(ctx); err != nil {
		return err
	}
	if err := s.deps.Port.RunForeignKeyCheck(ctx); err != nil {
		return err
	}
	return s.deps.Port.BackupTo(ctx, destPath)
}
