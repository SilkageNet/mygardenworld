package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultHarvestRunGap = 30 * time.Minute

// HarvestStatsOptions controls the operation_log window used for harvest
// summaries. AccountIDs and UserID are additive filters; RunGap groups the
// latest contiguous run by successful harvest timestamps.
type HarvestStatsOptions struct {
	AccountIDs []int64
	UserID     int64
	RunGap     time.Duration
}

// HarvestStats is the aggregate for the most recent contiguous harvest run.
type HarvestStats struct {
	WindowStart time.Time
	WindowEnd   time.Time
	RunGap      time.Duration
	HarvestOps  int
	Accounts    []AccountHarvestStats
}

// AccountHarvestStats is one account's contribution to a HarvestStats window.
type AccountHarvestStats struct {
	AccountID      int64
	AccountName    string
	FirstHarvestAt time.Time
	LastHarvestAt  time.Time
	HarvestOps     int
	Items          []HarvestItemTotal
}

// HarvestItemTotal is the total count gained for one item id.
type HarvestItemTotal struct {
	ItemID int32
	Count  int64
}

type harvestLogRow struct {
	accountID   int64
	accountName string
	ts          time.Time
	resultJSON  string
}

// LatestHarvestStats parses successful harvest operation results and returns
// the newest contiguous run. The runner stores the raw RPC v-fragment in
// result_json; harvested rewards are the additive inventory delta at 7.2.0.
func (d *DB) LatestHarvestStats(ctx context.Context, opts HarvestStatsOptions) (*HarvestStats, error) {
	gap := opts.RunGap
	if gap <= 0 {
		gap = defaultHarvestRunGap
	}
	query, args := harvestStatsQuery(opts)
	rows, err := d.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query harvest log: %w", err)
	}
	defer rows.Close()

	stats := &HarvestStats{RunGap: gap}
	byAccount := map[int64]*accountHarvestAccumulator{}
	var previous time.Time
	for rows.Next() {
		var rawTS string
		var row harvestLogRow
		if err := rows.Scan(&row.accountID, &row.accountName, &rawTS, &row.resultJSON); err != nil {
			return nil, err
		}
		row.ts, err = parseSQLiteTime(rawTS)
		if err != nil {
			return nil, fmt.Errorf("parse harvest ts %q: %w", rawTS, err)
		}
		deltas := harvestItemDeltas(row.resultJSON)
		if len(deltas) == 0 {
			continue
		}
		if stats.HarvestOps > 0 && previous.Sub(row.ts) > gap {
			break
		}
		previous = row.ts
		if stats.HarvestOps == 0 {
			stats.WindowStart = row.ts
			stats.WindowEnd = row.ts
		}
		if row.ts.Before(stats.WindowStart) {
			stats.WindowStart = row.ts
		}
		if row.ts.After(stats.WindowEnd) {
			stats.WindowEnd = row.ts
		}
		stats.HarvestOps++

		acc := byAccount[row.accountID]
		if acc == nil {
			acc = &accountHarvestAccumulator{
				AccountHarvestStats: AccountHarvestStats{
					AccountID:      row.accountID,
					AccountName:    row.accountName,
					FirstHarvestAt: row.ts,
					LastHarvestAt:  row.ts,
				},
				items: map[int32]int64{},
			}
			byAccount[row.accountID] = acc
		}
		if row.ts.Before(acc.FirstHarvestAt) {
			acc.FirstHarvestAt = row.ts
		}
		if row.ts.After(acc.LastHarvestAt) {
			acc.LastHarvestAt = row.ts
		}
		acc.HarvestOps++
		for id, count := range deltas {
			acc.items[id] += count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	accountIDs := make([]int64, 0, len(byAccount))
	for id := range byAccount {
		accountIDs = append(accountIDs, id)
	}
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })
	stats.Accounts = make([]AccountHarvestStats, 0, len(accountIDs))
	for _, id := range accountIDs {
		acc := byAccount[id]
		itemIDs := make([]int32, 0, len(acc.items))
		for itemID := range acc.items {
			itemIDs = append(itemIDs, itemID)
		}
		sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })
		for _, itemID := range itemIDs {
			acc.Items = append(acc.Items, HarvestItemTotal{ItemID: itemID, Count: acc.items[itemID]})
		}
		stats.Accounts = append(stats.Accounts, acc.AccountHarvestStats)
	}
	return stats, nil
}

type accountHarvestAccumulator struct {
	AccountHarvestStats
	items map[int32]int64
}

func harvestStatsQuery(opts HarvestStatsOptions) (string, []any) {
	var where []string
	args := make([]any, 0, len(opts.AccountIDs)+1)
	if len(opts.AccountIDs) > 0 {
		placeholders := make([]string, len(opts.AccountIDs))
		for i, id := range opts.AccountIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		where = append(where, "operation_log.account_id IN ("+strings.Join(placeholders, ",")+")")
	}
	if opts.UserID > 0 {
		where = append(where, "accounts.user_id = ?")
		args = append(args, opts.UserID)
	}
	filter := ""
	if len(where) > 0 {
		filter = " AND " + strings.Join(where, " AND ")
	}
	return `SELECT operation_log.account_id, accounts.name, operation_log.ts, operation_log.result_json
FROM operation_log
JOIN accounts ON accounts.id = operation_log.account_id
WHERE operation_log.kind IN ('usrLand.harvest', 'usrLand.harvestOneKey')` + filter + `
ORDER BY operation_log.ts DESC, operation_log.id DESC`, args
}

func harvestItemDeltas(resultJSON string) map[int32]int64 {
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(resultJSON), &top); err != nil {
		return nil
	}
	raw7, ok := top["7"]
	if !ok {
		return nil
	}
	var ns7 map[string]json.RawMessage
	if err := json.Unmarshal(raw7, &ns7); err != nil {
		return nil
	}
	raw2, ok := ns7["2"]
	if !ok {
		return nil
	}
	var cell2 map[string]json.RawMessage
	if err := json.Unmarshal(raw2, &cell2); err != nil {
		return nil
	}
	rawDelta, ok := cell2["0"]
	if !ok {
		return nil
	}
	var delta map[string]int64
	if err := json.Unmarshal(rawDelta, &delta); err != nil {
		return nil
	}
	out := make(map[int32]int64, len(delta))
	for key, count := range delta {
		if count == 0 {
			continue
		}
		itemID, err := strconv.ParseInt(key, 10, 32)
		if err != nil || itemID == 0 {
			continue
		}
		out[int32(itemID)] += count
	}
	return out
}

func parseSQLiteTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05.999999999", raw); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse("2006-01-02 15:04:05", raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
