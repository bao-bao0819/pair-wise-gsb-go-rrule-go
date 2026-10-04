package rrule

import (
	"testing"
	"time"
)

func TestAnalyzeEmptyYearlyFebruaryTricks(t *testing.T) {
	dt := func(year int) time.Time { return time.Date(year, time.February, 1, 0, 0, 0, 0, time.UTC) }

	empty := ROption{Freq: YEARLY, Dtstart: dt(2020), Bymonth: []int{2}, Bymonthday: []int{30}}
	analysis := mustAnalyze(t, empty)
	assertEmpty(t, analysis, "BYMONTH=2", "BYMONTHDAY=30")

	nonEmpty := ROption{Freq: YEARLY, Interval: 4, Dtstart: dt(2020), Bymonth: []int{2}, Bymonthday: []int{29}}
	assertWitness(t, mustAnalyze(t, nonEmpty), time.Date(2020, time.February, 29, 0, 0, 0, 0, time.UTC))
	assertIteratorReaches(t, nonEmpty, time.Date(2020, time.February, 29, 0, 0, 0, 0, time.UTC))

	phaseEmpty := ROption{Freq: YEARLY, Interval: 4, Dtstart: dt(2021), Bymonth: []int{2}, Bymonthday: []int{29}}
	assertEmpty(t, mustAnalyze(t, phaseEmpty), "INTERVAL=4", "DTSTART=20210201T000000Z", "BYMONTHDAY=29")

	centuryWitness := ROption{Freq: YEARLY, Interval: 100, Dtstart: time.Date(2000, time.February, 1, 0, 0, 0, 0, time.UTC), Bymonth: []int{2}, Bymonthday: []int{29}}
	assertWitness(t, mustAnalyze(t, centuryWitness), time.Date(2000, time.February, 29, 0, 0, 0, 0, time.UTC))

	centuryEmpty := ROption{Freq: YEARLY, Interval: 100, Dtstart: dt(2001), Bymonth: []int{2}, Bymonthday: []int{29}}
	assertEmpty(t, mustAnalyze(t, centuryEmpty), "INTERVAL=100", "DTSTART=20010201T000000Z", "BYMONTHDAY=29")
}

func TestAnalyzeEmptyExplicitDtstartPhase(t *testing.T) {
	implicit := ROption{Freq: YEARLY, Interval: 4, Dtstart: time.Date(2021, time.March, 1, 0, 0, 0, 0, time.UTC), Bymonth: []int{2}, Bymonthday: []int{29}}
	analysis := mustAnalyze(t, implicit)
	assertEmpty(t, analysis, "INTERVAL=4", "DTSTART=20210301T000000Z", "BYMONTHDAY=29")
}

func TestAnalyzeEmptyBydayBymonthdayIntersection(t *testing.T) {
	option := ROption{
		Freq:       MONTHLY,
		Dtstart:    time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
		Bymonthday: []int{15},
		Byweekday:  []Weekday{MO.Nth(2)},
	}
	analysis := mustAnalyze(t, option)
	assertEmpty(t, analysis, "BYMONTHDAY=15", "BYDAY=+2MO")
}

func TestAnalyzeEmptyWeeklyAndDailyPhase(t *testing.T) {
	weekly := ROption{Freq: WEEKLY, Interval: 2, Dtstart: time.Date(2023, time.January, 2, 0, 0, 0, 0, time.UTC)}
	assertWitness(t, mustAnalyze(t, weekly), time.Date(2023, time.January, 2, 0, 0, 0, 0, time.UTC))

	daily := ROption{Freq: DAILY, Interval: 4, Dtstart: time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC), Bymonth: []int{2}, Bymonthday: []int{30}}
	assertEmpty(t, mustAnalyze(t, daily), "BYMONTH=2", "BYMONTHDAY=30")

	dailyWitness := ROption{Freq: DAILY, Interval: 4, Dtstart: time.Date(2020, time.February, 29, 0, 0, 0, 0, time.UTC), Bymonth: []int{2}, Bymonthday: []int{29}}
	assertWitness(t, mustAnalyze(t, dailyWitness), time.Date(2020, time.February, 29, 0, 0, 0, 0, time.UTC))
}

