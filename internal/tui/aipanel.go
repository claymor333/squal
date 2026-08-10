package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/claymor333/squal/internal/ai"
)

type aiMode int

const (
	modeAsk aiMode = iota
	modeQuick
)

type aiEventMsg struct {
	Tool   string
	Result string
	Done   bool
	Answer string
	Err    error
}

type aiConfirmMsg struct {
	Tool string
	Args map[string]any
	OK   chan bool
}

type aiPanel struct {
	client           *ai.Client
	agent            *ai.Agent
	registry         *ai.Registry
	session          *ai.Session
	mode             aiMode
	request          string
	running          bool
	cancel           context.CancelFunc
	pendingConfirm   string
	pendingConfirmCh chan bool // answered by model.handleKey y/n; nil when no confirm pending
	events           []aiEventMsg
	history          []string
}

func newAIPanel() *aiPanel {
	return &aiPanel{mode: modeAsk}
}

func (p *aiPanel) toggleMode() {
	if p.mode == modeAsk {
		p.mode = modeQuick
	} else {
		p.mode = modeAsk
	}
}

func (p *aiPanel) interrupt() bool {
	if p.running && p.cancel != nil {
		p.cancel()
		p.running = false
		return true
	}
	return false
}

// confirm resolves a pending write confirmation. ok=true approves the write.
// The channel send unblocks the agent's confirm wait in model.go.
func (p *aiPanel) confirm(ok bool) {
	if p.pendingConfirm != "" && p.pendingConfirmCh != nil {
		p.pendingConfirmCh <- ok
	}
	p.pendingConfirm = ""
	p.pendingConfirmCh = nil
}

// runAsk streams a tool-calling loop into the panel.
func (p *aiPanel) runAsk(ctx context.Context) tea.Cmd {
	p.running = true
	ctx, p.cancel = context.WithCancel(ctx)
	user := p.request
	return func() tea.Msg {
		answer, err := p.agent.Run(ctx, user)
		p.running = false
		if err != nil && ctx.Err() != nil {
			return aiEventMsg{Err: err}
		}
		return aiEventMsg{Answer: answer, Done: true, Err: err}
	}
}

// runQuick is the one-shot NL→SQL path (no tools).
func (p *aiPanel) runQuick(ctx context.Context) tea.Cmd {
	p.running = true
	return func() tea.Msg {
		defer func() { p.running = false }()
		sql, err := p.client.Complete(ctx, quickPrompt, p.request)
		if err != nil {
			return aiEventMsg{Err: err}
		}
		return runQueryMsg{SQL: sql}
	}
}

const quickPrompt = `You translate natural language into MariaDB/MySQL SQL. Return ONLY the SQL.`

func (p *aiPanel) view() string {
	var b strings.Builder
	mode := "[Ask]"
	if p.mode == modeQuick {
		mode = "[Quick]"
	}
	b.WriteString(styleAccent.Render(mode) + " " + p.request + "\n")
	for _, e := range p.events {
		switch {
		case e.Tool != "" && e.Result == "":
			b.WriteString(styleDim.Render("▸ "+e.Tool+"…") + "\n")
		case e.Tool != "":
			fmt.Fprintf(&b, "  %s → %s\n", styleDim.Render(e.Tool), e.Result)
		case e.Answer != "":
			b.WriteString(styleAccent.Render("✓ "+e.Answer) + "\n")
		case e.Err != nil:
			b.WriteString(styleErr.Render("✗ "+e.Err.Error()) + "\n")
		}
	}
	if p.pendingConfirm != "" {
		b.WriteString(styleErr.Render("⚠ confirm "+p.pendingConfirm+"? (y/n)") + "\n")
	}
	return b.String()
}
