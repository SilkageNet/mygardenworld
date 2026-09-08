package runner

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrManualResumeRequired = errors.New("维护已结束，请先手动连接账号或由运维显式恢复已启用账号")

func (m *Manager) BackgroundStartsAllowed() bool {
	if m.MaintenanceStatus().Enabled {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return !m.manualResumeRequired
}

type MaintenanceStatus struct {
	Enabled  bool
	Draining bool
}

func (m *Manager) MaintenanceStatus() MaintenanceStatus {
	blocked, pending := m.gameGate.status()
	return MaintenanceStatus{Enabled: blocked, Draining: blocked && (pending > 0 || m.maintenanceDraining.Load())}
}

// ApplyMaintenance is used by the operator command consumer, never an
// administrative game-account API. Acknowledgement follows cancellation and
// drain; errors leave the admission gate closed and the command pending.
func (m *Manager) ApplyMaintenance(ctx context.Context) error {
	m.maintenanceMu.Lock()
	defer m.maintenanceMu.Unlock()
	if m.maintenanceInitErr != nil {
		return m.maintenanceInitErr
	}
	s, err := m.db.Maintenance(ctx)
	if err != nil {
		m.gameGate.block() // Fail closed if the durable command cannot be read.
		m.stopMaintenanceRunners()
		return err
	}
	if s.Enabled {
		m.maintenanceDraining.Store(true)
		m.mu.Lock()
		m.manualResumeRequired = true
		m.mu.Unlock()
		m.gameGate.block()
		m.stopMaintenanceRunners()
		if err := m.gameGate.wait(ctx); err != nil {
			return err
		}
		// A start admitted before block may have finished registering while we
		// drained. No new start can be admitted after the gate was closed.
		m.stopMaintenanceRunners()
		m.maintenanceDraining.Store(false)
	} else {
		if m.MaintenanceStatus().Enabled {
			if err := m.gameGate.wait(ctx); err != nil {
				return err
			}
			m.stopMaintenanceRunners()
			m.maintenanceDraining.Store(false)
		}
		if s.Revision != s.AppliedRevision && s.ResumeEnabled {
			m.mu.Lock()
			m.manualResumeRequired = false
			m.mu.Unlock()
		}
		m.gameGate.open()
	}
	if s.Revision == s.AppliedRevision {
		return nil
	}
	acked, err := m.db.AcknowledgeMaintenance(ctx, s.Revision)
	if err != nil {
		return err
	}
	if acked {
		m.log.Info("maintenance command applied", "enabled", s.Enabled, "revision", s.Revision)
		if !s.Enabled && s.ResumeEnabled {
			// Starts are idempotent and still respect active owners, their current
			// automation policy, account protection and a subsequent maintenance.
			m.maintenanceRestores.Add(1)
			go func() {
				defer m.maintenanceRestores.Done()
				report := m.RestoreEnabledRunners(ctx)
				m.log.Info("maintenance restore finished", "eligible", report.Eligible, "started", report.Started, "failed", report.Failed, "skipped", report.Skipped)
			}()
		}
	}
	return nil
}

func (m *Manager) stopMaintenanceRunners() {
	for _, r := range m.All() {
		r.Stop()
	}
}

// RunMaintenance consumes durable operator requests while Web remains alive.
// Polling carries only one singleton row, no game snapshots or user policies.
func (m *Manager) RunMaintenance(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var lastError string
	for {
		if err := m.ApplyMaintenance(ctx); err != nil && ctx.Err() == nil {
			message := fmt.Sprint(err)
			if message != lastError {
				m.log.Error("apply maintenance", "err", err)
			}
			lastError = message
		} else {
			lastError = ""
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
