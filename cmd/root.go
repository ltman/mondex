package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ltman/mondex/migration"
)

type Config struct {
	MongoURI       string `mapstructure:"mongo_uri"`
	DatabaseName   string `mapstructure:"database_name"`
	SchemaFilePath string `mapstructure:"schema_file_path"`
	MigrationDir   string `mapstructure:"migration_dir"`
	MigrationName  string `mapstructure:"-"`
	LogLevel       string `mapstructure:"log_level"`
}

var (
	cfg     Config
	cfgFile string

	dryRun bool
)

func Execute() {
	cobra.OnInitialize(initConfig)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initConfig() {
	const defaultConfigFile = "mondex.yml"

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigFile(defaultConfigFile)
	}

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		fmt.Printf("Unable to decode config into struct: %v\n", err)
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
		Use:   "mondex",
		Short: "MongoDB migration tool",
	}

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./mondex.yaml)")
	cmd.PersistentFlags().String("mongo_uri", "", "MongoDB connection URI")
	cmd.PersistentFlags().String("database_name", "", "Name of the database")
	cmd.PersistentFlags().String("schema_file_path", "", "Path to the schema file")
	cmd.PersistentFlags().String("migration_dir", "", "Directory for migration files")
	cmd.PersistentFlags().String("log_level", "info", "Logging level (debug, info, warn, error)")
	cmd.PersistentFlags().BoolVar(&dryRun, "dry_run", false, "Show changes without writing files")

	if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
		// Since this is called during initialization, we can't return an error.
		// Instead, we'll print the error and exit.
		fmt.Printf("Error binding flags: %v\n", err)
		os.Exit(1)
	}

	cmd.AddCommand(newDiffCmd(), newFormatCmd(), newInspectCmd())

	return cmd
}

func newDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff [migration_name]",
		Short: "Generate migration scripts based on schema differences",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDiff,
	}
}

func newFormatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "format",
		Short: "Format current schema file",
		RunE:  runFormat,
	}
}

func newInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect",
		Short: "Inspect and output the current database schema",
		RunE:  runInspect,
	}
}

func validateConfig(requiredFields []string) error {
	var missingFields []string
	for _, field := range requiredFields {
		if viper.GetString(field) == "" {
			missingFields = append(missingFields, field)
		}
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missingFields, ", "))
	}

	if cfg.SchemaFilePath != "" {
		if _, err := os.Stat(cfg.SchemaFilePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("schema file does not exist: %s", cfg.SchemaFilePath)
		}
	}

	return nil
}

func runDiff(cmd *cobra.Command, args []string) error {
	requiredFields := []string{"mongo_uri", "database_name", "schema_file_path"}
	if !dryRun {
		log.Println(args)
		if len(args) == 1 {
			viper.Set("migration_name", args[0])
			cfg.MigrationName = args[0]
		}
		requiredFields = append(requiredFields, "migration_dir", "migration_name")
	}

	if err := validateConfig(requiredFields); err != nil {
		return err
	}

	return runWithContext(cmd.Context(), func(ctx context.Context, logger *slog.Logger, config Config) error {
		return migration.GenerateMigrationScripts(
			ctx,
			logger,
			config.MongoURI,
			config.DatabaseName,
			config.SchemaFilePath,
			config.MigrationDir,
			config.MigrationName,
			dryRun,
		)
	})
}

func runFormat(cmd *cobra.Command, _ []string) error {
	requiredFields := []string{"schema_file_path"}

	if err := validateConfig(requiredFields); err != nil {
		return err
	}

	return runWithContext(cmd.Context(), func(ctx context.Context, logger *slog.Logger, config Config) error {
		return migration.FormatSchemaFile(
			ctx,
			logger,
			config.SchemaFilePath,
			dryRun,
		)
	})
}

func runInspect(cmd *cobra.Command, _ []string) error {
	requiredFields := []string{"mongo_uri", "database_name"}
	if !dryRun {
		requiredFields = append(requiredFields, "schema_file_path")
	}

	if err := validateConfig(requiredFields); err != nil {
		return err
	}

	return runWithContext(cmd.Context(), func(ctx context.Context, logger *slog.Logger, config Config) error {
		return migration.InspectCurrentSchema(
			ctx,
			logger,
			config.MongoURI,
			config.DatabaseName,
			config.SchemaFilePath,
			dryRun,
		)
	})
}

func runWithContext(ctx context.Context, fn func(context.Context, *slog.Logger, Config) error) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger, err := initLogger(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Debug("Starting operation")

	err = fn(ctx, logger, cfg)
	if err != nil {
		return fmt.Errorf("operation failed: %w", err)
	}

	return nil
}
