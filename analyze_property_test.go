// 2017-2022, Teambition. All rights reserved.

package rrule

import (
	"math/rand"
	"testing"
	"time"
)

// TestAnalyzeEmptyDifferential compares the analyzer against the concrete
// iterator over many randomly generated supported rules: whenever the
// iterator yields a first occurrence the analyzer must return HasWitness with
// exactly that date; whenever the iterator is empty the analyzer must not
// report a reachable witness.
func TestAnalyzeEmptyDifferential(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for iter := 0; iter < 400; iter++ {
		freq := []Frequency{YEARLY, MONTHLY, WEEKLY, DAILY}[rnd.Intn(4)]
		o := ROption{Freq: freq, Dtstart: midnight(2000+rnd.Intn(30),
			time.Month(1+rnd.Intn(12)), 1+rnd.Intn(28))}
		o.Interval = []int{1, 1, 2, 3, 4, 6, 7, 12}[rnd.Intn(8)]
		switch rnd.Intn(6) {
		case 0:
			o.Bymonth = randMonths(rnd)
			o.Bymonthday = randMonthdays(rnd)
		case 1:
			o.Bymonthday = randMonthdays(rnd)
		case 2:
			o.Byweekday = randWeekdays(rnd, freq)
		case 3:
			o.Bymonth = randMonths(rnd)
			o.Bymonthday = randMonthdays(rnd)
			o.Byweekday = randWeekdays(rnd, freq)
		case 4:
			o.Byyearday = []int{1 + rnd.Intn(366)}
		}
		if rnd.Intn(6) == 0 {
			o.Bysetpos = []int{1 + rnd.Intn(5)}
		}
		if rnd.Intn(8) == 0 {
			o.Until = midnight(2000+rnd.Intn(50), time.Month(1+rnd.Intn(12)), 1+rnd.Intn(28))
		}

		r, err := NewRRule(o)
		if err != nil {
			continue
		}
		res := r.AnalyzeEmpty()
		first, has := r.Iterator()()

		if has {
			if res.Status != HasWitness || !res.Witness.Equal(first) {
				t.Fatalf("rule %s\n iterator first=%s but analyzer status=%s witness=%s (%s)",
					o.RRuleString(), first, res.Status, res.Witness, res.Why)
			}
		} else if res.Status == HasWitness {
			t.Fatalf("rule %s\n analyzer witness=%s but iterator empty", o.RRuleString(), res.Witness)
		}
	}
}

func randMonths(rnd *rand.Rand) []int {
	out := make([]int, 1+rnd.Intn(2))
	for i := range out {
		out[i] = 1 + rnd.Intn(12)
	}
	return out
}

func randMonthdays(rnd *rand.Rand) []int {
	out := make([]int, 1+rnd.Intn(2))
	for i := range out {
		v := 1 + rnd.Intn(31)
		if rnd.Intn(4) == 0 {
			v = -v
		}
		out[i] = v
	}
	return out
}

func randWeekdays(rnd *rand.Rand, freq Frequency) []Weekday {
	all := []Weekday{MO, TU, WE, TH, FR, SA, SU}
	out := make([]Weekday, 1+rnd.Intn(3))
	for i := range out {
		w := all[rnd.Intn(7)]
		if (freq == MONTHLY || freq == YEARLY) && rnd.Intn(3) == 0 {
			w = w.Nth(1 + rnd.Intn(5))
		}
		out[i] = w
	}
	return out
}
