package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMaintenanceMigrationAndCommandRevisions(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.ExecContext(ctx, `DROP TABLE daemon_maintenance; PRAGMA user_version=11`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(ctx, db.writer); err != nil {
		t.Fatal(err)
	}
	s, err := db.Maintenance(ctx)
	if err != nil || s.Enabled || s.Revision != 0 {
		t.Fatalf("default %+v %v", s, err)
	}
	on, err := db.RequestMaintenance(ctx, true, true)
	if err != nil || !on.Enabled || on.ResumeEnabled || on.Revision != 1 || on.AppliedRevision != 0 {
		t.Fatalf("on %+v %v", on, err)
	}
	off, err := db.RequestMaintenance(ctx, false, true)
	if err != nil || off.Enabled || !off.ResumeEnabled || off.Revision != 2 {
		t.Fatalf("off %+v %v", off, err)
	}
	if ok, err := db.AcknowledgeMaintenance(ctx, on.Revision); err != nil || ok {
		t.Fatal("stale acknowledgement accepted", err)
	}
	if ok, err := db.AcknowledgeMaintenance(ctx, off.Revision); err != nil || !ok {
		t.Fatal(err)
	}
	if err := applyMigrations(ctx, db.writer); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()
	s, err = reopened.Maintenance(ctx)
	if err != nil || s.Revision != 2 || s.AppliedRevision != 2 || !s.ResumeEnabled {
		t.Fatalf("lost persisted state %+v %v", s, err)
	}
}
