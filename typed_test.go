// 2026, Teambition. All rights reserved.

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
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

// A floating rule must keep its wall-clock fields in every evaluation zone
// while the absolute instants differ by the zone offsets.
func TestTypedFloatingSameWallDifferentInstant(t *testing.T) {
	dt, err := StrToDateTime("20261003T090000")
	if err != nil {
		t.Fatal(err)
	}
	if dt.Kind() != KindFloating {
		t.Fatalf("kind = %v, want floating", dt.Kind())
	}

	r, err := NewTypedRRule(TypedROption{Freq: DAILY, Dtstart: dt, Count: 3})
	if err != nil {
		t.Fatal(err)
	}

	shanghai := mustLoc(t, "Asia/Shanghai")
	auckland := mustLoc(t, "Pacific/Auckland")

	sh, err := r.All(shanghai)
	if err != nil {
		t.Fatal(err)
	}
	ak, err := r.All(auckland)
	if err != nil {
		t.Fatal(err)
	}
	if len(sh) != 3 || len(ak) != 3 {
		t.Fatalf("got %d / %d occurrences, want 3 each", len(sh), len(ak))
	}

	for i := range sh {
		if sh[i].Kind() != KindFloating || ak[i].Kind() != KindFloating {
			t.Fatalf("kind lost after evaluation: %v / %v", sh[i].Kind(), ak[i].Kind())
		}
		sy, sm, sd := sh[i].Date()
		ay, am, ad := ak[i].Date()
		shH, shM, shS := sh[i].Clock()
		akH, akM, akS := ak[i].Clock()
		if sy != ay || sm != am || sd != ad ||
			shH != akH || shM != akM || shS != akS {
			t.Fatalf("wall time differs at %d: %d-%d-%d %02d:%02d:%02d vs %d-%d-%d %02d:%02d:%02d",
				i, sy, sm, sd, shH, shM, shS, ay, am, ad, akH, akM, akS)
		}
		if shH != 9 || shM != 0 || shS != 0 {
			t.Fatalf("clock = %02d:%02d:%02d, want 09:00:00", shH, shM, shS)
		}

		shAbs, err := sh[i].InLocation(shanghai)
		if err != nil {
			t.Fatal(err)
		}
		akAbs, err := ak[i].InLocation(auckland)
		if err != nil {
			t.Fatal(err)
		}
		// Same wall clock in two zones must be different instants; the
		// gap is exactly the difference of the zone offsets.
		_, offSh := shAbs.Zone()
		_, offAk := akAbs.Zone()
		wantDiff := time.Duration(offAk-offSh) * time.Second
		if diff := shAbs.Sub(akAbs); diff != wantDiff || diff == 0 {
			t.Fatalf("absolute difference = %v, want %v at occurrence %d", diff, wantDiff, i)
		}
	}

	// Evaluating a floating rule without a location must fail loudly.
	if _, err := r.All(nil); err == nil {
		t.Fatal("floating All(nil) succeeded, want error")
	}
	if _, err := r.Iterator(nil); err == nil {
		t.Fatal("floating Iterator(nil) succeeded, want error")
	}
}

// Parsing a floating value must not secretly attach time.Local, and pure
// date recurrence must not move when the server local zone changes.
func TestTypedDateUnaffectedByLocalZone(t *testing.T) {
	floating, err := StrToDateTime("20261003T090000")
	if err != nil {
		t.Fatal(err)
	}
	abs, err := floating.InLocation(mustLoc(t, "Asia/Shanghai"))
	if err != nil {
		t.Fatal(err)
	}
	if _, off := abs.Zone(); off != 8*3600 {
		t.Fatalf("floating evaluated with Shanghai offset = %d, want 28800", off)
	}

	date, err := StrToDateTime("VALUE=DATE:20261003")
	if err != nil {
		t.Fatal(err)
	}
	if date.Kind() != KindDate {
		t.Fatalf("kind = %v, want date", date.Kind())
	}
	r, err := NewTypedRRule(TypedROption{Freq: DAILY, Dtstart: date, Count: 5})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"VALUE=DATE:20261003",
		"VALUE=DATE:20261004",
		"VALUE=DATE:20261005",
		"VALUE=DATE:20261006",
		"VALUE=DATE:20261007",
	}

	run := func() {
		got, err := r.All(nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("got %d dates, want %d", len(got), len(want))
		}
		for i, w := range want {
			if got[i].String() != w {
				t.Errorf("local=%s: got[%d] = %s, want %s",
					time.Local, i, got[i].String(), w)
			}
		}
	}

	originLocal := time.Local
	defer func() { time.Local = originLocal }()
	for _, name := range []string{"Asia/Shanghai", "Pacific/Honolulu", "UTC"} {
		time.Local = mustLoc(t, name)
		run()
	}
}

