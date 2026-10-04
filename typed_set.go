// 2026, Teambition. All rights reserved.

package rrule

import (
	"sort"
	"strings"
	"time"
)

// TypedSet mixes one recurrence rule with explicit dates and exclusions
// while keeping every value's kind. Exclusions only match occurrences and
// dates of the exact same kind and TZID.
type TypedSet struct {
	dtstart DateTime
	rrule   *TypedRRule
	rdate   []DateTime
	exdate  []DateTime
}

// DTStart sets the DTSTART property of the set.
func (set *TypedSet) DTStart(dtstart DateTime) {
	set.dtstart = dtstart
	if set.rrule != nil {
		opt := set.rrule.OrigOptions
		opt.Dtstart = dtstart
		set.rrule = buildTypedRRule(opt, set.rrule.loc)
	}
}

// GetDTStart returns the set's DTSTART.
func (set *TypedSet) GetDTStart() DateTime {
	if !set.dtstart.IsZero() {
		return set.dtstart
	}
	if set.rrule != nil {
		return set.rrule.GetDTStart()
	}
	return DateTime{}
}

// RRule attaches a recurrence rule. Its DTSTART anchors the set; if the rule
// has none, the set DTSTART is pushed into it.
func (set *TypedSet) RRule(r *TypedRRule) {
	if !r.OrigOptions.Dtstart.IsZero() {
		set.dtstart = r.GetDTStart()
	} else if !set.dtstart.IsZero() {
		opt := r.OrigOptions
		opt.Dtstart = set.dtstart
		r = buildTypedRRule(opt, r.loc)
	}
	set.rrule = r
}

// GetRRule returns the attached rule, if any.
func (set *TypedSet) GetRRule() *TypedRRule { return set.rrule }

// RDate adds an explicit recurrence date. Per RFC 5545 its kind must match
// the DTSTART kind (and the TZID for zoned values); mismatched values are
// rejected instead of being silently coerced.
func (set *TypedSet) RDate(dt DateTime) error {
	if err := validateAuxDate(set.GetDTStart(), dt, "RDATE"); err != nil {
		return err
	}
	set.rdate = append(set.rdate, dt)
	return nil
}

// SetRDates replaces all explicit recurrence dates.
func (set *TypedSet) SetRDates(dts []DateTime) error {
	rdate := make([]DateTime, 0, len(dts))
	for _, dt := range dts {
		if err := validateAuxDate(set.GetDTStart(), dt, "RDATE"); err != nil {
			return err
		}
		rdate = append(rdate, dt)
	}
	set.rdate = rdate
	return nil
}

// GetRDate returns explicit recurrence dates.
func (set *TypedSet) GetRDate() []DateTime { return set.rdate }

// ExDate adds an exclusion date. Exclusions are kept as given and only
// suppress values of the same kind (and TZID); a DATE exclusion never
// removes a date-time occurrence and vice versa.
func (set *TypedSet) ExDate(dt DateTime) {
	set.exdate = append(set.exdate, dt)
}

// SetExDates replaces all exclusion dates.
func (set *TypedSet) SetExDates(dts []DateTime) {
	set.exdate = append([]DateTime(nil), dts...)
}

// GetExDate returns exclusion dates.
func (set *TypedSet) GetExDate() []DateTime { return set.exdate }

func validateAuxDate(dtstart, dt DateTime, prop string) error {
	if dtstart.IsZero() {
		return nil
	}
	if dt.kind != dtstart.kind {
		return kindMismatchError(prop, dt.kind, dtstart.kind)
	}
	if dt.kind == KindTZID && dt.tzid != dtstart.tzid {
		return kindMismatchError(prop+" TZID", dt.tzid, dtstart.tzid)
	}
	return nil
}

