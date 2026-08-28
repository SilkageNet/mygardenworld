package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

var opened = []int32{45138, 44155, 43078, 41052}

// before-open bag counts from prior diagnostic snapshot (batch 1352 field 12)
var beforeBag = map[int32]int32{
	45147: 1, 45157: 1, 45149: 1, 45159: 1, 45119: 1, 45128: 2, 45129: 1,
	1024: 43, 1025: 17, 1026: 6, 1027: 2, 1028: 1,
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, _ := store.Open(ctx, "e:/work/mygardenworld/data-b/garden.db")
	defer db.Close()
	user, pass, _ := db.GetCredentials(ctx, 1)
	acc, _ := db.GetAccountByID(ctx, 1)
	ch, _ := babigame.ParseChannel(acc.Channel)
	cfg, _ := babigame.ConfigForChannel(ch)
	httpc := babigame.NewHTTPClient(cfg, "", "", "")
	session, _ := babigame.PerformLoginWithPassword(ctx, httpc, user, pass, 1)
	client := babigame.NewClient(session)
	defer client.Close()
	client.Connect(ctx)
	time.Sleep(300 * time.Millisecond)
	v, _ := client.ReLogin(ctx, 1)

	var top map[string]json.RawMessage
	json.Unmarshal(v, &top)
	bag := parseBag(top["23"], 1352)
	ccRaw := parseCardCollectRaw(top["23"], 1352)

	fmt.Println("=== 五星卡包开包影响分析 · 顾依萱 ===")
	fmt.Println()

	fmt.Println("## 本次开出 4 张")
	for _, id := range opened {
		star, album, seq := parseCardID(id)
		before := beforeBag[id]
		after := bag[id]
		status := "新卡"
		if before >= 1 {
			status = fmt.Sprintf("重复 (开前 x%d → 开后 x%d)", before, after)
		} else {
			status = fmt.Sprintf("新卡 (开前无 → 开后 x%d)", after)
		}
		fmt.Printf("- %s\n  %s | %d星 卡册%d 第%d张 | %s\n", state.ItemLabel(id), cardID(id), star, album, seq, status)
	}

	fmt.Println("\n## 所属卡册集齐情况（当前）")
	albums := map[int32]bool{}
	for _, id := range opened {
		_, a, _ := parseCardID(id)
		albums[a] = true
	}
	keys := make([]int32, 0, len(albums))
	for a := range albums { keys = append(keys, a) }
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, album := range keys {
		for star := int32(1); star <= 5; star++ {
			cards := cardsInAlbum(album, star)
			if len(cards) == 0 { continue }
			owned := ownedSeqs(bag, album, star)
			newFromPack := newInPack(album, star)
			fmt.Printf("\n卡册 %d · %d星 (共 %d 张，已收 %d 张", album, star, len(cards), len(owned))
			if len(owned) >= len(cards) {
				fmt.Print(") ✅ 已集齐")
			} else {
				fmt.Printf("，缺 %d 张)", len(cards)-len(owned))
			}
			if newFromPack > 0 {
				fmt.Printf(" ← 本次新+%d", newFromPack)
			}
			fmt.Println()
			miss := missing(cards, owned)
			if len(miss) > 0 {
				fmt.Printf("  仍缺: %s\n", formatSeqCards(album, star, miss))
			}
		}
	}

	fmt.Println("\n## 五星卡整体（开包后）")
	var five []int32
	for id, cnt := range bag {
		s, _, _ := parseCardID(id)
		if s == 5 && cnt > 0 { five = append(five, id) }
	}
	sort.Slice(five, func(i, j int) bool { return five[i] < five[j] })
	dupCount := 0
	for _, id := range five {
		cnt := bag[id]
		tag := ""
		if cnt >= 2 { tag = fmt.Sprintf(" ⚠️ x%d 重复", cnt); dupCount++ }
		if id == 45138 { tag += " ← 本次开出" }
		fmt.Printf("  %s x%d%s\n", state.ItemLabel(id), cnt, tag)
	}
	fmt.Printf("五星单卡种类 %d，其中 %d 种有重复\n", len(five), dupCount)

	fmt.Println("\n## 活动数据 cardSetRcd / randonRcd 摘要")
	summarizeCardCollect(ccRaw, bag)
}

