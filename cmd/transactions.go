package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/internal/transaction"
	"github.com/julienbreux/agy-sync/pkg/config"
)

type transactionsOptions struct {
	direction      string
	inFlag         bool
	outFlag        bool
	entityType     string
	convFlag       bool
	artifactFlag   bool
	brainFlag      bool
	conversationID string
	limit          int
	offset         int
	dbPath         string
}

func newTransactionsCommand() *cobra.Command {
	opts := transactionsOptions{}

	cmd := &cobra.Command{
		Use:     "transactions",
		Aliases: []string{"tx", "txs"},
		GroupID: "sync",
		Short:   "Display sync transactions log",
		Long: `Displays audit log of synchronization transactions between local Antigravity brain and Firestore.
Allows filtering by direction (import/export), entity type (conversation, artifact, brain), and conversation ID.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve direction
			var dir transaction.Direction
			dirCount := 0
			if opts.inFlag {
				dir = transaction.DirectionIn
				dirCount++
			}
			if opts.outFlag {
				dir = transaction.DirectionOut
				dirCount++
			}
			if opts.direction != "" {
				d := strings.ToLower(opts.direction)
				switch d {
				case "in", "import":
					dir = transaction.DirectionIn
					dirCount++
				case "out", "export":
					dir = transaction.DirectionOut
					dirCount++
				default:
					return fmt.Errorf("invalid direction %q: must be 'in' or 'out'", opts.direction)
				}
			}
			if dirCount > 1 && (opts.inFlag && opts.outFlag) {
				return errors.New("cannot specify both --in and --out")
			}

			// Resolve entity type
			var entType transaction.EntityType
			typeCount := 0
			if opts.convFlag {
				entType = transaction.EntityTypeConv
				typeCount++
			}
			if opts.artifactFlag {
				entType = transaction.EntityTypeArtifact
				typeCount++
			}
			if opts.brainFlag {
				entType = transaction.EntityTypeBrain
				typeCount++
			}
			if opts.entityType != "" {
				t := strings.ToLower(opts.entityType)
				switch t {
				case "conv", "conversation":
					entType = transaction.EntityTypeConv
					typeCount++
				case "artifact", "artifacts":
					entType = transaction.EntityTypeArtifact
					typeCount++
				case "brain":
					entType = transaction.EntityTypeBrain
					typeCount++
				default:
					return fmt.Errorf("invalid entity type %q: must be 'conv', 'artifact', or 'brain'", opts.entityType)
				}
			}
			if typeCount > 1 {
				return errors.New("cannot specify multiple entity types (--conv, --artifact, --brain)")
			}

			// Resolve DB path
			dbPath := opts.dbPath
			if dbPath == "" {
				cfg, err := config.LoadConfig(globalOpts.ConfigFile)
				if err == nil && cfg != nil && cfg.TransactionsDB != "" {
					dbPath = cfg.TransactionsDB
				} else {
					dbPath = config.DefaultTransactionsDB()
				}
			}

			store, err := transaction.NewStore(dbPath)
			if err != nil {
				return fmt.Errorf("failed opening transactions database: %w", err)
			}
			defer func() { _ = store.Close() }()

			filter := transaction.Filter{
				Direction:      dir,
				EntityType:     entType,
				ConversationID: opts.conversationID,
				Limit:          opts.limit,
				Offset:         opts.offset,
			}

			txs, err := store.Query(cmd.Context(), filter)
			if err != nil {
				return fmt.Errorf("failed querying transactions: %w", err)
			}

			isJSON := globalOpts.JSON
			if !isJSON {
				if f := cmd.Flags().Lookup("json"); f != nil && f.Changed {
					isJSON, _ = cmd.Flags().GetBool("json")
				}
			}

			if isJSON {
				if txs == nil {
					txs = []transaction.Transaction{}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(txs)
			}

			if len(txs) == 0 {
				cmd.Println("No transactions found.")
				return nil
			}

			cmd.Printf("%-19s  %-8s  %-10s  %-36s  %-20s  %s\n", "TIMESTAMP", "ACTION", "TYPE", "CONVERSATION ID", "ENTITY", "DETAILS")
			cmd.Println(strings.Repeat("-", 110))

			for _, tx := range txs {
				timeStr := tx.Timestamp.Format("2006-01-02 15:04:05")
				cmd.Printf("%-19s  %-8s  %-10s  %-36s  %-20s  %s\n",
					timeStr,
					tx.Direction.Display(),
					tx.EntityType,
					tx.ConversationID,
					tx.EntityID,
					tx.Details,
				)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&opts.direction, "direction", "", "Filter by direction: 'in' (IMPORT) or 'out' (EXPORT)")
	cmd.Flags().BoolVar(&opts.inFlag, "in", false, "Filter to inbound (IMPORT) transactions")
	cmd.Flags().BoolVar(&opts.outFlag, "out", false, "Filter to outbound (EXPORT) transactions")
	cmd.Flags().StringVar(&opts.entityType, "type", "", "Filter by entity type: 'conv', 'artifact', or 'brain'")
	cmd.Flags().BoolVar(&opts.convFlag, "conv", false, "Filter to conversation transactions")
	cmd.Flags().BoolVar(&opts.artifactFlag, "artifact", false, "Filter to artifact transactions")
	cmd.Flags().BoolVar(&opts.brainFlag, "brain", false, "Filter to brain transactions")
	cmd.Flags().StringVarP(&opts.conversationID, "conversation", "c", "", "Filter by conversation ID")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 50, "Maximum number of transactions to display")
	cmd.Flags().IntVar(&opts.offset, "offset", 0, "Number of transactions to skip")
	cmd.Flags().StringVar(&opts.dbPath, "db", "", "Path to transactions database")

	return cmd
}