// DateTimeIterator evaluates the set. For floating members loc supplies the
// evaluation zone; pass nil for the other kinds.
func (set *TypedSet) Iterator(loc *time.Location) (DateTimeNext, error) {
	anchor := set.GetDTStart()
	if anchor.kind == KindFloating && loc == nil {
		return nil, errFloatingLoc
	}

	type genItem struct {
		dt  DateTime
		gen DateTimeNext
	}

	rlist := make([]genItem, 0, len(set.rdate)+1)
	for _, dt := range set.sortedRDates() {
		dt := dt
		rlist = append(rlist, genItem{
			dt: dt,
			// The value is already peeking in dt; advancing the
			// generator exhausts this one-shot source.
			gen: func() (DateTime, bool) { return DateTime{}, false },
		})
	}
	if set.rrule != nil {
		next, err := set.rrule.Iterator(loc)
		if err != nil {
			return nil, err
		}
		if dt, ok := next(); ok {
			rlist = append(rlist, genItem{dt: dt, gen: next})
		}
	}

	var exlist []DateTime
	for _, dt := range set.exdate {
		exlist = append(exlist, dt)
	}

	last := DateTime{}
	return func() (DateTime, bool) {
		for len(rlist) != 0 {
			sort.Slice(rlist, func(i, j int) bool {
				return dateTimeLess(rlist[i].dt, rlist[j].dt, loc)
			})
			cur := rlist[0].dt
			if next, ok := rlist[0].gen(); ok {
				rlist[0].dt = next
			} else {
				rlist = rlist[1:]
			}

			if !last.IsZero() && last.Equal(cur) {
				continue
			}
			last = cur
			if excludedMatch(exlist, cur) {
				continue
			}
			return cur, true
		}
		return DateTime{}, false
	}, nil
}

// excludedMatch reports whether cur is excluded, matching only the exact
// kind, TZID and wall-clock fields. The loc pointer is intentionally not
// compared so independently parsed equal TZIDs still match.
func excludedMatch(exlist []DateTime, cur DateTime) bool {
	for _, ex := range exlist {
		if ex.kind == cur.kind && ex.tzid == cur.tzid && ex.wall.Equal(cur.wall) {
			return true
		}
	}
	return false
}

func (set *TypedSet) sortedRDates() []DateTime {
	out := append([]DateTime(nil), set.rdate...)
	sort.Slice(out, func(i, j int) bool {
		return dateTimeLess(out[i], out[j], set.GetDTStart().loc)
	})
	return out
}

// dateTimeLess orders values for merge output. Different kinds are ordered
// by their evaluated instant when zones are known, otherwise by wall time.
func dateTimeLess(a, b DateTime, loc *time.Location) bool {
	ta, errA := a.InLocation(loc)
	tb, errB := b.InLocation(loc)
	if errA == nil && errB == nil {
		if ta.Equal(tb) {
			return a.kind < b.kind
		}
		return ta.Before(tb)
	}
	if a.wall.Equal(b.wall) {
		return a.kind < b.kind
	}
	return a.wall.Before(b.wall)
}

// All evaluates the whole set.
func (set *TypedSet) All(loc *time.Location) ([]DateTime, error) {
	next, err := set.Iterator(loc)
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

// Between evaluates occurrences within the bounds, which must match the
// DTSTART kind.
func (set *TypedSet) Between(after, before DateTime, inc bool, loc *time.Location) ([]DateTime, error) {
	anchor := set.GetDTStart()
	if err := checkQueryKind(anchor, after, before); err != nil {
		return nil, err
	}
	next, err := set.Iterator(loc)
	if err != nil {
		return nil, err
	}
	aAbs, err := after.InLocation(loc)
	if err != nil {
		return nil, err
	}
	bAbs, err := before.InLocation(loc)
	if err != nil {
		return nil, err
	}
	var result []DateTime
	for {
		v, ok := next()
		if !ok {
			return result, nil
		}
		abs, err := v.InLocation(loc)
		if err != nil {
			return nil, err
		}
		if inc {
			if abs.After(bAbs) {
				return result, nil
			}
			if !abs.Before(aAbs) {
				result = append(result, v)
			}
		} else if abs.After(aAbs) && abs.Before(bAbs) {
			result = append(result, v)
		}
	}
}

// Recurrence renders the set as RFC 5545 content lines, preserving kinds.
func (set *TypedSet) Recurrence() []string {
	var res []string
	anchor := set.GetDTStart()
	if !anchor.IsZero() {
		res = append(res, "DTSTART;"+anchor.String())
	}
	if set.rrule != nil {
		res = append(res, "RRULE:"+set.rrule.OrigOptions.RRuleString())
	}
	for _, dt := range set.rdate {
		res = append(res, "RDATE;"+dt.String())
	}
	for _, dt := range set.exdate {
		res = append(res, "EXDATE;"+dt.String())
	}
	return res
}

func (set *TypedSet) String() string {
	return strings.Join(set.Recurrence(), "\n")
}
