package rrule

import (
	"strings"
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%s): %v", name, err)
	}
	return loc
}

func wallClockKey(v DTValue) string {
	return v.WallClock().Format("2006-01-02 15:04:05")
}

// The same floating rule evaluated in two zones must produce the same
// wall clock but different absolute instants.
func TestFloatingRuleEvaluatedInTwoZones(t *testing.T) {
	r, err := StrToDTRule("DTSTART:20261003T090000\nRRULE:FREQ=DAILY;COUNT=3")
	if err != nil {
		t.Fatalf("StrToDTRule: %v", err)
	}
	if r.Kind() != KindFloating {
		t.Fatalf("kind = %v, want FLOATING", r.Kind())
	}
	ny := mustLoc(t, "America/New_York")
	sh := mustLoc(t, "Asia/Shanghai")

	nyAll, err := r.All(ny)
	if err != nil {
		t.Fatalf("All(ny): %v", err)
	}
	shAll, err := r.All(sh)
	if err != nil {
		t.Fatalf("All(sh): %v", err)
	}
	if len(nyAll) != 3 || len(shAll) != 3 {
		t.Fatalf("len = %d/%d, want 3/3", len(nyAll), len(shAll))
	}
	for i := range nyAll {
		if nyAll[i].Kind() != KindFloating || shAll[i].Kind() != KindFloating {
			t.Fatalf("occurrence lost floating kind: %v/%v", nyAll[i].Kind(), shAll[i].Kind())
		}
		if wallClockKey(nyAll[i]) != wallClockKey(shAll[i]) {
			t.Errorf("wall clock differs: %s vs %s", wallClockKey(nyAll[i]), wallClockKey(shAll[i]))
		}
		nyT, err := nyAll[i].In(ny)
		if err != nil {
			t.Fatalf("In(ny): %v", err)
		}
		shT, err := shAll[i].In(sh)
		if err != nil {
			t.Fatalf("In(sh): %v", err)
		}
		if nyT.Equal(shT) {
			t.Errorf("absolute instants should differ across zones: %v vs %v", nyT, shT)
		}
		if nyT.Format("15:04") != "09:00" || shT.Format("15:04") != "09:00" {
			t.Errorf("wall clock not preserved: %v / %v", nyT, shT)
		}
	}
}

// Parsing a floating time must not attach time.Local, and evaluating it
// must require an explicit location.
func TestFloatingParseDoesNotAttachLocal(t *testing.T) {
	restore := time.Local
	time.Local = mustLoc(t, "America/New_York")
	defer func() { time.Local = restore }()

	v, err := ParseDTValue("20261003T090000")
	if err != nil {
		t.Fatalf("ParseDTValue: %v", err)
	}
	if v.Kind() != KindFloating {
		t.Fatalf("kind = %v, want FLOATING", v.Kind())
	}
	if v.WallClock().Location() == time.Local {
		t.Errorf("floating value silently attached time.Local")
	}
	if _, err := v.In(nil); err == nil {
		t.Errorf("floating In(nil) should require an explicit location")
	}
}

// Pure-date results must not depend on the server local zone.
func TestDateRuleIndependentOfLocalZone(t *testing.T) {
	restore := time.Local
	defer func() { time.Local = restore }()

	collect := func() []string {
		r, err := StrToDTRule("DTSTART;VALUE=DATE:20261003\nRRULE:FREQ=DAILY;COUNT=3")
		if err != nil {
			t.Fatalf("StrToDTRule: %v", err)
		}
		all, err := r.All(nil) // date rules need no evaluation zone
		if err != nil {
			t.Fatalf("All: %v", err)
		}
		var keys []string
		for _, v := range all {
			if v.Kind() != KindDate {
				t.Fatalf("occurrence lost DATE kind: %v", v.Kind())
			}
			keys = append(keys, wallClockKey(v))
		}
		return keys
	}

	time.Local = mustLoc(t, "America/New_York")
	first := collect()
	time.Local = mustLoc(t, "Asia/Shanghai")
	second := collect()

	if strings.Join(first, ",") != strings.Join(second, ",") {
		t.Errorf("date results depend on time.Local: %v vs %v", first, second)
	}
	want := []string{"2026-10-03 00:00:00", "2026-10-04 00:00:00", "2026-10-05 00:00:00"}
	if strings.Join(first, ",") != strings.Join(want, ",") {
		t.Errorf("dates = %v, want %v", first, want)
	}
}

