package tui

import (
	"context"
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
