package clientproto

import (
	"encoding/json"
	"testing"
)

func TestFreeWaterRecvRequestIncludesZeroIndex(t *testing.T) {
	raw, err := json.Marshal(FreeWaterRecvRequest{Idx: 0})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(raw), `{"idx":0}`; got != want {
		t.Fatalf("FreeWaterRecvRequest JSON = %s, want %s", got, want)
	}
}

func TestOrderCustomerGenOrderRequestIncludesEmptyGuestList(t *testing.T) {
	raw, err := json.Marshal(OrderCustomerGenOrderRequest{GuestNpcIdList: RPCIDList{}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(raw), `{"guestNpcIdList":[]}`; got != want {
		t.Fatalf("OrderCustomerGenOrderRequest JSON = %s, want %s", got, want)
	}
}