// Pure-date queries work on date boundaries.
func TestDateRuleBetweenDateBoundaries(t *testing.T) {
	r, err := StrToDTRule("DTSTART;VALUE=DATE:20261003\nRRULE:FREQ=DAILY;UNTIL=20261010")
	if err != nil {
		t.Fatalf("StrToDTRule: %v", err)
	}
	inc, err := r.Between(DateValue(2026, 10, 4), DateValue(2026, 10, 6), true, nil)
	if err != nil {
		t.Fatalf("Between inc: %v", err)
	}
	if len(inc) != 3 {
		t.Fatalf("inc Between = %d results, want 3", len(inc))
	}
	exc, err := r.Between(DateValue(2026, 10, 4), DateValue(2026, 10, 6), false, nil)
	if err != nil {
		t.Fatalf("Between exc: %v", err)
	}
	if len(exc) != 1 || wallClockKey(exc[0]) != "2026-10-05 00:00:00" {
		t.Fatalf("exc Between = %v, want only 2026-10-05", exc)
	}
	if _, err := r.Between(FloatingValue(time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)), DateValue(2026, 10, 6), true, nil); err == nil {
		t.Errorf("DATE rule should reject non-DATE query bounds")
	}
}

// UNTIL must match the DTSTART kind; invalid combinations are rejected
// instead of being silently converted to time.Time.
func TestUntilKindMismatch(t *testing.T) {
	cases := []struct {
		name    string
		str     string
		wantErr bool
	}{
		{"floating with floating until", "DTSTART:20261003T090000\nRRULE:FREQ=DAILY;UNTIL=20261010T090000", false},
		{"floating with UTC until", "DTSTART:20261003T090000\nRRULE:FREQ=DAILY;UNTIL=20261010T090000Z", true},
		{"floating with date until", "DTSTART:20261003T090000\nRRULE:FREQ=DAILY;UNTIL=20261010", true},
		{"date with date until", "DTSTART;VALUE=DATE:20261003\nRRULE:FREQ=DAILY;UNTIL=20261010", false},
		{"date with floating until", "DTSTART;VALUE=DATE:20261003\nRRULE:FREQ=DAILY;UNTIL=20261010T090000", true},
		{"date with UTC until", "DTSTART;VALUE=DATE:20261003\nRRULE:FREQ=DAILY;UNTIL=20261010T090000Z", true},
		{"tzid with UTC until", "DTSTART;TZID=America/New_York:20261003T090000\nRRULE:FREQ=DAILY;UNTIL=20261010T090000Z", false},
		{"tzid with floating until", "DTSTART;TZID=America/New_York:20261003T090000\nRRULE:FREQ=DAILY;UNTIL=20261010T090000", true},
		{"utc with UTC until", "DTSTART:20261003T090000Z\nRRULE:FREQ=DAILY;UNTIL=20261010T090000Z", false},
		{"utc with floating until", "DTSTART:20261003T090000Z\nRRULE:FREQ=DAILY;UNTIL=20261010T090000", true},
	}
	for _, c := range cases {
		_, err := StrToDTRule(c.str)
		if c.wantErr && err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
		if !c.wantErr && err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
		}
	}
}

// Programmatic construction must apply the same UNTIL validation.
func TestUntilKindMismatchProgrammatic(t *testing.T) {
	if _, err := NewDTRule(DTROption{
		Freq:    DAILY,
		Dtstart: FloatingValue(time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)),
		Until:   UTCValue(time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)),
	}); err == nil {
		t.Errorf("floating DTSTART with UTC UNTIL should be rejected")
	}
	if _, err := NewDTRule(DTROption{
		Freq:    DAILY,
		Dtstart: DateValue(2026, 10, 3),
		Until:   DateValue(2026, 10, 10),
	}); err != nil {
		t.Errorf("DATE DTSTART with DATE UNTIL should be accepted: %v", err)
	}
	if _, err := NewDTRule(DTROption{
		Freq:    DAILY,
		Dtstart: FloatingValue(time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)),
	}); err != nil {
		t.Errorf("floating DTSTART without UNTIL should be accepted: %v", err)
	}
}

