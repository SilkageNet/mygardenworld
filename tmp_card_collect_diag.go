//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	db, err := store.Open(ctx, "e:/work/mygardenworld/data-b/garden.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	username, password, err := db.GetCredentials(ctx, 1)
	if err != nil {
		panic(err)
	}

	acc, err := db.GetAccountByID(ctx, 1)
	if err != nil {
		panic(err)
	}
	ch, err := babigame.ParseChannel(acc.Channel)
	if err != nil {
		panic(err)
	}
	cfg, err := babigame.ConfigForChannel(ch)
	if err != nil {
		panic(err)
	}

	httpc := babigame.NewHTTPClient(cfg, "", "", "")
	session, err := babigame.PerformLoginWithPassword(ctx, httpc, username, password, 1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("logged in aid=%d gs=%d\n", session.AID, session.GsIdx)

	client := babigame.NewClient(session)
	defer client.Close()
	if err := client.Connect(ctx); err != nil {
		panic(err)
	}
	rpc := clientrpc.NewClient(babigame.NewRPCClient(client, session, babigame.WithServerErrorsAsResults()))
	time.Sleep(300 * time.Millisecond)

	v, err := client.ReLogin(ctx, 1)
	if err != nil {
		panic(err)
	}
	st := state.New()
	st.ApplyV(v)

	var cardCollectBatch int32
	var cardCollectTmpType int32
	if raw23, ok := extractNS(v, "23"); ok {
		fmt.Println("namespace 23 present, scanning batches...")
		cardCollectBatch, cardCollectTmpType = findCardCollectBatch(raw23)
	}

	if raw146, ok := extractNS(v, "146"); ok {
		fmt.Println("namespace 146 already in relogin:")
		printCardCollectNS146(raw146)
	}

	if cardCollectBatch > 0 {
		fmt.Printf("\nentering card collect activity batch=%d tmpType=%d...\n", cardCollectBatch, cardCollectTmpType)
		printBatchCardCollectExt(raw23From(v), cardCollectBatch)

		resp, err := rpc.Act().Enter(ctx, clientproto.ActEnterRequest{BatchId: clientproto.RPCID(cardCollectBatch)}, babigame.WithPayloadApply(false))
		if err != nil {
			fmt.Fprintf(os.Stderr, "act.enter error: %v\n", err)
		} else if resp.HasPayload() {
			st.ApplyV(resp.Payload)
			tryPrint146("act.enter", resp.Payload, resp.Data)
		}

		for _, label := range []string{"checkLuckyCardSend", "refreshTaskData"} {
			var payload json.RawMessage
			var err error
			switch label {
			case "checkLuckyCardSend":
				r, e := rpc.ActCardCollect().CheckLuckyCardSend(ctx, clientproto.ActCardCollectCheckLuckyCardSendRequest{BatchId: clientproto.RPCID(cardCollectBatch)}, babigame.WithPayloadApply(false))
				payload, err = r.Payload, e
			case "refreshTaskData":
				r, e := rpc.ActCardCollect().RefreshTaskData(ctx, clientproto.ActCardCollectRefreshTaskDataRequest{BatchId: clientproto.RPCID(cardCollectBatch)}, babigame.WithPayloadApply(false))
				payload, err = r.Payload, e
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s error: %v\n", label, err)
				continue
			}
			if len(payload) > 0 {
				tryPrint146(label, payload, nil)
			}
		}

		if lazyV, err := client.LazySync(ctx); err == nil {
			tryPrint146("lazySync", lazyV, nil)
		}

		fmt.Println("\n=== openCardPack 五星卡包 (1028) ===")
		openResp, err := rpc.ActCardCollect().OpenCardPack(ctx, clientproto.ActCardCollectOpenCardPackRequest{
			CardPackId: clientproto.RPCID(1028),
			Num:        1,
		}, babigame.WithPayloadApply(false))
		if err != nil {
			fmt.Fprintf(os.Stderr, "openCardPack error: %v\n", err)
		} else {
			tryPrint146("openCardPack", openResp.Payload, openResp.Data)
			fmt.Println("full openCardPack payload:")
			fmt.Println(string(openResp.Payload))
		}
	} else {
		fmt.Println("no active card collect batch found in namespace 23")
	}

	// Also scan activity bag for 5-star card packs in inventory ns7 + ns23
	fmt.Println("\nactivity bag items (card pack candidates):")
	if raw23, ok := extractNS(v, "23"); ok {
		printActivityBags(raw23)
	}
	_ = st
}

func extractNS(v json.RawMessage, ns string) (json.RawMessage, bool) {
	var top map[string]json.RawMessage
	if json.Unmarshal(v, &top) != nil {
		return nil, false
	}
	raw, ok := top[ns]
	return raw, ok
}

func raw23From(v json.RawMessage) json.RawMessage {
	raw, ok := extractNS(v, "23")
	if ok {
		return raw
	}
	return v
}

