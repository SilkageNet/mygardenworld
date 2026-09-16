package state

import (
	"encoding/json"
	"math"
	"testing"
)

func TestCustomerArtFloralCoin(t *testing.T) {
	for _, tt := range []struct {
		name, row, fallback string
		want                int64
		known               bool
	}{
		{"default", `{}`, `{"$cPrice":[1002,1]}`, 1, true},
		{"empty uses default", `{"cPrice":[]}`, `{"$cPrice":[1002,2]}`, 2, true},
		{"explicit", `{"cPrice":[[11,100],[1002,7],[2,10]]}`, `{}`, 7, true},
		{"explicit zero", `{"cPrice":[[1002,0]]}`, `{"$cPrice":[1002,1]}`, 0, true},
		{"no coin in explicit reward", `{"cPrice":[[11,100]]}`, `{"$cPrice":[1002,1]}`, 0, true},
		{"unknown default", `{}`, `{}`, 0, false},
		{"invalid", `{"cPrice":[[1002,-1]]}`, `{}`, 0, false},
		{"invalid pair", `{"cPrice":[[1002]]}`, `{}`, 0, false},
		{"overflow", `{"cPrice":[[1002,9223372036854775807],[1002,1]]}`, `{}`, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, known := customerArtFloralCoin(json.RawMessage(tt.row), json.RawMessage(tt.fallback))
			if got != tt.want || known != tt.known {
				t.Fatalf("got %d/%v want %d/%v", got, known, tt.want, tt.known)
			}
		})
	}
}

func TestCustomerOrderFloralCoinReward(t *testing.T) {
	for _, tt := range []struct {
		name  string
		order *CustomerOrder
		want  int64
		known bool
	}{
		{"nil", nil, 0, false},
		{"empty", &CustomerOrder{}, 0, false},
		{"unknown art", &CustomerOrder{ItemRequires: []ItemRequire{{ItemID: 99999999, Count: 1}}}, 0, false},
		{"invalid quantity", &CustomerOrder{ItemRequires: []ItemRequire{{ItemID: 300207, Count: 0}}}, 0, false},
		{"two pieces", &CustomerOrder{ItemRequires: []ItemRequire{{ItemID: 300207, Count: 2}}}, 2, true},
		{"multiple arts", &CustomerOrder{ItemRequires: []ItemRequire{{ItemID: 300207, Count: 2}, {ItemID: 300208, Count: 3}}}, 5, true},
		{"large count", &CustomerOrder{ItemRequires: []ItemRequire{{ItemID: 300207, Count: math.MaxInt32}}}, math.MaxInt32, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, known := CustomerOrderFloralCoinReward(tt.order)
			if got != tt.want || known != tt.known {
				t.Fatalf("got %d/%v want %d/%v", got, known, tt.want, tt.known)
			}
		})
	}
}
