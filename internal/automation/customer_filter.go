package automation

import (
	"fmt"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// CustomerOrderRewardSkipReason is shared by demand planning, operations and
// read models. This explicit monetary preference is never bypassed by race work
// and never authorizes rejecting an order to generate a replacement.
func CustomerOrderRewardSkipReason(order *state.CustomerOrder, policy *pb.CustomerOrderPolicy) string {
	if policy == nil || policy.ExactFloralCoin == nil {
		return ""
	}
	if *policy.ExactFloralCoin < 0 {
		return "花坊币筛选值无效，保留订单"
	}
	coins, known := state.CustomerOrderFloralCoinReward(order)
	if !known {
		return "花坊币奖励尚未确认，保留订单"
	}
	if coins != *policy.ExactFloralCoin {
		return fmt.Sprintf("花坊币奖励 %d 不等于指定值 %d，保留订单", coins, *policy.ExactFloralCoin)
	}
	return ""
}
