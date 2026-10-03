package rrule

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	// ErrLeapSecond is returned when converting a positive leap second to time.Time.
	ErrLeapSecond = errors.New("leap second cannot be represented by time.Time")
	// ErrNotUTC is returned when leap-second evaluation receives a non-UTC location.
	ErrNotUTC = errors.New("leap-second recurrence is limited to UTC")
	// ErrLeapSecondNotFound is returned when a boundary asks for a leap second absent from the table.
	ErrLeapSecondNotFound = errors.New("leap second is not present in the supplied table")
)

// LeapSecondDate is the UTC date on which a positive leap second begins at 23:59:60.
type LeapSecondDate struct {
	Year  int
	Month time.Month
	Day   int
}

// NewLeapSecondDate validates and returns a positive leap second date.
func NewLeapSecondDate(year int, month time.Month, day int) (LeapSecondDate, error) {
	date := LeapSecondDate{year, month, day}
	normalized := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if normalized.Year() != year || normalized.Month() != month || normalized.Day() != day {
		return LeapSecondDate{}, fmt.Errorf("invalid leap second date: %d-%02d-%02d", year, month, day)
	}
	return date, nil
}

func (date LeapSecondDate) key() int {
	return date.Year*10000 + int(date.Month)*100 + date.Day
}

// LeapSecondTable is a caller-supplied, verified table of UTC positive leap seconds.
// It intentionally contains no network loader.
type LeapSecondTable struct {
	dates   map[int]LeapSecondDate
	list    []LeapSecondDate
	serials []int
}

// NewLeapSecondTable returns a sorted table from the supplied UTC positive leap second records.
func NewLeapSecondTable(dates ...LeapSecondDate) (*LeapSecondTable, error) {
	table := &LeapSecondTable{dates: make(map[int]LeapSecondDate, len(dates))}
	for _, date := range dates {
		valid, err := NewLeapSecondDate(date.Year, date.Month, date.Day)
		if err != nil {
			return nil, err
		}
		key := valid.key()
		if _, exists := table.dates[key]; exists {
			return nil, fmt.Errorf("duplicate leap second date: %s", valid)
		}
		table.dates[key] = valid
		table.list = append(table.list, valid)
	}
	sort.Slice(table.list, func(i, j int) bool {
		return table.list[i].key() < table.list[j].key()
	})
	table.serials = make([]int, len(table.list))
	for i, date := range table.list {
		table.serials[i] = daysFromCivil(date.Year, int(date.Month), date.Day)
	}
	return table, nil
}

func (table *LeapSecondTable) has(year int, month time.Month, day int) bool {
	_, ok := table.dates[LeapSecondDate{year, month, day}.key()]
	return ok
}

func (table *LeapSecondTable) hasDate(date LeapSecondDate) bool {
	_, ok := table.dates[date.key()]
	return ok
}

func (table *LeapSecondTable) leapCountBeforeSerial(serial int) int {
	return sort.Search(len(table.serials), func(i int) bool {
		return table.serials[i] >= serial
	})
}

func (table *LeapSecondTable) hasDateAtSerial(serial int) bool {
	index := sort.SearchInts(table.serials, serial)
	return index < len(table.serials) && table.serials[index] == serial
}

func (table *LeapSecondTable) leapCountAtOrBeforeSerial(serial int) int {
	return sort.Search(len(table.serials), func(i int) bool {
		return table.serials[i] > serial
	})
}

func (date LeapSecondDate) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", date.Year, date.Month, date.Day)
}

// Occurrence represents a UTC wall-clock occurrence. The zero Location is treated as UTC.
type Occurrence struct {
	year       int
	month      time.Month
	day        int
	hour       int
	minute     int
	second     int
	location   *time.Location
	leapSecond bool
	table      *LeapSecondTable
}

