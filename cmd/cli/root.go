package cli

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/megalypse/go/rmqiw/internal/cfg"
	poller2 "github.com/megalypse/go/rmqiw/internal/domain/impl/poller"
	"github.com/megalypse/go/rmqiw/internal/domain/impl/publisher"
	"github.com/megalypse/go/rmqiw/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rmq",
	Short: "Interactive RQM workflow runner",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := setup(); err != nil {
			return err
		}

		tuiModel, err := tui.NewModel()
		if err != nil {
			return err
		}

		p := tea.NewProgram(tuiModel)

		_, err = p.Run()
		return err
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func setup() error {
	ctx := context.Background()
	_, err := cfg.GetFlows()
	if err != nil {
		return fmt.Errorf("failed to load flows: %w", err)
	}

	_, err = cfg.GetCfg()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	_, err = poller2.GetPsql(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize database connection: %w", err)
	}

	_, err = publisher.GetRmq()
	if err != nil {
		return fmt.Errorf("failed to initialize RabbitMQ connection: %w", err)
	}

	return nil
}
