package rrule

import (
	"math/rand"
	"testing"
	"time"
)

func legacyBysetPosRule(option ROption) RRule {
	bysetpos := option.Bysetpos
	option.Bysetpos = nil
	legacy := buildRRule(option)
	legacy.bysetpos = bysetpos
	return legacy
}

func TestFactorBySetPosMatchesLegacy(t *testing.T) {
	random := rand.New(rand.NewSource(1))

	for caseIndex := 0; caseIndex < 300; caseIndex++ {
		freq := Frequency(random.Intn(4))
		start := factorTestStart(freq, random)
		option := ROption{
			Freq:       freq,
			Dtstart:    start,
			Interval:   1 + random.Intn(2),
			Byhour:     shuffledInts(randomUnique(random, 24, 1+random.Intn(6)), random),
			Byminute:   shuffledInts(randomUnique(random, 60, 1+random.Intn(8)), random),
			Bysecond:   shuffledInts(randomUnique(random, 60, 1+random.Intn(8)), random),
			Bysetpos:   factorTestPositions(random),
			Bymonthday: factorTestMonthDays(freq, random),
			Byweekday:  factorTestWeekdays(freq, random),
		}
		if freq == YEARLY && random.Intn(2) == 0 {
			option.Bymonth = []int{1, 2}
		}

		if random.Intn(2) == 0 {
			option.Count = 1 + random.Intn(20)
		} else {
			option.Until = factorTestUntil(freq, start, random)
		}

		optimized, err := NewRRule(option)
		if err != nil {
			t.Fatalf("case %d: NewRRule returned error: %v, option: %#v", caseIndex, err, option)
		}
		legacyRule := legacyBysetPosRule(option)

		got := optimized.All()
		want := legacyRule.All()
		if !timesEqual(got, want) {
			t.Fatalf("case %d: factorized BYSETPOS mismatch\noption: %#v\ngot: %v\nwant: %v", caseIndex, option, got, want)
		}
	}
}

func TestFactorBySetPosFullDayDoesNotAllocateTimeSet(t *testing.T) {
	r, err := NewRRule(ROption{
		Freq:     DAILY,
		Dtstart:  time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
		Count:    2,
		Byhour:   rang(0, 24),
		Byminute: rang(0, 60),
		Bysecond: rang(0, 60),
		Bysetpos: []int{1, -1},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !r.factorizedBysetPos {
		t.Fatal("expected full BYHOUR/BYMINUTE/BYSECOND rule to use factorized BYSETPOS")
	}
	if cap(r.timeset) != 0 {
		t.Fatalf("expected no full timeset allocation, got capacity %d", cap(r.timeset))
	}

	want := []time.Time{
		time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.January, 1, 23, 59, 59, 0, time.UTC),
	}
	if got := r.All(); !timesEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFactorBySetPosNamedLocationDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("time zone database is unavailable: %v", err)
	}

	for _, start := range []time.Time{
		time.Date(2020, time.March, 7, 0, 0, 0, 0, loc),
		time.Date(2020, time.November, 1, 0, 0, 0, 0, loc),
	} {
		option := ROption{
			Freq:     DAILY,
			Dtstart:  start,
			Count:    4,
			Byhour:   rang(0, 24),
			Byminute: rang(0, 60),
			Bysecond: rang(0, 60),
			Bysetpos: []int{1, -1},
		}
		optimized, err := NewRRule(option)
		if err != nil {
			t.Fatal(err)
		}
		if !optimized.factorizedBysetPos {
			t.Fatal("expected factorized BYSETPOS for fixed-offset nominal base times")
		}

		legacyRule := legacyBysetPosRule(option)
		if got, want := optimized.All(), legacyRule.All(); !timesEqual(got, want) {
			t.Fatalf("start %s: got %v, want %v", start.Format(time.RFC3339), got, want)
		}
	}
}

