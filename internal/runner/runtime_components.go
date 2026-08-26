package runner

import (
	"context"
	"sync"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

// Runner is the account-level aggregate root. These embedded components own
// lifecycle-specific mutable state while the root lock still provides a
// coherent public snapshot during the staged architecture transition.

type sessionRuntimeState struct {
	client                   *babigame.Client
	httpc                    *babigame.HTTPClient
	session                  *babigame.Session
	debugWriter              *babigame.DebugFrameWriter
	cancel                   context.CancelFunc
	done                     chan struct{}
	stopOnce                 sync.Once
	sessionInvalidated       bool
	sessionInvalidatedReason string
	sessionAutoRelogin       bool
	dessertSessionEpoch      uint64
}

type schedulerState struct {
	lastWaterSyncTick         time.Time
	lastReputationSyncTick    time.Time
	lastResidentOrderSyncTick time.Time
	nextDecisionAt            time.Time
	harvestBlockedUntil       map[int32]time.Time
	operationCooldowns        map[string]operationCooldown
	sideLaneFirstWait         map[string]time.Time
	sideLaneFarmTurn          bool
}

type executionState struct {
	operationMu                  sync.Mutex
	currentOperation             string
	currentOperationStartedAt    time.Time
	lastOperation                string
	lastOperationAt              time.Time
	lastOperationError           string
	lastOperationErrorAt         time.Time
	rqst                         rqstState
	unknownRPCCounts             map[string]int32
	lastCustomerOrderInfo        map[int32]string
	lastResidentOrderLimitReason string
	lastCustomerOrderLimitReason string
}

type activityRuntimeState struct {
	dessertRound dessertRoundRuntime
}
