// 2026, Teambition. All rights reserved.

package rrule

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// DateTimeKind identifies how a calendar value relates to the absolute
// timeline, as distinguished by RFC 5545.
type DateTimeKind int

const (
	// KindDate is a pure date (VALUE=DATE), e.g. 19970714.
	// It has no time of day and is not tied to any time zone.
	KindDate DateTimeKind = iota
	// KindFloating is a local date-time without a time zone reference,
	// e.g. 19970714T133000. It denotes wall-clock time; the caller must
	// supply a location to turn it into an absolute instant.
	KindFloating
	// KindTZID is a local date-time with a time zone reference,
	// e.g. DTSTART;TZID=America/New_York:19970714T133000.
	KindTZID
	// KindUTC is an absolute UTC date-time, e.g. 19970714T133000Z.
	KindUTC
)

// String returns the RFC 5545 name of the kind.
func (k DateTimeKind) String() string {
	switch k {
	case KindDate:
		return "DATE"
	case KindFloating:
		return "DATE-TIME (floating)"
	case KindTZID:
		return "DATE-TIME;TZID"
	case KindUTC:
		return "DATE-TIME (UTC)"
	default:
		return "UNKNOWN"
	}
}

// DateTime preserves the original iCalendar time kind together with its
// wall-clock fields. A zero DateTime (Kind == KindDate with the zero time)
// reports IsZero.
//
// The fields are:
//   - KindDate:     wall fields are read in UTC, TZID is empty
//   - KindFloating: wall fields are read in UTC, TZID is empty
//   - KindTZID:     wall fields are read in UTC, TZID names the zone and Loc
//     resolves it
//   - KindUTC:      wall fields are read in UTC, TZID is empty
//
// Floating values never carry time.Local; use InLocation to evaluate them.
type DateTime struct {
	kind DateTimeKind
	wall time.Time
	tzid string
	loc  *time.Location
}

// NewDate builds a pure date value (VALUE=DATE).
func NewDate(year int, month time.Month, day int) DateTime {
	return DateTime{
		kind: KindDate,
		wall: time.Date(year, month, day, 0, 0, 0, 0, time.UTC),
	}
}

// NewFloating builds a floating local date-time without a time zone.
func NewFloating(year int, month time.Month, day, hour, min, sec int) DateTime {
	return DateTime{
		kind: KindFloating,
		wall: time.Date(year, month, day, hour, min, sec, 0, time.UTC),
	}
}

// NewTZID builds a local date-time in the named time zone. The location is
// resolved eagerly so an unknown TZID is reported here rather than later.
func NewTZID(year int, month time.Month, day, hour, min, sec int, tzid string) (DateTime, error) {
	loc, err := time.LoadLocation(tzid)
	if err != nil {
		return DateTime{}, fmt.Errorf("invalid TZID %q: %w", tzid, err)
	}
	return DateTime{
		kind: KindTZID,
		wall: time.Date(year, month, day, hour, min, sec, 0, time.UTC),
		tzid: tzid,
		loc:  loc,
	}, nil
}

// NewUTC builds an absolute UTC date-time.
func NewUTC(year int, month time.Month, day, hour, min, sec int) DateTime {
	return DateTime{
		kind: KindUTC,
		wall: time.Date(year, month, day, hour, min, sec, 0, time.UTC),
	}
}

// FromTimeTime converts a time.Time into a DateTime, classifying it by its
// location. This is the bridge from APIs that accept plain time.Time:
// time.UTC becomes KindUTC, a named location becomes KindTZID and any other
// location (including time.Local without a name) becomes KindFloating.
func FromTimeTime(t time.Time) DateTime {
	t = t.Truncate(time.Second)
	switch loc := t.Location(); {
	case loc == time.UTC:
		return DateTime{kind: KindUTC, wall: t.UTC()}
	case loc.String() != "" && loc != time.Local:
		return DateTime{kind: KindTZID, wall: t.UTC(), tzid: loc.String(), loc: loc}
	default:
		return DateTime{kind: KindFloating, wall: t.UTC()}
	}
}

// Kind returns the preserved value kind.
func (dt DateTime) Kind() DateTimeKind { return dt.kind }

// TZID returns the time zone name for KindTZID and the empty string otherwise.
func (dt DateTime) TZID() string { return dt.tzid }

// Date returns the calendar date fields.
func (dt DateTime) Date() (int, time.Month, int) { return dt.wall.Date() }

// Clock returns the hour, minute and second. For KindDate these are all zero.
func (dt DateTime) Clock() (int, int, int) { return dt.wall.Clock() }

// Weekday returns the weekday of the wall-clock date.
func (dt DateTime) Weekday() time.Weekday { return dt.wall.Weekday() }

// YearDay returns the day of year of the wall-clock date.
func (dt DateTime) YearDay() int { return dt.wall.YearDay() }

// IsZero reports whether dt is the zero DateTime.
func (dt DateTime) IsZero() bool {
	return dt.kind == KindDate && dt.wall.IsZero() && dt.tzid == ""
}

