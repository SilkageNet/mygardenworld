package state

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCelebrityFullSyncObservedEmptyAndSparseLike(t *testing.T) {
	s := applyDessertCaptureFixture(t)
	view, _ := s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if !view.Celebrity.Valid || !view.Celebrity.LikesObserved || view.Celebrity.LikedThisBatch || len(s.celebrity.Likes) != 0 {
		t.Fatalf("full sync missing likeMap=%+v state=%+v", view.Celebrity, s.celebrity)
	}

	s.ApplyV(json.RawMessage(`{"166":{"2":{"5601":{"0":10002,"1":5601,"2":1783819878693,"3":1783819878693}}}}`))
	view, _ = s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if !view.Celebrity.Valid || !view.Celebrity.LikedThisBatch || view.Celebrity.LastLikeTimeMs != 1783819878693 ||
		view.Celebrity.RankingCount != 1 || len(s.celebrity.Types) != 1 {
		t.Fatalf("sparse like=%+v state=%+v", view.Celebrity, s.celebrity)
	}

	// A new controlled full sync owns all three maps; missing field 2 clears
	// the old like and records an authoritative observed-empty map.
	s.ApplyV(json.RawMessage(`{"166":{"0":[5601],"1":{"5601":[{"0":2,"1":10003,"2":9002,"3":1,"4":5601,"5":20000,"6":1783700000000,"7":1}]}}}`))
	view, _ = s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if !view.Celebrity.Valid || view.Celebrity.LikedThisBatch || !view.Celebrity.LikesObserved || len(s.celebrity.Likes) != 0 {
		t.Fatalf("new full sync did not clear likes: %+v state=%+v", view.Celebrity, s.celebrity)
	}
}

func TestCelebrityCanonicalNamespaceWinsSameDelta(t *testing.T) {
	s := applyDessertCaptureFixture(t)
	s.ApplyV(json.RawMessage(`{
		"165":{"0":[5601],"1":{"5601":[{"0":1,"1":20001,"2":8001,"3":1,"4":5601,"5":1,"6":1783600000000,"7":1}]},"2":{"5601":{"0":20001,"1":5601,"2":1783700000000,"3":1783700000000}}},
		"166":{"0":[5601],"1":{"5601":[{"0":2,"1":30001,"2":9001,"3":1,"4":5601,"5":2,"6":1783700000000,"7":1},{"0":3,"1":30002,"2":9001,"3":1,"4":5601,"5":3,"6":1783700001000,"7":1}]},"2":{"5601":{"0":30001,"1":5601,"2":1783819000001,"3":1783819000001}}}
	}`))
	view, _ := s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if s.celebrity.LastNamespace != "166" || !view.Celebrity.Valid || view.Celebrity.RankingCount != 2 ||
		view.Celebrity.LastLikeTimeMs != 1783819000001 || !view.Celebrity.LikedThisBatch {
		t.Fatalf("canonical precedence=%+v state=%+v", view.Celebrity, s.celebrity)
	}
}

func TestCelebrityMalformedFullSyncClearsExecutableView(t *testing.T) {
	s := applyDessertCaptureFixture(t)
	s.ApplyV(json.RawMessage(`{"166":{"0":[5601],"1":{"5601":[{"0":1,"1":10001,"2":9001,"3":1,"4":5602,"5":1,"6":1783600000000,"7":1}]}}}`))
	view, _ := s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if view.Celebrity.Valid || view.Celebrity.RankingObserved || len(s.celebrity.Rankings) != 0 || s.celebrity.Valid {
		t.Fatalf("malformed full sync retained state: %+v state=%+v", view.Celebrity, s.celebrity)
	}
}
