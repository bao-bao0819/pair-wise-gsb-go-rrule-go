// 2017-2022, Teambition. All rights reserved.

package rrule

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DTROption offers kind-preserving options to construct a DTRule.
// It mirrors ROption but uses DTValue for Dtstart and Until so the
// RFC 5545 value kind (DATE, floating, TZID, UTC) is retained.
type DTROption struct {
	Freq       Frequency
	Dtstart    DTValue
	Interval   int
	Wkst       Weekday
	Count      int
	Until      DTValue
	Bysetpos   []int
	Bymonth    []int
	Bymonthday []int
	Byyearday  []int
	Byweekno   []int
	Byeaster   []int
	Byweekday  []Weekday
	Byhour     []int
	Byminute   []int
	Bysecond   []int
}

// String returns the RFC 5545 representation with kind-preserving DTSTART, e.g.
//
//	DTSTART;TZID=America/New_York:20261003T090000
//	RRULE:FREQ=DAILY;UNTIL=20261010T090000Z
func (option *DTROption) String() string {
	str := option.RRuleString()
	if option.Dtstart.IsZero() {
		return str
	}
	return fmt.Sprintf("DTSTART%s\nRRULE:%s", option.Dtstart.String(), str)
}

// RRuleString returns the RRULE property value, formatting UNTIL
// according to its own kind.
func (option *DTROption) RRuleString() string {
	result := []string{fmt.Sprintf("FREQ=%v", option.Freq)}
	if option.Interval != 0 {
		result = append(result, fmt.Sprintf("INTERVAL=%v", option.Interval))
	}
	if option.Wkst != MO {
		result = append(result, fmt.Sprintf("WKST=%v", option.Wkst))
	}
	if option.Count != 0 {
		result = append(result, fmt.Sprintf("COUNT=%v", option.Count))
	}
	if !option.Until.IsZero() {
		result = append(result, fmt.Sprintf("UNTIL=%v", option.Until.rruleValue()))
	}
	result = appendIntsOption(result, "BYSETPOS", option.Bysetpos)
	result = appendIntsOption(result, "BYMONTH", option.Bymonth)
	result = appendIntsOption(result, "BYMONTHDAY", option.Bymonthday)
	result = appendIntsOption(result, "BYYEARDAY", option.Byyearday)
	result = appendIntsOption(result, "BYWEEKNO", option.Byweekno)
	if len(option.Byweekday) != 0 {
		valueStr := make([]string, len(option.Byweekday))
		for i, wday := range option.Byweekday {
			valueStr[i] = wday.String()
		}
		result = append(result, fmt.Sprintf("BYDAY=%s", strings.Join(valueStr, ",")))
	}
	result = appendIntsOption(result, "BYHOUR", option.Byhour)
	result = appendIntsOption(result, "BYMINUTE", option.Byminute)
	result = appendIntsOption(result, "BYSECOND", option.Bysecond)
	result = appendIntsOption(result, "BYEASTER", option.Byeaster)
	return strings.Join(result, ";")
}

func (r *DTRule) String() string {
	return r.OrigOptions.String()
}

// parseDTRRuleAttrs parses the attributes of an RRULE property value
// (without DTSTART) into kind-preserving options.
func parseDTRRuleAttrs(rruleStr string) (DTROption, error) {
	result := DTROption{}
	freqSet := false
	rruleStr = strings.TrimPrefix(rruleStr, "RRULE:")
	for _, attr := range strings.Split(rruleStr, ";") {
		keyValue := strings.Split(attr, "=")
		if len(keyValue) != 2 {
			return result, errors.New("wrong format")
		}
		key, value := keyValue[0], keyValue[1]
		if len(value) == 0 {
			return result, errors.New(key + " option has no value")
		}
		var e error
		switch key {
		case "FREQ":
			result.Freq, e = StrToFreq(value)
			freqSet = true
		case "INTERVAL":
			result.Interval, e = strconv.Atoi(value)
		case "WKST":
			result.Wkst, e = strToWeekday(value)
		case "COUNT":
			result.Count, e = strconv.Atoi(value)
		case "UNTIL":
			result.Until, e = ParseDTValue(value)
		case "BYSETPOS":
			result.Bysetpos, e = strToInts(value)
		case "BYMONTH":
			result.Bymonth, e = strToInts(value)
		case "BYMONTHDAY":
			result.Bymonthday, e = strToInts(value)
		case "BYYEARDAY":
			result.Byyearday, e = strToInts(value)
		case "BYWEEKNO":
			result.Byweekno, e = strToInts(value)
		case "BYDAY":
			result.Byweekday, e = strToWeekdays(value)
		case "BYHOUR":
			result.Byhour, e = strToInts(value)
		case "BYMINUTE":
			result.Byminute, e = strToInts(value)
		case "BYSECOND":
			result.Bysecond, e = strToInts(value)
		case "BYEASTER":
			result.Byeaster, e = strToInts(value)
		default:
			return result, errors.New("unknown RRULE property: " + key)
		}
		if e != nil {
			return result, e
		}
	}
	if !freqSet {
		return result, errors.New("RRULE property FREQ is required")
	}
	return result, nil
}

