package rrule

import (
	"errors"
	"testing"
	"time"
)

func testLeapSecondTable(t *testing.T) *LeapSecondTable {
	t.Helper()

	jan, err := NewLeapSecondDate(2100, time.January, 31)
	if err != nil {
		t.Fatal(err)
	}
	mar, err := NewLeapSecondDate(2100, time.March, 31)
	if err != nil {
		t.Fatal(err)
	}
	table, err := NewLeapSecondTable(jan, mar)
	if err != nil {
		t.Fatal(err)
	}
	return table
}

func TestLeapSecondMonthlyOccurrences(t *testing.T) {
	table := testLeapSecondTable(t)
	rule, err := NewLeapSecondRRule(LeapSecondROption{
		ROption: ROption{
			Freq:       MONTHLY,
			Interval:   2,
			Dtstart:    time.Date(2100, time.January, 31, 23, 59, 0, 0, time.UTC),
			Count:      3,
			Bymonthday: []int{31},
			Byminute:   []int{59},
			Bysecond:   []int{59, 60},
		},
	}, table)
	if err != nil {
		t.Fatal(err)
	}

	occurrences, err := rule.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(occurrences) != 3 {
		t.Fatalf("got %d occurrences, want 3: %v", len(occurrences), occurrences)
	}

	want := []string{
		"2100-01-31T23:59:59Z",
		"2100-01-31T23:59:60Z",
		"2100-03-31T23:59:59Z",
	}
	for i, occurrence := range occurrences {
		if occurrence.String() != want[i] {
			t.Fatalf("occurrence %d = %s, want %s", i, occurrence.String(), want[i])
		}
	}
	if !occurrences[1].IsLeapSecond() {
		t.Fatal("second occurrence is not marked as a leap second")
	}
	if occurrences[1].Compare(occurrences[2]) >= 0 {
		t.Fatal("leap second does not sort before the following ordinary second")
	}
	if _, err := occurrences[1].Time(); !errors.Is(err, ErrLeapSecond) {
		t.Fatalf("leap second Time error = %v, want ErrLeapSecond", err)
	}
	ordinary, err := occurrences[0].Time()
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2100, time.January, 31, 23, 59, 59, 0, time.UTC); !ordinary.Equal(want) {
		t.Fatalf("ordinary Time = %v, want %v", ordinary, want)
	}
}

func TestLeapSecondSeparatedFromNextMinuteAndDay(t *testing.T) {
	table := testLeapSecondTable(t)
	until := mustTableOccurrence(t, table, 2100, time.February, 1, 0, 0, 0)
	rule, err := NewLeapSecondRRule(LeapSecondROption{
		ROption: ROption{
			Freq:     MINUTELY,
			Dtstart:  time.Date(2100, time.January, 31, 23, 59, 59, 0, time.UTC),
			Bymonth:  []int{1, 2},
			Byminute: []int{59, 0},
			Bysecond: []int{59, 60, 0},
		},
		Until: &until,
	}, table)
	if err != nil {
		t.Fatal(err)
	}

	occurrences, err := rule.All()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"2100-01-31T23:59:59Z",
		"2100-01-31T23:59:60Z",
		"2100-02-01T00:00:00Z",
	}
	if len(occurrences) != len(want) {
		t.Fatalf("got %d occurrences, want %d: %v", len(occurrences), len(want), occurrences)
	}
	for i := range want {
		if occurrences[i].String() != want[i] {
			t.Fatalf("occurrence %d = %s, want %s", i, occurrences[i], want[i])
		}
	}
}

func TestLeapSecondSecondlyOrder(t *testing.T) {
	table := testLeapSecondTable(t)
	rule, err := NewLeapSecondRRule(LeapSecondROption{
		ROption: ROption{
			Freq:    SECONDLY,
			Dtstart: time.Date(2100, time.January, 31, 23, 59, 59, 0, time.UTC),
			Count:   3,
		},
	}, table)
	if err != nil {
		t.Fatal(err)
	}
	occurrences, err := rule.All()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"2100-01-31T23:59:59Z",
		"2100-01-31T23:59:60Z",
		"2100-02-01T00:00:00Z",
	}
	for i := range want {
		if occurrences[i].String() != want[i] {
			t.Fatalf("occurrence %d = %s, want %s", i, occurrences[i], want[i])
		}
	}
}

func TestLeapSecondBySecond60WithoutTableRecordDoesNotCount(t *testing.T) {
	table := testLeapSecondTable(t)
	rule, err := NewLeapSecondRRule(LeapSecondROption{
		ROption: ROption{
			Freq:       MONTHLY,
			Dtstart:    time.Date(2100, time.February, 28, 23, 59, 0, 0, time.UTC),
			Count:      1,
			Bymonthday: []int{28},
			Bysecond:   []int{59, 60},
		},
	}, table)
	if err != nil {
		t.Fatal(err)
	}
	occurrences, err := rule.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(occurrences) != 1 || occurrences[0].String() != "2100-02-28T23:59:59Z" {
		t.Fatalf("unexpected occurrences: %v", occurrences)
	}
}

