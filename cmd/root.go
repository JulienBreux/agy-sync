package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/pkg/config"
)

// GlobalOptions holds flags common to all commands.
type GlobalOptions struct {
	ConfigFile string
	Verbose    bool
	JSON       bool
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
	}

	rootCmd.PersistentFlags().StringVar(&globalOpts.ConfigFile, "config", config.DefaultConfigPath(), "path to config file")
	rootCmd.PersistentFlags().BoolVarP(&globalOpts.Verbose, "verbose", "v", false, "enable verbose / debug logging")
	rootCmd.PersistentFlags().BoolVar(&globalOpts.JSON, "json", false, "output results in JSON format")

	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newPushCommand())
	rootCmd.AddCommand(newPullCommand())
	rootCmd.AddCommand(newStartCommand())
	rootCmd.AddCommand(newStopCommand())
	rootCmd.AddCommand(newStatusCommand())

	return rootCmd
}

// Execute runs the root command.
func Execute() {
	rootCmd := NewRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
