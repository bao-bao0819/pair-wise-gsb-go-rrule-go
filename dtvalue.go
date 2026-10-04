// 2017-2022, Teambition. All rights reserved.

package rrule

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// DTKind identifies the semantic kind of a DTValue, mirroring the
// date-time forms defined by RFC 5545.
type DTKind int

const (
	// KindDate is a pure calendar date (VALUE=DATE), e.g. 20261003.
	KindDate DTKind = iota
	// KindFloating is a date-time without any zone reference, e.g. 20261003T090000.
	KindFloating
	// KindTZID is a local date-time with an explicit TZID, e.g.
	// TZID=America/New_York:20261003T090000.
	KindTZID
	// KindUTC is a date-time in UTC, e.g. 20261003T090000Z.
	KindUTC
)

func (k DTKind) String() string {
	return [...]string{"DATE", "FLOATING", "TZID", "UTC"}[k]
}

// DTValue is a date or date-time that preserves its RFC 5545 value kind.
// Unlike time.Time it does not silently attach a location to floating
// times: a floating value only becomes an absolute instant when the
// caller explicitly evaluates it in a location via In.
type DTValue struct {
	kind DTKind
	// t holds the wall clock. For KindDate it is midnight UTC of the date.
	// For KindFloating it is the wall clock stored in a neutral UTC
	// location (no zone semantics attached). For KindTZID it is the time
	// in the referenced location. For KindUTC it is the UTC instant.
	t    time.Time
	tzid string
}

// IsZero reports whether the value is uninitialized.
func (v DTValue) IsZero() bool {
	return v.t.IsZero()
}

// Kind returns the value kind of v.
func (v DTValue) Kind() DTKind {
	return v.kind
}

// TZID returns the original TZID parameter for KindTZID values,
// otherwise an empty string.
func (v DTValue) TZID() string {
	return v.tzid
}

// WallClock returns the wall clock of v without any zone semantics:
// for KindDate it is midnight of the date, for KindFloating the nominal
// local time, for KindTZID and KindUTC the time in its own location.
func (v DTValue) WallClock() time.Time {
	return v.t
}

