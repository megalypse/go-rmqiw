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
	"github.com/megalypse/go/rmqiw/internal/domain/models"
	"github.com/megalypse/go/rmqiw/internal/tui/colors"
	"github.com/megalypse/go/rmqiw/internal/tui/ui"
)

func NewStartJourneyContinuous(selectedFlow int) tea.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	stepChan := make(chan stepReport, 1)
	flow, profile, err := resolveSelectedFlow(selectedFlow)

	return &StartJourneyContinuous{
		selectedFlow: selectedFlow,
		flow:         flow,
		profile:      profile,
		spinner:      s,
		ctx:          context.Background(),
		stepChan:     stepChan,
		allDone:      err != nil,
		err:          err,
	}
}

type stepReport struct {
	stepNum int
	err     error
}

type StartJourneyContinuous struct {
	selectedFlow int
	flow         *models.Flow
	profile      *cfg.Config
	spinner      spinner.Model
	ctx          context.Context
	stepChan     chan stepReport
	currentStep  int
	allDone      bool
	err          error
}

func (s *StartJourneyContinuous) Init() tea.Cmd {
	if s.err != nil {
		return nil
	}

	go s.runSteps(s.ctx, s.stepChan)

	return tea.Batch(s.spinner.Tick, waitForStepReport(s.stepChan))
}

func (s *StartJourneyContinuous) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case stepReport:
		if msg.err != nil {
			s.err = msg.err
			return s, nil
		}

		s.currentStep = msg.stepNum + 1
		return s, waitForStepReport(s.stepChan)
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return s, tea.Quit
		default:
			if s.allDone {
				return NewViewSelectJourney(), nil
			}
		}
	}

	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

func (s *StartJourneyContinuous) View() string {
	render := strings.Builder{}

	spinnerIcon := lipgloss.NewStyle().Foreground(colors.MainColor).Render(s.spinner.View())
	successIcon := lipgloss.NewStyle().Foreground(colors.SuccessGreen).Render("✓")
	errorIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render("✗")

	flows, _ := cfg.GetFlows()
	flow := flows[s.selectedFlow]
	if s.flow != nil {
		flow = s.flow
	}

	if s.currentStep == len(flow.Steps) {
		render.WriteString(successIcon + " Journey completed!")
	} else if s.err != nil {
		render.WriteString(errorIcon + " Journey failed!")
	} else {
		render.WriteString(spinnerIcon + " Journey in progress...")
	}

	render.WriteString(ui.LineSkip)

	for i, step := range flow.Steps {
		if s.err != nil {
			render.WriteString(errorIcon + " " + s.err.Error() + ui.LineBreak)
			break
		}

		if i < s.currentStep {
			render.WriteString(successIcon + " " + step.Name + ui.LineBreak)
		}

		if i == s.currentStep {
			render.WriteString(spinnerIcon + " " + step.Name + ui.LineBreak)
		}

		if i > s.currentStep {
			render.WriteString("○" + " " + step.Name + ui.LineBreak)
		}
	}

	if s.allDone {
		render.WriteString(ui.LineSkip + "Press any key to go back to the main menu.")
	}

	return render.String()
}

func waitForStepReport(reportChan <-chan stepReport) tea.Cmd {
	return func() tea.Msg {
		report, ok := <-reportChan
		if !ok {
			return nil
		}

		return report
	}
}

func (s *StartJourneyContinuous) runSteps(ctx context.Context, reportChan chan<- stepReport) {
	defer func() { s.allDone = true }()
	defer close(reportChan)

	flows, _ := cfg.GetFlows()
	flow := flows[s.selectedFlow]
	if s.flow != nil {
		flow = s.flow
	}

	rmq, err := publisher.GetRmqForConfig(s.profile)
	if err != nil {
		reportChan <- stepReport{err: err}
		return
	}

	poller, err := poller2.GetPsqlForConfig(ctx, s.profile)
	if err != nil {
		reportChan <- stepReport{err: err}
		return
	}

	for i, step := range flow.Steps {
		if err := runJourneyStep(ctx, step, rmq, poller); err != nil {
			reportChan <- stepReport{stepNum: i, err: err}
			return
		}

		reportChan <- stepReport{stepNum: i, err: nil}

	}
}