func TestTypedUntilKindMismatch(t *testing.T) {
	date := NewDate(2026, time.October, 3)
	floating := NewFloating(2026, time.October, 3, 9, 0, 0)
	utc := NewUTC(2026, time.October, 3, 9, 0, 0)
	tzid, err := NewTZID(2026, time.October, 3, 9, 0, 0, "America/New_York")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		dtstart DateTime
		until   DateTime
	}{
		{"date start until datetime", date, utc},
		{"date start until floating", date, floating},
		{"floating start until utc", floating, utc},
		{"floating start until date", floating, date},
		{"utc start until floating", utc, floating},
		{"utc start until date", utc, date},
		{"tzid start until floating", tzid, floating},
		{"tzid start until date", tzid, date},
	}
	for _, c := range cases {
		if _, err := NewTypedRRule(TypedROption{
			Freq: DAILY, Dtstart: c.dtstart, Until: c.until,
		}); err == nil {
			t.Errorf("%s: NewTypedRRule accepted illegal UNTIL", c.name)
		}
	}

	// Legal combinations must be accepted.
	legal := []struct {
		dtstart DateTime
		until   DateTime
	}{
		{date, NewDate(2026, time.October, 10)},
		{floating, NewFloating(2026, time.October, 10, 9, 0, 0)},
		{utc, NewUTC(2026, time.October, 10, 9, 0, 0)},
		{tzid, NewUTC(2026, time.October, 10, 13, 0, 0)},
	}
	for _, c := range legal {
		if _, err := NewTypedRRule(TypedROption{
			Freq: DAILY, Dtstart: c.dtstart, Until: c.until,
		}); err != nil {
			t.Errorf("legal pair (%s until %s) rejected: %v",
				c.dtstart.Kind(), c.until.Kind(), err)
		}
	}

	// String parsing must reject the mismatch rather than coercing it.
	badStrings := []string{
		"DTSTART;VALUE=DATE:20261003\nRRULE:FREQ=DAILY;UNTIL=20261010T000000Z",
		"DTSTART:20261003T090000\nRRULE:FREQ=DAILY;UNTIL=20261010T000000Z",
		"DTSTART:20261003T090000Z\nRRULE:FREQ=DAILY;UNTIL=20261010",
		"DTSTART;TZID=America/New_York:20261003T090000\nRRULE:FREQ=DAILY;UNTIL=20261010T090000",
	}
	for _, s := range badStrings {
		if _, err := StrToTypedRRule(s); err == nil {
			t.Errorf("StrToTypedRRule(%q) accepted UNTIL mismatch", s)
		}
		if _, err := StrToTypedSet(s); err == nil {
			t.Errorf("StrToTypedSet(%q) accepted UNTIL mismatch", s)
		}
	}
}

func TestTypedExDateKindScoped(t *testing.T) {
	// A DATE exclusion must not remove a zoned date-time occurrence; only
	// the matching TZID exclusion removes it.
	tzidStart, err := NewTZID(2026, time.October, 3, 9, 0, 0, "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	rule, err := NewTypedRRule(TypedROption{Freq: DAILY, Dtstart: tzidStart, Count: 3})
	if err != nil {
		t.Fatal(err)
	}

	set := &TypedSet{}
	set.RRule(rule)
	set.ExDate(NewDate(2026, time.October, 4))
	got, err := set.All(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("DATE EXDATE removed a TZID occurrence: got %d, want 3", len(got))
	}

	matching, err := NewTZID(2026, time.October, 4, 9, 0, 0, "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	set.ExDate(matching)
	got, err = set.All(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("matching TZID EXDATE not applied: got %d, want 2", len(got))
	}
	for _, dt := range got {
		if dt.Equal(matching) {
			t.Fatal("excluded occurrence still present")
		}
	}

	// A floating date-time exclusion must not remove a pure date occurrence;
	// a DATE exclusion does.
	dateRule, err := NewTypedRRule(TypedROption{
		Freq: DAILY, Dtstart: NewDate(2026, time.October, 3), Count: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	dateSet := &TypedSet{}
	dateSet.RRule(dateRule)
	dateSet.ExDate(NewFloating(2026, time.October, 4, 0, 0, 0))
	dates, err := dateSet.All(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(dates) != 3 {
		t.Fatalf("floating EXDATE removed a DATE occurrence: got %d, want 3", len(dates))
	}
	dateSet.ExDate(NewDate(2026, time.October, 4))
	dates, err = dateSet.All(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(dates) != 2 {
		t.Fatalf("DATE EXDATE not applied: got %d, want 2", len(dates))
	}
}

func TestTypedStringRoundTrip(t *testing.T) {
	cases := []string{
		"DTSTART;VALUE=DATE:20261003\nRRULE:FREQ=DAILY;COUNT=3;UNTIL=20261010",
		"DTSTART:20261003T090000\nRRULE:FREQ=DAILY;COUNT=3;UNTIL=20261010T090000",
		"DTSTART:20261003T090000Z\nRRULE:FREQ=DAILY;COUNT=3;UNTIL=20261010T090000Z",
		"DTSTART;TZID=America/New_York:20261003T090000\nRRULE:FREQ=DAILY;COUNT=3;UNTIL=20261010T130000Z",
	}
	for _, in := range cases {
		r, err := StrToTypedRRule(in)
		if err != nil {
			t.Fatalf("parse %q: %v", in, err)
		}
		if out := r.String(); out != in {
			t.Errorf("round trip = %q, want %q", out, in)
		}
	}

	// Set round trip preserves kinds on RDATE/EXDATE.
	setStr := strings.Join([]string{
		"DTSTART;VALUE=DATE:20261003",
		"RRULE:FREQ=DAILY;COUNT=5",
		"RDATE;VALUE=DATE:20261225",
		"EXDATE;VALUE=DATE:20261005",
	}, "\n")
	set, err := StrToTypedSet(setStr)
	if err != nil {
		t.Fatal(err)
	}
	if out := set.String(); out != setStr {
		t.Fatalf("set round trip = %q, want %q", out, setStr)
	}
	all, err := set.All(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, dt := range all {
		if dt.Kind() != KindDate {
			t.Fatalf("set lost DATE kind: %v", dt.Kind())
		}
	}
}