// StrToDTRule converts an RFC 5545 string (optional DTSTART line plus an
// RRULE line) to a kind-preserving DTRule. UNTIL is validated against the
// DTSTART kind; invalid combinations are rejected instead of being
// silently converted.
func StrToDTRule(rfcString string) (*DTRule, error) {
	rfcString = strings.TrimSpace(rfcString)
	strs := strings.Split(rfcString, "\n")
	var rruleStr, dtstartStr string
	switch len(strs) {
	case 1:
		rruleStr = strs[0]
	case 2:
		dtstartStr = strs[0]
		rruleStr = strs[1]
	default:
		return nil, errors.New("invalid RRULE string")
	}

	option, err := parseDTRRuleAttrs(rruleStr)
	if err != nil {
		return nil, err
	}
	if dtstartStr != "" {
		option.Dtstart, err = ParseDTStartLine(dtstartStr)
		if err != nil {
			return nil, fmt.Errorf("ParseDTStartLine failed: %s", err)
		}
	}
	return NewDTRule(option)
}

// checkSetValueKind validates an RDATE/EXDATE value against the DTSTART
// kind: DATE only mixes with DATE, floating only with floating, and
// absolute (TZID/UTC) values only with absolute ones.
func checkSetValueKind(dtstart DTValue, v DTValue, prop string) error {
	switch dtstart.kind {
	case KindDate:
		if v.kind != KindDate {
			return fmt.Errorf("%s kind %s does not match DATE DTSTART", prop, v.kind)
		}
	case KindFloating:
		if v.kind != KindFloating {
			return fmt.Errorf("%s kind %s does not match floating DTSTART", prop, v.kind)
		}
	default:
		if !v.isAbsolute() {
			return fmt.Errorf("%s kind %s does not match zoned DTSTART", prop, v.kind)
		}
	}
	return nil
}

// DTSet is a recurrence set that preserves value kinds. RDATE and EXDATE
// values are validated against the DTSTART kind when added, and exclusion
// compares values kind-aware so an exclusion date only removes
// occurrences of its own kind.
type DTSet struct {
	dtstart DTValue
	rrule   *DTRule
	rdate   []DTValue
	exdate  []DTValue
}

// DTStart sets the DTSTART property of the set.
func (set *DTSet) DTStart(dtstart DTValue) error {
	if set.rrule != nil && set.rrule.kind != dtstart.kind {
		return fmt.Errorf("DTSTART kind %s does not match RRULE DTSTART kind %s", dtstart.kind, set.rrule.kind)
	}
	set.dtstart = dtstart
	return nil
}

// GetDTStart returns the DTSTART of the set.
func (set *DTSet) GetDTStart() DTValue {
	return set.dtstart
}

// RRule sets the RRULE of the set. The rule's DTSTART kind must match
// the set's DTSTART kind if the latter is already set.
func (set *DTSet) RRule(rrule *DTRule) error {
	if !set.dtstart.IsZero() && set.dtstart.kind != rrule.kind {
		return fmt.Errorf("RRULE DTSTART kind %s does not match set DTSTART kind %s", rrule.kind, set.dtstart.kind)
	}
	set.dtstart = rrule.OrigOptions.Dtstart
	set.rrule = rrule
	return nil
}

