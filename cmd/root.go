package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"bitbucket.org/ltman/mondex/migration"
	"github.com/spf13/cobra"
)

type Config struct {
	MongoURI       string
	DatabaseName   string
	SchemaFilePath string
	OutputDir      string
	MigrationName  string
	LogLevel       string
	DryRun         bool
}

var cfg Config

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initLogger(level string) (*slog.Logger, error) {
	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, err
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})), nil
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bitbucket.org/ltman/mondex",
		Short: "MongoDB migration tool",
	}

	registerPersistentFlags(cmd)

	cmd.AddCommand(
		newDiffCmd(),
		newInspectCmd(),
	)

	return cmd
}

func registerPersistentFlags(cmd *cobra.Command) {
	const defaultLogLevel = "info"

	cmd.PersistentFlags().StringVar(&cfg.MongoURI, "mongo-uri", "", "MongoDB connection URI")
	cmd.PersistentFlags().StringVar(&cfg.DatabaseName, "database-name", "", "Name of the database")
	cmd.PersistentFlags().StringVar(&cfg.SchemaFilePath, "schema-file-path", "", "Path to the schema file")
	cmd.PersistentFlags().StringVar(&cfg.OutputDir, "output-dir", "", "Directory for output files")
	cmd.PersistentFlags().StringVar(&cfg.MigrationName, "migration-name", "", "Name of the migration")
	cmd.PersistentFlags().StringVar(&cfg.LogLevel, "log-level", defaultLogLevel, "Logging level (debug, info, warn, error)")
	cmd.PersistentFlags().BoolVar(&cfg.DryRun, "dry-run", false, "Show changes without writing files")

	_ = cmd.MarkFlagRequired("mongo-uri")
	_ = cmd.MarkFlagRequired("database-name")
	_ = cmd.MarkFlagRequired("schema-file-path")
	_ = cmd.MarkFlagRequired("output-dir")
	_ = cmd.MarkFlagRequired("migration-name")
}

func newDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "diff",
		Short:   "Generate migration scripts based on schema differences",
		PreRunE: preRunDiff,
		RunE:    runDiff,
	}
}

func newInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "inspect",
		Short:   "Inspect and output the current database schema",
		PreRunE: preRunInspect,
		RunE:    runInspect,
	}
}

func preRunDiff(_ *cobra.Command, _ []string) error {
	var missingFields []string

	if cfg.MongoURI == "" {
		missingFields = append(missingFields, "mongo-uri")
	}
	if cfg.DatabaseName == "" {
		missingFields = append(missingFields, "database-name")
	}
	if cfg.SchemaFilePath == "" {
		missingFields = append(missingFields, "schema-file")
	}
	if !cfg.DryRun && cfg.OutputDir == "" {
		missingFields = append(missingFields, "output-dir")
	}
	if !cfg.DryRun && cfg.MigrationName == "" {
		missingFields = append(missingFields, "migration-name")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("missing required fields:\n  - %v", strings.Join(missingFields, "\n  - "))
	}

	if _, err := os.Stat(cfg.SchemaFilePath); cfg.SchemaFilePath != "" && err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("schema file does not exist: %s", cfg.SchemaFilePath)
	}
	if _, err := os.Stat(cfg.OutputDir); cfg.OutputDir != "" && err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("output directory does not exist: %s", cfg.OutputDir)
	}

	return nil
}

func runDiff(_ *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger, err := initLogger(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Debug("Starting migration script generator")

	err = migration.GenerateMigrationScripts(
		ctx,
		logger,
		cfg.MongoURI, cfg.DatabaseName,
		cfg.SchemaFilePath,
		cfg.OutputDir, cfg.MigrationName,
		cfg.DryRun,
	)
	if err != nil {
		return fmt.Errorf("failed to generate migration scripts: %w", err)
	}

	if _, err := os.Stat(cfg.SchemaFilePath); cfg.SchemaFilePath != "" && err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("schema file does not exist: %s", cfg.SchemaFilePath)
	}

	return nil
}

func preRunInspect(_ *cobra.Command, _ []string) error {
	var missingFields []string

	if cfg.MongoURI == "" {
		missingFields = append(missingFields, "mongo-uri")
	}
	if cfg.DatabaseName == "" {
		missingFields = append(missingFields, "database-name")
	}
	if !cfg.DryRun && cfg.SchemaFilePath == "" {
		missingFields = append(missingFields, "schema-file")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("missing required fields:\n  - %v", strings.Join(missingFields, "\n  - "))
	}

	return nil
}

func runInspect(_ *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger, err := initLogger(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Debug("Starting migration script generator")

	err = migration.InspectCurrentSchema(
		ctx,
		logger,
		cfg.MongoURI, cfg.DatabaseName,
		cfg.SchemaFilePath,
		cfg.DryRun,
	)
	if err != nil {
		return fmt.Errorf("failed to generate migration scripts: %w", err)
	}

	return nil
}
