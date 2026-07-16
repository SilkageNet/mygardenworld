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

func TestActRecvRequestIncludesZeroTaskIndex(t *testing.T) {
	raw, err := json.Marshal(ActRecvRequest{BatchId: 1312, TaskIdx: 0, TaskId: 1})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(raw), `{"batchId":1312,"taskIdx":0,"taskId":1}`; got != want {
		t.Fatalf("ActRecvRequest JSON = %s, want %s", got, want)
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

func TestDessertStateTypesDecodeCapturedShapes(t *testing.T) {
	const raw = `{"0":2220,"1":{"1":{"0":100,"1":{"1345":1},"2":[],"3":0,"4":{"2":5},"5":true,"6":{"1343":222},"7":3,"8":20,"9":{"1":2}}}}`
	var got IActDessertData
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	mode, ok := got.MapData[1]
	if !ok {
		t.Fatalf("mode 1 missing from %+v", got.MapData)
	}
	if got.TotalScore != 2220 || mode.Step != 100 || !mode.IsRunning || mode.CurId != 3 || mode.Score != 20 {
		t.Fatalf("decoded dessert state = %+v, mode=%+v", got, mode)
	}
	if mode.ItemUse[1345] != 1 || mode.FirstMerge[2] != 5 || mode.TotalGain[1343] != 222 || mode.LvMap[1] != 2 {
		t.Fatalf("decoded dessert maps = itemUse:%v firstMerge:%v totalGain:%v lvMap:%v", mode.ItemUse, mode.FirstMerge, mode.TotalGain, mode.LvMap)
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
