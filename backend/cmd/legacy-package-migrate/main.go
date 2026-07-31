//go:build legacy_migration

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/database/mysql"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

func usage() {
	fmt.Fprintf(os.Stderr, `amitia-legacy-migrate - Legacy Package Migration CLI

Usage:
  amitia-legacy-migrate <command> [flags]

Commands:
  detect     Detect legacy packages and print report
  migrate    Execute full legacy package migration to kernel
  status     Show migration status for all legacy packages
  help       Show this help message

Flags:
  --data-dir <path>     Override data directory (default: from config)
  --config-path <path>  Override config path (default: <exe-dir>/config)
  --yes                 Skip confirmation prompt (for migrate command)

Examples:
  amitia-legacy-migrate detect
  amitia-legacy-migrate detect --data-dir ./data
  amitia-legacy-migrate migrate --yes
  amitia-legacy-migrate status
`)
}

type cliOptions struct {
	command    string
	dataDir    string
	configPath string
	yes        bool
}

func parseArgs(args []string) cliOptions {
	opts := cliOptions{}
	if len(args) == 0 {
		opts.command = "help"
		return opts
	}
	opts.command = args[0]
	fs := flag.NewFlagSet("legacy-migrate", flag.ContinueOnError)
	fs.StringVar(&opts.dataDir, "data-dir", "", "override data directory")
	fs.StringVar(&opts.configPath, "config-path", "", "override config path")
	fs.BoolVar(&opts.yes, "yes", false, "skip confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		opts.command = "help"
	}
	return opts
}

func main() {
	opts := parseArgs(os.Args[1:])
	if opts.command == "help" || opts.command == "--help" || opts.command == "-h" {
		usage()
		os.Exit(0)
	}
	if opts.command != "detect" && opts.command != "migrate" && opts.command != "status" {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", opts.command)
		usage()
		os.Exit(1)
	}

	runtimeRoot := util.RuntimeRoot()

	configPath := opts.configPath
	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
	}
	if configPath == "" {
		configPath = filepath.Join(runtimeRoot, "config")
	} else if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(runtimeRoot, configPath)
	}
	config.InitConfig(configPath)

	if opts.dataDir != "" {
		if filepath.IsAbs(opts.dataDir) {
			config.AppCfg.Storage.DataDir = opts.dataDir
		} else {
			config.AppCfg.Storage.DataDir = filepath.Join(runtimeRoot, opts.dataDir)
		}
	} else {
		config.AppCfg.Storage.DataDir = util.ResolveRuntimePath(runtimeRoot, config.AppCfg.Storage.DataDir)
	}
	config.AppCfg.Surreal.DataPath = util.ResolveRuntimePath(runtimeRoot, config.AppCfg.Surreal.DataPath)

	log.InitLogger(filepath.Join(runtimeRoot, "logs"))

	rootCtx, stop := context.WithCancel(context.Background())
	defer stop()

	db := mysql.NewSQLite(config.AppCfg.Storage.DataDir)
	if err := applyMigrations(db); err != nil {
		fmt.Fprintf(os.Stderr, "database migration failed: %v\n", err)
		os.Exit(1)
	}

	extRuntime, err := extension.NewRuntimeWithOptions(rootCtx, db, config.AppCfg.App.Version, extension.RuntimeOptions{SkipPluginManagerStart: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "extension runtime init failed: %v\n", err)
		os.Exit(1)
	}

	kernelRoot := filepath.Join(config.AppCfg.Storage.DataDir, "extensions-v2")
	if err := extRuntime.AttachKernel(kernelRoot); err != nil {
		fmt.Fprintf(os.Stderr, "extension kernel attach failed: %v\n", err)
		os.Exit(1)
	}

	kernelDBPath := filepath.Join(kernelRoot, "kernel.db")
	kernelContainer, err := kernel.NewContainerBuilder().
		WithDBPath(kernelDBPath).
		WithExtensionRoot(kernelRoot).
		Build(rootCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kernel container build failed: %v\n", err)
		os.Exit(1)
	}
	extRuntime.Kernel.SetContainer(kernelContainer)

	if err := extRuntime.Kernel.RecoverPackageOperations(rootCtx); err != nil {
		log.Warn("kernel package operation recovery warning: ", err)
	}
	if err := kernelContainer.Recover(rootCtx); err != nil {
		log.Warn("kernel recovery warning: ", err)
	}

	kernelProxy := extension.NewKernelLifecycleProxy(extRuntime.Kernel)
	if err := extRuntime.Packages.AttachKernelProxy(kernelProxy); err != nil {
		fmt.Fprintf(os.Stderr, "kernel proxy attach failed: %v\n", err)
		os.Exit(1)
	}
	if err := extRuntime.Packages.StartupCleanup(rootCtx); err != nil {
		log.Warn("package startup cleanup warning: ", err)
	}

	switch opts.command {
	case "detect":
		runDetect(rootCtx, extRuntime)
	case "migrate":
		runMigrate(rootCtx, extRuntime, opts.yes)
	case "status":
		runStatus(rootCtx, extRuntime)
	}
}

