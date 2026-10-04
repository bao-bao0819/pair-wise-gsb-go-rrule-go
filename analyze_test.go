// 2017-2022, Teambition. All rights reserved.

package rrule

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func midnight(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

type analyzeCase struct {
	name    string
	options ROption
	want    EmptyStatus
	// witness, when want == HasWitness, must equal the iterator's first
	// occurrence.
	witness time.Time
	// bounded is the expected Bounded flag when want == EmptyProven.
	bounded bool
	// decisiveTokens are the expected Item tokens of decisive rules (order
	// independent), when want == EmptyProven.
	decisiveTokens []string
}

func runAnalyzeCases(t *testing.T, cases []analyzeCase, opts ...AnalyzeOption) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewRRule(tc.options)
			if err != nil {
				t.Fatalf("NewRRule error: %v", err)
			}
			res := r.AnalyzeEmpty(opts...)
			if res.Status != tc.want {
				t.Fatalf("status = %s, want %s (why: %s)", res.Status, tc.want, res.Why)
			}

			// Cross-check against the normal iterator.
			itFirst, has := r.Iterator()()
			switch tc.want {
			case HasWitness:
				if !has {
					t.Fatalf("analyzer found witness %s but iterator is empty", res.Witness)
				}
				if !res.Witness.Equal(itFirst) {
					t.Fatalf("witness %s != iterator first %s", res.Witness, itFirst)
				}
				if !res.Witness.Equal(tc.witness) {
					t.Fatalf("witness = %s, want %s", res.Witness, tc.witness)
				}
			case EmptyProven:
				if has {
					t.Fatalf("analyzer proved empty but iterator yields %s", itFirst)
				}
				if res.Bounded != tc.bounded {
					t.Fatalf("bounded = %v, want %v", res.Bounded, tc.bounded)
				}
				assertTokens(t, res.Decisive, tc.decisiveTokens)
				assertDecisiveWitnesses(t, r, res.Decisive)
			}
		})
	}
}

func assertTokens(t *testing.T, got []DecisiveRule, want []string) {
	t.Helper()
	gotSet := map[string]bool{}
	for _, d := range got {
		gotSet[d.Item] = true
	}
	for _, w := range want {
		if !gotSet[w] {
			t.Errorf("missing decisive item %q; got %v", w, got)
		}
	}
	if len(want) != 0 && len(got) != len(want) {
		t.Errorf("decisive items = %v, want exactly %v", got, want)
	}
}

// assertDecisiveWitnesses verifies that relaxing each reported decisive item
// in isolation really does make the (otherwise built) rule emit that witness.
// This guards the ablation logic.
func assertDecisiveWitnesses(t *testing.T, r *RRule, decisive []DecisiveRule) {
	t.Helper()
	for _, d := range decisive {
		if d.RemovingWitness.IsZero() {
			continue // boundary-only items need not carry a witness
		}
		o := r.OrigOptions
		switch {
		case d.Item == "INTERVAL="+strconv.Itoa(r.interval):
			o.Interval = 1
		case strings.HasPrefix(d.Item, "UNTIL="):
			o.Until = time.Time{}
		default:
			clearGroup(&o, d.Item)
		}
		trial, err := NewRRule(o)
		if err != nil {
			t.Fatalf("ablation trial for %s invalid: %v", d.Item, err)
		}
		first, ok := trial.Iterator()()
		if !ok {
			t.Errorf("decisive item %s claims witness %s, but relaxed rule is empty within the horizon",
				d.Item, d.RemovingWitness)
			continue
		}
		if !first.Equal(d.RemovingWitness) {
			t.Errorf("decisive item %s removing-witness = %s, relaxed iterator first = %s",
				d.Item, d.RemovingWitness, first)
		}
	}
}

func clearGroup(o *ROption, token string) {
	switch {
	case strings.HasPrefix(token, "BYMONTHDAY="):
		o.Bymonthday = nil
	case strings.HasPrefix(token, "BYMONTH="):
		o.Bymonth = nil
	case strings.HasPrefix(token, "BYWEEKNO="):
		o.Byweekno = nil
	case strings.HasPrefix(token, "BYYEARDAY="):
		o.Byyearday = nil
	case strings.HasPrefix(token, "BYDAY="):
		o.Byweekday = nil
	case strings.HasPrefix(token, "BYSETPOS="):
		o.Bysetpos = nil
	}
}