func TestAnalyzeEmptyUntilAndCount(t *testing.T) {
	beforeUntil := ROption{
		Freq:       MONTHLY,
		Dtstart:    time.Date(2020, time.February, 1, 0, 0, 0, 0, time.UTC),
		Bymonthday: []int{30},
		Until:      time.Date(2020, time.February, 27, 0, 0, 0, 0, time.UTC),
	}
	analysis := mustAnalyze(t, beforeUntil)
	if analysis.Status != AnalyzeEmptyConfirmed {
		t.Fatalf("status = %s, want empty", analysis.Status)
	}
	assertEmpty(t, analysis, "DTSTART=20200201T000000Z", "UNTIL=20200227T000000Z", "BYMONTHDAY=30")

	witnessUntil := ROption{
		Freq:       MONTHLY,
		Dtstart:    time.Date(2020, time.February, 1, 0, 0, 0, 0, time.UTC),
		Bymonthday: []int{31},
		Until:      time.Date(2020, time.March, 31, 0, 0, 0, 0, time.UTC),
	}
	assertWitness(t, mustAnalyze(t, witnessUntil), time.Date(2020, time.March, 31, 0, 0, 0, 0, time.UTC))

	countZero := ROption{Freq: YEARLY, Dtstart: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Count: 0}
	assertWitness(t, mustAnalyze(t, countZero), time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC))

	countZeroContradiction := ROption{Freq: YEARLY, Count: 0, Dtstart: time.Date(2020, time.February, 1, 0, 0, 0, 0, time.UTC), Bymonth: []int{2}, Bymonthday: []int{30}}
	assertEmpty(t, mustAnalyze(t, countZeroContradiction), "BYMONTH=2", "BYMONTHDAY=30")

	countNegative := ROption{Freq: YEARLY, Count: -1, Dtstart: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)}
	assertWitness(t, mustAnalyze(t, countNegative), time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC))
}

func TestAnalyzeEmptyBysetpos(t *testing.T) {
	option := ROption{
		Freq:       MONTHLY,
		Dtstart:    time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
		Bymonthday: []int{1},
		Bysetpos:   []int{2},
	}
	assertEmpty(t, mustAnalyze(t, option), "BYSETPOS=2", "BYMONTHDAY=1")
}

func TestAnalyzeEmptyExistingDateFilters(t *testing.T) {
	yearDay := ROption{Freq: YEARLY, Interval: 4, Dtstart: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Byyearday: []int{366}}
	assertWitness(t, mustAnalyze(t, yearDay), time.Date(2020, time.December, 31, 0, 0, 0, 0, time.UTC))

	weekNo := ROption{Freq: YEARLY, Interval: 4, Dtstart: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC), Byweekno: []int{53}, Byweekday: []Weekday{MO}}
	assertWitness(t, mustAnalyze(t, weekNo), time.Date(2043, time.December, 28, 0, 0, 0, 0, time.UTC))

	negativeMonthDay := ROption{Freq: MONTHLY, Dtstart: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC), Bymonthday: []int{-1}}
	assertWitness(t, mustAnalyze(t, negativeMonthDay), time.Date(2023, time.January, 31, 0, 0, 0, 0, time.UTC))

	weekNoMonthEmpty := ROption{Freq: YEARLY, Dtstart: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC), Bymonth: []int{6}, Byweekno: []int{1}, Byweekday: []Weekday{MO}}
	assertEmpty(t, mustAnalyze(t, weekNoMonthEmpty), "BYWEEKNO=1", "BYMONTH=6")
}

func TestAnalyzeEmptyBudgetAndUnsupported(t *testing.T) {
	option := ROption{Freq: YEARLY, Dtstart: time.Date(2020, time.February, 1, 0, 0, 0, 0, time.UTC), Bymonth: []int{2}, Bymonthday: []int{30}}
	rule, err := NewRRule(option)
	if err != nil {
		t.Fatal(err)
	}
	analysis := rule.AnalyzeEmpty(0)
	if analysis.Status != AnalyzeEmptyUnknown || !analysis.BudgetExhausted {
		t.Fatalf("analysis = %+v, want budget-exhausted unknown", analysis)
	}

	intraday := ROption{Freq: DAILY, Dtstart: time.Date(2020, time.January, 1, 1, 0, 0, 0, time.UTC)}
	rule, err = NewRRule(intraday)
	if err != nil {
		t.Fatal(err)
	}
	if rule.AnalyzeEmpty().Status != AnalyzeEmptyUnknown {
		t.Fatal("intraday rule was analyzed")
	}

	location := time.FixedZone("test", 60)
	local := ROption{Freq: DAILY, Dtstart: time.Date(2020, time.January, 1, 0, 0, 0, 0, location)}
	rule, err = NewRRule(local)
	if err != nil {
		t.Fatal(err)
	}
	if rule.AnalyzeEmpty().Status != AnalyzeEmptyUnknown {
		t.Fatal("non-UTC rule was analyzed")
	}
}

func TestAnalyzeEmptyCivilDates(t *testing.T) {
	for day := -1_000_000; day <= 1_000_000; day++ {
		date := civilFromDays(int64(day))
		if daysFromCivil(date) != day {
			t.Fatalf("round trip day %d gave %04d-%02d-%02d", day, date.year, date.month, date.day)
		}
	}
}

