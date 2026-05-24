package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/megalypse/go/rmqiw/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rmq",
	Short: "Interactive RQM workflow runner",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		fmt.Println(err)
		os.Exit(1)
	}

}
