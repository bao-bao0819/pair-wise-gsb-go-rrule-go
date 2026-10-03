package rrule

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

var ErrLeapSecond = errors.New("rrule: UTC positive leap second cannot be represented by time.Time")

type timeSpec struct {
	hour     int
	minute   int
	second   int
	location *time.Location
}

func newTimeSpec(hour, minute, second int, location *time.Location) timeSpec {
	return timeSpec{hour: hour, minute: minute, second: second, location: location}
}

type timeSpecSlice []timeSpec

func (s timeSpecSlice) Len() int           { return len(s) }
func (s timeSpecSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s timeSpecSlice) Less(i, j int) bool { return timeSpecBefore(s[i], s[j]) }

func timeSpecBefore(a, b timeSpec) bool {
	if a.hour != b.hour {
		return a.hour < b.hour
	}
	if a.minute != b.minute {
		return a.minute < b.minute
	}
	return a.second < b.second
}

type occurrenceValue struct {
	year       int
	month      time.Month
	day        int
	hour       int
	minute     int
	second     int
	location   *time.Location
	leapSecond bool
}

type occurrenceValueSlice []occurrenceValue

func (s occurrenceValueSlice) Len() int           { return len(s) }
func (s occurrenceValueSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s occurrenceValueSlice) Less(i, j int) bool { return occurrenceValueBefore(s[i], s[j]) }

func occurrenceValueFromTime(t time.Time) occurrenceValue {
	year, month, day := t.Date()
	hour, minute, second := t.Clock()
	return occurrenceValue{
		year:     year,
		month:    month,
		day:      day,
		hour:     hour,
		minute:   minute,
		second:   second,
		location: t.Location(),
	}
}

func occurrenceValueAt(year int, month time.Month, day int, spec timeSpec, table LeapSecondTable) (occurrenceValue, bool) {
	value := occurrenceValue{
		year:     year,
		month:    month,
		day:      day,
		hour:     spec.hour,
		minute:   spec.minute,
		second:   spec.second,
		location: spec.location,
	}
	if spec.second == 60 {
		if spec.hour != 23 || spec.minute != 59 || !table.hasDate(year, month, day) {
			return occurrenceValue{}, false
		}
		value.leapSecond = true
	}
	return value, true
}

func occurrenceValueBefore(a, b occurrenceValue) bool {
	left := [6]int{a.year, int(a.month), a.day, a.hour, a.minute, a.second}
	right := [6]int{b.year, int(b.month), b.day, b.hour, b.minute, b.second}
	for i := range left {
		if left[i] != right[i] {
			return left[i] < right[i]
		}
	}
	return false
}

func occurrenceValueContains(list []occurrenceValue, value occurrenceValue) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func (value occurrenceValue) before(other occurrenceValue) bool {
	return occurrenceValueBefore(value, other)
}

func (value occurrenceValue) after(other occurrenceValue) bool {
	return occurrenceValueBefore(other, value)
}

func (value occurrenceValue) time() (time.Time, error) {
	if value.leapSecond {
		return time.Time{}, ErrLeapSecond
	}
	location := value.location
	if location == nil {
		location = time.UTC
	}
	return time.Date(value.year, value.month, value.day, value.hour, value.minute, value.second, 0, location), nil
}

func (value occurrenceValue) string() string {
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02dZ",
		value.year, value.month, value.day, value.hour, value.minute, value.second)
}

// LeapSecondTime is a UTC wall-clock time that can represent 23:59:60.
type LeapSecondTime struct {
	value occurrenceValue
}