// NewOccurrence constructs an occurrence in UTC. Leap seconds must be exactly 23:59:60.
func NewOccurrence(year int, month time.Month, day, hour, minute, second int) (Occurrence, error) {
	leapSecond := second == 60
	if leapSecond && (hour != 23 || minute != 59) {
		return Occurrence{}, errors.New("leap second must occur at 23:59:60")
	}
	if !leapSecond && (second < 0 || second > 59) {
		return Occurrence{}, errors.New("second must be between 0 and 60")
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return Occurrence{}, errors.New("invalid occurrence time")
	}
	normalSecond := second
	if leapSecond {
		normalSecond = 59
	}
	normalized := time.Date(year, month, day, hour, minute, normalSecond, 0, time.UTC)
	if normalized.Hour() != hour || normalized.Minute() != minute {
		return Occurrence{}, errors.New("invalid occurrence time")
	}
	if !leapSecond && normalized.Second() != second {
		return Occurrence{}, errors.New("invalid occurrence time")
	}
	return Occurrence{
		year:       normalized.Year(),
		month:      normalized.Month(),
		day:        normalized.Day(),
		hour:       normalized.Hour(),
		minute:     normalized.Minute(),
		second:     second,
		location:   time.UTC,
		leapSecond: leapSecond,
	}, nil
}

// OccurrenceFromTime constructs a regular UTC occurrence.
func OccurrenceFromTime(dt time.Time) (Occurrence, error) {
	if err := checkUTC(dt.Location()); err != nil {
		return Occurrence{}, err
	}
	return NewOccurrence(dt.Year(), dt.Month(), dt.Day(), dt.Hour(), dt.Minute(), dt.Second())
}

// OccurrenceAt constructs a UTC occurrence and, for second 60, verifies it against the table.
func (table *LeapSecondTable) OccurrenceAt(year int, month time.Month, day, hour, minute, second int) (Occurrence, error) {
	occurrence, err := NewOccurrence(year, month, day, hour, minute, second)
	if err != nil {
		return Occurrence{}, err
	}
	if occurrence.leapSecond && !table.hasDate(LeapSecondDate{year, month, day}) {
		return Occurrence{}, ErrLeapSecondNotFound
	}
	occurrence.table = table
	return occurrence, nil
}

// IsLeapSecond reports whether the occurrence is 23:59:60.
func (occurrence Occurrence) IsLeapSecond() bool {
	return occurrence.leapSecond
}

// Clock returns hour, minute, second, and 60 for a positive leap second.
func (occurrence Occurrence) Clock() (hour, minute, second int) {
	return occurrence.hour, occurrence.minute, occurrence.second
}

// Date returns the UTC calendar date.
func (occurrence Occurrence) Date() (year int, month time.Month, day int) {
	return occurrence.year, occurrence.month, occurrence.day
}

// Location always returns UTC.
func (occurrence Occurrence) Location() *time.Location {
	if occurrence.location == nil {
		return time.UTC
	}
	return occurrence.location
}

// IsZero reports whether the occurrence is the zero value.
func (occurrence Occurrence) IsZero() bool {
	return occurrence == Occurrence{}
}

// Time returns the ordinary Go time. It fails for 23:59:60 instead of rolling it forward.
func (occurrence Occurrence) Time() (time.Time, error) {
	if occurrence.leapSecond {
		return time.Time{}, ErrLeapSecond
	}
	return time.Date(occurrence.year, occurrence.month, occurrence.day,
		occurrence.hour, occurrence.minute, occurrence.second,
		0, occurrence.Location()), nil
}

// Compare orders occurrences chronologically and places 23:59:60 before the next 00:00:00.
func (occurrence Occurrence) Compare(other Occurrence) int {
	left := occurrence.orderKey()
	right := other.orderKey()
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func (occurrence Occurrence) before(other Occurrence) bool {
	return occurrence.Compare(other) < 0
}

func (occurrence Occurrence) after(other Occurrence) bool {
	return occurrence.Compare(other) > 0
}

func (occurrence Occurrence) equal(other Occurrence) bool {
	return occurrence.Compare(other) == 0
}

func (occurrence Occurrence) orderKey() int64 {
	day := civilFromDate(occurrence.year, occurrence.month, occurrence.day)
	second := int64(occurrence.hour)*3600 + int64(occurrence.minute)*60 + int64(occurrence.second)
	if occurrence.table == nil {
		return day*86400 + second
	}
	leapsBefore := int64(occurrence.table.leapCountBeforeSerial(int(day)))
	return day*86400 + leapsBefore + second
}

func (occurrence Occurrence) String() string {
	if occurrence.leapSecond {
		return fmt.Sprintf("%04d-%02d-%02dT23:59:60Z", occurrence.year, occurrence.month, occurrence.day)
	}
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02dZ",
		occurrence.year, occurrence.month, occurrence.day,
		occurrence.hour, occurrence.minute, occurrence.second)
}

