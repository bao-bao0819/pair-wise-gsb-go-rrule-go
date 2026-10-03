package rrule

import (
	"math/rand"
	"runtime"
	"testing"
	"time"
)

// referenceRule builds a rule that is forced through the original
// materialized-timeset code path, serving as the reference implementation
// for the factorized BYSETPOS path.
func referenceRule(t *testing.T, opt ROption) *RRule {
	t.Helper()
	r, err := NewRRule(opt)
	if err != nil {
		t.Fatalf("NewRRule(%+v) error: %v", opt, err)
	}
	if r.timesetFactor != nil {
		r.timesetFactor = nil
		if r.freq < HOURLY {
			r.buildTimeSet()
		}
	}
	return r
}

func randomSubset(rng *rand.Rand, n int) []int {
	size := 1 + rng.Intn(6)
	set := rng.Perm(n)[:size]
	if rng.Intn(10) == 0 {
		// Duplicate value: the factorized path must refuse these rules and
		// fall back to the materialized timeset.
		set = append(set, set[rng.Intn(len(set))])
	}
	return set
}

func randomBysetpos(rng *rand.Rand, timeSetSize int) []int {
	size := 1 + rng.Intn(3)
	pos := make([]int, 0, size)
	for len(pos) < size {
		// Keep positions within the period's time set so they always land
		// on a day; out-of-range positions would produce no output at all,
		// which neither COUNT nor UNTIL can terminate (the iterator would
		// scan to MAXYEAR on both code paths).
		v := 1 + rng.Intn(timeSetSize)
		if rng.Intn(2) == 0 {
			v = -v
		}
		pos = append(pos, v)
	}
	return pos
}

// TestFactorizedBySetPosMatchesMaterialized compares the factorized BYSETPOS
// path against the materialized-timeset path on randomly generated rules of
// all frequencies.
func TestFactorizedBySetPosMatchesMaterialized(t *testing.T) {
	rng := rand.New(rand.NewSource(20261004))
	freqs := []Frequency{YEARLY, MONTHLY, WEEKLY, DAILY, HOURLY, MINUTELY, SECONDLY}
	for i := 0; i < 300; i++ {
		freq := freqs[rng.Intn(len(freqs))]
		dtstart := time.Date(1990+rng.Intn(110), time.Month(1+rng.Intn(12)), 1+rng.Intn(28),
			rng.Intn(24), rng.Intn(60), rng.Intn(60), 0, time.UTC)
		opt := ROption{
			Freq:     freq,
			Dtstart:  dtstart,
			Interval: 1 + rng.Intn(3),
			Count:    1 + rng.Intn(120),
		}
		if freq >= HOURLY {
			// With INTERVAL > 1 the sub-day increment loop steps through a
			// fixed congruence class and may never reach the BYHOUR/
			// BYMINUTE/BYSECOND values, spinning forever on both code
			// paths; keep the interval at 1 for sub-day frequencies.
			opt.Interval = 1
		}
		if rng.Intn(4) != 0 {
			opt.Byhour = randomSubset(rng, 24)
		}
		if rng.Intn(4) != 0 {
			opt.Byminute = randomSubset(rng, 60)
		}
		if rng.Intn(4) != 0 {
			opt.Bysecond = randomSubset(rng, 60)
		}
		if rng.Intn(4) == 0 {
			opt.Bymonth = randomSubset(rng, 12)
			for j := range opt.Bymonth {
				opt.Bymonth[j]++
			}
		}
		if rng.Intn(4) == 0 {
			days := []int{1, 7, 13, 28, -1, -15}
			opt.Bymonthday = []int{days[rng.Intn(len(days))]}
		}
		if rng.Intn(4) == 0 {
			opt.Byweekday = []Weekday{{weekday: rng.Intn(7)}}
		}
		// Size of the period's time set, mirroring the defaults buildRRule
		// applies for dimensions the options leave out.
		dimLen := func(dim []int) int {
			if len(dim) != 0 {
				return len(dim)
			}
			return 1
		}
		timeSetSize := 1
		if freq < HOURLY {
			timeSetSize *= dimLen(opt.Byhour)
		}
		if freq < MINUTELY {
			timeSetSize *= dimLen(opt.Byminute)
		}
		if freq < SECONDLY {
			timeSetSize *= dimLen(opt.Bysecond)
		}
		if timeSetSize > 366 {
			timeSetSize = 366
		}
		opt.Bysetpos = randomBysetpos(rng, timeSetSize)
		// Bound the iteration so that rules which never reach COUNT still
		// terminate quickly; COUNT and UNTIL clipping order is exercised on
		// both code paths.
		switch {
		case freq == SECONDLY:
			opt.Until = dtstart.Add(time.Duration(1+rng.Intn(6)) * time.Hour)
		case freq == MINUTELY:
			opt.Until = dtstart.Add(time.Duration(12+rng.Intn(36)) * time.Hour)
		case freq == HOURLY:
			opt.Until = dtstart.AddDate(0, 0, 2+rng.Intn(8))
		case freq >= WEEKLY:
			if rng.Intn(2) == 0 {
				opt.Until = dtstart.AddDate(0, 0, 30+rng.Intn(300))
			}
		default:
			if rng.Intn(2) == 0 {
				opt.Until = dtstart.AddDate(1+rng.Intn(8), 0, 0)
			}
		}

		rule, err := NewRRule(opt)
		if err != nil {
			t.Fatalf("case %d: NewRRule(%+v) error: %v", i, opt, err)
		}
		ref := referenceRule(t, opt)
		got, want := rule.All(), ref.All()
		if !timesEqual(got, want) || rule.len != ref.len {
			t.Errorf("case %d mismatch\nrule: %s\ngot:  %v\nwant: %v", i, rule.String(), got, want)
		}
	}
}

