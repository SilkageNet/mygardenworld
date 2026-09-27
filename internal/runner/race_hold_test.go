package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestRaceHeldMutationExpiresWhileQueued(t *testing.T) {
	for _, kind := range []string{clientproto.RPCUsrLandPlantBatch.String(), clientproto.RPCUsrLandWaterBatch.String(),
		clientproto.RPCUsrLandHarvest.String(), clientproto.RPCUsrLandSpeedUpBatch.String(),
		clientproto.RPCFmlRaceFinishTask.String(), clientproto.RPCFmlRaceGiveUpTask.String(), clientproto.RPCPearlPlaceHire.String()} {
		t.Run(kind, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := newOperationEventTestRunner()
				r.state.ApplyV(json.RawMessage(fmt.Sprintf(`{"25":{"111":{"0":42,"1":1},"110":{"42":{"7":{"0":99,"1":3036,"2":10,"5":%d}}}}}`, time.Now().Add(time.Second).UnixMilli())))
				op := &automation.PlannedOp{Kind: kind, RaceHoldTaskMsID: 99, RaceBatchID: 42}
				ctx := context.WithValue(t.Context(), raceHeldOperationKey{}, op)
				if err := r.validateRaceHeldBeforeSend(ctx, kind); err != nil {
					t.Fatal(err)
				}
				time.Sleep(time.Second)
				if err := r.validateRaceHeldBeforeSend(ctx, kind); err == nil {
					t.Fatal("expired work admitted")
				}
				if err := r.validateRaceHeldBeforeSend(ctx, clientproto.RPCFmlRaceGetTaskList.String()); err != nil {
					t.Fatal("confirmation read interrupted", err)
				}
			})
		})
	}
}