// LeapSecondROption contains an ordinary recurrence option and an optional leap-second UNTIL.
type LeapSecondROption struct {
	ROption
	Until *Occurrence
}

// LeapSecondRRule is a recurrence rule with an independently enabled leap-second-aware iterator.
type LeapSecondRRule struct {
	rule  *RRule
	table *LeapSecondTable
}

type leapSecondState struct {
	table    *LeapSecondTable
	until    time.Time
	untilOcc *Occurrence
	count    int
	remain   []Occurrence
	finished bool
	iterator *rIterator
}

// NewLeapSecondRRule constructs a UTC, positive-leap-second-aware recurrence rule.
func NewLeapSecondRRule(arg LeapSecondROption, table *LeapSecondTable) (*LeapSecondRRule, error) {
	if table == nil {
		return nil, errors.New("leap second table is required")
	}
	if err := validateBoundsWithSecondLimit(arg.ROption, 60); err != nil {
		return nil, err
	}
	if arg.Interval < 0 {
		return nil, errors.New("interval must be greater than 0")
	}
	if !arg.Dtstart.IsZero() {
		if err := checkUTC(arg.Dtstart.Location()); err != nil {
			return nil, err
		}
	}
	if !arg.ROption.Until.IsZero() && arg.Until != nil {
		return nil, errors.New("UNTIL must be supplied either in ROption or as an Occurrence, not both")
	}
	if arg.Until != nil && !arg.Until.IsZero() {
		if err := checkUTC(arg.Until.Location()); err != nil {
			return nil, err
		}
		if arg.Until.leapSecond && !table.hasDate(LeapSecondDate{
			arg.Until.year, arg.Until.month, arg.Until.day,
		}) {
			return nil, ErrLeapSecondNotFound
		}
		arg.Until.table = table
	}

	if arg.Dtstart.IsZero() {
		arg.Dtstart = time.Now().UTC().Truncate(time.Second)
	}
	if arg.Until != nil {
		if arg.Until.IsZero() {
			arg.ROption.Until = time.Time{}
		} else {
			arg.ROption.Until = arg.Until.regularTime()
		}
	}

	rule := buildRRule(arg.ROption, table)
	if arg.Until != nil && !arg.Until.IsZero() {
		rule.until = arg.Until.regularTime()
		rule.leapUntil = arg.Until
	}

	return &LeapSecondRRule{rule: &rule, table: table}, nil
}

func checkUTC(location *time.Location) error {
	if location != time.UTC {
		return ErrNotUTC
	}
	return nil
}

func (occurrence Occurrence) regularTime() time.Time {
	second := occurrence.second
	if occurrence.leapSecond {
		second = 59
	}
	return time.Date(occurrence.year, occurrence.month, occurrence.day,
		occurrence.hour, occurrence.minute, second,
		0, time.UTC)
}

// Iterator returns a leap-second-aware occurrence iterator.
func (rule *LeapSecondRRule) Iterator() LeapSecondNext {
	state := &leapSecondState{
		table: rule.table,
		until: rule.rule.until,
		count: rule.rule.count,
	}
	if rule.rule.leapUntil != nil {
		state.until = rule.rule.leapUntil.regularTime()
		state.untilOcc = rule.rule.leapUntil
	}
	return rule.rule.leapSecondIterator(state)
}

// All returns all bounded occurrences. COUNT or UNTIL must be set.
func (rule *LeapSecondRRule) All() ([]Occurrence, error) {
	if rule.rule.OrigOptions.Count == 0 &&
		rule.rule.OrigOptions.Until.IsZero() &&
		rule.rule.leapUntil == nil {
		return nil, errors.New("All requires COUNT or UNTIL for a leap-second-aware rule")
	}
	return leapSecondAll(rule.Iterator())
}

// Rule returns the underlying UTC rule. Its ordinary time.Time APIs retain their existing behavior.
func (rule *LeapSecondRRule) Rule() *RRule {
	return rule.rule
}

