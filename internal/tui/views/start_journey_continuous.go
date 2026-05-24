package views

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/megalypse/go/rmqiw/internal/cfg"
	poller2 "github.com/megalypse/go/rmqiw/internal/domain/impl/poller"
	"github.com/megalypse/go/rmqiw/internal/domain/impl/publisher"
	"github.com/megalypse/go/rmqiw/internal/tui/ui"
)

func NewStartJourneyContinuous(selectedFlow int) tea.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	stepChan := make(chan stepReport, 1)

	return &StartJourneyContinuous{
		selectedFlow: selectedFlow,
		spinner:      s,
		ctx:          context.Background(),
		stepChan:     stepChan,
	}
}

type stepReport struct {
	stepNum int
	err     error
}

type StartJourneyContinuous struct {
	selectedFlow int
	spinner      spinner.Model
	ctx          context.Context
	stepChan     chan stepReport
	currentStep  int
	err          error
}

func (s *StartJourneyContinuous) Init() tea.Cmd {
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
		}
	}

	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

func (s *StartJourneyContinuous) View() string {
	render := strings.Builder{}
	flows, _ := cfg.GetFlows()
	flow := flows[s.selectedFlow]

	for i, step := range flow.Steps {
		if s.err != nil {
			render.WriteString("✗ " + step.Name + ui.LineBreak)
			render.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(s.err.Error()) + ui.LineBreak)
			break
		}

		if i < s.currentStep {
			render.WriteString("✓ " + step.Name + ui.LineBreak)
		}

		if i == s.currentStep {
			render.WriteString(s.spinner.View() + " " + step.Name + ui.LineBreak)
		}

		if i > s.currentStep {
			render.WriteString("  " + step.Name + ui.LineBreak)
		}
	}

	return render.String()
}

func waitForStepReport(reportChan <-chan stepReport) tea.Cmd {
	return func() tea.Msg {
		report, ok := <-reportChan
		if !ok {
			return tea.Quit()
		}

		return report
	}
}

func (s *StartJourneyContinuous) runSteps(ctx context.Context, reportChan chan<- stepReport) {
	defer close(reportChan)

	flows, _ := cfg.GetFlows()
	flow := flows[s.selectedFlow]

	rmq, err := publisher.GetRmq()
	if err != nil {
		reportChan <- stepReport{err: err}
		return
	}

	poller, err := poller2.GetPsql(ctx)
	if err != nil {
		reportChan <- stepReport{err: err}
		return
	}

	for i, step := range flow.Steps {
		if err := rmq.Publish(
			ctx,
			step.Message.Exchange,
			step.Message.RoutingKey,
			step.Message.Headers,
			[]byte(step.Message.Body),
		); err != nil {
			reportChan <- stepReport{stepNum: i, err: err}
			return
		}

		ticker := time.NewTicker(step.PollInterval)
		timeout := time.NewTimer(10 * time.Second)
		stepDone := false

		for !stepDone {
			select {
			case <-timeout.C:
				reportChan <- stepReport{stepNum: i, err: context.DeadlineExceeded}
				ticker.Stop()
				return
			case <-ctx.Done():
				ticker.Stop()
				timeout.Stop()
				return
			case _ = <-ticker.C:
				ok, err := poller.Poll(ctx, step.PollQuery)
				if err != nil {
					ticker.Stop()
					timeout.Stop()
					reportChan <- stepReport{stepNum: i, err: err}
					return
				}

				if ok {
					reportChan <- stepReport{stepNum: i, err: nil}
					ticker.Stop()
					timeout.Stop()
					stepDone = true
				}
			}
		}

	}
}