func TestFactorBySetPosNamedLocationSpringGapDeduplicates(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("time zone database is unavailable: %v", err)
	}

	option := ROption{
		Freq:     DAILY,
		Dtstart:  time.Date(2020, time.March, 8, 0, 0, 0, 0, loc),
		Count:    1,
		Byhour:   []int{1, 2, 3},
		Byminute: []int{0},
		Bysecond: []int{0},
		Bysetpos: []int{3, 4},
	}
	optimized, err := NewRRule(option)
	if err != nil {
		t.Fatal(err)
	}

	legacyRule := legacyBysetPosRule(option)
	want := []time.Time{time.Date(2020, time.March, 8, 3, 0, 0, 0, loc)}
	if got := optimized.All(); !timesEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := legacyRule.All(); !timesEqual(got, want) {
		t.Fatalf("legacy got %v, want %v", got, want)
	}
}

func TestFactorBySetPosFallsBackWhenUnsafe(t *testing.T) {
	tests := []struct {
		name   string
		option ROption
	}{
		{
			name: "duplicate hour",
			option: ROption{
				Freq:     DAILY,
				Dtstart:  time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
				Count:    1,
				Byhour:   []int{1, 1, 2},
				Byminute: []int{0, 1},
				Bysecond: []int{0, 1},
				Bysetpos: []int{1, -1},
			},
		},
		{
			name: "hourly frequency",
			option: ROption{
				Freq:     HOURLY,
				Dtstart:  time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
				Count:    1,
				Byhour:   []int{1, 2},
				Byminute: []int{0, 1},
				Bysecond: []int{0, 1},
				Bysetpos: []int{1, -1},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := NewRRule(test.option)
			if err != nil {
				t.Fatal(err)
			}
			if r.factorizedBysetPos {
				t.Fatal("expected legacy BYSETPOS path")
			}

			legacyRule := legacyBysetPosRule(test.option)
			if got, want := r.All(), legacyRule.All(); !timesEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func randomUnique(random *rand.Rand, bound, count int) []int {
	return random.Perm(bound)[:count]
}

func shuffledInts(values []int, random *rand.Rand) []int {
	random.Shuffle(len(values), func(i, j int) {
		values[i], values[j] = values[j], values[i]
	})
	return values
}

func factorTestPositions(random *rand.Rand) []int {
	positions := []int{1, 1, -1, -1}
	for _, position := range randomUnique(random, 16, 1+random.Intn(4)) {
		position -= 8
		if position >= 0 {
			position++
		}
		if position != 0 {
			positions = append(positions, position)
		}
	}
	return shuffledInts(positions, random)
}

func factorTestMonthDays(freq Frequency, random *rand.Rand) []int {
	if freq == WEEKLY || random.Intn(2) == 0 {
		return nil
	}
	days := randomUnique(random, 28, 1+random.Intn(5))
	for index := range days {
		days[index]++
	}
	return days
}

func factorTestWeekdays(freq Frequency, random *rand.Rand) []Weekday {
	if freq == YEARLY && random.Intn(2) == 0 {
		return nil
	}
	days := randomUnique(random, 7, 1+random.Intn(4))
	result := make([]Weekday, len(days))
	for index, day := range days {
		result[index] = Weekday{weekday: day}
	}
	return result
}

func factorTestStart(freq Frequency, random *rand.Rand) time.Time {
	base := time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC)
	switch freq {
	case YEARLY:
		return base.AddDate(0, 0, random.Intn(60))
	case MONTHLY:
		return base.AddDate(0, 0, random.Intn(28))
	case WEEKLY:
		return base.AddDate(0, 0, random.Intn(14))
	default:
		return base.AddDate(0, 0, random.Intn(7))
	}
}

func factorTestUntil(freq Frequency, start time.Time, random *rand.Rand) time.Time {
	switch freq {
	case YEARLY:
		return start.AddDate(1+random.Intn(2), 0, 0)
	case MONTHLY:
		return start.AddDate(0, 1+random.Intn(4), 0)
	case WEEKLY:
		return start.AddDate(0, 0, 7+random.Intn(35))
	default:
		return start.AddDate(0, 0, 1+random.Intn(13))
	}
}
