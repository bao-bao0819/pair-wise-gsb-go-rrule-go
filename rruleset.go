// 2017-2022, Teambition. All rights reserved.

package rrule

import (
	"fmt"
	"sort"
	"time"
)

// Set allows more complex recurrence setups, mixing multiple rules, dates, exclusion rules, and exclusion dates
type Set struct {
	dtstart time.Time
	rrule   *RRule
	rdate   []time.Time
	exdate  []time.Time
}

// Recurrence returns a slice of all the recurrence rules for a set
func (set *Set) Recurrence() []string {
	var res []string

	if !set.dtstart.IsZero() {
		// No colon, DTSTART may have TZID, which would require a semicolon after DTSTART
		res = append(res, fmt.Sprintf("DTSTART%s", timeToRFCDatetimeStr(set.dtstart)))
	}

	if set.rrule != nil {
		res = append(res, fmt.Sprintf("RRULE:%s", set.rrule.OrigOptions.RRuleString()))
	}

	for _, item := range set.rdate {
		res = append(res, fmt.Sprintf("RDATE%s", timeToRFCDatetimeStr(item)))
	}

	for _, item := range set.exdate {
		res = append(res, fmt.Sprintf("EXDATE%s", timeToRFCDatetimeStr(item)))
	}
	return res
}

// DTStart sets dtstart property for set.
// It will be truncated to second precision.
func (set *Set) DTStart(dtstart time.Time) {
	set.dtstart = dtstart.Truncate(time.Second)

	if set.rrule != nil {
		set.rrule.DTStart(set.dtstart)
	}
}

// GetDTStart gets DTSTART for set
func (set *Set) GetDTStart() time.Time {
	return set.dtstart
}

// RRule set the RRULE for set.
// There is the only one RRULE in the set as https://tools.ietf.org/html/rfc5545#appendix-A.1
func (set *Set) RRule(rrule *RRule) {
	if !rrule.OrigOptions.Dtstart.IsZero() {
		set.dtstart = rrule.dtstart
	} else if !set.dtstart.IsZero() {
		rrule.DTStart(set.dtstart)
	}
	set.rrule = rrule
}

// GetRRule returns the rrules in the set
func (set *Set) GetRRule() *RRule {
	return set.rrule
}

// RDate include the given datetime instance in the recurrence set generation.
// It will be truncated to second precision.
func (set *Set) RDate(rdate time.Time) {
	set.rdate = append(set.rdate, rdate.Truncate(time.Second))
}

// SetRDates sets explicitly added dates (rdates) in the set.
// It will be truncated to second precision.
func (set *Set) SetRDates(rdates []time.Time) {
	set.rdate = make([]time.Time, 0, len(rdates))
	for _, rdate := range rdates {
		set.rdate = append(set.rdate, rdate.Truncate(time.Second))
	}
}

// GetRDate returns explicitly added dates (rdates) in the set
func (set *Set) GetRDate() []time.Time {
	return set.rdate
}

// ExDate include the given datetime instance in the recurrence set exclusion list.
// Dates included that way will not be generated,
// even if some inclusive rrule or rdate matches them.
// It will be truncated to second precision.
func (set *Set) ExDate(exdate time.Time) {
	set.exdate = append(set.exdate, exdate.Truncate(time.Second))
}

// SetExDates sets explicitly excluded dates (exdates) in the set.
// It will be truncated to second precision.
func (set *Set) SetExDates(exdates []time.Time) {
	set.exdate = make([]time.Time, 0, len(exdates))
	for _, exdate := range exdates {
		set.exdate = append(set.exdate, exdate.Truncate(time.Second))
	}
}

// GetExDate returns explicitly excluded dates (exdates) in the set
func (set *Set) GetExDate() []time.Time {
	return set.exdate
}

type genItem struct {
	dt  time.Time
	gen Next
}

type genItemSlice []genItem

func (s genItemSlice) Len() int           { return len(s) }
func (s genItemSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s genItemSlice) Less(i, j int) bool { return s[i].dt.Before(s[j].dt) }

func addGenList(genList *[]genItem, next Next) {
	dt, ok := next()
	if ok {
		*genList = append(*genList, genItem{dt, next})
	}
}

