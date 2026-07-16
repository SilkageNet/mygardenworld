package state

import (
	"encoding/json"
	"strconv"
)

type celebrityState struct {
	Observed         bool
	Valid            bool
	Types            []int32
	TypesObserved    bool
	TypesValid       bool
	Rankings         map[int32][]celebrityEntryState
	RankingsObserved bool
	RankingsValid    bool
	Likes            map[int32]celebrityLikeState
	LikesObserved    bool
	LikesValid       bool
	LastNamespace    string
}

type celebrityEntryState struct {
	ID       int32
	UID      int64
	BatchID  int32
	RankType int32
	Type     int32
	Score    int32
	CreateMs int64
	TitleLvl int32
}

type celebrityLikeState struct {
	UID            int64
	Type           int32
	LastLikeTimeMs int64
	CreateTimeMs   int64
}

func (s *State) applyCelebrityLocked(raw json.RawMessage, namespace string) {
	if isJSONNull(raw) {
		s.celebrity = celebrityState{
			Observed: true, Valid: true, TypesObserved: true, TypesValid: true, RankingsObserved: true, RankingsValid: true,
			LikesObserved: true, LikesValid: true, Rankings: make(map[int32][]celebrityEntryState),
			Likes: make(map[int32]celebrityLikeState), LastNamespace: namespace,
		}
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		s.celebrity = celebrityState{
			Observed: true, Valid: false, Rankings: make(map[int32][]celebrityEntryState),
			Likes: make(map[int32]celebrityLikeState), LastNamespace: namespace,
		}
		return
	}
	_, hasTypes := fields["0"]
	_, hasRankings := fields["1"]
	if hasTypes && hasRankings {
		s.applyCelebrityFullLocked(fields, namespace)
		return
	}

	next := s.celebrity
	next.Observed = true
	next.LastNamespace = namespace
	if next.Rankings == nil {
		next.Rankings = make(map[int32][]celebrityEntryState)
	}
	if next.Likes == nil {
		next.Likes = make(map[int32]celebrityLikeState)
	}
	if rawTypes, present := fields["0"]; present {
		types, valid := decodeCelebrityTypes(rawTypes)
		next.Types = types
		next.TypesObserved = true
		next.TypesValid = valid
	}
	if rawRankings, present := fields["1"]; present {
		rankings, touched, valid := decodeCelebrityRankings(rawRankings)
		next.RankingsObserved = true
		next.RankingsValid = valid
		if valid {
			if isJSONNull(rawRankings) {
				next.Rankings = make(map[int32][]celebrityEntryState)
			} else {
				for _, typeID := range touched {
					delete(next.Rankings, typeID)
					if entries, exists := rankings[typeID]; exists {
						next.Rankings[typeID] = cloneCelebrityEntries(entries)
					}
				}
			}
		} else {
			next.Rankings = make(map[int32][]celebrityEntryState)
		}
	}
	if rawLikes, present := fields["2"]; present {
		likes, touched, valid := decodeCelebrityLikes(rawLikes)
		next.LikesObserved = true
		next.LikesValid = valid
		if valid {
			if isJSONNull(rawLikes) {
				next.Likes = make(map[int32]celebrityLikeState)
			} else {
				for _, typeID := range touched {
					delete(next.Likes, typeID)
					if like, exists := likes[typeID]; exists {
						next.Likes[typeID] = like
					}
				}
			}
		} else {
			next.Likes = make(map[int32]celebrityLikeState)
		}
	}
	next.Valid = (!next.TypesObserved || next.TypesValid) && (!next.RankingsObserved || next.RankingsValid) &&
		(!next.LikesObserved || next.LikesValid)
	s.celebrity = next
}