// DateValue constructs a pure date (VALUE=DATE).
func DateValue(year int, month time.Month, day int) DTValue {
	return DTValue{kind: KindDate, t: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// FloatingValue constructs a floating date-time from the wall clock of t.
// The location of t is intentionally discarded.
func FloatingValue(t time.Time) DTValue {
	year, month, day := t.Date()
	hour, minute, second := t.Clock()
	return DTValue{kind: KindFloating, t: time.Date(year, month, day, hour, minute, second, 0, time.UTC)}
}

// TZIDValue constructs a local date-time referencing tzid.
// If tzid is empty, the location name of t is used.
func TZIDValue(t time.Time, tzid string) DTValue {
	if tzid == "" {
		tzid = t.Location().String()
	}
	return DTValue{kind: KindTZID, t: t.Truncate(time.Second), tzid: tzid}
}

// UTCValue constructs a UTC date-time.
func UTCValue(t time.Time) DTValue {
	return DTValue{kind: KindUTC, t: t.UTC().Truncate(time.Second)}
}

// In evaluates v to an absolute time.Time.
//   - KindDate has no absolute instant; use date-boundary queries instead.
//   - KindFloating requires an explicit non-nil loc; the wall clock is
//     interpreted in loc. loc is never defaulted to time.Local.
//   - KindTZID and KindUTC are already absolute; loc is ignored.
func (v DTValue) In(loc *time.Location) (time.Time, error) {
	switch v.kind {
	case KindDate:
		return time.Time{}, errors.New("pure date has no absolute instant; use date-boundary queries")
	case KindFloating:
		if loc == nil {
			return time.Time{}, errors.New("floating time requires an explicit evaluation location")
		}
		year, month, day := v.t.Date()
		hour, minute, second := v.t.Clock()
		return time.Date(year, month, day, hour, minute, second, 0, loc), nil
	default:
		return v.t, nil
	}
}

// Equal reports whether v and o denote the same value. Comparison is
// kind-aware: a pure date never equals a date-time, and a floating time
// never equals an absolute (TZID/UTC) time. Two absolute times are equal
// when they denote the same instant, even across KindTZID and KindUTC.
func (v DTValue) Equal(o DTValue) bool {
	if v.kind == o.kind {
		return v.t.Equal(o.t)
	}
	if v.isAbsolute() && o.isAbsolute() {
		return v.t.Equal(o.t)
	}
	return false
}

func (v DTValue) isAbsolute() bool {
	return v.kind == KindTZID || v.kind == KindUTC
}

// String returns the RFC 5545 property suffix of v (parameters and value),
// suitable to append after a property name such as DTSTART, RDATE or EXDATE:
//
//	;VALUE=DATE:20261003
//	:20261003T090000
//	;TZID=America/New_York:20261003T090000
//	:20261003T090000Z
func (v DTValue) String() string {
	switch v.kind {
	case KindDate:
		return ";VALUE=DATE:" + v.t.Format(DateFormat)
	case KindFloating:
		return ":" + v.t.Format(LocalDateTimeFormat)
	case KindTZID:
		return fmt.Sprintf(";TZID=%s:%s", v.tzid, v.t.Format(LocalDateTimeFormat))
	default:
		return ":" + v.t.UTC().Format(DateTimeFormat)
	}
}

// rruleValue returns the value form used inside RRULE properties (UNTIL),
// which carries no parameters.
func (v DTValue) rruleValue() string {
	switch v.kind {
	case KindDate:
		return v.t.Format(DateFormat)
	case KindFloating:
		return v.t.Format(LocalDateTimeFormat)
	default:
		return v.t.UTC().Format(DateTimeFormat)
	}
}

// ParseDTValue parses an RFC 5545 date or date-time, preserving its kind.
// It accepts a bare value ("20261003", "20261003T090000", "20261003T090000Z")
// or a value with property parameters (";VALUE=DATE:20261003",
// ";TZID=America/New_York:20261003T090000", "VALUE=DATE-TIME:...").
// Floating times are stored without attaching any location.
func ParseDTValue(s string) (DTValue, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DTValue{}, errors.New("empty date-time value")
	}
	value := s
	params := ""
	if i := strings.IndexByte(s, ':'); i >= 0 {
		params = s[:i]
		value = s[i+1:]
	}
	params = strings.TrimPrefix(params, ";")

	var loc *time.Location
	tzid := ""
	valueType := ""
	if params != "" {
		for _, param := range strings.Split(params, ";") {
			switch {
			case strings.HasPrefix(param, "TZID="):
				tzid = param[len("TZID="):]
				if tzid == "" {
					return DTValue{}, fmt.Errorf("bad TZID parameter format")
				}
				l, err := time.LoadLocation(tzid)
				if err != nil {
					return DTValue{}, err
				}
				loc = l
			case strings.HasPrefix(param, "VALUE="):
				valueType = param[len("VALUE="):]
				if valueType != "DATE" && valueType != "DATE-TIME" {
					return DTValue{}, fmt.Errorf("unsupported: %s", param)
				}
			default:
				return DTValue{}, fmt.Errorf("unsupported: %s", param)
			}
		}
	}
	if value == "" {
		return DTValue{}, fmt.Errorf("bad format")
	}

	if len(value) == len(DateFormat) {
		if valueType == "DATE-TIME" {
			return DTValue{}, fmt.Errorf("VALUE=DATE-TIME requires a date-time value: %s", value)
		}
		if loc != nil {
			return DTValue{}, fmt.Errorf("TZID is not allowed on a DATE value")
		}
		t, err := time.ParseInLocation(DateFormat, value, time.UTC)
		if err != nil {
			return DTValue{}, err
		}
		return DTValue{kind: KindDate, t: t}, nil
	}

	if strings.HasSuffix(value, "Z") {
		if loc != nil {
			return DTValue{}, fmt.Errorf("TZID is not allowed on a UTC date-time")
		}
		if valueType == "DATE" {
			return DTValue{}, fmt.Errorf("VALUE=DATE requires a date value: %s", value)
		}
		t, err := time.Parse(DateTimeFormat, value)
		if err != nil {
			return DTValue{}, err
		}
		return DTValue{kind: KindUTC, t: t}, nil
	}

	if valueType == "DATE" {
		return DTValue{}, fmt.Errorf("VALUE=DATE requires a date value: %s", value)
	}
	if loc != nil {
		t, err := time.ParseInLocation(LocalDateTimeFormat, value, loc)
		if err != nil {
			return DTValue{}, err
		}
		return DTValue{kind: KindTZID, t: t, tzid: tzid}, nil
	}
	// Floating time: parse in a neutral location, no zone semantics attached.
	t, err := time.ParseInLocation(LocalDateTimeFormat, value, time.UTC)
	if err != nil {
		return DTValue{}, err
	}
	return DTValue{kind: KindFloating, t: t}, nil
}

// ParseDTStartLine parses a full DTSTART property line, e.g.
// "DTSTART;TZID=America/New_York:20261003T090000" or
// "DTSTART;VALUE=DATE:20261003".
func ParseDTStartLine(line string) (DTValue, error) {
	name, err := processRRuleName(line)
	if err != nil {
		return DTValue{}, err
	}
	if name != "DTSTART" {
		return DTValue{}, fmt.Errorf("expect DTSTART but: %s", name)
	}
	return ParseDTValue(line[len(name):])
}

// parseDTValueList parses the part after an RDATE/EXDATE property name,
// e.g. ";TZID=America/New_York:20261003T090000,20261004T090000" or
// ";VALUE=DATE:20261003,20261004".
func parseDTValueList(rule string) ([]DTValue, error) {
	params := ""
	values := rule
	if i := strings.IndexByte(rule, ':'); i >= 0 {
		params = rule[:i]
		values = rule[i+1:]
	}
	var res []DTValue
	for _, vs := range strings.Split(values, ",") {
		v, err := ParseDTValue(params + ":" + vs)
		if err != nil {
			return nil, err
		}
		res = append(res, v)
	}
	return res, nil
}
