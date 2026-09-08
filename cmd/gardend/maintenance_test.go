package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestMaintenanceCLIQueuesOfflineWithoutClaimingCompletion(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(context.Background(), filepath.Join(dir, "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	cmd := newMaintenanceCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"on", "--data-dir", dir, "--wait", "0"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "not yet confirmed") {
		t.Fatal(out.String())
	}
	s, err := db.Maintenance(context.Background())
	if err != nil || !s.Enabled || s.AppliedRevision == s.Revision {
		t.Fatalf("unexpected command %+v %v", s, err)
	}
}

func TestMaintenanceCLIRejectsInvalidOptionsBeforeOpeningDatabase(t *testing.T) {
	for _, args := range [][]string{{"on", "--resume-enabled"}, {"on", "--wait", "-1s"}, {"unknown"}} {
		cmd := newMaintenanceCmd()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestMaintenanceCLIWaitsForDaemonConsumer(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(context.Background(), filepath.Join(dir, "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	m := runner.NewManager(db, runner.NewBus(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); m.RunMaintenance(ctx) }()
	defer func() { cancel(); <-done; m.Shutdown() }()
	for _, action := range []string{"on", "off"} {
		cmd := newMaintenanceCmd()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetArgs([]string{action, "--data-dir", dir, "--wait", "3s"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), "confirmed by daemon") {
			t.Fatal(output.String())
		}
		if m.MaintenanceStatus().Enabled != (action == "on") {
			t.Fatal("ack does not match gate")
		}
	}
	if m.BackgroundStartsAllowed() {
		t.Fatal("off implicitly resumed background starts")
	}
}
