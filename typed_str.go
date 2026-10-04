// 2026, Teambition. All rights reserved.

package rrule

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var errFloatingLoc = errors.New("rrule: evaluating a floating recurrence requires an explicit location")

func kindMismatchError(prop string, got, want interface{}) error {
	return fmt.Errorf("rrule: %s value %v does not match DTSTART %v", prop, got, want)
}

// dtStartParam renders the DTSTART property suffix (starting with the
// parameter separator), preserving the kind and TZID.
func dtStartParam(dt DateTime) string {
	return dateTimeProp(dt)
}

// dateTimeProp renders ";<params>:value" or ":value" for RDATE/EXDATE and
// DTSTART properties.
func dateTimeProp(dt DateTime) string {
	switch dt.kind {
	case KindDate:
		return ";VALUE=DATE:" + dt.wall.Format(DateFormat)
	case KindFloating:
		return ":" + dt.wall.Format(LocalDateTimeFormat)
	case KindTZID:
		return ";TZID=" + dt.tzid + ":" + dt.wall.Format(LocalDateTimeFormat)
	default:
		return ":" + dt.wall.Format(DateTimeFormat)
	}
}

// untilValue renders an UNTIL value per RFC 5545.
func untilValue(dt DateTime) string {
	switch dt.kind {
	case KindDate:
		return dt.wall.Format(DateFormat)
	case KindFloating:
		return dt.wall.Format(LocalDateTimeFormat)
	default:
		return dt.wall.Format(DateTimeFormat)
	}
}

// StrToTypedRRule parses a typed recurrence rule. The string may be a bare
// RRULE line or "DTSTART<params>:<value>\nRRULE:..." form; all values keep
// their kinds and UNTIL is validated against DTSTART.
func StrToTypedRRule(rfcString string) (*TypedRRule, error) {
	option, err := StrToTypedROption(rfcString)
	if err != nil {
		return nil, err
	}
	return NewTypedRRule(*option)
}

// StrToTypedROption parses the option form without constructing the rule.
func StrToTypedROption(rfcString string) (*TypedROption, error) {
	rfcString = strings.TrimSpace(rfcString)
	strs := strings.Split(rfcString, "\n")
	var rruleStr string
	result := TypedROption{}
	freqSet := false

	switch len(strs) {
	case 1:
		rruleStr = strs[0]
	case 2:
		firstName, err := processRRuleName(strs[0])
		if err != nil || firstName != "DTSTART" {
			return nil, fmt.Errorf("expect DTSTART but: %s", strs[0])
		}
		result.Dtstart, err = StrToDateTime(strs[0][len(firstName)+1:])
		if err != nil {
			return nil, fmt.Errorf("StrToDateTime failed: %s", err)
		}
		rruleStr = strs[1]
	default:
		return nil, errors.New("invalid RRULE string")
	}

	rruleStr = strings.TrimPrefix(rruleStr, "RRULE:")
	freqSet, e := parseTypedRRuleAttrs(rruleStr, &result)
	if e != nil {
		return nil, e
	}
	if !freqSet {
		return nil, errors.New("RRULE property FREQ is required")
	}
	if result.Dtstart.IsZero() {
		return nil, errors.New("typed RRULE requires an explicit DTSTART")
	}
	if err := validateUntil(result.Dtstart, result.Until); err != nil {
		return nil, err
	}
	return &result, nil
}

// parseTypedRRuleAttrs fills opt from an RRULE body (without the "RRULE:"
// prefix). It neither requires DTSTART nor validates UNTIL, so set parsing
// can inherit DTSTART and validate UNTIL against it afterwards.
func parseTypedRRuleAttrs(rruleStr string, result *TypedROption) (bool, error) {
	freqSet := false
	for _, attr := range strings.Split(rruleStr, ";") {
		if attr == "" {
			continue
		}
		keyValue := strings.SplitN(attr, "=", 2)
		if len(keyValue) != 2 || keyValue[1] == "" {
			return false, errors.New("wrong format: " + attr)
		}
		key, value := keyValue[0], keyValue[1]
		var e error
		switch key {
		case "FREQ":
			result.Freq, e = StrToFreq(value)
			freqSet = true
		case "DTSTART":
			result.Dtstart, e = StrToDateTime(value)
		case "INTERVAL":
			result.Interval, e = strconv.Atoi(value)
		case "WKST":
			result.Wkst, e = strToWeekday(value)
		case "COUNT":
			result.Count, e = strconv.Atoi(value)
		case "UNTIL":
			result.Until, e = StrToDateTime(value)
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
			return false, errors.New("unknown RRULE property: " + key)
		}
		if e != nil {
			return false, e
		}
	}
	return freqSet, nil
}

// StrToTypedSet parses an RFC 5545 recurrence set while preserving the kind
// of every DTSTART, RDATE and EXDATE value.
func StrToTypedSet(s string) (*TypedSet, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty string")
	}

	set := &TypedSet{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, err := processRRuleName(line)
		if err != nil {
			return nil, err
		}
		rule := line[len(name)+1:]

		switch name {
		case "DTSTART":
			dt, err := StrToDateTime(rule)
			if err != nil {
				return nil, fmt.Errorf("StrToDateTime failed: %v", err)
			}
			set.DTStart(dt)
		case "RRULE":
			opt := &TypedROption{}
			freqSet, err := parseTypedRRuleAttrs(rule, opt)
			if err != nil {
				return nil, err
			}
			if !freqSet {
				return nil, errors.New("RRULE property FREQ is required")
			}
			if opt.Dtstart.IsZero() {
				if set.GetDTStart().IsZero() {
					return nil, errors.New("RRULE requires DTSTART")
				}
				opt.Dtstart = set.GetDTStart()
			}
			if e := validateUntil(opt.Dtstart, opt.Until); e != nil {
				return nil, e
			}
			r, err := NewTypedRRule(*opt)
			if err != nil {
				return nil, fmt.Errorf("NewTypedRRule failed: %v", err)
			}
			set.RRule(r)
		case "RDATE", "EXDATE":
			dts, err := parseDateList(rule)
			if err != nil {
				return nil, fmt.Errorf("%s parse failed: %v", name, err)
			}
			for _, dt := range dts {
				if name == "RDATE" {
					if err := set.RDate(dt); err != nil {
						return nil, err
					}
				} else {
					set.ExDate(dt)
				}
			}
		default:
			return nil, errors.New("unknown recurrence property: " + name)
		}
	}

	return set, nil
}

// parseDateList handles "VALUE=DATE-TIME;TZID=X:a,b" as well as bare lists.
func parseDateList(rule string) ([]DateTime, error) {
	params := ""
	values := rule
	if idx := strings.LastIndex(rule, ":"); idx >= 0 &&
		// Only treat as params when the prefix is the parameter section.
		!strings.Contains(rule[:idx], ",") {
		params, values = rule[:idx], rule[idx+1:]
	}
	var out []DateTime
	for _, v := range strings.Split(values, ",") {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		full := v
		if params != "" {
			full = params + ":" + v
		}
		dt, err := StrToDateTime(full)
		if err != nil {
			return nil, err
		}
		out = append(out, dt)
	}
	return out, nil
}
