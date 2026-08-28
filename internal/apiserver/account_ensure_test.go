package apiserver

import (
	"testing"
	"time"
)

func TestNextMidnightAfter(t *testing.T) {
	loc := ShanghaiMidnightLocation()
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before midnight",
			now:  time.Date(2026, 8, 26, 23, 30, 0, 0, loc),
			want: time.Date(2026, 8, 27, 0, 0, 0, 0, loc),
		},
		{
			name: "exact midnight",
			now:  time.Date(2026, 8, 27, 0, 0, 0, 0, loc),
			want: time.Date(2026, 8, 28, 0, 0, 0, 0, loc),
		},
		{
			name: "after midnight",
			now:  time.Date(2026, 8, 27, 0, 0, 1, 0, loc),
			want: time.Date(2026, 8, 28, 0, 0, 0, 0, loc),
		},
		{
			name: "midday",
			now:  time.Date(2026, 8, 27, 12, 0, 0, 0, loc),
			want: time.Date(2026, 8, 28, 0, 0, 0, 0, loc),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NextMidnightAfter(tc.now, loc)
			if !got.Equal(tc.want) {
				t.Fatalf("NextMidnightAfter(%v)=%v, want %v", tc.now, got, tc.want)
			}
		})
	}
}
