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

func TestCelebrityGetAllTypesInfoRequestIsEmptyObject(t *testing.T) {
	raw, err := json.Marshal(CelebrityGetAllTypesInfoRequest{})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(raw), `{}`; got != want {
		t.Fatalf("CelebrityGetAllTypesInfoRequest JSON = %s, want %s", got, want)
	}
}

func TestZooInheritedRequestFieldsMarshal(t *testing.T) {
	tests := []struct {
		name string
		req  any
		want string
	}{
		{
			name: "add foodstuff",
			req:  ZooAddFoodstuffRequest{PetId: 7, FoodstuffIds: RPCIDList{1501, 1502}},
			want: `{"foodstuffIds":[1501,1502],"petId":7}`,
		},
		{
			name: "find pet",
			req:  ZooFindPetRequest{PetId: 7, IsShareVideo: 1},
			want: `{"isShareVideo":1,"petId":7}`,
		},
		{
			name: "refresh status",
			req:  ZooRefreshPetStatusRequest{PetIdList: RPCIDList{7, 8}},
			want: `{"petIdList":[7,8]}`,
		},
		{
			name: "read log",
			req:  ZooReadLogRequest{PetId: 7},
			want: `{"petId":7}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if got := string(raw); got != tt.want {
				t.Fatalf("request JSON = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestObservedNumericListRequestFieldsMarshal(t *testing.T) {
	tests := []struct {
		name string
		req  any
		want string
	}{
		{
			name: "opponent ext keys",
			req:  OpptGetDetailOpptsRequest{UIDs: RPCUIDList{9000000001}, ExtKeys: RPCIDList{1}},
			want: `{"uids":[9000000001],"extKeys":[1]}`,
		},
		{
			name: "souvenir indices",
			req:  ZooRecvSouvenirRwdRequest{IdxList: RPCIDList{1, 2}},
			want: `{"idxList":[1,2]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if got := string(raw); got != tt.want {
				t.Fatalf("request JSON = %s, want %s", got, tt.want)
			}
		})
	}
}