// GetRRule returns the rule of the set.
func (set *DTSet) GetRRule() *DTRule {
	return set.rrule
}

// RDate includes the given value in the recurrence set.
// Its kind must be compatible with the DTSTART kind.
func (set *DTSet) RDate(rdate DTValue) error {
	if err := checkSetValueKind(set.dtstart, rdate, "RDATE"); err != nil {
		return err
	}
	set.rdate = append(set.rdate, rdate)
	return nil
}

// GetRDate returns explicitly added dates of the set.
func (set *DTSet) GetRDate() []DTValue {
	return set.rdate
}

// ExDate includes the given value in the exclusion list.
// Its kind must be compatible with the DTSTART kind.
func (set *DTSet) ExDate(exdate DTValue) error {
	if err := checkSetValueKind(set.dtstart, exdate, "EXDATE"); err != nil {
		return err
	}
	set.exdate = append(set.exdate, exdate)
	return nil
}

// GetExDate returns explicitly excluded dates of the set.
func (set *DTSet) GetExDate() []DTValue {
	return set.exdate
}

// kind returns the value kind governing the set.
func (set *DTSet) kind() DTKind {
	if set.rrule != nil {
		return set.rrule.kind
	}
	if !set.dtstart.IsZero() {
		return set.dtstart.kind
	}
	if len(set.rdate) != 0 {
		return set.rdate[0].kind
	}
	return KindUTC
}

// Recurrence returns the RFC 5545 lines of the set, preserving kinds.
func (set *DTSet) Recurrence() []string {
	var res []string
	if !set.dtstart.IsZero() {
		res = append(res, "DTSTART"+set.dtstart.String())
	}
	if set.rrule != nil {
		res = append(res, "RRULE:"+set.rrule.OrigOptions.RRuleString())
	}
	for _, item := range set.rdate {
		res = append(res, "RDATE"+item.String())
	}
	for _, item := range set.exdate {
		res = append(res, "EXDATE"+item.String())
	}
	return res
}

func (set *DTSet) String() string {
	return strings.Join(set.Recurrence(), "\n")
}

// evalLoc resolves the evaluation zone for the set.
func (set *DTSet) evalLoc(loc *time.Location) (*time.Location, error) {
	if set.rrule != nil {
		return set.rrule.evalLoc(loc)
	}
	switch set.kind() {
	case KindDate, KindUTC:
		return time.UTC, nil
	case KindTZID:
		return set.dtstart.t.Location(), nil
	default:
		if loc == nil {
			return nil, errors.New("floating DTSTART requires an explicit evaluation location")
		}
		return loc, nil
	}
}

// Iterator returns a kind-preserving iterator for the set.
// loc is the explicit evaluation zone, required for floating sets.
func (set *DTSet) Iterator(loc *time.Location) (NextDT, error) {
	evalLoc, err := set.evalLoc(loc)
	if err != nil {
		return nil, err
	}
	var rruleNext NextDT
	if set.rrule != nil {
		rruleNext, err = set.rrule.Iterator(loc)
		if err != nil {
			return nil, err
		}
	}
	key := func(v DTValue) time.Time { return dtEvalForRule(v, evalLoc) }

	rdates := make([]DTValue, len(set.rdate))
	copy(rdates, set.rdate)
	sort.Slice(rdates, func(i, j int) bool { return key(rdates[i]).Before(key(rdates[j])) })
	exdates := set.exdate

	var pending DTValue
	hasPending := false
	ri := 0
	var last DTValue
	hasLast := false

	return func() (DTValue, bool) {
		for {
			if !hasPending && rruleNext != nil {
				if v, ok := rruleNext(); ok {
					pending = v
					hasPending = true
				} else {
					rruleNext = nil
				}
			}
			var next DTValue
			have := false
			if hasPending && (ri >= len(rdates) || !key(rdates[ri]).Before(key(pending))) {
				next = pending
				hasPending = false
				have = true
			} else if ri < len(rdates) {
				next = rdates[ri]
				ri++
				have = true
			}
			if !have {
				return DTValue{}, false
			}
			if hasLast && last.Equal(next) {
				continue
			}
			excluded := false
			for _, ex := range exdates {
				if ex.Equal(next) {
					excluded = true
					break
				}
			}
			last = next
			hasLast = true
			if excluded {
				continue
			}
			return next, true
		}
	}, nil
}