func (s *State) applyCelebrityFullLocked(fields map[string]json.RawMessage, namespace string) {
	types, typesValid := decodeCelebrityTypes(fields["0"])
	rankings, _, rankingsValid := decodeCelebrityRankings(fields["1"])
	likes := make(map[int32]celebrityLikeState)
	likesValid := true
	if rawLikes, present := fields["2"]; present {
		var touched []int32
		likes, touched, likesValid = decodeCelebrityLikes(rawLikes)
		_ = touched
	}
	if typesValid && rankingsValid && !celebrityRankingsMatchTypes(types, rankings) {
		rankingsValid = false
		rankings = make(map[int32][]celebrityEntryState)
	}
	s.celebrity = celebrityState{
		Observed: true, Valid: typesValid && rankingsValid && likesValid,
		Types: append([]int32(nil), types...), TypesObserved: true, TypesValid: typesValid,
		Rankings: cloneCelebrityRankings(rankings), RankingsObserved: true, RankingsValid: rankingsValid,
		Likes: cloneCelebrityLikes(likes), LikesObserved: true, LikesValid: likesValid, LastNamespace: namespace,
	}
}

func decodeCelebrityTypes(raw json.RawMessage) ([]int32, bool) {
	if isJSONNull(raw) {
		return nil, true
	}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return nil, false
	}
	out := make([]int32, 0, len(values))
	seen := make(map[int32]struct{}, len(values))
	for _, rawValue := range values {
		value, ok := readActivityInt32Raw(rawValue)
		if !ok || value <= 0 {
			return nil, false
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, false
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, true
}

func decodeCelebrityRankings(raw json.RawMessage) (map[int32][]celebrityEntryState, []int32, bool) {
	if isJSONNull(raw) {
		return make(map[int32][]celebrityEntryState), nil, true
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil || entries == nil {
		return nil, nil, false
	}
	out := make(map[int32][]celebrityEntryState, len(entries))
	touched := make([]int32, 0, len(entries))
	for key, rawList := range entries {
		typeID, ok := parseCelebrityTypeKey(key)
		if !ok {
			return nil, nil, false
		}
		touched = append(touched, typeID)
		if isJSONNull(rawList) {
			continue
		}
		var rows []json.RawMessage
		if json.Unmarshal(rawList, &rows) != nil || rows == nil {
			return nil, nil, false
		}
		decoded := make([]celebrityEntryState, 0, len(rows))
		for _, rawRow := range rows {
			entry, valid := decodeCelebrityEntry(rawRow, typeID)
			if !valid {
				return nil, nil, false
			}
			decoded = append(decoded, entry)
		}
		out[typeID] = decoded
	}
	return out, touched, true
}

func decodeCelebrityEntry(raw json.RawMessage, expectedType int32) (celebrityEntryState, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return celebrityEntryState{}, false
	}
	for index := 0; index <= 7; index++ {
		if _, present := fields[strconv.Itoa(index)]; !present {
			return celebrityEntryState{}, false
		}
	}
	id, idOK := readActivityInt32Raw(fields["0"])
	uid, uidOK := readActivityInt64Raw(fields["1"])
	batchID, batchOK := readActivityInt32Raw(fields["2"])
	rankType, rankOK := readActivityInt32Raw(fields["3"])
	typeID, typeOK := readActivityInt32Raw(fields["4"])
	score, scoreOK := readActivityInt32Raw(fields["5"])
	createMs, createOK := readActivityInt64Raw(fields["6"])
	titleLvl, titleOK := readActivityInt32Raw(fields["7"])
	if !idOK || id <= 0 || !uidOK || uid <= 0 || !batchOK || batchID <= 0 || !rankOK || rankType <= 0 ||
		!typeOK || typeID != expectedType || !scoreOK || score < 0 || !createOK || createMs <= 0 || !titleOK || titleLvl < 0 {
		return celebrityEntryState{}, false
	}
	return celebrityEntryState{ID: id, UID: uid, BatchID: batchID, RankType: rankType, Type: typeID, Score: score, CreateMs: createMs, TitleLvl: titleLvl}, true
}

func decodeCelebrityLikes(raw json.RawMessage) (map[int32]celebrityLikeState, []int32, bool) {
	if isJSONNull(raw) {
		return make(map[int32]celebrityLikeState), nil, true
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil || entries == nil {
		return nil, nil, false
	}
	out := make(map[int32]celebrityLikeState, len(entries))
	touched := make([]int32, 0, len(entries))
	for key, rawLike := range entries {
		typeID, ok := parseCelebrityTypeKey(key)
		if !ok {
			return nil, nil, false
		}
		touched = append(touched, typeID)
		if isJSONNull(rawLike) {
			continue
		}
		like, valid := decodeCelebrityLike(rawLike, typeID)
		if !valid {
			return nil, nil, false
		}
		out[typeID] = like
	}
	return out, touched, true
}

func decodeCelebrityLike(raw json.RawMessage, expectedType int32) (celebrityLikeState, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return celebrityLikeState{}, false
	}
	for index := 0; index <= 3; index++ {
		if _, present := fields[strconv.Itoa(index)]; !present {
			return celebrityLikeState{}, false
		}
	}
	uid, uidOK := readActivityInt64Raw(fields["0"])
	typeID, typeOK := readActivityInt32Raw(fields["1"])
	lastLikeMs, lastLikeOK := readActivityInt64Raw(fields["2"])
	createMs, createOK := readActivityInt64Raw(fields["3"])
	if !uidOK || uid <= 0 || !typeOK || typeID != expectedType || !lastLikeOK || lastLikeMs <= 0 || !createOK || createMs <= 0 {
		return celebrityLikeState{}, false
	}
	return celebrityLikeState{UID: uid, Type: typeID, LastLikeTimeMs: lastLikeMs, CreateTimeMs: createMs}, true
}

func parseCelebrityTypeKey(key string) (int32, bool) {
	value, err := strconv.ParseInt(key, 10, 32)
	if err != nil || value <= 0 || strconv.FormatInt(value, 10) != key {
		return 0, false
	}
	return int32(value), true
}

func celebrityRankingsMatchTypes(types []int32, rankings map[int32][]celebrityEntryState) bool {
	if len(types) != len(rankings) {
		return false
	}
	for _, typeID := range types {
		if _, exists := rankings[typeID]; !exists {
			return false
		}
	}
	return true
}

func (s *State) dessertCelebrityViewLocked(batch *activityBatchState) DessertCelebrityLikeView {
	view := DessertCelebrityLikeView{
		Observed: s.celebrity.Observed, TypesObserved: s.celebrity.TypesObserved && s.celebrity.TypesValid,
		RankingsObserved: s.celebrity.RankingsObserved && s.celebrity.RankingsValid,
		LikesObserved:    s.celebrity.LikesObserved && s.celebrity.LikesValid,
	}
	for _, typeID := range s.celebrity.Types {
		if typeID == dessertTmpType {
			view.TypeListed = true
			break
		}
	}
	if entries, exists := s.celebrity.Rankings[dessertTmpType]; exists && view.RankingsObserved {
		view.RankingObserved = true
		view.RankingCount = int32(len(entries))
	}
	if like, exists := s.celebrity.Likes[dessertTmpType]; exists && view.LikesObserved {
		view.LastLikeTimeMs = like.LastLikeTimeMs
		view.CreateTimeMs = like.CreateTimeMs
		view.LikedThisBatch = batch != nil && like.LastLikeTimeMs > batch.BeginMs
	}
	view.Valid = s.celebrity.Valid && view.TypesObserved && view.RankingsObserved && view.LikesObserved &&
		view.TypeListed && view.RankingObserved
	return view
}

func cloneCelebrityEntries(in []celebrityEntryState) []celebrityEntryState {
	return append([]celebrityEntryState(nil), in...)
}

func cloneCelebrityRankings(in map[int32][]celebrityEntryState) map[int32][]celebrityEntryState {
	out := make(map[int32][]celebrityEntryState, len(in))
	for typeID, entries := range in {
		out[typeID] = cloneCelebrityEntries(entries)
	}
	return out
}

func cloneCelebrityLikes(in map[int32]celebrityLikeState) map[int32]celebrityLikeState {
	out := make(map[int32]celebrityLikeState, len(in))
	for typeID, like := range in {
		out[typeID] = like
	}
	return out
}
