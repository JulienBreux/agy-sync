package cmd

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/internal/setup"
	"github.com/julienbreux/agy-sync/pkg/config"
)

func newSetupCommand() *cobra.Command {
	return newSetupCommandWith(setup.NewGCPChecker(), setup.NewGCPProvisioner())
}

func newSetupCommandWith(checker setup.Checker, provisioner setup.Provisioner) *cobra.Command {
	var (
		dryRun      bool
		autoApprove bool
		projectID   string
		databaseID  string
		location    string
		configPath  string
		jsonOutput  bool
	)

	cmd := &cobra.Command{
		Use:     "setup",
		Aliases: []string{"check", "doctor"},
		GroupID: "setup",
		Short:   "Verify Google Cloud environment, credentials, APIs, and Firestore database",
		Long: `Inspects local credentials and configuration, verifies Google Cloud APIs and permissions,
checks or provisions the Firestore database, and verifies Cloud Storage access.
Includes --dry-run mode for non-mutating inspection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve config path
			cfgFile := configPath
			if cfgFile == "" {
				cfgFile = globalOpts.ConfigFile
			}
			if cfgFile == "" {
				cfgFile = config.DefaultConfigPath()
			}

			// Attempt loading existing configuration
			cfg, err := config.LoadConfig(cfgFile)
			if err != nil {
				cfg = config.DefaultConfig()
			}

			targetProject := cmp.Or(strings.TrimSpace(projectID), cfg.ProjectID)
			targetDB := cmp.Or(strings.TrimSpace(databaseID), cfg.DatabaseID, "(default)")
			targetLocation := cmp.Or(strings.TrimSpace(location), "nam5")
			isJSON := jsonOutput || globalOpts.JSON

			checkOpts := setup.CheckOptions{
				ProjectID:        targetProject,
				DatabaseID:       targetDB,
				DryRun:           dryRun,
				ConfigPath:       cfgFile,
				BrainDir:         cfg.BrainDir,
				ConversationsDir: cfg.ConversationsDir,
				SummariesDB:      cfg.SummariesDB,
				TransactionsDB:   cfg.TransactionsDB,
			}

			report, err := checker.Check(cmd.Context(), checkOpts)
			if err != nil {
				return fmt.Errorf("diagnostic check failed: %w", err)
			}

			// If database is missing and not dry-run, handle provisioning
			for i, item := range report.Items {
				if item.Name == "Firestore Database Access" && item.Status == setup.StatusWarn && strings.Contains(item.Message, "does not exist") && !dryRun {
					provOpts := setup.ProvisionOptions{
						ProjectID:   targetProject,
						DatabaseID:  targetDB,
						Location:    targetLocation,
						DryRun:      dryRun,
						AutoApprove: autoApprove,
						ConfirmPrompt: func(prompt string) (bool, error) {
							cmd.Printf("\n[?] Firestore database %q does not exist in project %q.\n", targetDB, targetProject)
							cmd.Printf("    Location: %s (Native mode)\n", targetLocation)
							cmd.Printf("    Would you like to create it now? [y/N]: ")
							reader := bufio.NewReader(cmd.InOrStdin())
							input, rErr := reader.ReadString('\n')
							if rErr != nil && !errors.Is(rErr, io.EOF) {
								return false, rErr
							}
							ans := strings.TrimSpace(strings.ToLower(input))
							return ans == "y" || ans == "yes", nil
						},
					}

					provRes, pErr := provisioner.CreateDatabase(cmd.Context(), provOpts)
					if pErr != nil {
						report.Items[i].Status = setup.StatusFail
						report.Items[i].Message = fmt.Sprintf("Failed to create database: %v", pErr)
					} else if provRes.Created {
						report.Items[i].Status = setup.StatusPass
						report.Items[i].Message = fmt.Sprintf("Database %q created successfully in region %s", targetDB, targetLocation)
						report.Items[i].Remediation = ""
					}
				}
			}

			// Recalculate overall status
			hasFailure := false
			for _, item := range report.Items {
				if item.Status == setup.StatusFail {
					hasFailure = true
					break
				}
			}
			report.AllPassed = !hasFailure

			if isJSON {
				data, mErr := json.MarshalIndent(report, "", "  ")
				if mErr != nil {
					return mErr
				}
				cmd.Println(string(data))
				if !report.AllPassed {
					return errors.New("setup diagnostic checks reported failures")
				}
				return nil
			}

			// Terminal visual checklist output
			printReport(cmd, report)

			if !report.AllPassed {
				return errors.New("setup diagnostic checks reported failures")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "inspect environment and cloud resources without making changes")
	cmd.Flags().BoolVarP(&autoApprove, "yes", "y", false, "automatically confirm database creation without interactive prompting")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Google Cloud project ID")
	cmd.Flags().StringVar(&databaseID, "database-id", "", "Firestore database ID (default: (default))")
	cmd.Flags().StringVar(&location, "location", "nam5", "Firestore database region/location for creation (default: nam5)")
	cmd.Flags().StringVar(&configPath, "config", "", "path to config file")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output results in JSON format")

	return cmd
}

func printReport(cmd *cobra.Command, report *setup.SetupReport) {
	cmd.Println("AGY-SYNC ENVIRONMENT & CLOUD SETUP")
	cmd.Println("==================================")
	cmd.Printf("Target Project:  %s\n", report.ProjectID)
	cmd.Printf("Target Database: %s\n", report.DatabaseID)
	mode := "Live Mode"
	if report.DryRun {
		mode = "Dry Run (Inspection Only)"
	}
	cmd.Printf("Execution Mode:  %s\n\n", mode)
	cmd.Println("DIAGNOSTIC CHECKS:")

	var remediations []setup.CheckItem
	for _, item := range report.Items {
		badge := "[?]"
		switch item.Status {
		case setup.StatusPass:
			badge = "[✓]"
		case setup.StatusFail:
			badge = "[✗]"
		case setup.StatusWarn:
			badge = "[!]"
		case setup.StatusSkipped:
			badge = "[-]"
		}

		durStr := ""
		if item.Duration > 0 {
			durStr = fmt.Sprintf(" (%dms)", item.Duration.Milliseconds())
		}

		cmd.Printf("  %s %s%s\n", badge, item.Name, durStr)
		if item.Message != "" {
			cmd.Printf("      %s\n", item.Message)
		}
		for _, d := range item.Details {
			cmd.Printf("      • %s\n", d)
		}

		if item.Remediation != "" && item.Status != setup.StatusPass {
			remediations = append(remediations, item)
		}
	}

	cmd.Println("\n==================================")
	if report.AllPassed {
		cmd.Println("STATUS: ALL CHECKS PASSED")
		cmd.Println("Ready to run 'agy-sync start' or 'agy-sync push'.")
	} else {
		cmd.Println("STATUS: ISSUES DETECTED")
		cmd.Println("\nRemediation Steps:")
		for _, r := range remediations {
			cmd.Printf("  • [%s]: %s\n", r.Name, r.Remediation)
		}
	}
}