// Equal reports whether two values have the same kind, TZID and wall-clock
// fields. Comparing different kinds always returns false; a DATE never
// equals a date-time, even if they fall on the same calendar day.
func (dt DateTime) Equal(other DateTime) bool {
	if dt.kind != other.kind {
		return false
	}
	if dt.kind == KindTZID && dt.tzid != other.tzid {
		return false
	}
	return dt.wall.Equal(other.wall)
}

// InLocation evaluates the value into an absolute time.Time in the given
// location. The loc parameter is used for floating values; for other kinds
// the preserved zone is used and loc is ignored (it may be nil).
func (dt DateTime) InLocation(loc *time.Location) (time.Time, error) {
	y, m, d := dt.wall.Date()
	h, mi, s := dt.wall.Clock()
	switch dt.kind {
	case KindDate:
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
	case KindFloating:
		if loc == nil {
			return time.Time{}, errors.New("rrule: evaluating a floating date-time requires an explicit location")
		}
		return time.Date(y, m, d, h, mi, s, 0, loc), nil
	case KindTZID:
		if dt.loc == nil {
			return time.Time{}, fmt.Errorf("rrule: unresolved TZID %q", dt.tzid)
		}
		return time.Date(y, m, d, h, mi, s, 0, dt.loc), nil
	case KindUTC:
		return time.Date(y, m, d, h, mi, s, 0, time.UTC), nil
	default:
		return time.Time{}, fmt.Errorf("rrule: unknown date-time kind %d", dt.kind)
	}
}

// String renders the value in RFC 5545 form, preserving the kind and TZID:
//
//	VALUE=DATE:19970714
//	19970714T133000
//	TZID=America/New_York:19970714T133000
//	19970714T133000Z
func (dt DateTime) String() string {
	switch dt.kind {
	case KindDate:
		return "VALUE=DATE:" + dt.wall.Format(DateFormat)
	case KindFloating:
		return dt.wall.Format(LocalDateTimeFormat)
	case KindTZID:
		return "TZID=" + dt.tzid + ":" + dt.wall.Format(LocalDateTimeFormat)
	case KindUTC:
		return dt.wall.Format(DateTimeFormat)
	default:
		return ""
	}
}

// StrToDateTime parses a single RFC 5545 date or date-time value while
// preserving its kind. The str argument is the value with optional
// parameters, e.g.
//
//	VALUE=DATE:20261003
//	20261003T090000                    (floating)
//	TZID=America/New_York:20261003T090000
//	20261003T090000Z                   (UTC)
//
// Floating values are not attached to time.Local; defaultLoc is ignored for
// them and only used for bare values when explicitly requested by callers
// that build on the legacy parser.
func StrToDateTime(str string) (DateTime, error) {
	return strToDateTime(str, nil)
}

func strToDateTime(str string, defaultLoc *time.Location) (DateTime, error) {
	str = strings.TrimSpace(str)
	if str == "" {
		return DateTime{}, errors.New("empty date-time value")
	}

	params := ""
	value := str
	if idx := strings.Index(str, ":"); idx >= 0 {
		params, value = str[:idx], str[idx+1:]
	}
	if strings.Contains(value, ":") {
		return DateTime{}, fmt.Errorf("bad date-time value: %q", str)
	}

	kind := KindFloating
	var loc *time.Location
	if params != "" {
		for _, param := range strings.Split(params, ";") {
			switch {
			case param == "VALUE=DATE":
				kind = KindDate
			case param == "VALUE=DATE-TIME":
				kind = KindFloating
			case strings.HasPrefix(param, "TZID="):
				var err error
				loc, err = parseTZID(param)
				if err != nil {
					return DateTime{}, err
				}
				kind = KindTZID
			default:
				return DateTime{}, fmt.Errorf("unsupported date-time parameter: %q", param)
			}
		}
	}

	switch {
	case len(value) == len(DateFormat):
		if kind == KindTZID {
			return DateTime{}, errors.New("TZID cannot be combined with VALUE=DATE")
		}
		t, err := time.Parse(DateFormat, value)
		if err != nil {
			return DateTime{}, err
		}
		return DateTime{kind: KindDate, wall: t}, nil
	case len(value) == len(DateTimeFormat) && strings.HasSuffix(value, "Z"):
		if kind == KindDate {
			return DateTime{}, errors.New("VALUE=DATE requires a date without time")
		}
		t, err := time.Parse(DateTimeFormat, value)
		if err != nil {
			return DateTime{}, err
		}
		return DateTime{kind: KindUTC, wall: t.UTC()}, nil
	case len(value) == len(LocalDateTimeFormat):
		if kind == KindDate {
			return DateTime{}, errors.New("VALUE=DATE requires a date without time")
		}
		t, err := time.Parse(LocalDateTimeFormat, value)
		if err != nil {
			return DateTime{}, err
		}
		if kind == KindTZID {
			if loc == nil {
				return DateTime{}, errors.New("missing location for TZID value")
			}
			return DateTime{kind: KindTZID, wall: t.UTC(), tzid: loc.String(), loc: loc}, nil
		}
		// A bare local date-time stays floating. defaultLoc is deliberately
		// not applied: parsing must never invent a zone.
		_ = defaultLoc
		return DateTime{kind: KindFloating, wall: t.UTC()}, nil
	default:
		return DateTime{}, fmt.Errorf("unrecognized date-time value: %q", value)
	}
}
