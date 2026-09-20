package services

import (
	"testing"
	"time"

	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ts(y int, m time.Month, d int) *timestamppb.Timestamp {
	return timestamppb.New(time.Date(y, m, d, 0, 0, 0, 0, time.UTC))
}

func TestUnionWindows(t *testing.T) {
	cases := []struct {
		name     string
		a, b     *lotteryv1.DateWindow
		wantFrom time.Time
		wantTo   time.Time
	}{
		{
			name:     "simulate window beyond archive extends the load range",
			a:        &lotteryv1.DateWindow{From: ts(2010, 1, 1), To: ts(2025, 12, 31)},
			b:        &lotteryv1.DateWindow{From: ts(2026, 8, 19), To: ts(2026, 9, 19)},
			wantFrom: time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
			wantTo:   time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "simulate window inside archive keeps archive bounds",
			a:        &lotteryv1.DateWindow{From: ts(2010, 1, 1), To: ts(2025, 12, 31)},
			b:        &lotteryv1.DateWindow{From: ts(2020, 1, 1), To: ts(2020, 12, 31)},
			wantFrom: time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
			wantTo:   time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "open lower bound in simulate wins",
			a:        &lotteryv1.DateWindow{From: ts(2010, 1, 1), To: ts(2025, 12, 31)},
			b:        &lotteryv1.DateWindow{To: ts(2026, 9, 19)},
			wantFrom: time.Time{},
			wantTo:   time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "open upper bound in simulate wins",
			a:        &lotteryv1.DateWindow{From: ts(2010, 1, 1), To: ts(2025, 12, 31)},
			b:        &lotteryv1.DateWindow{From: ts(2026, 8, 19)},
			wantFrom: time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
			wantTo:   time.Time{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := unionWindows(tc.a, tc.b)
			if got == nil {
				t.Fatal("unionWindows returned nil")
			}
			gotFrom, gotTo := windowFromProto(got)
			if !gotFrom.Equal(tc.wantFrom) {
				t.Errorf("from = %v, want %v", gotFrom, tc.wantFrom)
			}
			if !gotTo.Equal(tc.wantTo) {
				t.Errorf("to = %v, want %v", gotTo, tc.wantTo)
			}
		})
	}
}

func TestUnionWindows_Nil(t *testing.T) {
	w := &lotteryv1.DateWindow{From: ts(2020, 1, 1), To: ts(2021, 1, 1)}
	if unionWindows(nil, w) != w {
		t.Error("unionWindows(nil, w) should return w")
	}
	if unionWindows(w, nil) != w {
		t.Error("unionWindows(w, nil) should return w")
	}
}