// TestAnalyzeEmptyYearly covers February 30 (empty), February 29 with
// various intervals, and the DTSTART/INTERVAL phase requirements.
func TestAnalyzeEmptyYearly(t *testing.T) {
	runAnalyzeCases(t, []analyzeCase{
		{
			name: "february-30-never-exists",
			options: ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{30},
				Dtstart: midnight(2000, time.January, 1)},
			want:           EmptyProven,
			bounded:        false,
			decisiveTokens: []string{"BYMONTH=2", "BYMONTHDAY=30"},
		},
		{
			name: "february-29-interval-4-from-odd-year-empty",
			options: ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{29},
				Interval: 4, Dtstart: midnight(2003, time.January, 1)},
			want:           EmptyProven,
			bounded:        false,
			decisiveTokens: []string{"BYMONTH=2", "BYMONTHDAY=29", "INTERVAL=4"},
		},
		{
			name: "february-29-interval-4-from-leap-day-witness",
			options: ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{29},
				Dtstart: midnight(2000, time.February, 29)},
			want:    HasWitness,
			witness: midnight(2000, time.February, 29),
		},
		{
			name: "february-29-interval-100-residue-empty",
			options: ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{29},
				Interval: 100, Dtstart: midnight(2002, time.January, 1)},
			want:           EmptyProven,
			bounded:        false,
			decisiveTokens: []string{"BYMONTH=2", "BYMONTHDAY=29", "INTERVAL=100"},
		},
		{
			name:    "plain-yearly-always-witness",
			options: ROption{Freq: YEARLY, Dtstart: midnight(2021, time.June, 15)},
			want:    HasWitness,
			witness: midnight(2021, time.June, 15),
		},
	})
}

// TestAnalyzeEmptyIntersections covers BYDAY intersecting BYMONTHDAY.
func TestAnalyzeEmptyIntersections(t *testing.T) {
	runAnalyzeCases(t, []analyzeCase{
		{
			name: "5th-monday-of-1st-never",
			options: ROption{Freq: MONTHLY, Bymonthday: []int{1},
				Byweekday: []Weekday{MO.Nth(5)}, Dtstart: midnight(2000, time.January, 1)},
			want: EmptyProven,
			// The full phase cycle (4800 months) is ~400 years and fits
			// beyond the horizon only for the mathematical check; the
			// library horizon is shorter, but the conjunction is
			// intrinsically impossible, so bounded=false.
			bounded:        false,
			decisiveTokens: []string{"BYMONTHDAY=1", "BYDAY=+5MO"},
		},
		{
			name: "5th-monday-of-month-can-exist",
			options: ROption{Freq: MONTHLY, Byweekday: []Weekday{MO.Nth(5)},
				Dtstart: midnight(2000, time.January, 1)},
			want:    HasWitness,
			witness: midnight(2000, time.January, 31),
		},
		{
			name: "weekly-contradictory-day-filters",
			options: ROption{Freq: WEEKLY, Bymonthday: []int{31}, Byyearday: []int{1},
				Dtstart: midnight(2000, time.January, 1)},
			want:           EmptyProven,
			bounded:        false,
			decisiveTokens: []string{"BYMONTHDAY=31", "BYYEARDAY=1"},
		},
		{
			name: "february-day-31-intersection-empty",
			options: ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{31},
				Byweekday: []Weekday{MO}, Dtstart: midnight(2000, time.January, 1)},
			want:           EmptyProven,
			bounded:        false,
			decisiveTokens: []string{"BYMONTH=2", "BYMONTHDAY=31"},
		},
	})
}

func TestAnalyzeEmptyMonthlyWeeklyDaily(t *testing.T) {
	runAnalyzeCases(t, []analyzeCase{
		{
			name: "monthly-31-interval-2-witness",
			options: ROption{Freq: MONTHLY, Bymonthday: []int{31}, Interval: 2,
				Dtstart: midnight(2000, time.January, 1)},
			want:    HasWitness,
			witness: midnight(2000, time.January, 31),
		},
		{
			name: "monthly-30-interval-2-feb-start-witness-april",
			options: ROption{Freq: MONTHLY, Bymonthday: []int{30}, Interval: 2,
				Dtstart: midnight(2000, time.February, 1)},
			want:    HasWitness,
			witness: midnight(2000, time.April, 30),
		},
		{
			name: "monthly-feb-29-interval-2-first-2004",
			options: ROption{Freq: MONTHLY, Bymonth: []int{2}, Bymonthday: []int{29},
				Interval: 2, Dtstart: midnight(2001, time.February, 1)},
			want:    HasWitness,
			witness: midnight(2004, time.February, 29),
		},
		{
			name: "daily-friday-interval-7-from-monday-empty",
			options: ROption{Freq: DAILY, Byweekday: []Weekday{FR}, Interval: 7,
				Dtstart: midnight(2000, time.January, 3)},
			want:           EmptyProven,
			bounded:        false,
			decisiveTokens: []string{"BYDAY=FR", "INTERVAL=7"},
		},
		{
			name:    "daily-plain-witness",
			options: ROption{Freq: DAILY, Dtstart: midnight(2020, time.January, 1)},
			want:    HasWitness,
			witness: midnight(2020, time.January, 1),
		},
		{
			name:    "weekly-plain-interval-2-witness",
			options: ROption{Freq: WEEKLY, Interval: 2, Dtstart: midnight(2018, time.December, 30)},
			want:    HasWitness,
			witness: midnight(2018, time.December, 30),
		},
	})
}