// Iterator returns an iterator for rrule.Set
func (set *Set) Iterator() (next func() (time.Time, bool)) {
	rlist := []genItem{}
	exlist := []genItem{}

	sort.Sort(timeSlice(set.rdate))
	addGenList(&rlist, timeSliceIterator(set.rdate))
	if set.rrule != nil {
		addGenList(&rlist, set.rrule.Iterator())
	}
	sort.Sort(genItemSlice(rlist))

	sort.Sort(timeSlice(set.exdate))
	addGenList(&exlist, timeSliceIterator(set.exdate))
	sort.Sort(genItemSlice(exlist))

	lastdt := time.Time{}
	return func() (time.Time, bool) {
		for len(rlist) != 0 {
			dt := rlist[0].dt
			var ok bool
			rlist[0].dt, ok = rlist[0].gen()
			if !ok {
				rlist = rlist[1:]
			}
			sort.Sort(genItemSlice(rlist))
			if lastdt.IsZero() || !lastdt.Equal(dt) {
				for len(exlist) != 0 && exlist[0].dt.Before(dt) {
					exlist[0].dt, ok = exlist[0].gen()
					if !ok {
						exlist = exlist[1:]
					}
					sort.Sort(genItemSlice(exlist))
				}
				lastdt = dt
				if len(exlist) == 0 || !dt.Equal(exlist[0].dt) {
					return dt, true
				}
			}
		}
		return time.Time{}, false
	}
}

// ReverseIterator returns an iterator that yields the occurrences of the
// rrule.Set inside the closed interval [start, end] in descending
// (reverse chronological) order. The RRULE stream, RDATEs and EXDATEs are
// merged and deduplicated in reverse order, so the result is exactly the
// reverse of set.Between(start, end, true). The date slices held by the
// set are copied first and are never sorted or modified in place.
// It is only supported second precision.
func (set *Set) ReverseIterator(start, end time.Time) Next {
	rdates := make([]time.Time, 0, len(set.rdate))
	for _, dt := range set.rdate {
		if !dt.Before(start) && !dt.After(end) {
			rdates = append(rdates, dt)
		}
	}
	sort.Sort(timeSlice(rdates))

	exdates := make([]time.Time, 0, len(set.exdate))
	for _, dt := range set.exdate {
		if !dt.Before(start) && !dt.After(end) {
			exdates = append(exdates, dt)
		}
	}
	sort.Sort(timeSlice(exdates))

	var ruleNext Next
	var ruleDt time.Time
	var ruleOk bool
	if set.rrule != nil {
		ruleNext = set.rrule.ReverseIterator(start, end)
		ruleDt, ruleOk = ruleNext()
	}

	ri := len(rdates) - 1
	ei := len(exdates) - 1
	var last time.Time
	hasLast := false

	return func() (time.Time, bool) {
		for {
			var dt time.Time
			if ruleOk && (ri < 0 || !ruleDt.Before(rdates[ri])) {
				dt = ruleDt
				ruleDt, ruleOk = ruleNext()
			} else if ri >= 0 {
				dt = rdates[ri]
				ri--
			} else {
				return time.Time{}, false
			}
			if hasLast && dt.Equal(last) {
				continue
			}
			for ei >= 0 && exdates[ei].After(dt) {
				ei--
			}
			last, hasLast = dt, true
			if ei >= 0 && exdates[ei].Equal(dt) {
				continue
			}
			return dt, true
		}
	}
}

// All returns all occurrences of the rrule.Set.
// It is only supported second precision.
func (set *Set) All() []time.Time {
	return all(set.Iterator())
}

// Between returns all the occurrences of the rrule between after and before.
// The inc keyword defines what happens if after and/or before are themselves occurrences.
// With inc == True, they will be included in the list, if they are found in the recurrence set.
// It is only supported second precision.
func (set *Set) Between(after, before time.Time, inc bool) []time.Time {
	return between(set.Iterator(), after, before, inc)
}

// Before Returns the last recurrence before the given datetime instance,
// or time.Time's zero value if no recurrence match.
// The inc keyword defines what happens if dt is an occurrence.
// With inc == True, if dt itself is an occurrence, it will be returned.
// It is only supported second precision.
func (set *Set) Before(dt time.Time, inc bool) time.Time {
	return before(set.Iterator(), dt, inc)
}

// After returns the first recurrence after the given datetime instance,
// or time.Time's zero value if no recurrence match.
// The inc keyword defines what happens if dt is an occurrence.
// With inc == True, if dt itself is an occurrence, it will be returned.
// It is only supported second precision.
func (set *Set) After(dt time.Time, inc bool) time.Time {
	return after(set.Iterator(), dt, inc)
}
