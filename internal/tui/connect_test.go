package tui

import (
	"testing"

	"github.com/claymor333/squal/internal/config"
)

func TestConnectModalFields(t *testing.T) {
	cv := newConnectView()
	cv.setField(hostField, "localhost")
	cv.setField(userField, "root")
	if got := cv.value(hostField); got != "localhost" {
		t.Fatalf("host = %q", got)
	}
	prof, ok := cv.buildProfile("dev")
	if !ok {
		t.Fatal("buildProfile failed")
	}
	if prof.Host != "localhost" || prof.User != "root" {
		t.Fatalf("profile = %+v", prof)
	}
}

func TestConfigAddRemoveProfile(t *testing.T) {
	cfg := &config.Config{}
	cfg.AddProfile(config.Profile{Name: "x", Host: "h"})
	if len(cfg.Profiles) != 1 {
		t.Fatalf("profiles = %d", len(cfg.Profiles))
	}
	cfg.RemoveProfile("x")
	if len(cfg.Profiles) != 0 {
		t.Fatalf("profiles after remove = %d", len(cfg.Profiles))
	}
}
