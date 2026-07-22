package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/megalypse/go/rmqiw/internal/cfg"
	"github.com/megalypse/go/rmqiw/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "",
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
	_, err := cfg.GetFlows()
	if err != nil {
		return fmt.Errorf("failed to load flows: %w", err)
	}

	_, err = cfg.GetCfg()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	return nil
}
