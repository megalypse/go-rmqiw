package views

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/megalypse/go/rmqiw/internal/cfg"
	poller2 "github.com/megalypse/go/rmqiw/internal/domain/impl/poller"
	"github.com/megalypse/go/rmqiw/internal/domain/impl/publisher"
	"github.com/megalypse/go/rmqiw/internal/tui/colors"
	"github.com/megalypse/go/rmqiw/internal/tui/ui"
)

func NewStartJourneyStepByStep(selectedFlow int) tea.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	return &StartJourneyStepByStep{
		selectedFlow: selectedFlow,
		spinner:      s,
		ctx:          context.Background(),
		stepChan:     make(chan stepReport, 1),
	}
}

type StartJourneyStepByStep struct {
	selectedFlow int
	spinner      spinner.Model
	ctx          context.Context
	stepChan     chan stepReport
	currentStep  int
	running      bool
	err          error
}

func (s *StartJourneyStepByStep) Init() tea.Cmd {
	return s.spinner.Tick
}

func (s *StartJourneyStepByStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case stepReport:
		s.running = false
		if msg.err != nil {
			s.err = msg.err
			return s, nil
		}

		s.currentStep = msg.stepNum + 1
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return s, tea.Quit
		case "enter":
			if s.err != nil || s.done() {
				return NewViewSelectJourney(), nil
			}

			if s.running {
				return s, nil
			}

			s.running = true
			go s.runCurrentStep(s.ctx, s.stepChan)
			return s, waitForStepReport(s.stepChan)
		}
	}

	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

func (s *StartJourneyStepByStep) View() string {
	render := strings.Builder{}

	spinnerIcon := lipgloss.NewStyle().Foreground(colors.MainColor).Render(s.spinner.View())
	successIcon := lipgloss.NewStyle().Foreground(colors.SuccessGreen).Render("✓")
	errorIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render("✗")

	flows, _ := cfg.GetFlows()
	flow := flows[s.selectedFlow]

	if s.done() {
		render.WriteString(successIcon + " Journey completed!")
	} else if s.err != nil {
		render.WriteString(errorIcon + " Journey failed!")
	} else if s.running {
		render.WriteString(spinnerIcon + " Step in progress...")
	} else {
		render.WriteString("Paused. Press enter to run the next step.")
	}

	render.WriteString(ui.LineSkip)

	for i, step := range flow.Steps {
		if s.err != nil && i == s.currentStep {
			render.WriteString(errorIcon + " " + step.Name + ui.LineBreak)
			render.WriteString(errorIcon + " " + s.err.Error() + ui.LineBreak)
			break
		}

		if i < s.currentStep {
			render.WriteString(successIcon + " " + step.Name + ui.LineBreak)
		}

		if i == s.currentStep && s.running {
			render.WriteString(spinnerIcon + " " + step.Name + ui.LineBreak)
		}

		if i == s.currentStep && !s.running && s.err == nil {
			render.WriteString("○ " + step.Name + ui.LineBreak)
		}

		if i > s.currentStep {
			render.WriteString("○ " + step.Name + ui.LineBreak)
		}
	}

	if s.err != nil || s.done() {
		render.WriteString(ui.LineSkip + "Press enter to go back to the main menu.")
	}

	return render.String()
}

func (s *StartJourneyStepByStep) done() bool {
	flows, _ := cfg.GetFlows()
	flow := flows[s.selectedFlow]
	return s.currentStep == len(flow.Steps)
}

func (s *StartJourneyStepByStep) runCurrentStep(ctx context.Context, reportChan chan<- stepReport) {
	stepNum := s.currentStep

	flows, _ := cfg.GetFlows()
	flow := flows[s.selectedFlow]

	rmq, err := publisher.GetRmq()
	if err != nil {
		reportChan <- stepReport{stepNum: stepNum, err: err}
		return
	}

	poller, err := poller2.GetPsql(ctx)
	if err != nil {
		reportChan <- stepReport{stepNum: stepNum, err: err}
		return
	}

	err = runJourneyStep(ctx, flow.Steps[stepNum], rmq, poller)
	reportChan <- stepReport{stepNum: stepNum, err: err}
}