// NewLeapSecondTime constructs a UTC wall-clock time. Second may be 60 only for
// a positive leap second at 23:59:60.
func NewLeapSecondTime(year int, month time.Month, day, hour, minute, second int) (LeapSecondTime, error) {
	if year < 1 || year > MAXYEAR {
		return LeapSecondTime{}, errors.New("rrule: year is out of range")
	}
	if month < time.January || month > time.December {
		return LeapSecondTime{}, errors.New("rrule: month is out of range")
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 60 {
		return LeapSecondTime{}, errors.New("rrule: time is out of range")
	}
	if second == 60 && (hour != 23 || minute != 59) {
		return LeapSecondTime{}, errors.New("rrule: second 60 is only valid at 23:59:60")
	}

	checkSecond := second
	if checkSecond == 60 {
		checkSecond = 59
	}
	check := time.Date(year, month, day, hour, minute, checkSecond, 0, time.UTC)
	checkYear, checkMonth, checkDay := check.Date()
	checkHour, checkMinute, checkSecondValue := check.Clock()
	if checkYear != year || checkMonth != month || checkDay != day ||
		checkHour != hour || checkMinute != minute || checkSecondValue != checkSecond {
		return LeapSecondTime{}, errors.New("rrule: date is out of range")
	}
	if second == 60 && check.AddDate(0, 0, 1).Day() != 1 {
		return LeapSecondTime{}, errors.New("rrule: second 60 is only valid at a UTC month end")
	}

	return LeapSecondTime{value: occurrenceValue{
		year:       year,
		month:      month,
		day:        day,
		hour:       hour,
		minute:     minute,
		second:     second,
		location:   time.UTC,
		leapSecond: second == 60,
	}}, nil
}

// LeapSecondTimeFromTime converts an ordinary UTC time.Time.
func LeapSecondTimeFromTime(t time.Time) (LeapSecondTime, error) {
	if t.Location() != time.UTC {
		return LeapSecondTime{}, errors.New("rrule: leap second evaluation requires UTC")
	}
	year, month, day := t.Date()
	hour, minute, second := t.Clock()
	return NewLeapSecondTime(year, month, day, hour, minute, second)
}

// Date returns the UTC date.
func (t LeapSecondTime) Date() (int, time.Month, int) {
	return t.value.year, t.value.month, t.value.day
}

// Clock returns the UTC hour, minute, and second. The second can be 60.
func (t LeapSecondTime) Clock() (int, int, int) {
	return t.value.hour, t.value.minute, t.value.second
}

// IsLeapSecond reports whether the time is 23:59:60.
func (t LeapSecondTime) IsLeapSecond() bool {
	return t.value.leapSecond
}

// Time returns the time.Time representation, or ErrLeapSecond for 23:59:60.
func (t LeapSecondTime) Time() (time.Time, error) {
	return t.value.time()
}

// Compare returns -1, 0, or 1 in chronological order. A leap second sorts after
// 23:59:59 of the same date and before 00:00:00 of the next date.
func (t LeapSecondTime) Compare(other LeapSecondTime) int {
	if occurrenceValueBefore(t.value, other.value) {
		return -1
	}
	if occurrenceValueBefore(other.value, t.value) {
		return 1
	}
	return 0
}

func (t LeapSecondTime) String() string {
	return t.value.string()
}

// LeapSecondOccurrence is a recurrence occurrence that can be a UTC leap second.
type LeapSecondOccurrence struct {
	value occurrenceValue
}

// Date returns the UTC date.
func (o LeapSecondOccurrence) Date() (int, time.Month, int) {
	return o.value.year, o.value.month, o.value.day
}

// Clock returns the UTC hour, minute, and second. The second can be 60.
func (o LeapSecondOccurrence) Clock() (int, int, int) {
	return o.value.hour, o.value.minute, o.value.second
}

// IsLeapSecond reports whether the occurrence is 23:59:60.
func (o LeapSecondOccurrence) IsLeapSecond() bool {
	return o.value.leapSecond
}

// LeapSecondTime returns the wall-clock time represented by the occurrence.
func (o LeapSecondOccurrence) LeapSecondTime() LeapSecondTime {
	return LeapSecondTime{value: o.value}
}

// Time returns the time.Time representation, or ErrLeapSecond for 23:59:60.
func (o LeapSecondOccurrence) Time() (time.Time, error) {
	return o.value.time()
}

// Compare returns -1, 0, or 1 in chronological order.
func (o LeapSecondOccurrence) Compare(other LeapSecondOccurrence) int {
	return o.LeapSecondTime().Compare(other.LeapSecondTime())
}

func (o LeapSecondOccurrence) String() string {
	return o.value.string()
}

// SortLeapSecondOccurrences sorts occurrences chronologically.
func SortLeapSecondOccurrences(values []LeapSecondOccurrence) {
	sort.Slice(values, func(i, j int) bool {
		return occurrenceValueBefore(values[i].value, values[j].value)
	})
}

// LeapSecondTable contains caller-verified positive UTC leap second dates.
// A missing date means this table has no record for that date; it is not a
// claim that the date is known to have no leap second.
type LeapSecondTable map[time.Time]struct{}

// NewLeapSecondTable constructs a table from UTC month-end dates at midnight.
// It does not download or infer leap-second data.
func NewLeapSecondTable(dates ...time.Time) (LeapSecondTable, error) {
	if len(dates) == 0 {
		return nil, errors.New("rrule: leap second table must not be empty")
	}
	table := make(LeapSecondTable, len(dates))
	for _, date := range dates {
		if date.Location() != time.UTC {
			return nil, errors.New("rrule: leap second table dates must be UTC")
		}
		year, month, day := date.Date()
		hour, minute, second := date.Clock()
		if hour != 0 || minute != 0 || second != 0 {
			return nil, errors.New("rrule: leap second table date must be midnight UTC")
		}
		if date.AddDate(0, 0, 1).Day() != 1 {
			return nil, errors.New("rrule: UTC leap second date must be the last day of a month")
		}
		table[time.Date(year, month, day, 0, 0, 0, 0, time.UTC)] = struct{}{}
	}
	return table, nil
}

func (table LeapSecondTable) hasDate(year int, month time.Month, day int) bool {
	_, ok := table[time.Date(year, month, day, 0, 0, 0, 0, time.UTC)]
	return ok
}

// LeapSecondROption configures leap-second-aware recurrence evaluation.
type LeapSecondROption struct {
	ROption
	LeapUntil *LeapSecondTime
}

// LeapSecondRRule evaluates a recurrence with a caller-provided UTC positive
// leap-second table. It does not change the behavior of RRule.
type LeapSecondRRule struct {
	rrule *RRule
}

// NewLeapSecondRRule constructs a UTC, positive-leap-second-aware rule.
func NewLeapSecondRRule(option LeapSecondROption, table LeapSecondTable) (*LeapSecondRRule, error) {
	if len(table) == 0 {
		return nil, errors.New("rrule: leap second table must not be empty")
	}
	if !option.Until.IsZero() {
		return nil, errors.New("rrule: use LeapUntil for leap-second-aware UNTIL")
	}
	if !option.Dtstart.IsZero() && option.Dtstart.Location() != time.UTC {
		return nil, errors.New("rrule: leap second evaluation requires UTC")
	}
	if option.Dtstart.IsZero() {
		option.Dtstart = time.Now().UTC()
	}
	if err := validateBoundsWithLeapSeconds(option.ROption, true); err != nil {
		return nil, err
	}

	arg := option.ROption
	arg.Until = time.Time{}
	r := buildRRule(arg)
	r.leapSeconds = table
	if r.freq >= HOURLY && r.freq < SECONDLY && contains(arg.Bysecond, 60) {
		r.bysecond = nil
		for _, second := range arg.Bysecond {
			if second == 60 {
				second = 59
			}
			if !contains(r.bysecond, second) {
				r.bysecond = append(r.bysecond, second)
			}
		}
	}
	if r.freq < HOURLY && contains(arg.Bysecond, 60) {
		r.timeset = make([]timeSpec, 0, len(r.byhour)*len(r.byminute)*len(arg.Bysecond))
		for _, hour := range r.byhour {
			for _, minute := range r.byminute {
				for _, second := range arg.Bysecond {
					r.timeset = append(r.timeset, newTimeSpec(hour, minute, second, time.UTC))
				}
			}
		}
		sort.Sort(timeSpecSlice(r.timeset))
	}
	if option.LeapUntil != nil {
		boundary := option.LeapUntil.value
		if boundary.leapSecond && !table.hasDate(boundary.year, boundary.month, boundary.day) {
			return nil, errors.New("rrule: leap-second UNTIL date must exist in the table")
		}
		r.untilValue = boundary
		r.until = time.Date(boundary.year, boundary.month, boundary.day,
			boundary.hour, boundary.minute, 59, 0, time.UTC)
	}

	return &LeapSecondRRule{rrule: &r}, nil
}

// NextLeapOccurrence returns the next leap-second-aware occurrence.
type NextLeapOccurrence func() (LeapSecondOccurrence, bool)

// Iterator returns an iterator whose occurrences retain second 60.
func (r *LeapSecondRRule) Iterator() NextLeapOccurrence {
	next := r.rrule.valueIterator()
	return func() (LeapSecondOccurrence, bool) {
		value, ok := next()
		return LeapSecondOccurrence{value: value}, ok
	}
}

// All returns every occurrence emitted by the rule.
func (r *LeapSecondRRule) All() []LeapSecondOccurrence {
	next := r.Iterator()
	result := []LeapSecondOccurrence{}
	for {
		value, ok := next()
		if !ok {
			return result
		}
		result = append(result, value)
	}
}

// Between returns occurrences in the half-open or inclusive UTC range.
func (r *LeapSecondRRule) Between(after, before LeapSecondTime, inc bool) []LeapSecondOccurrence {
	result := []LeapSecondOccurrence{}
	next := r.Iterator()
	for {
		value, ok := next()
		if !ok {
			return result
		}
		occurrenceTime := value.LeapSecondTime()
		if inc && occurrenceTime.Compare(before) > 0 ||
			!inc && occurrenceTime.Compare(before) >= 0 {
			return result
		}
		if inc && occurrenceTime.Compare(after) >= 0 ||
			!inc && occurrenceTime.Compare(after) > 0 {
			result = append(result, value)
		}
	}
}

// Before returns the last occurrence before the UTC boundary.
func (r *LeapSecondRRule) Before(boundary LeapSecondTime, inc bool) (LeapSecondOccurrence, bool) {
	var result LeapSecondOccurrence
	found := false
	next := r.Iterator()
	for {
		value, ok := next()
		if !ok {
			return result, found
		}
		occurrenceTime := value.LeapSecondTime()
		if inc && occurrenceTime.Compare(boundary) > 0 ||
			!inc && occurrenceTime.Compare(boundary) >= 0 {
			return result, found
		}
		result = value
		found = true
	}
}

// After returns the first occurrence after the UTC boundary.
func (r *LeapSecondRRule) After(boundary LeapSecondTime, inc bool) (LeapSecondOccurrence, bool) {
	next := r.Iterator()
	for {
		value, ok := next()
		if !ok {
			return LeapSecondOccurrence{}, false
		}
		occurrenceTime := value.LeapSecondTime()
		if inc && occurrenceTime.Compare(boundary) >= 0 ||
			!inc && occurrenceTime.Compare(boundary) > 0 {
			return value, true
		}
	}
}

// RRule returns the underlying ordinary rule. Its time.Time iterator cannot
// return leap seconds; use Iterator for leap-second-aware evaluation.
func (r *LeapSecondRRule) RRule() *RRule {
	return r.rrule
}
