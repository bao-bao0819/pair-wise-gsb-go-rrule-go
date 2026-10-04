// 2017-2022, Teambition. All rights reserved.

package rrule

import (
	"testing"
	"time"
)

func reverseCollect(next Next) []time.Time {
	out := []time.Time{}
	for {
		t, ok := next()
		if !ok {
			return out
		}
		out = append(out, t)
	}
}

func reversedTimes(s []time.Time) []time.Time {
	out := make([]time.Time, len(s))
	for i, t := range s {
		out[len(s)-1-i] = t
	}
	return out
}

func checkReverseMatchesForward(t *testing.T, r *RRule, start, end time.Time) {
	t.Helper()
	want := reversedTimes(r.Between(start, end, true))
	value := reverseCollect(r.ReverseIterator(start, end))
	if !timesEqual(value, want) {
		t.Errorf("get %v, want %v", value, want)
	}
}

func TestReverseIteratorMatchesForward(t *testing.T) {
	dtstart := time.Date(2025, 11, 30, 10, 30, 0, 0, time.UTC)
	cases := []struct {
		name  string
		arg   ROption
		start time.Time
		end   time.Time
	}{
		{"dailyInterval", ROption{Freq: DAILY, Interval: 3,
			Dtstart: time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)},
			time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{"negativeBymonthday", ROption{Freq: MONTHLY, Bymonthday: []int{-1},
			Dtstart: dtstart},
			time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)},
		{"ordinalByday", ROption{Freq: MONTHLY, Byweekday: []Weekday{FR.Nth(-1)},
			Dtstart: dtstart},
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)},
		{"bysetpos", ROption{Freq: MONTHLY,
			Byweekday: []Weekday{MO, TU, WE, TH, FR}, Bysetpos: []int{-1},
			Dtstart: dtstart},
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)},
		{"weeklyCrossYear", ROption{Freq: WEEKLY, Interval: 2, Wkst: SU,
			Byweekday: []Weekday{MO, TH},
			Dtstart:   time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)},
			time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"byweeknoCrossYear", ROption{Freq: YEARLY, Byweekno: []int{1},
			Byweekday: []Weekday{MO},
			Dtstart:   time.Date(2020, 1, 6, 9, 0, 0, 0, time.UTC)},
			time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC)},
		{"leapDay", ROption{Freq: YEARLY, Bymonth: []int{2}, Bymonthday: []int{29},
			Dtstart: time.Date(2016, 2, 29, 12, 0, 0, 0, time.UTC)},
			time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2029, 12, 31, 0, 0, 0, 0, time.UTC)},
		{"hourly", ROption{Freq: HOURLY, Interval: 6,
			Dtstart: time.Date(2026, 4, 1, 3, 30, 0, 0, time.UTC)},
			time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
		{"minutely", ROption{Freq: MINUTELY, Interval: 90,
			Dtstart: time.Date(2026, 4, 1, 0, 15, 0, 0, time.UTC)},
			time.Date(2026, 4, 2, 6, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 2, 18, 0, 0, 0, time.UTC)},
		{"secondly", ROption{Freq: SECONDLY, Interval: 3600,
			Dtstart: time.Date(2026, 4, 1, 0, 0, 30, 0, time.UTC)},
			time.Date(2026, 4, 1, 5, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := NewRRule(c.arg)
			if err != nil {
				t.Fatal(err)
			}
			checkReverseMatchesForward(t, r, c.start, c.end)
		})
	}
}

func TestReverseIteratorBoundariesInclusive(t *testing.T) {
	r, _ := NewRRule(ROption{Freq: DAILY,
		Dtstart: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)})
	start := time.Date(2026, 1, 3, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	want := []time.Time{
		time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 4, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 3, 9, 0, 0, 0, time.UTC),
	}
	if value := reverseCollect(r.ReverseIterator(start, end)); !timesEqual(value, want) {
		t.Errorf("get %v, want %v", value, want)
	}
}

func TestReverseIteratorEmpty(t *testing.T) {
	r, _ := NewRRule(ROption{Freq: DAILY,
		Dtstart: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)})
	// Window entirely before DTSTART.
	if v, ok := r.ReverseIterator(
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))(); ok {
		t.Errorf("get %v, want no occurrence", v)
	}
	// Start after end.
	if v, ok := r.ReverseIterator(
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))(); ok {
		t.Errorf("get %v, want no occurrence", v)
	}
}