func tryPrint146(label string, payload json.RawMessage, data clientproto.StateDelta) {
	if raw146, ok := extractNS(payload, "146"); ok {
		fmt.Printf("\nafter %s namespace 146:\n", label)
		printCardCollectNS146(raw146)
		return
	}
	if data != nil {
		if raw146, ok := data["146"]; ok {
			fmt.Printf("\nafter %s namespace 146 (data):\n", label)
			printCardCollectNS146(raw146)
			return
		}
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(payload, &top) == nil {
		if raw146, ok := top["146"]; ok {
			fmt.Printf("\nafter %s namespace 146 (top):\n", label)
			printCardCollectNS146(raw146)
			return
		}
	}
	fmt.Printf("\nno namespace 146 after %s\n", label)
	printVKeys(payload)
}

func printBatchCardCollectExt(raw23 json.RawMessage, batchID int32) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw23, &fields) != nil {
		return
	}
	rawBatches, ok := fields["0"]
	if !ok {
		return
	}
	var batches map[string]json.RawMessage
	if json.Unmarshal(rawBatches, &batches) != nil {
		return
	}
	rawEntry, ok := batches[strconv.FormatInt(int64(batchID), 10)]
	if !ok {
		return
	}
	var entry map[string]json.RawMessage
	if json.Unmarshal(rawEntry, &entry) != nil {
		return
	}
	rawExt, ok := entry["14"]
	if !ok {
		fmt.Println("no ext field on batch")
		return
	}
	var ext map[string]json.RawMessage
	if json.Unmarshal(rawExt, &ext) != nil {
		return
	}
	if rawCC, ok := ext["110"]; ok {
		fmt.Println("\n=== cardCollectData (ext 110) ===")
		fmt.Println(string(rawCC))
		printLuckyCardRcd(rawCC)
	}
}

func printLuckyCardRcd(rawCC json.RawMessage) {
	var cc map[string]json.RawMessage
	if json.Unmarshal(rawCC, &cc) != nil {
		return
	}
	if raw, ok := cc["10"]; ok {
		fmt.Println("\n--- luckyCardRcd ---")
		fmt.Println(string(raw))
		var lucky map[string]json.RawMessage
		if json.Unmarshal(raw, &lucky) == nil {
			if star5, ok := lucky["5"]; ok {
				fmt.Println("\nstar 5 lucky cards:")
				printCardList(star5)
			}
		}
	}
}

func printVKeys(v json.RawMessage) {
	var top map[string]json.RawMessage
	if json.Unmarshal(v, &top) != nil {
		return
	}
	keys := make([]string, 0, len(top))
	for k := range top {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println("v keys:", keys)
}

func findCardCollectBatch(raw23 json.RawMessage) (batchID, tmpType int32) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw23, &fields) != nil {
		return 0, 0
	}
	rawBatches, ok := fields["0"]
	if !ok {
		return 0, 0
	}
	var batches map[string]json.RawMessage
	if json.Unmarshal(rawBatches, &batches) != nil {
		return 0, 0
	}
	type candidate struct {
		id      int32
		tmpType int32
		status  int32
		name    string
	}
	var cands []candidate
	for key, rawEntry := range batches {
		id64, err := strconv.ParseInt(key, 10, 32)
		if err != nil || id64 <= 0 {
			continue
		}
		var entry map[string]json.RawMessage
		if json.Unmarshal(rawEntry, &entry) != nil {
			continue
		}
		var tmpID, tt, status int32
		if raw, ok := entry["1"]; ok {
			json.Unmarshal(raw, &tmpID)
		}
		if raw, ok := entry["2"]; ok {
			json.Unmarshal(raw, &tt)
		}
		if raw, ok := entry["3"]; ok {
			json.Unmarshal(raw, &status)
		}
		name := lookupActTmpName(tmpID)
		if stringsLooksLikeCardCollect(name, tt, entry) {
			cands = append(cands, candidate{id: int32(id64), tmpType: tt, status: status, name: name})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].id > cands[j].id })
	for _, c := range cands {
		fmt.Printf("  batch candidate id=%d tmpType=%d status=%d name=%q\n", c.id, c.tmpType, c.status, c.name)
		if c.status == 2 || c.status == 1 {
			return c.id, c.tmpType
		}
	}
	if len(cands) > 0 {
		return cands[0].id, cands[0].tmpType
	}
	return 0, 0
}

func lookupActTmpName(tmpID int32) string {
	if row, ok := state.StaticRow("c_act_tmp", tmpID); ok {
		var obj map[string]json.RawMessage
		if json.Unmarshal(row, &obj) == nil {
			if raw, ok := obj["name"]; ok {
				var name string
				if json.Unmarshal(raw, &name) == nil {
					return name
				}
			}
		}
	}
	return ""
}

func stringsLooksLikeCardCollect(name string, tmpType int32, entry map[string]json.RawMessage) bool {
	if containsCardCollect(name) {
		return true
	}
	// ext field 110 = cardCollectData
	if rawExt, ok := entry["14"]; ok {
		var ext map[string]json.RawMessage
		if json.Unmarshal(rawExt, &ext) == nil {
			if _, ok := ext["110"]; ok {
				return true
			}
		}
	}
	// common card-collect tmpTypes observed in similar games; also check bag for pack items
	if rawBag, ok := entry["12"]; ok {
		s := string(rawBag)
		if stringsContainsAny(s, `"11":`, `"12":`, `"13":`, `"14":`, `"15":`) {
			return true
		}
	}
	return false
}