// TestFactorizedBySetPosDoesNotMaterializeTimeSet uses the widest possible
// BYHOUR/BYMINUTE/BYSECOND dimensions with BYSETPOS=1,-1 and proves that no
// full timeset is allocated: memory stays proportional to the dimension
// lengths and the number of requested positions.
func TestFactorizedBySetPosDoesNotMaterializeTimeSet(t *testing.T) {
	r, err := NewRRule(ROption{
		Freq:     DAILY,
		Dtstart:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Count:    6,
		Byhour:   rang(0, 24),
		Byminute: rang(0, 60),
		Bysecond: rang(0, 60),
		Bysetpos: []int{1, -1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.timesetFactor == nil {
		t.Fatal("want factorized time set, got none")
	}
	if r.timeset != nil {
		t.Errorf("materialized timeset allocated with %d entries", len(r.timeset))
	}
	if got := r.timesetFactor.size; got != 24*60*60 {
		t.Errorf("factorized size = %d, want %d", got, 24*60*60)
	}
	if len(r.timesetFactor.hours) != 24 || len(r.timesetFactor.minutes) != 60 || len(r.timesetFactor.seconds) != 60 {
		t.Errorf("factorized dimensions = %d/%d/%d, want 24/60/60",
			len(r.timesetFactor.hours), len(r.timesetFactor.minutes), len(r.timesetFactor.seconds))
	}

	want := []time.Time{
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC),
		time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 2, 23, 59, 59, 0, time.UTC),
		time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 3, 23, 59, 59, 0, time.UTC),
	}
	if got := r.All(); !timesEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Iterating must stay far below the ~2 MB a materialized 24x60x60
	// timeset would cost per run.
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	const runs = 20
	for i := 0; i < runs; i++ {
		r.All()
	}
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 1<<20 {
		t.Errorf("iteration allocated %d bytes over %d runs, want a factorized set well below 1 MiB", allocated, runs)
	}
}

// TestFactorizedBySetPosHourly covers FREQ=HOURLY, where the hour is pinned
// by the frequency iteration and only BYMINUTE x BYSECOND is factorized. The
// first period must be skipped when DTSTART's clock is not part of the rule,
// exactly like the materialized path.
func TestFactorizedBySetPosHourly(t *testing.T) {
	r, err := NewRRule(ROption{
		Freq:     HOURLY,
		Dtstart:  time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC),
		Count:    2,
		Byhour:   []int{10},
		Byminute: []int{15, 45},
		Bysecond: []int{0},
		Bysetpos: []int{1, -1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.timesetFactor == nil {
		t.Fatal("want factorized time set, got none")
	}
	if got := r.timesetFactor.size; got != 2 {
		t.Errorf("factorized size = %d, want 2", got)
	}
	if r.timesetFactor.hours != nil {
		t.Errorf("hours must be pinned by the frequency iteration, got %v", r.timesetFactor.hours)
	}
	want := []time.Time{
		time.Date(2026, 3, 1, 10, 15, 0, 0, time.UTC),
		time.Date(2026, 3, 1, 10, 45, 0, 0, time.UTC),
	}
	if got := r.All(); !timesEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestTimeSetFactorFallbackOnDuplicates ensures rules whose BYHOUR/BYMINUTE/
// BYSECOND dimensions contain duplicates keep using the materialized
// timeset, which preserves duplicates and their position arithmetic.
func TestTimeSetFactorFallbackOnDuplicates(t *testing.T) {
	r, err := NewRRule(ROption{
		Freq:     DAILY,
		Dtstart:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Count:    3,
		Byhour:   []int{5, 5, 3},
		Bysetpos: []int{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.timesetFactor != nil {
		t.Error("duplicated BYHOUR must fall back to the materialized timeset")
	}
	if len(r.timeset) != 3 {
		t.Errorf("materialized timeset = %d entries, want 3 (duplicates preserved)", len(r.timeset))
	}

	r, err = NewRRule(ROption{
		Freq:     HOURLY,
		Dtstart:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Count:    3,
		Byminute: []int{7, 7},
		Bysetpos: []int{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.timesetFactor != nil {
		t.Error("duplicated BYMINUTE must fall back to the materialized timeset")
	}
}
