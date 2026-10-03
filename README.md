# rrule-go

Go library for working with recurrence rules for calendar dates.

[![CI](https://github.com/teambition/rrule-go/actions/workflows/ci.yml/badge.svg)](https://github.com/teambition/rrule-go/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/teambition/rrule-go/master/main/graph/badge.svg)](https://codecov.io/gh/teambition/rrule-go)
[![CodeQL](https://github.com/teambition/rrule-go/actions/workflows/codeql.yml/badge.svg)](https://github.com/teambition/rrule-go/actions/workflows/codeql.yml)
[![License](http://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://raw.githubusercontent.com/teambition/rrule-go/master/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/teambition/rrule-go.svg)](https://pkg.go.dev/github.com/teambition/rrule-go)

The rrule module offers a complete implementation of the recurrence rules documented in the [iCalendar
RFC](http://www.ietf.org/rfc/rfc2445.txt). It is a partial port of the rrule module from the excellent [python-dateutil](http://labix.org/python-dateutil/) library.

## Demo

### rrule.RRule

```go
package main

import (
  "fmt"
  "time"

  "github.com/teambition/rrule-go"
)

func printTimeSlice(ts []time.Time) {
	for _, t := range ts {
		fmt.Println(t)
	}
}

func main() {
	// Daily, for 10 occurrences.
	r, _ := rrule.NewRRule(rrule.ROption{
		Freq:    rrule.DAILY,
		Count:   10,
		Dtstart: time.Date(1997, 9, 2, 9, 0, 0, 0, time.UTC),
	})

	fmt.Println(r.String())
	// DTSTART:19970902T090000Z
	// RRULE:FREQ=DAILY;COUNT=10

	printTimeSlice(r.All())
	// 1997-09-02 09:00:00 +0000 UTC
	// 1997-09-03 09:00:00 +0000 UTC
	// ...
	// 1997-09-07 09:00:00 +0000 UTC

	printTimeSlice(r.Between(
		time.Date(1997, 9, 6, 0, 0, 0, 0, time.UTC),
		time.Date(1997, 9, 8, 0, 0, 0, 0, time.UTC), true))
	// [1997-09-06 09:00:00 +0000 UTC
	//  1997-09-07 09:00:00 +0000 UTC]

	// Every four years, the first Tuesday after a Monday in November, 3 occurrences (U.S. Presidential Election day).
	r, _ = rrule.NewRRule(rrule.ROption{
		Freq:       rrule.YEARLY,
		Interval:   4,
		Count:      3,
		Bymonth:    []int{11},
		Byweekday:  []rrule.Weekday{rrule.TU},
		Bymonthday: []int{2, 3, 4, 5, 6, 7, 8},
		Dtstart:    time.Date(1996, 11, 5, 9, 0, 0, 0, time.UTC),
	})

	fmt.Println(r.String())
	// DTSTART:19961105T090000Z
	// RRULE:FREQ=YEARLY;INTERVAL=4;COUNT=3;BYMONTH=11;BYMONTHDAY=2,3,4,5,6,7,8;BYDAY=TU

	printTimeSlice(r.All())
	// 1996-11-05 09:00:00 +0000 UTC
	// 2000-11-07 09:00:00 +0000 UTC
	// 2004-11-02 09:00:00 +0000 UTC
}

```

### rrule.Set

```go
func ExampleSet() {
	// Daily, for 7 days, jumping Saturday and Sunday occurrences.
	set := rrule.Set{}
	r, _ := rrule.NewRRule(rrule.ROption{
		Freq:    rrule.DAILY,
		Count:   7,
		Dtstart: time.Date(1997, 9, 2, 9, 0, 0, 0, time.UTC)})
	set.RRule(r)

	fmt.Println(set.String())
	// DTSTART:19970902T090000Z
	// RRULE:FREQ=DAILY;COUNT=7

	printTimeSlice(set.All())
	// 1997-09-02 09:00:00 +0000 UTC
	// 1997-09-03 09:00:00 +0000 UTC
	// 1997-09-04 09:00:00 +0000 UTC
	// 1997-09-05 09:00:00 +0000 UTC
	// 1997-09-06 09:00:00 +0000 UTC
	// 1997-09-07 09:00:00 +0000 UTC
	// 1997-09-08 09:00:00 +0000 UTC

	// Weekly, for 4 weeks, plus one time on day 7, and not on day 16.
	set = rrule.Set{}
	r, _ = rrule.NewRRule(rrule.ROption{
		Freq:    rrule.WEEKLY,
		Count:   4,
		Dtstart: time.Date(1997, 9, 2, 9, 0, 0, 0, time.UTC)})
	set.RRule(r)
	set.RDate(time.Date(1997, 9, 7, 9, 0, 0, 0, time.UTC))
	set.ExDate(time.Date(1997, 9, 16, 9, 0, 0, 0, time.UTC))

	fmt.Println(set.String())
	// DTSTART:19970902T090000Z
	// RRULE:FREQ=WEEKLY;COUNT=4
	// RDATE:19970907T090000Z
	// EXDATE:19970916T090000Z

	printTimeSlice(set.All())
	// 1997-09-02 09:00:00 +0000 UTC
	// 1997-09-07 09:00:00 +0000 UTC
	// 1997-09-09 09:00:00 +0000 UTC
	// 1997-09-23 09:00:00 +0000 UTC
}
```

### Leap seconds

The ordinary `RRule` API continues to return `time.Time` values. Use `LeapSecondRRule` when an
archive must preserve UTC `23:59:60`. The caller supplies a finite, already-verified table of
positive leap second dates; the library does not download one.

```go
january, _ := rrule.NewLeapSecondDate(2100, time.January, 31)
table, _ := rrule.NewLeapSecondTable(january)

r, _ := rrule.NewLeapSecondRRule(rrule.LeapSecondROption{
  ROption: rrule.ROption{
    Freq:    rrule.SECONDLY,
    Count:   3,
    Dtstart: time.Date(2100, 1, 31, 23, 59, 59, 0, time.UTC),
  },
}, table)

occurrences, _ := r.All()
for _, occurrence := range occurrences {
  fmt.Println(occurrence)
  if occurrence.IsLeapSecond() {
    _, err := occurrence.Time() // rrule.ErrLeapSecond
    _ = err
  }
}
// 2100-01-31T23:59:59Z
// 2100-01-31T23:59:60Z
// 2100-02-01T00:00:00Z
```

`BYSECOND=60` produces a candidate only when the rule reaches a date present in the supplied
table. A date absent from the finite table means "no data for this date"; it must not be
interpreted as confirmation that the date had no leap second. Leap-second evaluation is limited
to UTC and positive leap seconds; negative leap seconds and non-UTC locations are rejected.

### rrule.StrToRRule

```go
func ExampleStrToRRule() {
	// Compatible with old DTSTART
	r, _ := rrule.StrToRRule("FREQ=DAILY;DTSTART=20060101T150405Z;COUNT=5")
	fmt.Println(r.OrigOptions.RRuleString())
	// FREQ=DAILY;COUNT=5

	fmt.Println(r.OrigOptions.String())
	// DTSTART:20060101T150405Z
	// RRULE:FREQ=DAILY;COUNT=5

	fmt.Println(r.String())
	// DTSTART:20060101T150405Z
	// RRULE:FREQ=DAILY;COUNT=5

	printTimeSlice(r.All())
	// 2006-01-01 15:04:05 +0000 UTC
	// 2006-01-02 15:04:05 +0000 UTC
	// 2006-01-03 15:04:05 +0000 UTC
	// 2006-01-04 15:04:05 +0000 UTC
	// 2006-01-05 15:04:05 +0000 UTC
}
```

### rrule.StrToRRuleSet

```go
func ExampleStrToRRuleSet() {
	s, _ := rrule.StrToRRuleSet("DTSTART:20060101T150405Z\nRRULE:FREQ=DAILY;COUNT=5\nEXDATE:20060102T150405Z")
	fmt.Println(s.String())
	// DTSTART:20060101T150405Z
	// RRULE:FREQ=DAILY;COUNT=5
	// EXDATE:20060102T150405Z

	printTimeSlice(s.All())
	// 2006-01-01 15:04:05 +0000 UTC
	// 2006-01-03 15:04:05 +0000 UTC
	// 2006-01-04 15:04:05 +0000 UTC
	// 2006-01-05 15:04:05 +0000 UTC
}
```

For more examples see [python-dateutil](http://labix.org/python-dateutil/) documentation.

## License

Gear is licensed under the [MIT](https://github.com/teambition/gear/blob/master/LICENSE) license.
Copyright &copy; 2017-2023 [Teambition](https://www.teambition.com).