func containsCardCollect(name string) bool {
	return len(name) > 0 && (contains(name, "卡册") || contains(name, "集卡") || contains(name, "卡牌"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func stringsContainsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

func printActivityBags(raw23 json.RawMessage) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw23, &fields) != nil {
		return
	}
	rawBatches, ok := fields["0"]
	if !ok {
		return
	}
	var batches map[string]json.RawMessage
	if json.Unmarshal(rawBatches, &batches) != nil {
		return
	}
	for key, rawEntry := range batches {
		var entry map[string]json.RawMessage
		if json.Unmarshal(rawEntry, &entry) != nil {
			continue
		}
		rawBag, ok := entry["12"]
		if !ok {
			continue
		}
		var bag map[string]json.RawMessage
		if json.Unmarshal(rawBag, &bag) != nil {
			continue
		}
		for itemKey, rawCount := range bag {
			itemID, _ := strconv.ParseInt(itemKey, 10, 32)
			var count int32
			json.Unmarshal(rawCount, &count)
			if count <= 0 {
				continue
			}
			label := state.ItemLabel(int32(itemID))
			if contains(label, "卡包") || contains(label, "五星") || contains(label, "5星") || int32(itemID) >= 1100000 {
				fmt.Printf("  batch %s: item %d (%s) x%d\n", key, itemID, label, count)
			}
		}
	}
}

func printCardCollectNS146(raw146 json.RawMessage) {
	fmt.Println(string(raw146))
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw146, &fields) != nil {
		return
	}
	rawShow, ok := fields["0"]
	if !ok {
		fmt.Println("(no showData field 0)")
		return
	}
	var show map[string]json.RawMessage
	if json.Unmarshal(rawShow, &show) != nil {
		return
	}

	if rawPacks, ok := show["0"]; ok {
		fmt.Println("\n=== packOpen (pre-determined pack contents) ===")
		var packs []map[string]json.RawMessage
		if json.Unmarshal(rawPacks, &packs) == nil {
			for i, pack := range packs {
				var packID int32
				if raw, ok := pack["0"]; ok {
					json.Unmarshal(raw, &packID)
				}
				fmt.Printf("pack[%d] packId=%d (%s)\n", i, packID, packName(packID))
				if rawCards, ok := pack["1"]; ok {
					printCardList(rawCards)
				}
			}
		} else {
			fmt.Println(string(rawPacks))
		}
	}

	if rawLucky, ok := show["2"]; ok {
		fmt.Println("\n=== luckyCardMap (by star) ===")
		var lucky map[string]json.RawMessage
		if json.Unmarshal(rawLucky, &lucky) == nil {
			starKeys := make([]string, 0, len(lucky))
			for k := range lucky {
				starKeys = append(starKeys, k)
			}
			sort.Strings(starKeys)
			for _, starKey := range starKeys {
				fmt.Printf("star %s:\n", starKey)
				printCardList(lucky[starKey])
			}
		} else {
			fmt.Println(string(rawLucky))
		}
	}
}

func printCardList(raw json.RawMessage) {
	// cardList may be array of card ids or nested structure
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		for i, item := range arr {
			var id int32
			if json.Unmarshal(item, &id) == nil {
				fmt.Printf("  card[%d] id=%d (%s)\n", i, id, cardName(id))
				continue
			}
			fmt.Printf("  card[%d] raw=%s\n", i, string(item))
		}
		return
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) == nil {
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			var id int32
			if json.Unmarshal(obj[k], &id) == nil {
				fmt.Printf("  card[%s] id=%d (%s)\n", k, id, cardName(id))
			} else {
				fmt.Printf("  card[%s] raw=%s\n", k, string(obj[k]))
			}
		}
		return
	}
	fmt.Println("  raw:", string(raw))
}

func cardName(id int32) string {
	for _, table := range []string{"c_act_cardCollect_card", "c_act_card_collect_card", "c_cardCollect_card", "c_act_card_collect_card"} {
		if row, ok := state.StaticRow(table, id); ok {
			var obj map[string]json.RawMessage
			if json.Unmarshal(row, &obj) == nil {
				for _, key := range []string{"name", "cardName", "title"} {
					if raw, ok := obj[key]; ok {
						var name string
						if json.Unmarshal(raw, &name) == nil && name != "" {
							return name
						}
					}
				}
			}
			return string(row)
		}
	}
	return state.ItemLabel(id)
}

func packName(id int32) string {
	for _, table := range []string{"c_act_cardCollect_cardPack", "c_act_card_collect_pack", "c_cardCollect_cardPack"} {
		if row, ok := state.StaticRow(table, id); ok {
			var obj map[string]json.RawMessage
			if json.Unmarshal(row, &obj) == nil {
				if raw, ok := obj["name"]; ok {
					var name string
					if json.Unmarshal(raw, &name) == nil {
						return name
					}
				}
			}
		}
	}
	return state.ItemLabel(id)
}
