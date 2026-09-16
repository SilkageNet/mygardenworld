package policycfg

import (
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"google.golang.org/protobuf/proto"
)

func TestCustomerRewardFilterRoundTrip(t *testing.T) {
	for _, exact := range []*int64{nil, proto.Int64(0), proto.Int64(3)} {
		p := automation.DefaultPolicy()
		p.Order.Customer.ExactFloralCoin = exact
		raw, err := ToJSON(p)
		if err != nil {
			t.Fatal(err)
		}
		got, err := FromJSON(raw)
		if err != nil {
			t.Fatal(err)
		}
		value := got.GetOrder().GetCustomer().ExactFloralCoin
		if (value == nil) != (exact == nil) || (value != nil && *value != *exact) {
			t.Fatalf("presence/value lost: %s", raw)
		}
	}
}
