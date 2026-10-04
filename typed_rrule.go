// 2026, Teambition. All rights reserved.

package rrule

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// TypedROption builds a recurrence rule while preserving the kind of
// Dtstart and Until. It mirrors ROption but never collapses a pure date or a
// floating date-time into the server's local zone.
type TypedROption struct {
	Freq       Frequency
	Dtstart    DateTime
	Interval   int
	Wkst       Weekday
	Count      int
	Until      DateTime
	Bysetpos   []int
	Bymonth    []int
	Bymonthday []int
	Byyearday  []int
	Byweekno   []int
	Byweekday  []Weekday
	Byhour     []int
	Byminute   []int
	Bysecond   []int
	Byeaster   []int
}

// TypedRRule is a recurrence rule that keeps the original semantic kind of
// its DTSTART. The recurrence engine is shared with RRule; only the inputs
// and outputs carry the kind marker.
type TypedRRule struct {
	OrigOptions TypedROption
	dtstart     DateTime
	until       DateTime

	// loc resolves floating values for one evaluation run. It is nil for
	// date, TZID and UTC rules and mandatory when evaluating floating rules.
	loc *time.Location

	// engine is the shared recurrence engine, built in the evaluation zone.
	// For KindDate everything is computed on UTC midnight boundaries.
	engine    *RRule
	engineErr error
}

// NewTypedRRule constructs a TypedRRule. UNTIL is validated against the
// DTSTART kind up front; an illegal combination (RFC 5545 3.3.10) is rejected
// without ever being silently converted to a time.Time.
func NewTypedRRule(opt TypedROption) (*TypedRRule, error) {
	if opt.Dtstart.IsZero() {
		return nil, errors.New("rrule: TypedROption requires an explicit Dtstart")
	}
	if err := validateUntil(opt.Dtstart, opt.Until); err != nil {
		return nil, err
	}
	return buildTypedRRule(opt, nil), nil
}

func buildTypedRRule(opt TypedROption, loc *time.Location) *TypedRRule {
	r := &TypedRRule{
		OrigOptions: opt,
		dtstart:     opt.Dtstart,
		until:       opt.Until,
		loc:         loc,
	}
	if err := validateUntil(opt.Dtstart, opt.Until); err != nil {
		r.engineErr = err
		return r
	}

	dtstartAbs, err := opt.Dtstart.InLocation(loc)
	if err != nil {
		// NewTypedRRule validates floating evaluation lazily; the error
		// surfaces on evaluation, never as a wrong result.
		r.engineErr = err
		return r
	}

	rOpt := ROption{
		Freq:       opt.Freq,
		Dtstart:    dtstartAbs,
		Interval:   opt.Interval,
		Wkst:       opt.Wkst,
		Count:      opt.Count,
		Bysetpos:   opt.Bysetpos,
		Bymonth:    opt.Bymonth,
		Bymonthday: opt.Bymonthday,
		Byyearday:  opt.Byyearday,
		Byweekno:   opt.Byweekno,
		Byweekday:  opt.Byweekday,
		Byhour:     opt.Byhour,
		Byminute:   opt.Byminute,
		Bysecond:   opt.Bysecond,
		Byeaster:   opt.Byeaster,
	}
	if !opt.Until.IsZero() {
		untilAbs, err := opt.Until.InLocation(loc)
		if err != nil {
			r.engineErr = err
			return r
		}
		rOpt.Until = untilAbs
	}
	engine, err := NewRRule(rOpt)
	if err != nil {
		r.engineErr = err
		return r
	}
	r.engine = engine
	return r
}

// validateUntil enforces the RFC 5545 UNTIL rules keyed on the DTSTART kind:
//   - DATE DTSTART needs a DATE UNTIL.
//   - floating DTSTART needs a floating UNTIL.
//   - TZID DTSTART needs a UTC UNTIL.
//   - UTC DTSTART needs a UTC UNTIL.
func validateUntil(dtstart, until DateTime) error {
	if until.IsZero() {
		return nil
	}
	valid := false
	switch dtstart.kind {
	case KindDate:
		valid = until.kind == KindDate
	case KindFloating:
		valid = until.kind == KindFloating
	case KindTZID:
		valid = until.kind == KindUTC
	case KindUTC:
		valid = until.kind == KindUTC
	}
	if !valid {
		return fmt.Errorf(
			"rrule: UNTIL kind %s is not valid with DTSTART kind %s",
			until.kind, dtstart.kind)
	}
	return nil
}