func parseBag(raw23 json.RawMessage, batch int32) map[int32]int32 {
	out := map[int32]int32{}
	var f, batches, entry map[string]json.RawMessage
	json.Unmarshal(raw23, &f)
	json.Unmarshal(f["0"], &batches)
	json.Unmarshal(batches[strconv.Itoa(int(batch))], &entry)
	var m map[string]json.RawMessage
	json.Unmarshal(entry["12"], &m)
	for k, v := range m {
		ki, _ := strconv.Atoi(k)
		var n int32
		json.Unmarshal(v, &n)
		if n > 0 { out[int32(ki)] = n }
	}
	return out
}

func parseCardCollectRaw(raw23 json.RawMessage, batch int32) map[string]json.RawMessage {
	var f, batches, entry, ext map[string]json.RawMessage
	json.Unmarshal(raw23, &f)
	json.Unmarshal(f["0"], &batches)
	json.Unmarshal(batches[strconv.Itoa(int(batch))], &entry)
	json.Unmarshal(entry["14"], &ext)
	var cc map[string]json.RawMessage
	json.Unmarshal(ext["110"], &cc)
	return cc
}

func parseCardID(id int32) (star, album, seq int32) {
	s := fmt.Sprintf("%05d", id)
	p, _ := strconv.Atoi(s[:2])
	a, _ := strconv.Atoi(s[2:4])
	sq, _ := strconv.Atoi(s[4:])
	return int32(p - 40), int32(a), int32(sq)
}

func cardID(id int32) string { return fmt.Sprintf("#%d", id) }

func cardsInAlbum(album, star int32) []int32 {
	var ids []int32
	prefix := int32(40+star)*1000 + album*10
	for seq := int32(1); seq <= 9; seq++ {
		id := prefix + seq
		label := state.ItemLabel(id)
		if label == "" || label == fmt.Sprintf("#%d", id) { continue }
		if strings.Contains(label, "卡册") { ids = append(ids, id) }
	}
	return ids
}

func ownedSeqs(bag map[int32]int32, album, star int32) []int32 {
	var seqs []int32
	for id, cnt := range bag {
		s, a, seq := parseCardID(id)
		if s == star && a == album && cnt > 0 { seqs = append(seqs, seq) }
	}
	sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
	return seqs
}

func newInPack(album, star int32) int {
	n := 0
	for _, id := range opened {
		s, a, _ := parseCardID(id)
		if s == star && a == album && beforeBag[id] == 0 { n++ }
	}
	return n
}

func missing(all []int32, owned []int32) []int32 {
	have := map[int32]bool{}
	for _, s := range owned { have[s] = true }
	var miss []int32
	for _, id := range all {
		_, _, seq := parseCardID(id)
		if !have[seq] { miss = append(miss, seq) }
	}
	return miss
}

func formatSeqCards(album, star int32, seqs []int32) string {
	parts := make([]string, 0, len(seqs))
	for _, seq := range seqs {
		id := int32(40+star)*1000 + album*10 + seq
		parts = append(parts, fmt.Sprintf("序列%d(%s)", seq, state.ItemLabel(id)))
	}
	return strings.Join(parts, "、")
}

func summarizeCardCollect(cc map[string]json.RawMessage, bag map[int32]int32) {
	if raw, ok := cc["8"]; ok {
		fmt.Println("luckyMap (各星累计):", string(raw))
	}
	if raw, ok := cc["5"]; ok {
		fmt.Println("protectMap (保底计数):", string(raw))
	}
	// count albums with all 5-star cards
	complete5 := 0
	for album := int32(1); album <= 15; album++ {
		cards := cardsInAlbum(album, 5)
		if len(cards) == 0 { continue }
		owned := ownedSeqs(bag, album, 5)
		if len(owned) >= len(cards) { complete5++ }
	}
	fmt.Printf("15套五星卡册中已集齐 %d 套\n", complete5)
}