func TestAnalyzeEmptyUntilCount(t *testing.T) {
	// Explicit finite UNTIL.
	t.Run("until-before-first-survivor", func(t *testing.T) {
		r, _ := NewRRule(ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{29},
			Dtstart: midnight(2001, time.January, 1), Until: midnight(2003, time.December, 31)})
		res := r.AnalyzeEmpty()
		if res.Status != EmptyProven || !res.Bounded {
			t.Fatalf("got status=%s bounded=%v, want EmptyProven bounded=true", res.Status, res.Bounded)
		}
		if _, ok := r.Iterator()(); ok {
			t.Fatal("iterator should be empty")
		}
		assertTokens(t, res.Decisive, []string{"BYMONTH=2", "BYMONTHDAY=29", "UNTIL=20031231"})
	})

	t.Run("until-after-first-survivor", func(t *testing.T) {
		r, _ := NewRRule(ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{29},
			Dtstart: midnight(2000, time.January, 1), Until: midnight(2003, time.January, 1)})
		res := r.AnalyzeEmpty()
		if res.Status != HasWitness || !res.Witness.Equal(midnight(2000, time.February, 29)) {
			t.Fatalf("got status=%s witness=%s", res.Status, res.Witness)
		}
	})

	t.Run("until-precedes-dtstart", func(t *testing.T) {
		r, _ := NewRRule(ROption{Freq: DAILY, Dtstart: midnight(2010, time.January, 1),
			Until: midnight(2009, time.January, 1)})
		res := r.AnalyzeEmpty()
		if res.Status != EmptyProven || !res.Bounded {
			t.Fatalf("got status=%s bounded=%v", res.Status, res.Bounded)
		}
		if len(res.Decisive) != 1 || res.Decisive[0].Item != "UNTIL=20090101" {
			t.Fatalf("decisive = %v", res.Decisive)
		}
	})

	// COUNT=0 means unlimited; COUNT>0 never changes emptiness.
	t.Run("count-zero-is-unlimited", func(t *testing.T) {
		r, _ := NewRRule(ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{29},
			Dtstart: midnight(2000, time.January, 1), Count: 0})
		res := r.AnalyzeEmpty()
		if res.Status != HasWitness || res.CountLimited {
			t.Fatalf("got status=%s countLimited=%v", res.Status, res.CountLimited)
		}
	})
	t.Run("count-positive-empty-rule-still-empty", func(t *testing.T) {
		r, _ := NewRRule(ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{30},
			Dtstart: midnight(2000, time.January, 1), Count: 5})
		res := r.AnalyzeEmpty()
		if res.Status != EmptyProven || !res.CountLimited {
			t.Fatalf("got status=%s countLimited=%v", res.Status, res.CountLimited)
		}
		if _, ok := r.Iterator()(); ok {
			t.Fatal("iterator should be empty")
		}
	})
	t.Run("count-positive-nonempty-rule-witness", func(t *testing.T) {
		r, _ := NewRRule(ROption{Freq: MONTHLY, Dtstart: midnight(2020, time.January, 15), Count: 1})
		res := r.AnalyzeEmpty()
		if res.Status != HasWitness || !res.CountLimited {
			t.Fatalf("got status=%s countLimited=%v", res.Status, res.CountLimited)
		}
	})
}