// Between returns occurrences in the UTC interval. With inc=true, inclusive boundaries must be table-verified leap seconds.
func (rule *LeapSecondRRule) Between(after, before Occurrence, inc bool) ([]Occurrence, error) {
	var err error
	if after, err = rule.checkBoundary(after); err != nil {
		return nil, err
	}
	if before, err = rule.checkBoundary(before); err != nil {
		return nil, err
	}
	return leapSecondBetween(rule.Iterator(), after, before, inc), nil
}

// Before returns the last occurrence before dt.
func (rule *LeapSecondRRule) Before(dt Occurrence, inc bool) (Occurrence, bool, error) {
	var err error
	if dt, err = rule.checkBoundary(dt); err != nil {
		return Occurrence{}, false, err
	}
	return leapSecondBefore(rule.Iterator(), dt, inc)
}

// After returns the first occurrence after dt.
func (rule *LeapSecondRRule) After(dt Occurrence, inc bool) (Occurrence, bool, error) {
	var err error
	if dt, err = rule.checkBoundary(dt); err != nil {
		return Occurrence{}, false, err
	}
	return leapSecondAfter(rule.Iterator(), dt, inc)
}

func (rule *LeapSecondRRule) checkBoundary(occurrence Occurrence) (Occurrence, error) {
	if occurrence.location != nil && occurrence.location != time.UTC {
		return Occurrence{}, ErrNotUTC
	}
	if occurrence.leapSecond &&
		!rule.table.hasDate(LeapSecondDate{occurrence.year, occurrence.month, occurrence.day}) {
		return Occurrence{}, ErrLeapSecondNotFound
	}
	occurrence.table = rule.table
	return occurrence, nil
}

func (state *leapSecondState) next() (Occurrence, bool) {
	if len(state.remain) == 0 && !state.finished {
		state.iterator.generate()
	}
	if len(state.remain) == 0 {
		return Occurrence{}, false
	}
	result := state.remain[0]
	state.remain = state.remain[1:]
	return result, true
}

func (state *leapSecondState) add(occurrence Occurrence) bool {
	dtstart, _ := OccurrenceFromTime(state.iterator.ii.rrule.dtstart)
	if occurrence.before(dtstart) {
		return false
	}
	if state.pastUntil(occurrence) {
		state.finished = true
		return true
	}
	if state.count > 0 {
		state.count--
		if state.count == 0 {
			state.remain = append(state.remain, occurrence)
			state.finished = true
			return true
		}
	}
	state.remain = append(state.remain, occurrence)
	return false
}

func (state *leapSecondState) pastUntil(occurrence Occurrence) bool {
	if state.until.IsZero() {
		return false
	}
	if state.untilOcc != nil {
		return occurrence.after(*state.untilOcc)
	}
	until, _ := OccurrenceFromTime(state.until)
	return occurrence.after(until)
}

type occurrenceSlice []Occurrence

func (values occurrenceSlice) Len() int           { return len(values) }
func (values occurrenceSlice) Swap(i, j int)      { values[i], values[j] = values[j], values[i] }
func (values occurrenceSlice) Less(i, j int) bool { return values[i].before(values[j]) }

func occurrenceContains(values []Occurrence, value Occurrence) bool {
	for _, existing := range values {
		if existing.equal(value) {
			return true
		}
	}
	return false
}

func leapSecondAll(next LeapSecondNext) ([]Occurrence, error) {
	result := []Occurrence{}
	for {
		value, ok := next()
		if !ok {
			return result, nil
		}
		result = append(result, value)
	}
}

func leapSecondBetween(next LeapSecondNext, after, before Occurrence, inc bool) []Occurrence {
	result := []Occurrence{}
	for {
		value, ok := next()
		if !ok {
			return result
		}
		if inc {
			if value.after(before) {
				return result
			}
			if !value.before(after) {
				result = append(result, value)
			}
		} else if value.after(after) && value.before(before) {
			result = append(result, value)
		} else if !value.before(before) {
			return result
		}
	}
}

func leapSecondBefore(next LeapSecondNext, dt Occurrence, inc bool) (Occurrence, bool, error) {
	result := Occurrence{}
	found := false
	for {
		value, ok := next()
		if !ok {
			return result, found, nil
		}
		if inc {
			if value.after(dt) {
				return result, found, nil
			}
		} else if !value.before(dt) {
			return result, found, nil
		}
		result = value
		found = true
	}
}