func TestAnalyzeEmptyAgreesWithIterator(t *testing.T) {
	start := time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC)
	filters := []func() ROption{
		func() ROption { return ROption{Bymonth: []int{2}, Bymonthday: []int{29}} },
		func() ROption { return ROption{Bymonthday: []int{31}} },
		func() ROption { return ROption{Byweekday: []Weekday{MO, FR}} },
		func() ROption { return ROption{Byweekday: []Weekday{MO.Nth(2)}} },
		func() ROption { return ROption{Byyearday: []int{1, -1, 366}} },
		func() ROption { return ROption{Byweekno: []int{1, 53}, Byweekday: []Weekday{MO}} },
		func() ROption { return ROption{Bymonthday: []int{15}, Byweekday: []Weekday{MO.Nth(2)}} },
	}

	for freq := DAILY; freq <= YEARLY; freq++ {
		for _, interval := range []int{1, 2, 3, 4, 5, 7, 8, 13, 16, 17, 25, 29, 31, 50, 97, 100, 101, 200, 365, 400, 500, 1461} {
			for _, startOffset := range []int{0, 31, 59, 60, 120, 365, 730, 1000} {
				ruleStart := start.AddDate(0, 0, startOffset)
				for index, newFilter := range filters {
					option := newFilter()
					option.Freq = Frequency(freq)
					option.Interval = interval
					option.Dtstart = ruleStart
					if option.Freq == WEEKLY && index == 5 {
						continue
					}
					rule, err := NewRRule(option)
					if err != nil {
						t.Fatal(err)
					}
					analysis := rule.AnalyzeEmpty(2_000_000)
					if analysis.BudgetExhausted {
						t.Fatalf("budget exhausted for freq=%d interval=%d filter=%d start=%d", freq, interval, index, startOffset)
					}
					hasIteratorResult := iteratorHasResult(rule)
					if (analysis.Status == AnalyzeEmptyNonEmpty) != hasIteratorResult {
						t.Fatalf("freq=%d interval=%d filter=%d start=%d analysis=%s iterator=%v witness=%v rules=%v",
							freq, interval, index, startOffset, analysis.Status, hasIteratorResult, analysis.Witness, analysis.DecisiveRules)
					}
					if analysis.Status == AnalyzeEmptyNonEmpty && !iteratorReaches(rule, analysis.Witness) {
						t.Fatalf("witness %v not produced by iterator", analysis.Witness)
					}
				}
			}
		}
	}
}

func iteratorHasResult(rule *RRule) bool {
	value, ok := rule.Iterator()()
	return ok && !value.IsZero()
}

func iteratorReaches(rule *RRule, target time.Time) bool {
	next := rule.Iterator()
	for {
		value, ok := next()
		if !ok || value.After(target) {
			return false
		}
		if value.Equal(target) {
			return true
		}
	}
}

func mustAnalyze(t *testing.T, option ROption) EmptyAnalysis {
	t.Helper()
	rule, err := NewRRule(option)
	if err != nil {
		t.Fatal(err)
	}
	return rule.AnalyzeEmpty()
}

func assertIteratorReaches(t *testing.T, option ROption, target time.Time) {
	t.Helper()
	rule, err := NewRRule(option)
	if err != nil {
		t.Fatal(err)
	}
	next := rule.Iterator()
	for {
		value, ok := next()
		if !ok {
			t.Fatalf("iterator did not reach %v", target)
		}
		if value.Equal(target) {
			return
		}
		if value.After(target) {
			t.Fatalf("iterator passed %v with %v", target, value)
		}
	}
}

func assertEmpty(t *testing.T, analysis EmptyAnalysis, rules ...string) {
	t.Helper()
	if analysis.Status != AnalyzeEmptyConfirmed {
		t.Fatalf("status = %s witness=%v, want empty", analysis.Status, analysis.Witness)
	}
	if analysis.BudgetExhausted {
		t.Fatal("confirmed analysis reported budget exhaustion")
	}
	for _, rule := range rules {
		if !containsString(analysis.DecisiveRules, rule) {
			t.Fatalf("decisive rules = %v, missing %q", analysis.DecisiveRules, rule)
		}
	}
}

func assertWitness(t *testing.T, analysis EmptyAnalysis, want time.Time) {
	t.Helper()
	if analysis.Status != AnalyzeEmptyNonEmpty {
		t.Fatalf("status = %s, want non-empty", analysis.Status)
	}
	if analysis.Witness.IsZero() {
		t.Fatal("non-empty analysis returned zero witness")
	}
	if !analysis.Witness.Equal(want) {
		t.Fatalf("witness = %v, want %v", analysis.Witness, want)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
