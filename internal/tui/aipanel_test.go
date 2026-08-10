package tui

import (
	"context"
	"strings"
	"testing"
)

func TestAIPanelToggleModes(t *testing.T) {
	p := newAIPanel()
	if p.mode != modeAsk {
		t.Fatalf("default mode = %v", p.mode)
	}
	p.toggleMode()
	if p.mode != modeQuick {
		t.Fatalf("after toggle = %v", p.mode)
	}
}

func TestAIPanelConfirmApprove(t *testing.T) {
	p := newAIPanel()
	p.pendingConfirm = "wipe"
	p.confirm(true) // approve
	if p.pendingConfirm != "" {
		t.Fatalf("pending = %q after approve", p.pendingConfirm)
	}
}

func TestAIPanelEscCancels(t *testing.T) {
	p := newAIPanel()
	// simulate a running loop: runAsk wires cancel; here we set it directly
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.running = true
	_ = ctx
	if !p.interrupt() {
		t.Fatal("interrupt should stop running loop")
	}
	if p.running {
		t.Fatal("still running")
	}
}

func TestAIPanelModeLabels(t *testing.T) {
	p := newAIPanel()
	if !strings.Contains(p.view(), "Ask AI") {
		t.Fatalf("ask mode label missing: %q", p.view())
	}
	p.toggleMode()
	if !strings.Contains(p.view(), "NL→SQL") {
		t.Fatalf("quick mode label missing: %q", p.view())
	}
}

func TestAIPanelConfirmModal(t *testing.T) {
	p := newAIPanel()
	p.pendingConfirm = "run_write"
	if !strings.Contains(p.view(), "run_write") {
		t.Fatalf("confirm modal missing tool name: %q", p.view())
	}
}

func TestAIPanelTranscriptClipped(t *testing.T) {
	p := newAIPanel()
	for i := 0; i < 20; i++ {
		p.events = append(p.events, aiEventMsg{Tool: "run_query"})
	}
	p.SetLines(5)
	out := p.view()
	if n := lineCount(out); n > 5 {
		t.Fatalf("transcript overflowed: %d lines > 5", n)
	}
}