func leapSecondAfter(next LeapSecondNext, dt Occurrence, inc bool) (Occurrence, bool, error) {
	for {
		value, ok := next()
		if !ok {
			return Occurrence{}, false, nil
		}
		if inc {
			if !value.before(dt) {
				return value, true, nil
			}
		} else if value.after(dt) {
			return value, true, nil
		}
	}
}

func utcOccurrence(table *LeapSecondTable, year int, month time.Month, day, hour, minute, second int, leapSecond bool) Occurrence {
	return Occurrence{
		year:       year,
		month:      month,
		day:        day,
		hour:       hour,
		minute:     minute,
		second:     second,
		location:   time.UTC,
		leapSecond: leapSecond,
		table:      table,
	}
}

func (iterator *rIterator) generateLeapSecondOccurrences(dayset []optInt, filtered bool) bool {
	r := iterator.ii.rrule
	candidates := make([]Occurrence, 0)

	for _, day := range dayset {
		if !day.Defined {
			continue
		}
		dateYear, dateMonth, dateDay := iterator.ii.firstyday.AddDate(0, 0, day.Int).Date()

		if r.freq == SECONDLY {
			iterator.addSecondCandidate(&candidates, dateYear, dateMonth, dateDay)
		} else {
			iterator.addPeriodCandidates(&candidates, dateYear, dateMonth, dateDay)
		}
	}

	sort.Sort(occurrenceSlice(candidates))
	if len(r.bysetpos) != 0 {
		selected := make([]Occurrence, 0, len(r.bysetpos))
		for _, pos := range r.bysetpos {
			index := pos - 1
			if pos < 0 {
				index = pos + len(candidates)
			}
			if index < 0 || index >= len(candidates) {
				continue
			}
			candidate := candidates[index]
			if !occurrenceContains(selected, candidate) {
				selected = append(selected, candidate)
			}
		}
		sort.Sort(occurrenceSlice(selected))
		candidates = selected
	}

	for _, candidate := range candidates {
		if iterator.leap.add(candidate) {
			return true
		}
	}

	if r.freq == SECONDLY {
		return iterator.advanceLeapSecond(filtered)
	}
	return false
}

func (iterator *rIterator) addPeriodCandidates(candidates *[]Occurrence, year int, month time.Month, day int) {
	r := iterator.ii.rrule
	hours := r.byhour
	minutes := r.byminute
	seconds := r.bysecond

	if r.freq == HOURLY {
		hours = []int{iterator.hour}
	} else if r.freq == MINUTELY {
		hours = []int{iterator.hour}
		minutes = []int{iterator.minute}
	}

	for _, hour := range hours {
		for _, minute := range minutes {
			for _, second := range seconds {
				if second == 60 {
					if hour == 23 && minute == 59 && iterator.leap.table.has(year, month, day) {
						iterator.appendCandidate(candidates, utcOccurrence(iterator.leap.table, year, month, day, 23, 59, 60, true))
					}
					continue
				}
				iterator.appendCandidate(candidates, utcOccurrence(iterator.leap.table, year, month, day, hour, minute, second, false))
			}
		}
	}
}

func (iterator *rIterator) addSecondCandidate(candidates *[]Occurrence, year int, month time.Month, day int) {
	if iterator.second <= 59 {
		if len(iterator.ii.rrule.bysecond) == 0 || contains(iterator.ii.rrule.bysecond, iterator.second) {
			iterator.appendCandidate(candidates,
				utcOccurrence(iterator.leap.table, year, month, day, iterator.hour, iterator.minute, iterator.second, false))
		}
		return
	}

	if iterator.hour == 23 && iterator.minute == 59 && iterator.leap.table.has(year, month, day) &&
		(len(iterator.ii.rrule.bysecond) == 0 || contains(iterator.ii.rrule.bysecond, 60)) {
		iterator.appendCandidate(candidates, utcOccurrence(iterator.leap.table, year, month, day, 23, 59, 60, true))
	}
}

func (iterator *rIterator) appendCandidate(candidates *[]Occurrence, candidate Occurrence) {
	if len(iterator.ii.rrule.byhour) != 0 && !contains(iterator.ii.rrule.byhour, candidate.hour) ||
		len(iterator.ii.rrule.byminute) != 0 && !contains(iterator.ii.rrule.byminute, candidate.minute) {
		return
	}
	if !candidate.leapSecond && len(iterator.ii.rrule.bysecond) != 0 &&
		!contains(iterator.ii.rrule.bysecond, candidate.second) {
		return
	}
	if !occurrenceContains(*candidates, candidate) {
		*candidates = append(*candidates, candidate)
	}
}