// All returns all occurrences of the set, preserving kinds.
func (set *DTSet) All(loc *time.Location) ([]DTValue, error) {
	next, err := set.Iterator(loc)
	if err != nil {
		return nil, err
	}
	var result []DTValue
	for {
		v, ok := next()
		if !ok {
			return result, nil
		}
		result = append(result, v)
	}
}

// Between returns all occurrences of the set between after and before,
// preserving kinds. Bounds follow the same kind rules as DTRule.
func (set *DTSet) Between(after, before DTValue, inc bool, loc *time.Location) ([]DTValue, error) {
	evalLoc, err := set.evalLoc(loc)
	if err != nil {
		return nil, err
	}
	if set.kind() == KindDate {
		if after.kind != KindDate || before.kind != KindDate {
			return nil, errors.New("DATE set requires DATE query bounds")
		}
	}
	afterT := dtEvalForRule(after, evalLoc)
	beforeT := dtEvalForRule(before, evalLoc)
	next, err := set.Iterator(loc)
	if err != nil {
		return nil, err
	}
	key := func(v DTValue) time.Time { return dtEvalForRule(v, evalLoc) }
	var result []DTValue
	for {
		v, ok := next()
		t := key(v)
		if !ok || inc && t.After(beforeT) || !inc && !t.Before(beforeT) {
			return result, nil
		}
		if inc && !t.Before(afterT) || !inc && t.After(afterT) {
			result = append(result, v)
		}
	}
}

// StrToDTSet parses RFC 5545 lines (DTSTART, RRULE, RDATE, EXDATE) into a
// kind-preserving DTSet. RDATE/EXDATE kinds are validated against the
// DTSTART kind; invalid combinations are rejected.
func StrToDTSet(s string) (*DTSet, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty string")
	}
	set := &DTSet{}
	for _, line := range strings.Split(s, "\n") {
		name, err := processRRuleName(line)
		if err != nil {
			return nil, err
		}
		rule := line[len(name):]
		switch name {
		case "DTSTART":
			v, err := ParseDTValue(rule)
			if err != nil {
				return nil, fmt.Errorf("ParseDTValue failed: %v", err)
			}
			if err := set.DTStart(v); err != nil {
				return nil, err
			}
		case "RRULE":
			option, err := parseDTRRuleAttrs(strings.TrimPrefix(rule, ":"))
			if err != nil {
				return nil, err
			}
			option.Dtstart = set.dtstart
			r, err := NewDTRule(option)
			if err != nil {
				return nil, err
			}
			if err := set.RRule(r); err != nil {
				return nil, err
			}
		case "RDATE", "EXDATE":
			vs, err := parseDTValueList(rule)
			if err != nil {
				return nil, fmt.Errorf("parseDTValueList failed: %v", err)
			}
			for _, v := range vs {
				if name == "RDATE" {
					err = set.RDate(v)
				} else {
					err = set.ExDate(v)
				}
				if err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("unsupported property: %s", name)
		}
	}
	return set, nil
}

// DTRule is a recurrence rule that preserves the value kind of its
// DTSTART. It shares the recurrence computation with RRule but never
// drops the semantic kind: floating rules are only evaluated against an
// explicit location supplied by the caller, and pure-date rules iterate
// calendar dates independent of any zone.
type DTRule struct {
	OrigOptions DTROption
	kind        DTKind
	tzid        string
}

// checkUntilKind validates UNTIL against the DTSTART kind per RFC 5545:
// the value types must match, and when DTSTART is an absolute date-time
// (TZID or UTC) UNTIL must be UTC.
func checkUntilKind(dtstart, until DTValue) error {
	if until.IsZero() {
		return nil
	}
	switch dtstart.kind {
	case KindDate:
		if until.kind != KindDate {
			return fmt.Errorf("UNTIL kind %s does not match DATE DTSTART", until.kind)
		}
	case KindFloating:
		if until.kind != KindFloating {
			return fmt.Errorf("UNTIL kind %s does not match floating DTSTART", until.kind)
		}
	default:
		if until.kind != KindUTC {
			return fmt.Errorf("UNTIL must be UTC when DTSTART carries zone information, got %s", until.kind)
		}
	}
	return nil
}

