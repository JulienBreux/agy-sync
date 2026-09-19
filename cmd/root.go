package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/internal/logger"
	"github.com/julienbreux/agy-sync/pkg/config"
)

// GlobalOptions holds flags common to all commands.
type GlobalOptions struct {
	ConfigFile string
	Verbose    bool
	JSON       bool
	LogLevel   string
}

var globalOpts GlobalOptions

// NewRootCommand creates the root cobra command for agy-sync.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "agy-sync",
		Short: "Bidirectional sync tool for Antigravity conversations and Firestore",
		Long: `agy-sync is a high-performance synchronization and storage CLI for Google Antigravity (AGY).
It asynchronously monitors local AGY transcripts and artifacts, syncs them to Google Cloud Firestore,
and allows multi-machine conversation history synchronization.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			lvl := globalOpts.LogLevel
			if globalOpts.Verbose {
				lvl = "debug"
			}
			l := logger.InitDefault(logger.Options{
				Level:  logger.ParseLevel(lvl),
				JSON:   globalOpts.JSON,
				Output: cmd.ErrOrStderr(),
			})
			cmd.SetContext(logger.WithLogger(cmd.Context(), l))
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&globalOpts.ConfigFile, "config", config.DefaultConfigPath(), "path to config file")
	rootCmd.PersistentFlags().BoolVarP(&globalOpts.Verbose, "verbose", "v", false, "enable verbose / debug logging")
	rootCmd.PersistentFlags().BoolVar(&globalOpts.JSON, "json", false, "output results in JSON format")
	rootCmd.PersistentFlags().StringVar(&globalOpts.LogLevel, "log-level", "info", "log level (debug, info, warn, error)")

	rootCmd.AddGroup(
		&cobra.Group{
			ID:    "daemon",
			Title: "Daemon Management Commands:",
		},
		&cobra.Group{
			ID:    "sync",
			Title: "Data Synchronization Commands:",
		},
		&cobra.Group{
			ID:    "setup",
			Title: "Configuration & Setup Commands:",
		},
	)

	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newPushCommand())
	rootCmd.AddCommand(newPullCommand())
	rootCmd.AddCommand(newClearCommand())
	rootCmd.AddCommand(newStartCommand())
	rootCmd.AddCommand(newStopCommand())
	rootCmd.AddCommand(newStatusCommand())
	rootCmd.AddCommand(newTransactionsCommand())
	rootCmd.AddCommand(newVersionCommand())

	return rootCmd
}

// Execute runs the root command.
func Execute() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	rootCmd := NewRootCommand()
	return rootCmd.ExecuteContext(ctx)
}
