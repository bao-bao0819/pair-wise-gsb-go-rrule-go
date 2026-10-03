package rrule

import (
	"errors"
	"testing"
	"time"
)

func testLeapSecondTable(t *testing.T) LeapSecondTable {
	t.Helper()
	table, err := NewLeapSecondTable(
		time.Date(2020, time.June, 30, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.August, 31, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	return table
}

func TestLeapSecondSecondlySequence(t *testing.T) {
	rule, err := NewLeapSecondRRule(LeapSecondROption{ROption: ROption{
		Freq:    SECONDLY,
		Dtstart: time.Date(2020, time.June, 30, 23, 59, 59, 0, time.UTC),
		Count:   3,
	}}, testLeapSecondTable(t))
	if err != nil {
		t.Fatal(err)
	}

	values := rule.All()
	want := []string{
		"2020-06-30T23:59:59Z",
		"2020-06-30T23:59:60Z",
		"2020-07-01T00:00:00Z",
	}
	if len(values) != len(want) {
		t.Fatalf("got %d occurrences, want %d: %v", len(values), len(want), values)
	}
	for i := range want {
		if got := values[i].String(); got != want[i] {
			t.Fatalf("occurrence %d = %s, want %s", i, got, want[i])
		}
	}

	if values[0].IsLeapSecond() || !values[1].IsLeapSecond() || values[2].IsLeapSecond() {
		t.Fatalf("only the middle occurrence should be a leap second: %v", values)
	}
	if values[0].Compare(values[1]) >= 0 || values[1].Compare(values[2]) >= 0 {
		t.Fatalf("occurrences are not ordered: %v", values)
	}
	sorted := []LeapSecondOccurrence{values[2], values[0], values[1]}
	SortLeapSecondOccurrences(sorted)
	for i := range values {
		if sorted[i].Compare(values[i]) != 0 {
			t.Fatalf("sorted occurrence %d = %s, want %s", i, sorted[i], values[i])
		}
	}
	if _, err := values[1].Time(); !errors.Is(err, ErrLeapSecond) {
		t.Fatalf("leap second Time error = %v, want ErrLeapSecond", err)
	}
	ordinary, err := values[2].Time()
	if err != nil {
		t.Fatal(err)
	}
	if wantTime := time.Date(2020, time.July, 1, 0, 0, 0, 0, time.UTC); !ordinary.Equal(wantTime) {
		t.Fatalf("ordinary Time = %s, want %s", ordinary, wantTime)
	}
}

func TestLeapSecondBySecondOnlySkipsDatesOutsideTable(t *testing.T) {
	rule, err := NewLeapSecondRRule(LeapSecondROption{ROption: ROption{
		Freq:       MONTHLY,
		Dtstart:    time.Date(2020, time.June, 1, 0, 0, 0, 0, time.UTC),
		Count:      2,
		Bymonthday: []int{-1},
		Byhour:     []int{23},
		Byminute:   []int{59},
		Bysecond:   []int{60},
	}}, testLeapSecondTable(t))
	if err != nil {
		t.Fatal(err)
	}

	values := rule.All()
	want := []string{
		"2020-06-30T23:59:60Z",
		"2020-08-31T23:59:60Z",
	}
	if len(values) != len(want) {
		t.Fatalf("got %v, want %v", values, want)
	}
	for i := range want {
		if got := values[i].String(); got != want[i] {
			t.Fatalf("occurrence %d = %s, want %s", i, got, want[i])
		}
	}
}

func TestLeapSecondInvalidMinuteAdvancesToNextMinute(t *testing.T) {
	rule, err := NewLeapSecondRRule(LeapSecondROption{ROption: ROption{
		Freq:     SECONDLY,
		Dtstart:  time.Date(2020, time.July, 1, 10, 58, 59, 0, time.UTC),
		Count:    3,
		Bysecond: []int{59, 60},
	}}, testLeapSecondTable(t))
	if err != nil {
		t.Fatal(err)
	}

	values := rule.All()
	want := []string{
		"2020-07-01T10:58:59Z",
		"2020-07-01T10:59:59Z",
		"2020-07-01T11:00:59Z",
	}
	if len(values) != len(want) {
		t.Fatalf("got %v, want %v", values, want)
	}
	for i := range want {
		if values[i].String() != want[i] {
			t.Fatalf("occurrence %d = %s, want %s", i, values[i], want[i])
		}
	}
}

func TestLeapSecondExplicitSecondSixtyFindsProxySecond(t *testing.T) {
	rule, err := NewLeapSecondRRule(LeapSecondROption{ROption: ROption{
		Freq:     SECONDLY,
		Dtstart:  time.Date(2020, time.June, 30, 0, 0, 0, 0, time.UTC),
		Count:    1,
		Bysecond: []int{60},
	}}, testLeapSecondTable(t))
	if err != nil {
		t.Fatal(err)
	}

	values := rule.All()
	if len(values) != 1 || values[0].String() != "2020-06-30T23:59:60Z" {
		t.Fatalf("got %v, want only the table-verified leap second", values)
	}
}

func TestLeapSecondExplicitSecondSixtyOutsideTableMovesToNextYear(t *testing.T) {
	rule, err := NewLeapSecondRRule(LeapSecondROption{ROption: ROption{
		Freq:     SECONDLY,
		Dtstart:  time.Date(2019, time.December, 31, 23, 59, 0, 0, time.UTC),
		Count:    1,
		Bysecond: []int{60},
	}}, testLeapSecondTable(t))
	if err != nil {
		t.Fatal(err)
	}

	values := rule.All()
	if len(values) != 1 || values[0].String() != "2020-06-30T23:59:60Z" {
		t.Fatalf("invalid end-of-year :60 should advance to the next table date: %v", values)
	}
}

func TestLeapSecondMinutelyExplicitSecondSixty(t *testing.T) {
	rule, err := NewLeapSecondRRule(LeapSecondROption{ROption: ROption{
		Freq:     MINUTELY,
		Dtstart:  time.Date(2020, time.June, 30, 23, 57, 0, 0, time.UTC),
		Count:    2,
		Bysecond: []int{60},
	}}, testLeapSecondTable(t))
	if err != nil {
		t.Fatal(err)
	}

	values := rule.All()
	want := []string{
		"2020-06-30T23:59:60Z",
		"2020-08-31T23:59:60Z",
	}
	if len(values) != len(want) {
		t.Fatalf("got %v, want %v", values, want)
	}
	for i := range want {
		if values[i].String() != want[i] {
			t.Fatalf("occurrence %d = %s, want %s", i, values[i], want[i])
		}
	}
}

func TestLeapSecondHourlyExplicitSecondSixty(t *testing.T) {
	rule, err := NewLeapSecondRRule(LeapSecondROption{ROption: ROption{
		Freq:     HOURLY,
		Dtstart:  time.Date(2020, time.June, 30, 21, 0, 0, 0, time.UTC),
		Count:    2,
		Byminute: []int{59},
		Bysecond: []int{60},
	}}, testLeapSecondTable(t))
	if err != nil {
		t.Fatal(err)
	}

	values := rule.All()
	want := []string{
		"2020-06-30T23:59:60Z",
		"2020-08-31T23:59:60Z",
	}
	if len(values) != len(want) {
		t.Fatalf("got %v, want %v", values, want)
	}
	for i := range want {
		if values[i].String() != want[i] {
			t.Fatalf("occurrence %d = %s, want %s", i, values[i], want[i])
		}
	}
}

func TestLeapSecondMinutelyOutsideTableDoesNotStall(t *testing.T) {
	rule, err := NewLeapSecondRRule(LeapSecondROption{ROption: ROption{
		Freq:     MINUTELY,
		Dtstart:  time.Date(2020, time.July, 31, 23, 58, 0, 0, time.UTC),
		Count:    1,
		Bysecond: []int{60},
	}}, testLeapSecondTable(t))
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan []LeapSecondOccurrence, 1)
	go func() {
		done <- rule.All()
	}()
	select {
	case values := <-done:
		if len(values) != 1 || values[0].String() != "2020-08-31T23:59:60Z" {
			t.Fatalf("got %v, want the next table-verified leap second", values)
		}
	case <-time.After(time.Second):
		t.Fatal("iteration stalled on a date without a table record")
	}
}

func TestLeapSecondUntilAndQueryBoundary(t *testing.T) {
	until, err := NewLeapSecondTime(2020, time.June, 30, 23, 59, 60)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := NewLeapSecondRRule(LeapSecondROption{ROption: ROption{
		Freq:    SECONDLY,
		Dtstart: time.Date(2020, time.June, 30, 23, 59, 58, 0, time.UTC),
	}, LeapUntil: &until}, testLeapSecondTable(t))
	if err != nil {
		t.Fatal(err)
	}

	values := rule.All()
	if len(values) != 3 || values[2].String() != "2020-06-30T23:59:60Z" {
		t.Fatalf("UNTIL should include the leap second: %v", values)
	}
	atLeap := values[2]
	if after, ok := rule.After(until, false); ok {
		t.Fatalf("exclusive boundary with leap-second UNTIL should end: %v", after)
	}
	if before, ok := rule.Before(until, true); !ok || before.Compare(atLeap) != 0 {
		t.Fatalf("inclusive Before should return leap second: %v", before)
	}

	nextMidnight, err := NewLeapSecondTime(2020, time.July, 1, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	between := rule.Between(until, nextMidnight, true)
	if len(between) != 1 || !between[0].IsLeapSecond() {
		t.Fatalf("range beginning at :60 should contain exactly the leap second: %v", between)
	}
}

func TestLeapSecondRejectsNonUTCAndInvalidArguments(t *testing.T) {
	location := time.FixedZone("test", 60)
	_, err := NewLeapSecondRRule(LeapSecondROption{ROption: ROption{
		Freq:    SECONDLY,
		Dtstart: time.Date(2020, time.June, 30, 23, 59, 59, 0, location),
	}}, testLeapSecondTable(t))
	if err == nil {
		t.Fatal("non-UTC Dtstart should be rejected")
	}

	if _, err := NewRRule(ROption{Freq: SECONDLY, Bysecond: []int{60}}); err == nil {
		t.Fatal("ordinary RRule should still reject BYSECOND=60")
	}
	if _, err := NewLeapSecondTime(2020, time.June, 30, 23, 58, 60); err == nil {
		t.Fatal("23:58:60 should be rejected")
	}
	if _, err := NewLeapSecondTime(2020, time.June, 29, 23, 59, 60); err == nil {
		t.Fatal("non-month-end 23:59:60 should be rejected")
	}
	if _, err := NewLeapSecondTable(time.Date(2020, time.June, 29, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("non-month-end table dates should be rejected")
	}
	if _, err := NewLeapSecondTable(time.Date(2020, time.June, 30, 1, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("non-midnight table dates should be rejected")
	}
}

func TestLeapSecondTableMissingDateIsAbsenceOfData(t *testing.T) {
	rule, err := NewLeapSecondRRule(LeapSecondROption{ROption: ROption{
		Freq:     DAILY,
		Dtstart:  time.Date(2020, time.July, 1, 0, 0, 0, 0, time.UTC),
		Count:    1,
		Byhour:   []int{23},
		Byminute: []int{59},
		Bysecond: []int{60},
	}}, testLeapSecondTable(t))
	if err != nil {
		t.Fatal(err)
	}
	if values := rule.All(); len(values) != 1 {
		t.Fatalf("a missing table date should be skipped, not declared non-leap: %v", values)
	}
}