func applyMigrations(db *gorm.DB) error {
	isNew, err := migration.IsNewDatabase(db)
	if err != nil {
		return fmt.Errorf("check existing database: %w", err)
	}
	migrations := migration.DefaultMigrations()
	migRunner := migration.Runner{DB: db, SkipBackup: !isNew}
	if isNew {
		if err := migration.ApplyBaseline(db); err != nil {
			return fmt.Errorf("apply baseline: %w", err)
		}
		if err := migration.MarkAllMigrationsApplied(db, migrations); err != nil {
			return fmt.Errorf("mark all migrations applied: %w", err)
		}
		return nil
	}
	if err := migRunner.CreatePreMigrationBackup(); err != nil {
		return fmt.Errorf("pre-migration backup failed: %w", err)
	}
	if err := migration.ApplyBaseline(db); err != nil {
		return fmt.Errorf("apply baseline: %w", err)
	}
	if err := migRunner.Apply(migrations); err != nil {
		return fmt.Errorf("apply versioned migrations: %w", err)
	}
	return nil
}

func runDetect(ctx context.Context, extRuntime *extension.Runtime) {
	report, err := extRuntime.Packages.DetectLegacyPackages(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "legacy package detection failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Legacy Package Detection Report\n")
	fmt.Printf("================================\n")
	fmt.Printf("Total extensions:     %d\n", report.Total)
	fmt.Printf("Completed migrations: %d\n", report.Completed)
	fmt.Printf("Pending manual:       %d\n", report.PendingManual)
	if len(report.PendingExtensions) > 0 {
		fmt.Printf("\nPending extensions:\n")
		for _, extID := range report.PendingExtensions {
			fmt.Printf("  - %s\n", extID)
		}
	}
	if report.PendingManual > 0 {
		fmt.Printf("\nRun 'amitia-legacy-migrate migrate' to execute migration.\n")
		os.Exit(2)
	}
}

func runMigrate(ctx context.Context, extRuntime *extension.Runtime, skipConfirm bool) {
	report, err := extRuntime.Packages.DetectLegacyPackages(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pre-migration detection failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Legacy Package Migration\n")
	fmt.Printf("========================\n")
	fmt.Printf("Total extensions:     %d\n", report.Total)
	fmt.Printf("Completed migrations: %d\n", report.Completed)
	fmt.Printf("Pending manual:       %d\n", report.PendingManual)
	if len(report.PendingExtensions) > 0 {
		fmt.Printf("\nExtensions to migrate:\n")
		for _, extID := range report.PendingExtensions {
			fmt.Printf("  - %s\n", extID)
		}
	}
	if report.PendingManual == 0 {
		fmt.Printf("\nNo pending migrations. All legacy packages are already migrated or blocked.\n")
		return
	}
	if !skipConfirm {
		fmt.Printf("\nThis will migrate %d legacy package(s) to the extension kernel.\n", report.PendingManual)
		fmt.Printf("Type 'yes' to continue: ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "yes" {
			fmt.Printf("Migration cancelled.\n")
			os.Exit(0)
		}
	}
	fmt.Printf("\nStarting migration at %s...\n", time.Now().Format(time.RFC3339))
	startTime := time.Now()
	if err := extRuntime.Packages.MigrateLegacyPackages(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "migration execution failed: %v\n", err)
		os.Exit(1)
	}
	elapsed := time.Since(startTime)
	finalReport, _ := extRuntime.Packages.DetectLegacyPackages(ctx)
	fmt.Printf("\nMigration completed in %s\n", elapsed)
	fmt.Printf("Completed: %d / %d\n", finalReport.Completed, finalReport.Total)
	fmt.Printf("Pending:   %d\n", finalReport.PendingManual)
	if finalReport.PendingManual > 0 {
		fmt.Printf("\nSome packages require manual migration. Run 'amitia-legacy-migrate status' for details.\n")
		os.Exit(2)
	}
}

func runStatus(ctx context.Context, extRuntime *extension.Runtime) {
	metrics := extRuntime.Packages.Metrics()
	fmt.Printf("Legacy Migration Status\n")
	fmt.Printf("=======================\n\n")
	fmt.Printf("Metrics:\n")
	for key, value := range metrics {
		fmt.Printf("  %-35s %d\n", key+":", value)
	}
	fmt.Printf("\nDetection:\n")
	report, err := extRuntime.Packages.DetectLegacyPackages(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "detection failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Total extensions:     %d\n", report.Total)
	fmt.Printf("  Completed migrations: %d\n", report.Completed)
	fmt.Printf("  Pending manual:       %d\n", report.PendingManual)
	if len(report.PendingExtensions) > 0 {
		fmt.Printf("\nPending extensions:\n")
		for _, extID := range report.PendingExtensions {
			fmt.Printf("  - %s\n", extID)
		}
	}
	legacyWriteCalls := metrics["package_legacy_write_calls"]
	legacyWriteAttempts := metrics["legacy_write_attempts"]
	if legacyWriteCalls > 0 || legacyWriteAttempts > 0 {
		fmt.Printf("\nWARNING: Legacy write calls detected (%d calls, %d attempts)\n", legacyWriteCalls, legacyWriteAttempts)
	}
}