// GetDTStart returns the preserved DTSTART.
func (r *TypedRRule) GetDTStart() DateTime { return r.dtstart }

// GetUntil returns the preserved UNTIL (zero DateTime if unset).
func (r *TypedRRule) GetUntil() DateTime { return r.until }

// InLocation returns a copy of the rule bound to loc for evaluation. Only
// floating rules need this; date, TZID and UTC rules ignore loc.
func (r *TypedRRule) InLocation(loc *time.Location) *TypedRRule {
	return buildTypedRRule(r.OrigOptions, loc)
}

func (r *TypedRRule) prepare(loc *time.Location) (*RRule, error) {
	if r.engine == nil || loc != r.loc {
		rr := buildTypedRRule(r.OrigOptions, loc)
		if rr.engineErr != nil {
			return nil, rr.engineErr
		}
		return rr.engine, nil
	}
	if r.engineErr != nil {
		return nil, r.engineErr
	}
	return r.engine, nil
}

// DateTimeNext generates typed occurrences.
type DateTimeNext func() (DateTime, bool)

// Iterator returns an iterator over typed occurrences. For floating rules,
// loc must be non-nil and supplies the evaluation zone; pass nil for the
// other kinds.
func (r *TypedRRule) Iterator(loc *time.Location) (DateTimeNext, error) {
	if r.dtstart.kind == KindFloating && loc == nil {
		return nil, errors.New("rrule: Iterator on a floating rule needs an explicit location")
	}
	engine, err := r.prepare(loc)
	if err != nil {
		return nil, err
	}
	next := engine.Iterator()
	kind := r.dtstart.kind
	tzid := r.dtstart.tzid
	eloc := r.dtstart.loc
	return func() (DateTime, bool) {
		t, ok := next()
		if !ok {
			return DateTime{}, false
		}
		return fromEngineTime(t, kind, tzid, eloc), true
	}, nil
}

// All returns all occurrences. loc follows the same rules as Iterator.
func (r *TypedRRule) All(loc *time.Location) ([]DateTime, error) {
	next, err := r.Iterator(loc)
	if err != nil {
		return nil, err
	}
	var result []DateTime
	for {
		v, ok := next()
		if !ok {
			return result, nil
		}
		result = append(result, v)
	}
}

// Between returns occurrences in [after, before) (or inclusive with inc).
// The bounds must share the rule's kind; dates are compared on date
// boundaries and date-times on their evaluated instants.
func (r *TypedRRule) Between(after, before DateTime, inc bool, loc *time.Location) ([]DateTime, error) {
	if err := checkQueryKind(r.dtstart, after, before); err != nil {
		return nil, err
	}
	if r.dtstart.kind == KindFloating && loc == nil {
		return nil, errors.New("rrule: Between on a floating rule needs an explicit location")
	}
	engine, err := r.prepare(loc)
	if err != nil {
		return nil, err
	}
	a, err := after.InLocation(loc)
	if err != nil {
		return nil, err
	}
	b, err := before.InLocation(loc)
	if err != nil {
		return nil, err
	}
	return toDateTimes(between(engine.Iterator(), a, b, inc), r.dtstart), nil
}

// Before returns the last occurrence before dt.
func (r *TypedRRule) Before(dt DateTime, inc bool, loc *time.Location) (DateTime, error) {
	if err := checkQueryKind(r.dtstart, dt); err != nil {
		return DateTime{}, err
	}
	if r.dtstart.kind == KindFloating && loc == nil {
		return DateTime{}, errors.New("rrule: Before on a floating rule needs an explicit location")
	}
	engine, err := r.prepare(loc)
	if err != nil {
		return DateTime{}, err
	}
	abs, err := dt.InLocation(loc)
	if err != nil {
		return DateTime{}, err
	}
	t := before(engine.Iterator(), abs, inc)
	if t.IsZero() {
		return DateTime{}, nil
	}
	return fromEngineTime(t, r.dtstart.kind, r.dtstart.tzid, r.dtstart.loc), nil
}