// Exclusion dates only exclude occurrences of their own kind, and
// kind-mismatched EXDATE/RDATE combinations are rejected.
func TestExDateKindSpecific(t *testing.T) {
	// Floating set: a floating EXDATE removes exactly that occurrence.
	floatingSet, err := StrToDTSet("DTSTART:20261003T090000\nRRULE:FREQ=DAILY;COUNT=3\nEXDATE:20261004T090000")
	if err != nil {
		t.Fatalf("StrToDTSet floating: %v", err)
	}
	all, err := floatingSet.All(mustLoc(t, "America/New_York"))
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("floating set = %d occurrences, want 2", len(all))
	}
	for _, v := range all {
		if v.Kind() != KindFloating {
			t.Errorf("occurrence lost floating kind: %v", v.Kind())
		}
		if wallClockKey(v) == "2026-10-04 09:00:00" {
			t.Errorf("floating EXDATE did not exclude its occurrence")
		}
	}

	// DATE set: a DATE EXDATE removes exactly that date.
	dateSet, err := StrToDTSet("DTSTART;VALUE=DATE:20261003\nRRULE:FREQ=DAILY;COUNT=3\nEXDATE;VALUE=DATE:20261004")
	if err != nil {
		t.Fatalf("StrToDTSet date: %v", err)
	}
	dates, err := dateSet.All(nil)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(dates) != 2 {
		t.Fatalf("date set = %d occurrences, want 2", len(dates))
	}
	for _, v := range dates {
		if v.Kind() != KindDate {
			t.Errorf("occurrence lost DATE kind: %v", v.Kind())
		}
		if wallClockKey(v) == "2026-10-04 00:00:00" {
			t.Errorf("DATE EXDATE did not exclude its date")
		}
	}

	// Kind-mismatched EXDATE/RDATE lines must be rejected, not silently
	// converted and accepted.
	bad := []struct {
		name string
		str  string
	}{
		{"floating dtstart with UTC exdate", "DTSTART:20261003T090000\nRRULE:FREQ=DAILY;COUNT=3\nEXDATE:20261004T090000Z"},
		{"floating dtstart with date exdate", "DTSTART:20261003T090000\nRRULE:FREQ=DAILY;COUNT=3\nEXDATE;VALUE=DATE:20261004"},
		{"date dtstart with floating exdate", "DTSTART;VALUE=DATE:20261003\nRRULE:FREQ=DAILY;COUNT=3\nEXDATE:20261004T090000"},
		{"date dtstart with floating rdate", "DTSTART;VALUE=DATE:20261003\nRRULE:FREQ=DAILY;COUNT=3\nRDATE:20261004T090000"},
		{"utc dtstart with floating exdate", "DTSTART:20261003T090000Z\nRRULE:FREQ=DAILY;COUNT=3\nEXDATE:20261004T090000"},
	}
	for _, c := range bad {
		if _, err := StrToDTSet(c.str); err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
	}

	// Programmatic API applies the same validation.
	set := &DTSet{}
	if err := set.DTStart(FloatingValue(time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC))); err != nil {
		t.Fatalf("DTStart: %v", err)
	}
	if err := set.ExDate(UTCValue(time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC))); err == nil {
		t.Errorf("UTC EXDATE on floating set should be rejected")
	}
	if err := set.ExDate(DateValue(2026, 10, 4)); err == nil {
		t.Errorf("DATE EXDATE on floating set should be rejected")
	}
	if err := set.ExDate(FloatingValue(time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC))); err != nil {
		t.Errorf("floating EXDATE on floating set should be accepted: %v", err)
	}
}

// For absolute kinds, exclusion compares instants: a UTC EXDATE denoting
// the same instant as a TZID occurrence excludes it.
func TestExDateAbsoluteComparesByInstant(t *testing.T) {
	set, err := StrToDTSet("DTSTART;TZID=America/New_York:20261003T090000\nRRULE:FREQ=DAILY;COUNT=3\nEXDATE:20261004T130000Z")
	if err != nil {
		t.Fatalf("StrToDTSet: %v", err)
	}
	all, err := set.All(nil)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("set = %d occurrences, want 2 (2026-10-04 09:00 EDT == 13:00 UTC excluded)", len(all))
	}
	for _, v := range all {
		if v.Kind() != KindTZID || v.TZID() != "America/New_York" {
			t.Errorf("occurrence lost TZID kind: %v %q", v.Kind(), v.TZID())
		}
	}
}