func (iterator *rIterator) advanceLeapSecond(filtered bool) bool {
	r := iterator.ii.rrule
	if filtered {
		iterator.jumpLeapSecondToDayEnd()
	}

	interval := int64(r.interval)
	for {
		if iterator.second == 60 {
			iterator.ordinal++
		} else {
			iterator.ordinal += interval
		}
		if iterator.setCivilFromOrdinal() {
			iterator.leap.finished = true
			return true
		}
		current := utcOccurrence(iterator.leap.table, iterator.year, iterator.month, iterator.day,
			iterator.hour, iterator.minute, iterator.second, iterator.second == 60)
		if (iterator.leap.count == 0 || len(iterator.leap.remain) > 0) &&
			iterator.leap.pastUntil(current) {
			iterator.leap.finished = true
			return true
		}
		if len(r.byhour) == 0 || contains(r.byhour, iterator.hour) {
			if len(r.byminute) == 0 || contains(r.byminute, iterator.minute) {
				if iterator.second == 60 || len(r.bysecond) == 0 || contains(r.bysecond, iterator.second) {
					return false
				}
			}
		}
	}
}

func (iterator *rIterator) jumpLeapSecondToDayEnd() {
	serial := daysFromCivil(iterator.year, int(iterator.month), iterator.day)
	base := int64(serial)*86400 + int64(iterator.leap.table.leapCountBeforeSerial(serial))
	if iterator.leap.table.has(iterator.year, iterator.month, iterator.day) {
		iterator.ordinal = base + 86400
	} else {
		iterator.ordinal = base + 86399
	}
}

func (iterator *rIterator) setCivilFromOrdinal() bool {
	ordinal := iterator.ordinal
	if ordinal < 0 {
		return true
	}
	table := iterator.leap.table

	serial := int(ordinal / 86400)
	var remainder int
	for {
		leapsBefore := table.leapCountBeforeSerial(serial)
		start := int64(serial)*86400 + int64(leapsBefore)
		dayLength := 86400
		if table.hasDateAtSerial(serial) {
			dayLength = 86401
		}
		end := start + int64(dayLength)
		if ordinal < start {
			serial--
			continue
		}
		if ordinal >= end {
			serial++
			continue
		}
		remainder = int(ordinal - start)
		break
	}

	previousYear, previousMonth := iterator.year, iterator.month
	year, month, day := civilFromDays(serial)
	isLeap := remainder == 86400 && table.hasDateAtSerial(serial)
	if isLeap {
		remainder = 86399
	}
	iterator.year = year
	iterator.month = month
	iterator.day = day
	iterator.hour = remainder / 3600
	iterator.minute = remainder % 3600 / 60
	iterator.second = remainder % 60
	if isLeap {
		iterator.second = 60
	}
	if year != previousYear || month != previousMonth {
		iterator.weekday = toPyWeekday(utcDate(year, month, day).Weekday())
		iterator.ii.rebuild(year, month)
	}
	return year > MAXYEAR
}

func utcDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func daysFromCivil(year int, month, day int) int {
	y := year
	if month <= 2 {
		y--
	}
	era := 0
	if y >= 0 {
		era = y / 400
	} else {
		era = (y - 399) / 400
	}
	yoe := y - era*400
	doy := (153*(month-3+12*btoi(month <= 2))+2)/5 + day - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

func civilFromDays(serial int) (int, time.Month, int) {
	z := serial + 719468
	era := 0
	if z >= 0 {
		era = z / 146097
	} else {
		era = (z - 146096) / 146097
	}
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d := doy - (153*mp+2)/5 + 1
	m := mp + 3
	if m > 12 {
		m -= 12
	}
	if m <= 2 {
		y++
	}
	return y, time.Month(m), d
}

func civilFromDate(year int, month time.Month, day int) int64 {
	return int64(daysFromCivil(year, int(month), day))
}

func btoi(value bool) int {
	if value {
		return 1
	}
	return 0
}