// After returns the first occurrence after dt.
func (r *TypedRRule) After(dt DateTime, inc bool, loc *time.Location) (DateTime, error) {
	if err := checkQueryKind(r.dtstart, dt); err != nil {
		return DateTime{}, err
	}
	if r.dtstart.kind == KindFloating && loc == nil {
		return DateTime{}, errors.New("rrule: After on a floating rule needs an explicit location")
	}
	engine, err := r.prepare(loc)
	if err != nil {
		return DateTime{}, err
	}
	abs, err := dt.InLocation(loc)
	if err != nil {
		return DateTime{}, err
	}
	t := after(engine.Iterator(), abs, inc)
	if t.IsZero() {
		return DateTime{}, nil
	}
	return fromEngineTime(t, r.dtstart.kind, r.dtstart.tzid, r.dtstart.loc), nil
}

func checkQueryKind(dtstart DateTime, dts ...DateTime) error {
	for _, d := range dts {
		if d.IsZero() {
			continue
		}
		if d.kind != dtstart.kind {
			return fmt.Errorf(
				"rrule: query value kind %s does not match DTSTART kind %s",
				d.kind, dtstart.kind)
		}
	}
	return nil
}

// fromEngineTime converts an engine result back into the original kind.
// Engine times carry the evaluation zone; wall-clock fields are what matter
// for date and floating results.
func fromEngineTime(t time.Time, kind DateTimeKind, tzid string, loc *time.Location) DateTime {
	switch kind {
	case KindDate:
		y, m, d := t.Date()
		return NewDate(y, m, d)
	case KindFloating:
		y, m, d := t.Date()
		h, mi, s := t.Clock()
		return NewFloating(y, m, d, h, mi, s)
	case KindTZID:
		y, m, d := t.Date()
		h, mi, s := t.Clock()
		return DateTime{
			kind: KindTZID,
			wall: time.Date(y, m, d, h, mi, s, 0, time.UTC),
			tzid: tzid,
			loc:  loc,
		}
	default:
		return DateTime{kind: KindUTC, wall: t.UTC()}
	}
}

func toDateTimes(ts []time.Time, anchor DateTime) []DateTime {
	result := make([]DateTime, 0, len(ts))
	for _, t := range ts {
		result = append(result, fromEngineTime(t, anchor.kind, anchor.tzid, anchor.loc))
	}
	return result
}

// String renders the rule in RFC 5545 form with DTSTART and RRULE lines,
// preserving both the kind and TZID.
func (opt *TypedROption) String() string {
	str := opt.RRuleString()
	if opt.Dtstart.IsZero() {
		return str
	}
	return fmt.Sprintf("DTSTART%s\nRRULE:%s", dtStartParam(opt.Dtstart), str)
}

// RRuleString renders only the RRULE line; UNTIL keeps its original kind.
func (opt *TypedROption) RRuleString() string {
	result := []string{fmt.Sprintf("FREQ=%v", opt.Freq)}
	if opt.Interval != 0 {
		result = append(result, fmt.Sprintf("INTERVAL=%v", opt.Interval))
	}
	if opt.Wkst != MO {
		result = append(result, fmt.Sprintf("WKST=%v", opt.Wkst))
	}
	if opt.Count != 0 {
		result = append(result, fmt.Sprintf("COUNT=%v", opt.Count))
	}
	if !opt.Until.IsZero() {
		result = append(result, "UNTIL="+untilValue(opt.Until))
	}
	result = appendIntsOption(result, "BYSETPOS", opt.Bysetpos)
	result = appendIntsOption(result, "BYMONTH", opt.Bymonth)
	result = appendIntsOption(result, "BYMONTHDAY", opt.Bymonthday)
	result = appendIntsOption(result, "BYYEARDAY", opt.Byyearday)
	result = appendIntsOption(result, "BYWEEKNO", opt.Byweekno)
	if len(opt.Byweekday) != 0 {
		valueStr := make([]string, len(opt.Byweekday))
		for i, wday := range opt.Byweekday {
			valueStr[i] = wday.String()
		}
		result = append(result, fmt.Sprintf("BYDAY=%s", strings.Join(valueStr, ",")))
	}
	result = appendIntsOption(result, "BYHOUR", opt.Byhour)
	result = appendIntsOption(result, "BYMINUTE", opt.Byminute)
	result = appendIntsOption(result, "BYSECOND", opt.Bysecond)
	result = appendIntsOption(result, "BYEASTER", opt.Byeaster)
	return strings.Join(result, ";")
}

func (r *TypedRRule) String() string {
	return r.OrigOptions.String()
}