// String round-trips must preserve the value kind and TZID.
func TestDTValueRoundTrip(t *testing.T) {
	cases := []struct {
		str  string
		kind DTKind
		tzid string
	}{
		{";VALUE=DATE:20261003", KindDate, ""},
		{":20261003T090000", KindFloating, ""},
		{";TZID=America/New_York:20261003T090000", KindTZID, "America/New_York"},
		{":20261003T090000Z", KindUTC, ""},
	}
	for _, c := range cases {
		v, err := ParseDTValue(c.str)
		if err != nil {
			t.Fatalf("ParseDTValue(%q): %v", c.str, err)
		}
		if v.Kind() != c.kind {
			t.Errorf("ParseDTValue(%q) kind = %v, want %v", c.str, v.Kind(), c.kind)
		}
		if v.TZID() != c.tzid {
			t.Errorf("ParseDTValue(%q) tzid = %q, want %q", c.str, v.TZID(), c.tzid)
		}
		if got := v.String(); got != c.str {
			t.Errorf("round trip = %q, want %q", got, c.str)
		}
		v2, err := ParseDTValue(v.String())
		if err != nil {
			t.Fatalf("re-parse of %q: %v", v.String(), err)
		}
		if !v.Equal(v2) || v2.Kind() != v.Kind() {
			t.Errorf("re-parsed value differs: %v vs %v", v2, v)
		}
	}
}

// Rule and set strings round-trip preserving kinds and TZID.
func TestDTRuleAndSetRoundTrip(t *testing.T) {
	ruleStrs := []string{
		"DTSTART;VALUE=DATE:20261003\nRRULE:FREQ=DAILY;UNTIL=20261010",
		"DTSTART:20261003T090000\nRRULE:FREQ=DAILY;UNTIL=20261010T090000",
		"DTSTART;TZID=America/New_York:20261003T090000\nRRULE:FREQ=DAILY;UNTIL=20261010T130000Z",
		"DTSTART:20261003T090000Z\nRRULE:FREQ=DAILY;UNTIL=20261010T090000Z",
	}
	for _, s := range ruleStrs {
		r, err := StrToDTRule(s)
		if err != nil {
			t.Fatalf("StrToDTRule(%q): %v", s, err)
		}
		if got := r.String(); got != s {
			t.Errorf("rule round trip = %q, want %q", got, s)
		}
	}

	setStr := "DTSTART;TZID=America/New_York:20261003T090000\n" +
		"RRULE:FREQ=DAILY;COUNT=3\n" +
		"EXDATE;TZID=America/New_York:20261004T090000"
	set, err := StrToDTSet(setStr)
	if err != nil {
		t.Fatalf("StrToDTSet: %v", err)
	}
	if got := set.String(); got != setStr {
		t.Errorf("set round trip = %q, want %q", got, setStr)
	}

	dateSetStr := "DTSTART;VALUE=DATE:20261003\n" +
		"RRULE:FREQ=DAILY;COUNT=3\n" +
		"EXDATE;VALUE=DATE:20261004"
	dateSet, err := StrToDTSet(dateSetStr)
	if err != nil {
		t.Fatalf("StrToDTSet: %v", err)
	}
	if got := dateSet.String(); got != dateSetStr {
		t.Errorf("date set round trip = %q, want %q", got, dateSetStr)
	}
}

// TZID rules keep their zone on occurrences and ignore the evaluation loc.
func TestTZIDRuleKeepsZone(t *testing.T) {
	r, err := StrToDTRule("DTSTART;TZID=America/New_York:20261003T090000\nRRULE:FREQ=DAILY;COUNT=2")
	if err != nil {
		t.Fatalf("StrToDTRule: %v", err)
	}
	// Even a caller-supplied loc must not reinterpret a TZID rule.
	all, err := r.All(mustLoc(t, "Asia/Shanghai"))
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len = %d, want 2", len(all))
	}
	for _, v := range all {
		if v.Kind() != KindTZID || v.TZID() != "America/New_York" {
			t.Fatalf("occurrence = %v %q, want TZID America/New_York", v.Kind(), v.TZID())
		}
		ts, err := v.In(nil)
		if err != nil {
			t.Fatalf("In: %v", err)
		}
		if ts.Format("15:04") != "09:00" || ts.Location().String() != "America/New_York" {
			t.Errorf("TZID occurrence evaluated wrong: %v", ts)
		}
	}
}

// A floating rule evaluated without an explicit location must fail
// rather than silently falling back to time.Local.
func TestFloatingRuleRequiresExplicitLocation(t *testing.T) {
	r, err := StrToDTRule("DTSTART:20261003T090000\nRRULE:FREQ=DAILY;COUNT=2")
	if err != nil {
		t.Fatalf("StrToDTRule: %v", err)
	}
	if _, err := r.All(nil); err == nil {
		t.Errorf("floating rule evaluated without a location should fail")
	}
	if _, err := r.Iterator(nil); err == nil {
		t.Errorf("floating rule iterator without a location should fail")
	}
}