func TestReverseIteratorCount(t *testing.T) {
	r, _ := NewRRule(ROption{Freq: DAILY, Count: 10,
		Dtstart: time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)})

	// COUNT limits the first 10 forward occurrences; a window extending
	// far beyond the counted range still yields exactly those 10, reversed.
	want := reversedTimes(r.All())
	value := reverseCollect(r.ReverseIterator(
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC)))
	if !timesEqual(value, want) {
		t.Errorf("get %v, want %v", value, want)
	}

	// A window cutting into the counted range.
	checkReverseMatchesForward(t, r,
		time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
}

func TestReverseIteratorEarlyDtstartNarrowWindow(t *testing.T) {
	// No COUNT: the iterator jumps to the window instead of scanning
	// every occurrence since DTSTART.
	daily, _ := NewRRule(ROption{Freq: DAILY,
		Dtstart: time.Date(1990, 1, 1, 8, 0, 0, 0, time.UTC)})
	start := time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	checkReverseMatchesForward(t, daily, start, end)

	// Take the 10 most recent occurrences of 2026, like a "recent
	// events" view would.
	next := daily.ReverseIterator(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), end)
	for i := 0; i < 10; i++ {
		v, ok := next()
		if !ok {
			t.Fatalf("get no occurrence at position %d", i)
		}
		want := time.Date(2026, 12, 31-i, 8, 0, 0, 0, time.UTC)
		if !v.Equal(want) {
			t.Errorf("get %v, want %v", v, want)
		}
	}

	monthly, _ := NewRRule(ROption{Freq: MONTHLY, Bymonthday: []int{-1},
		Dtstart: time.Date(1990, 1, 31, 8, 0, 0, 0, time.UTC)})
	checkReverseMatchesForward(t, monthly,
		time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC))

	weekly, _ := NewRRule(ROption{Freq: WEEKLY, Interval: 3, Wkst: SU,
		Dtstart: time.Date(1990, 1, 1, 8, 0, 0, 0, time.UTC)})
	checkReverseMatchesForward(t, weekly, start, end)
}

func TestSetReverseIterator(t *testing.T) {
	build := func() *Set {
		set := &Set{}
		r, _ := NewRRule(ROption{Freq: DAILY, Interval: 2,
			Dtstart: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)})
		set.RRule(r)
		// Duplicate RDATE of an existing occurrence, plus a unique one.
		set.RDate(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))
		set.RDate(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))
		set.RDate(time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC))
		// Exclusions exactly on the window boundaries.
		set.ExDate(time.Date(2026, 1, 3, 9, 0, 0, 0, time.UTC))
		set.ExDate(time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC))
		return set
	}
	start := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 15, 23, 59, 59, 0, time.UTC)

	want := reversedTimes(build().Between(start, end, true))
	value := reverseCollect(build().ReverseIterator(start, end))
	if !timesEqual(value, want) {
		t.Errorf("get %v, want %v", value, want)
	}

	// Boundary exclusions actually applied, duplicate RDATE delivered once.
	for _, v := range value {
		if v.Equal(time.Date(2026, 1, 3, 9, 0, 0, 0, time.UTC)) ||
			v.Equal(time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)) {
			t.Errorf("excluded date %v delivered", v)
		}
	}
	count := 0
	for _, v := range value {
		if v.Equal(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)) {
			count++
		}
	}
	if count != 1 {
		t.Errorf("duplicate RDATE delivered %d times, want 1", count)
	}
}

func TestSetReverseIteratorKeepsSlices(t *testing.T) {
	set := &Set{}
	r, _ := NewRRule(ROption{Freq: DAILY,
		Dtstart: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)})
	set.RRule(r)
	// Deliberately unordered.
	set.RDate(time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC))
	set.RDate(time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC))
	set.ExDate(time.Date(2026, 1, 3, 9, 0, 0, 0, time.UTC))
	set.ExDate(time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC))

	rdatesBefore := append([]time.Time(nil), set.GetRDate()...)
	exdatesBefore := append([]time.Time(nil), set.GetExDate()...)

	reverseCollect(set.ReverseIterator(
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)))

	if !timesEqual(set.GetRDate(), rdatesBefore) {
		t.Errorf("rdates modified: get %v, want %v", set.GetRDate(), rdatesBefore)
	}
	if !timesEqual(set.GetExDate(), exdatesBefore) {
		t.Errorf("exdates modified: get %v, want %v", set.GetExDate(), exdatesBefore)
	}
}