// NewDTRule constructs a new kind-preserving DTRule.
// Dtstart is required so the kind is always explicit.
func NewDTRule(arg DTROption) (*DTRule, error) {
	if arg.Dtstart.IsZero() {
		return nil, errors.New("Dtstart is required for DTRule")
	}
	if err := checkUntilKind(arg.Dtstart, arg.Until); err != nil {
		return nil, err
	}
	if err := validateBounds(dtROptionToROption(arg, time.UTC)); err != nil {
		return nil, err
	}
	return &DTRule{OrigOptions: arg, kind: arg.Dtstart.kind, tzid: arg.Dtstart.tzid}, nil
}

// Kind returns the value kind of the rule's DTSTART.
func (r *DTRule) Kind() DTKind {
	return r.kind
}

// GetDTStart returns the kind-preserving DTSTART of the rule.
func (r *DTRule) GetDTStart() DTValue {
	return r.OrigOptions.Dtstart
}

// GetUntil returns the kind-preserving UNTIL of the rule,
// or the zero DTValue if unbounded.
func (r *DTRule) GetUntil() DTValue {
	return r.OrigOptions.Until
}

// evalLoc resolves the location used to compute occurrences.
// Floating rules require the caller to supply loc explicitly;
// pure-date rules always evaluate in a fixed neutral zone so results
// never depend on the server's local zone.
func (r *DTRule) evalLoc(loc *time.Location) (*time.Location, error) {
	switch r.kind {
	case KindDate, KindUTC:
		return time.UTC, nil
	case KindTZID:
		return r.OrigOptions.Dtstart.t.Location(), nil
	default:
		if loc == nil {
			return nil, errors.New("floating DTSTART requires an explicit evaluation location")
		}
		return loc, nil
	}
}

// dtROptionToROption converts kind-preserving options to plain ROption,
// evaluating Dtstart and Until in evalLoc.
func dtROptionToROption(opt DTROption, evalLoc *time.Location) ROption {
	ro := ROption{
		Freq:       opt.Freq,
		Interval:   opt.Interval,
		Wkst:       opt.Wkst,
		Count:      opt.Count,
		Bysetpos:   opt.Bysetpos,
		Bymonth:    opt.Bymonth,
		Bymonthday: opt.Bymonthday,
		Byyearday:  opt.Byyearday,
		Byweekno:   opt.Byweekno,
		Byweekday:  opt.Byweekday,
		Byeaster:   opt.Byeaster,
	}
	if opt.Dtstart.kind != KindDate {
		ro.Byhour = opt.Byhour
		ro.Byminute = opt.Byminute
		ro.Bysecond = opt.Bysecond
	}
	ro.Dtstart = dtEvalForRule(opt.Dtstart, evalLoc)
	if !opt.Until.IsZero() {
		ro.Until = dtEvalForRule(opt.Until, evalLoc)
	}
	return ro
}

// dtEvalForRule maps a DTValue onto the time.Time line used by the
// shared RRule computation, without attaching semantic zones to
// floating values beyond the caller-supplied evalLoc.
func dtEvalForRule(v DTValue, evalLoc *time.Location) time.Time {
	switch v.kind {
	case KindDate:
		return v.t // midnight UTC of the date
	case KindFloating:
		t, _ := v.In(evalLoc) // evalLoc is non-nil here
		return t
	default:
		return v.t
	}
}

// wrap converts a computed occurrence back to a kind-preserving DTValue.
func (r *DTRule) wrap(t time.Time) DTValue {
	switch r.kind {
	case KindDate:
		year, month, day := t.Date()
		return DateValue(year, month, day)
	case KindFloating:
		return FloatingValue(t)
	case KindTZID:
		return DTValue{kind: KindTZID, t: t, tzid: r.tzid}
	default:
		return UTCValue(t)
	}
}