func TestLeapSecondBySecond60OnlyProducesTableDates(t *testing.T) {
	table := testLeapSecondTable(t)
	until := mustTableOccurrence(t, table, 2100, time.April, 1, 0, 0, 0)
	rule, err := NewLeapSecondRRule(LeapSecondROption{
		ROption: ROption{
			Freq:     SECONDLY,
			Dtstart:  time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
			Bysecond: []int{60},
		},
		Until: &until,
	}, table)
	if err != nil {
		t.Fatal(err)
	}
	occurrences, err := rule.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(occurrences) != 2 {
		t.Fatalf("got %d leap-second occurrences, want 2: %v", len(occurrences), occurrences)
	}
	want := []string{"2100-01-31T23:59:60Z", "2100-03-31T23:59:60Z"}
	for i := range want {
		if occurrences[i].String() != want[i] {
			t.Fatalf("occurrence %d = %s, want %s", i, occurrences[i], want[i])
		}
	}
}

func TestLeapSecondUntilCanBeLeapSecond(t *testing.T) {
	table := testLeapSecondTable(t)
	until := mustTableOccurrence(t, table, 2100, time.January, 31, 23, 59, 60)
	rule, err := NewLeapSecondRRule(LeapSecondROption{
		ROption: ROption{
			Freq:    SECONDLY,
			Dtstart: time.Date(2100, time.January, 31, 23, 59, 58, 0, time.UTC),
		},
		Until: &until,
	}, table)
	if err != nil {
		t.Fatal(err)
	}
	occurrences, err := rule.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(occurrences) != 3 {
		t.Fatalf("got %d occurrences, want 3: %v", len(occurrences), occurrences)
	}
	if last := occurrences[len(occurrences)-1]; !last.IsLeapSecond() {
		t.Fatalf("last occurrence = %s, want inclusive leap second", last)
	}
}

func TestLeapSecondBySetPosIncludesLeapSecond(t *testing.T) {
	table := testLeapSecondTable(t)
	rule, err := NewLeapSecondRRule(LeapSecondROption{
		ROption: ROption{
			Freq:       MONTHLY,
			Dtstart:    time.Date(2100, time.January, 31, 23, 59, 59, 0, time.UTC),
			Count:      2,
			Bymonthday: []int{31},
			Byminute:   []int{59},
			Bysecond:   []int{59, 60},
			Bysetpos:   []int{1, -1},
		},
	}, table)
	if err != nil {
		t.Fatal(err)
	}
	occurrences, err := rule.All()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2100-01-31T23:59:59Z", "2100-01-31T23:59:60Z"}
	if len(occurrences) != len(want) {
		t.Fatalf("got %d occurrences, want %d: %v", len(occurrences), len(want), occurrences)
	}
	for i := range want {
		if occurrences[i].String() != want[i] {
			t.Fatalf("occurrence %d = %s, want %s", i, occurrences[i], want[i])
		}
	}
}

func TestLeapSecondBoundary(t *testing.T) {
	table := testLeapSecondTable(t)
	rule, err := NewLeapSecondRRule(LeapSecondROption{
		ROption: ROption{
			Freq:       MONTHLY,
			Dtstart:    time.Date(2100, time.January, 31, 23, 59, 59, 0, time.UTC),
			Bymonthday: []int{31},
			Bysecond:   []int{59, 60},
		},
	}, table)
	if err != nil {
		t.Fatal(err)
	}

	leap := mustTableOccurrence(t, table, 2100, time.January, 31, 23, 59, 60)
	values, err := rule.Between(leap, leap, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || !values[0].IsLeapSecond() {
		t.Fatalf("unexpected inclusive boundary result: %v", values)
	}

	unknown, err := NewOccurrence(2100, time.February, 28, 23, 59, 60)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rule.Between(unknown, leap, true); !errors.Is(err, ErrLeapSecondNotFound) {
		t.Fatalf("unknown leap boundary error = %v, want ErrLeapSecondNotFound", err)
	}
}

func TestLeapSecondRejectsNonUTC(t *testing.T) {
	table := testLeapSecondTable(t)
	_, err := NewLeapSecondRRule(LeapSecondROption{
		ROption: ROption{
			Freq:    SECONDLY,
			Dtstart: time.Date(2100, time.January, 31, 23, 59, 59, 0, time.FixedZone("test", 60)),
		},
	}, table)
	if !errors.Is(err, ErrNotUTC) {
		t.Fatalf("non-UTC error = %v, want ErrNotUTC", err)
	}
}

func TestLeapSecondRejectsDuplicateUntilAndUnboundedAll(t *testing.T) {
	table := testLeapSecondTable(t)
	until := mustTableOccurrence(t, table, 2100, time.March, 31, 23, 59, 60)
	_, err := NewLeapSecondRRule(LeapSecondROption{
		ROption: ROption{
			Freq:    SECONDLY,
			Dtstart: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
			Until:   time.Date(2100, time.March, 1, 0, 0, 0, 0, time.UTC),
		},
		Until: &until,
	}, table)
	if err == nil {
		t.Fatal("duplicate UNTIL did not return an error")
	}

	rule, err := NewLeapSecondRRule(LeapSecondROption{
		ROption: ROption{
			Freq:    SECONDLY,
			Dtstart: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	}, table)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rule.All(); err == nil {
		t.Fatal("unbounded leap-second All did not return an error")
	}
}

func mustOccurrence(t *testing.T, occurrence Occurrence, err error) Occurrence {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return occurrence
}

func mustTableOccurrence(t *testing.T, table *LeapSecondTable, year int, month time.Month, day, hour, minute, second int) Occurrence {
	t.Helper()
	occurrence, err := table.OccurrenceAt(year, month, day, hour, minute, second)
	return mustOccurrence(t, occurrence, err)
}
