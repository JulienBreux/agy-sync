package cmd

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/pkg/config"
)

func newInitCommand() *cobra.Command {
	var (
		projectID  string
		databaseID string
		machineID  string
		brainDir   string
	)

	cmd := &cobra.Command{
		Use:     "init",
		GroupID: "setup",
		Short:   "Initialize configuration for agy-sync",
		Long:    "Creates or updates the local configuration file with Google Cloud Project and Antigravity directories.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(projectID) == "" {
				return errors.New("project-id is required: specify with --project-id")
			}

			cfg := config.DefaultConfig()
			cfg.ProjectID = projectID

			cfg.DatabaseID = cmp.Or(strings.TrimSpace(databaseID), cfg.DatabaseID)
			cfg.MachineID = cmp.Or(strings.TrimSpace(machineID), cfg.MachineID)
			cfg.BrainDir = cmp.Or(strings.TrimSpace(brainDir), cfg.BrainDir)

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}

			configTarget := globalOpts.ConfigFile
			if err := config.SaveConfig(configTarget, cfg); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			if globalOpts.JSON {
				fmt.Printf(`{"status":"success","config_file":"%s","project_id":"%s","machine_id":"%s"}`+"\n",
					configTarget, cfg.ProjectID, cfg.MachineID)
			} else {
				cmd.Printf("Configuration successfully written to %s\n", configTarget)
				cmd.Printf("  Project ID: %s\n", cfg.ProjectID)
				cmd.Printf("  Database ID: %s\n", cfg.DatabaseID)
				cmd.Printf("  Machine ID: %s\n", cfg.MachineID)
				cmd.Printf("  Brain Dir: %s\n", cfg.BrainDir)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project-id", "", "Google Cloud Project ID (required)")
	cmd.Flags().StringVar(&databaseID, "database-id", "(default)", "Firestore Database ID")
	cmd.Flags().StringVar(&machineID, "machine-id", config.DefaultMachineID(), "Unique Machine Identifier")
	cmd.Flags().StringVar(&brainDir, "brain-dir", config.DefaultBrainDir(), "Path to Antigravity brain directory")

	return cmd
}
