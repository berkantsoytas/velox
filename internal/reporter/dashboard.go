package reporter

import (
	"fmt"
	"strings"
	"time"

	"github.com/berkantsoytas/velox/internal/metrics"
	"github.com/berkantsoytas/velox/internal/requester"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type (
	tickMsg   struct{ Time time.Time }
	resultMsg requester.Result
	doneMsg   struct{}
)

var (
	colorSuccess = lipgloss.Color("42")
	colorWarning = lipgloss.Color("214")
	colorError   = lipgloss.Color("196")
	colorInfo    = lipgloss.Color("87")
	colorTitle   = lipgloss.Color("205")
	colorBorder  = lipgloss.Color("60")
	colorMuted   = lipgloss.Color("240")
	colorID      = lipgloss.Color("245")
)

type model struct {
	resultsChan <-chan requester.Result
	stats       *metrics.Report
	logs        []string
	startTime   time.Time
	targetReq   int
	width       int
	height      int
	done        bool
	liveMin     time.Duration
	liveMax     time.Duration
	liveTotal   time.Duration
	restart     bool
}

func RunDashboard(results <-chan requester.Result, totalRequests int) (bool, error) {
	m := model{
		resultsChan: results,
		stats: &metrics.Report{
			StatusCodes: make(map[int]int),
			Latencies:   make([]time.Duration, 0, totalRequests),
		},
		logs:      make([]string, 0),
		startTime: time.Now(),
		targetReq: totalRequests,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}

	return finalModel.(model).restart, nil
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), waitForResult(m.resultsChan))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r", "R":
			m.restart = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case resultMsg:
		m.processResult(requester.Result(msg))
		return m, waitForResult(m.resultsChan)
	case doneMsg:
		m.done = true
		m.stats.Finalize()
		return m, nil
	case tickMsg:
		if m.done {
			return m, nil
		}
		return m, tick()
	}
	return m, nil
}

func (m model) View() string {
	if m.width < 70 || m.height < 15 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, "Terminal size too small.")
	}

	halfWidth := (m.width / 2) - 2
	boxHeight := m.height - 4

	baseBox := lipgloss.NewStyle().
		Width(halfWidth).Height(boxHeight).
		Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder)

	logTitle := lipgloss.NewStyle().Bold(true).Foreground(colorInfo).Padding(0, 1).Render("⚡ LIVE TRAFFIC")
	logContent := strings.Join(m.logs, "\n")
	leftBox := baseBox.Copy().Render(logTitle + "\n\n" + logContent)

	statsTitle := lipgloss.NewStyle().Bold(true).Foreground(colorTitle).Padding(0, 1).Render("📊 VELOX METRICS")
	statsContent := m.renderRightPanel(halfWidth - 4)
	rightBox := baseBox.Copy().Render(statsTitle + "\n\n" + statsContent)

	header := lipgloss.NewStyle().
		Width(m.width).Align(lipgloss.Center).Bold(true).
		Background(lipgloss.Color("62")).Foreground(lipgloss.Color("255")).
		Render(" VELOX LOAD TESTER ")

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, "  ", rightBox)
	footer := lipgloss.NewStyle().Foreground(colorMuted).Render("\n  [R] Restart   │   [Q] Quit")

	return header + "\n" + body + footer
}

func (m *model) processResult(res requester.Result) {
	if m.liveMin == 0 || res.Duration < m.liveMin {
		m.liveMin = res.Duration
	}
	if res.Duration > m.liveMax {
		m.liveMax = res.Duration
	}
	m.liveTotal += res.Duration

	durColor := colorSuccess
	if res.Duration > 300*time.Millisecond {
		durColor = colorError
	} else if res.Duration > 100*time.Millisecond {
		durColor = colorWarning
	}

	durStr := lipgloss.NewStyle().Foreground(durColor).Render(fmt.Sprintf("%10s", res.Duration.Round(10*time.Microsecond)))

	icon := "✓"
	statusColor := colorSuccess
	if res.StatusCode >= 400 && res.StatusCode < 500 {
		icon = "⚠"
		statusColor = colorWarning
	} else if res.StatusCode >= 500 || res.Error != nil {
		icon = "✗"
		statusColor = colorError
	}

	idStr := lipgloss.NewStyle().Foreground(colorID).Render(fmt.Sprintf("[#%04d]", m.stats.TotalRequests+1))
	statusStr := lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(fmt.Sprintf("%s %d", icon, res.StatusCode))
	sizeStr := lipgloss.NewStyle().Foreground(colorMuted).Render(fmt.Sprintf("%6.2f KB", float64(res.BytesRead)/1024.0))
	sep := lipgloss.NewStyle().Foreground(colorBorder).Render("│")

	logLine := fmt.Sprintf("  %s  %s  %s  %s  %s  %s  %s", idStr, sep, statusStr, sep, durStr, sep, sizeStr)
	if res.Error != nil {
		logLine = fmt.Sprintf("  %s  %s  %s", idStr, sep, lipgloss.NewStyle().Foreground(colorError).Render(res.Error.Error()))
	}

	m.logs = append(m.logs, logLine)
	maxLogs := m.height - 8
	if maxLogs < 5 {
		maxLogs = 5
	}
	if len(m.logs) > maxLogs {
		m.logs = m.logs[len(m.logs)-maxLogs:]
	}

	m.stats.Add(res)
}

