package store

import "context"

// Maintenance is an operator-owned durable command/acknowledgement. A new
// revision must be acknowledged by the daemon; writing enabled=true alone
// must never be reported as completion of draining game I/O.
type Maintenance struct {
	Enabled         bool
	Revision        int64
	AppliedRevision int64
	ResumeEnabled   bool
}

func (d *DB) Maintenance(ctx context.Context) (Maintenance, error) {
	var s Maintenance
	err := d.QueryRowContext(ctx, `SELECT enabled, revision, applied_revision, resume_enabled FROM daemon_maintenance WHERE id=1`).Scan(&s.Enabled, &s.Revision, &s.AppliedRevision, &s.ResumeEnabled)
	return s, err
}

func (d *DB) RequestMaintenance(ctx context.Context, enabled, resume bool) (Maintenance, error) {
	var s Maintenance
	err := d.QueryRowContext(ctx, `UPDATE daemon_maintenance SET enabled=?, resume_enabled=?, revision=revision+1 WHERE id=1 RETURNING enabled, revision, applied_revision, resume_enabled`, enabled, !enabled && resume).Scan(&s.Enabled, &s.Revision, &s.AppliedRevision, &s.ResumeEnabled)
	return s, err
}

func (d *DB) AcknowledgeMaintenance(ctx context.Context, revision int64) (bool, error) {
	result, err := d.ExecContext(ctx, `UPDATE daemon_maintenance SET applied_revision=? WHERE id=1 AND revision=?`, revision, revision)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