// NextDT is a generator of kind-preserving occurrences.
type NextDT func() (value DTValue, ok bool)

// Iterator returns a kind-preserving iterator for the rule.
// loc is the explicit evaluation zone and is required (non-nil) when
// the rule's DTSTART is floating; it is ignored otherwise.
func (r *DTRule) Iterator(loc *time.Location) (NextDT, error) {
	evalLoc, err := r.evalLoc(loc)
	if err != nil {
		return nil, err
	}
	rr, err := NewRRule(dtROptionToROption(r.OrigOptions, evalLoc))
	if err != nil {
		return nil, err
	}
	next := rr.Iterator()
	return func() (DTValue, bool) {
		t, ok := next()
		if !ok {
			return DTValue{}, false
		}
		return r.wrap(t), true
	}, nil
}

// All returns all occurrences of the rule, preserving the DTSTART kind.
func (r *DTRule) All(loc *time.Location) ([]DTValue, error) {
	next, err := r.Iterator(loc)
	if err != nil {
		return nil, err
	}
	var result []DTValue
	for {
		v, ok := next()
		if !ok {
			return result, nil
		}
		result = append(result, v)
	}
}

// checkQueryBound validates a query bound against the rule kind:
// pure-date rules only accept DATE bounds (compared on date boundaries),
// date-time rules reject DATE bounds.
func (r *DTRule) checkQueryBound(dt DTValue) error {
	if r.kind == KindDate {
		if dt.kind != KindDate {
			return fmt.Errorf("query bound kind %s does not match DATE rule", dt.kind)
		}
	} else if dt.kind == KindDate {
		return errors.New("DATE query bound is ambiguous for a date-time rule")
	}
	return nil
}

// boundToTime maps a query bound onto the evaluation time line.
func (r *DTRule) boundToTime(dt DTValue, evalLoc *time.Location) time.Time {
	return dtEvalForRule(dt, evalLoc)
}

// Between returns all occurrences between after and before, preserving kind.
// For pure-date rules the bounds are compared on date boundaries.
func (r *DTRule) Between(after, before DTValue, inc bool, loc *time.Location) ([]DTValue, error) {
	if err := r.checkQueryBound(after); err != nil {
		return nil, err
	}
	if err := r.checkQueryBound(before); err != nil {
		return nil, err
	}
	evalLoc, err := r.evalLoc(loc)
	if err != nil {
		return nil, err
	}
	rr, err := NewRRule(dtROptionToROption(r.OrigOptions, evalLoc))
	if err != nil {
		return nil, err
	}
	ts := between(rr.Iterator(), r.boundToTime(after, evalLoc), r.boundToTime(before, evalLoc), inc)
	result := make([]DTValue, 0, len(ts))
	for _, t := range ts {
		result = append(result, r.wrap(t))
	}
	return result, nil
}

// Before returns the last occurrence before dt, preserving kind.
func (r *DTRule) Before(dt DTValue, inc bool, loc *time.Location) (DTValue, error) {
	if err := r.checkQueryBound(dt); err != nil {
		return DTValue{}, err
	}
	evalLoc, err := r.evalLoc(loc)
	if err != nil {
		return DTValue{}, err
	}
	rr, err := NewRRule(dtROptionToROption(r.OrigOptions, evalLoc))
	if err != nil {
		return DTValue{}, err
	}
	t := before(rr.Iterator(), r.boundToTime(dt, evalLoc), inc)
	if t.IsZero() {
		return DTValue{}, nil
	}
	return r.wrap(t), nil
}

// After returns the first occurrence after dt, preserving kind.
func (r *DTRule) After(dt DTValue, inc bool, loc *time.Location) (DTValue, error) {
	if err := r.checkQueryBound(dt); err != nil {
		return DTValue{}, err
	}
	evalLoc, err := r.evalLoc(loc)
	if err != nil {
		return DTValue{}, err
	}
	rr, err := NewRRule(dtROptionToROption(r.OrigOptions, evalLoc))
	if err != nil {
		return DTValue{}, err
	}
	t := after(rr.Iterator(), r.boundToTime(dt, evalLoc), inc)
	if t.IsZero() {
		return DTValue{}, nil
	}
	return r.wrap(t), nil
}