func TestAnalyzeEmptyUnsupported(t *testing.T) {
	check := func(t *testing.T, o ROption, wantSubstr string) {
		r, err := NewRRule(o)
		if err != nil {
			t.Fatalf("NewRRule: %v", err)
		}
		res := r.AnalyzeEmpty()
		if res.Status != EmptyUnknown {
			t.Fatalf("status=%s want EmptyUnknown", res.Status)
		}
		if !strings.Contains(res.Why, wantSubstr) {
			t.Fatalf("why=%q want substring %q", res.Why, wantSubstr)
		}
	}
	t.Run("hourly", func(t *testing.T) {
		check(t, ROption{Freq: HOURLY, Dtstart: midnight(2000, time.January, 1)}, "DAILY/WEEKLY/MONTHLY/YEARLY")
	})
	t.Run("non-utc", func(t *testing.T) {
		check(t, ROption{Freq: DAILY, Dtstart: time.Date(2000, 1, 1, 0, 0, 0, 0, time.FixedZone("X", 3600))}, "UTC")
	})
	t.Run("intraday-dtstart", func(t *testing.T) {
		check(t, ROption{Freq: DAILY, Dtstart: time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)}, "intraday")
	})
	t.Run("datetime-until", func(t *testing.T) {
		check(t, ROption{Freq: DAILY, Dtstart: midnight(2000, time.January, 1),
			Until: time.Date(2000, 6, 1, 12, 0, 0, 0, time.UTC)}, "UNTIL")
	})
	t.Run("byhour", func(t *testing.T) {
		check(t, ROption{Freq: DAILY, Byhour: []int{9}, Dtstart: midnight(2000, time.January, 1)}, "BYHOUR")
	})
}

func TestAnalyzeEmptyBudget(t *testing.T) {
	r, _ := NewRRule(ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{30},
		Dtstart: midnight(2000, time.January, 1)})
	res := r.AnalyzeEmpty(AnalyzeBudget(10))
	if res.Status != EmptyUnknown || !res.BudgetExhausted {
		t.Fatalf("got status=%s budgetExhausted=%v", res.Status, res.BudgetExhausted)
	}

	// Non-positive budget selects the default and still yields a verdict.
	res = r.AnalyzeEmpty(AnalyzeBudget(0))
	if res.Status != EmptyProven || res.BudgetExhausted {
		t.Fatalf("got status=%s budgetExhausted=%v", res.Status, res.BudgetExhausted)
	}
}

func TestAnalyzeEmptyPackageFunc(t *testing.T) {
	res, err := AnalyzeEmpty(ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{30},
		Dtstart: midnight(2000, time.January, 1)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != EmptyProven {
		t.Fatalf("status=%s", res.Status)
	}

	// Invalid options surface as an error and EmptyUnknown.
	if _, err := AnalyzeEmpty(ROption{Freq: MONTHLY, Bymonthday: []int{32}}); err == nil {
		t.Fatal("expected error for BYMONTHDAY=32")
	}
}

func TestAnalyzeEmptyStatusString(t *testing.T) {
	cases := map[EmptyStatus]string{
		EmptyUnknown: "UNKNOWN",
		EmptyProven:  "EMPTY-PROVEN",
		HasWitness:   "HAS-WITNESS",
	}
	for s, want := range cases {
		if s.String() != want {
			t.Errorf("%v.String()=%q want %q", s, s.String(), want)
		}
	}
}

// TestAnalyzeEmptyDeceptiveInterval covers a rule that is non-empty in the
// mathematical Gregorian sense but whose first survivor sits beyond the
// library's ~292-year horizon (INTERVAL=59; naive 400-year-forward scanning
// visits only 6 years and would wrongly conclude emptiness).
func TestAnalyzeEmptyDeceptiveInterval(t *testing.T) {
	r, _ := NewRRule(ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{29},
		Interval: 59, Dtstart: midnight(2023, time.January, 1)})
	res := r.AnalyzeEmpty()
	if res.Status != EmptyProven || !res.Bounded {
		t.Fatalf("got status=%s bounded=%v; want EmptyProven bounded=true", res.Status, res.Bounded)
	}
	if _, ok := r.Iterator()(); ok {
		t.Fatal("iterator must not emit past its horizon")
	}
	// INTERVAL must be identified as one of the decisive components.
	found := false
	for _, d := range res.Decisive {
		if d.Item == "INTERVAL=59" {
			found = true
			if d.RemovingWitness.IsZero() {
				t.Fatal("INTERVAL decisive item should carry a removing-witness")
			}
		}
	}
	if !found {
		t.Fatalf("INTERVAL=59 not listed as decisive: %v", res.Decisive)
	}

	// The same phase analysis from a leap-year anchor reaches the survivor
	// within the horizon and reports it.
	r2, _ := NewRRule(ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{29},
		Interval: 4, Dtstart: midnight(2000, time.February, 29)})
	if r2.AnalyzeEmpty().Status != HasWitness {
		t.Fatal("leap-anchored INTERVAL=4 Feb 29 should have a witness")
	}
}