func (m *model) renderRightPanel(width int) string {
	if m.stats.TotalRequests == 0 {
		return "  Waiting for traffic..."
	}

	elapsed := time.Since(m.startTime).Seconds()
	rps := float64(m.stats.TotalRequests) / elapsed

	percent := float64(m.stats.TotalRequests) / float64(m.targetReq)
	barWidth := width - 30
	if barWidth < 5 {
		barWidth = 5
	}
	filled := int(percent * float64(barWidth))
	empty := barWidth - filled
	progressBar := fmt.Sprintf("[%s%s] %.1f%%", strings.Repeat("█", filled), strings.Repeat("░", empty), percent*100)

	statusLabel := lipgloss.NewStyle().Foreground(colorWarning).Render("► RUNNING")
	if m.done {
		statusLabel = lipgloss.NewStyle().Foreground(colorSuccess).Render("■ FINISHED")
	}

	overview := fmt.Sprintf(`  %s
  
  Progress    %s
  Requests    %d / %d
  Throughput  %s
  Data Xfer   %.2f MB`,
		statusLabel, lipgloss.NewStyle().Foreground(colorInfo).Render(progressBar),
		m.stats.TotalRequests, m.targetReq,
		lipgloss.NewStyle().Foreground(colorSuccess).Bold(true).Render(fmt.Sprintf("%.2f req/s", rps)),
		float64(m.stats.TotalData)/(1024*1024))

	liveAvg := (m.liveTotal / time.Duration(m.stats.TotalRequests)).Round(time.Microsecond)
	sparkline := generateFixedSparkline(m.stats.Latencies, m.targetReq, width-4)

	latenciesBox := ""
	if m.done && len(m.stats.Latencies) > 0 {
		latenciesBox = fmt.Sprintf(`
  [ Latency Grid ]
  MIN: %-12v │ P50: %-12v │ P90: %v
  MAX: %-12v │ P95: %-12v │ P99: %v`,
			m.stats.Min.Round(time.Microsecond), m.stats.P50.Round(time.Microsecond), m.stats.P90.Round(time.Microsecond),
			m.stats.Max.Round(time.Microsecond), m.stats.P95.Round(time.Microsecond), m.stats.P99.Round(time.Microsecond))
	} else {
		latenciesBox = fmt.Sprintf(`
  [ Live Latencies ]
  MIN: %-12v │ AVG: %-12v │ MAX: %v
  
  Trend (Fixed Timeline):
  %s`, m.liveMin.Round(time.Microsecond), liveAvg, m.liveMax.Round(time.Microsecond),
			lipgloss.NewStyle().Foreground(colorTitle).Render(sparkline))
	}

	var sb strings.Builder
	sb.WriteString("\n\n  [ Status Codes ]\n")
	for code, count := range m.stats.StatusCodes {
		c := colorSuccess
		if code >= 400 {
			c = colorWarning
		}
		if code >= 500 {
			c = colorError
		}
		coloredCode := lipgloss.NewStyle().Foreground(c).Bold(true).Render(fmt.Sprintf("  %d", code))

		cPercent := float64(count) / float64(m.stats.TotalRequests)
		cBarLen := int(cPercent * 20)
		if cBarLen < 1 && count > 0 {
			cBarLen = 1
		}
		cBar := lipgloss.NewStyle().Foreground(c).Render(strings.Repeat("■", cBarLen))

		sb.WriteString(fmt.Sprintf("%s : %-6d %s\n", coloredCode, count, cBar))
	}

	return overview + "\n" + latenciesBox + sb.String()
}

func generateFixedSparkline(latencies []time.Duration, target, width int) string {
	if width <= 0 || len(latencies) == 0 {
		return ""
	}
	buckets := make([]time.Duration, width)
	counts := make([]int, width)

	for i, l := range latencies {
		idx := int((float64(i) / float64(target)) * float64(width))
		if idx >= width {
			idx = width - 1
		}
		buckets[idx] += l
		counts[idx]++
	}

	var maxAvg time.Duration
	avgs := make([]time.Duration, width)
	for i := range buckets {
		if counts[i] > 0 {
			avg := buckets[i] / time.Duration(counts[i])
			avgs[i] = avg
			if avg > maxAvg {
				maxAvg = avg
			}
		}
	}

	if maxAvg == 0 {
		maxAvg = 1
	}

	bars := []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var sb strings.Builder
	sb.WriteString("  ")
	for _, avg := range avgs {
		if avg == 0 {
			sb.WriteRune(' ')
		} else {
			idx := int((float64(avg) / float64(maxAvg)) * float64(len(bars)-1))
			sb.WriteRune(bars[idx])
		}
	}
	return sb.String()
}

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg { return tickMsg{Time: t} })
}

func waitForResult(c <-chan requester.Result) tea.Cmd {
	return func() tea.Msg {
		res, ok := <-c
		if !ok {
			return doneMsg{}
		}
		return resultMsg(res)
	}
}
