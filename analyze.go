// 2017-2022, Teambition. All rights reserved.

package rrule

import (
	"fmt"
	"time"
)

// EmptyStatus describes the result of a read-only emptiness analysis.
type EmptyStatus int

const (
	// EmptyUnknown means the analyzer could neither produce an occurrence
	// nor prove the rule empty within its work budget or supported scope.
	EmptyUnknown EmptyStatus = iota
	// EmptyProven means the rule can never emit any occurrence.
	EmptyProven
	// HasWitness means at least one occurrence exists within the rule's
	// effective horizon; Witness is that occurrence.
	HasWitness
)

// String returns the name of the status.
func (s EmptyStatus) String() string {
	switch s {
	case EmptyProven:
		return "EMPTY-PROVEN"
	case HasWitness:
		return "HAS-WITNESS"
	default:
		return "UNKNOWN"
	}
}

// DecisiveRule is a rule item that contributes to proving a recurrence empty.
type DecisiveRule struct {
	// Item is the offending rule component in RFC-like form, e.g.
	// "BYMONTHDAY=30", "INTERVAL=4" or "UNTIL=20030101".
	Item string
	// Why describes the role of this item in the emptiness proof.
	Why string
	// RemovingWitness is an occurrence that would exist if this item alone
	// were relaxed (the item is therefore necessary for the proof).
	RemovingWitness time.Time
}

// EmptyResult is the result of AnalyzeEmpty.
type EmptyResult struct {
	// Status is EmptyProven, HasWitness or EmptyUnknown.
	Status EmptyStatus
	// Witness is an occurrence verifiable by the normal iterator.
	// It is the zero time unless Status is HasWitness.
	Witness time.Time
	// Decisive lists the rule items responsible when Status is EmptyProven.
	Decisive []DecisiveRule
	// Bounded is true when the proof only covers the rule's effective
	// horizon (an explicit UNTIL or the library's ~292-year default horizon
	// or the year-9999 limit) rather than a full Gregorian/INTERVAL cycle.
	Bounded bool
	// BudgetExhausted is true when the work budget ran out before a
	// definite answer was found.
	BudgetExhausted bool
	// CountLimited records that a positive COUNT was present. COUNT only
	// truncates output, so it never makes a non-empty rule empty; a
	// COUNT>0 rule is empty exactly when its underlying rule is empty.
	CountLimited bool
	// Why summarizes how the result was reached.
	Why string
}

// DefaultAnalyzeBudget is the default work budget for AnalyzeEmpty, measured
// in candidate day evaluations. It comfortably covers a full
// Gregorian/INTERVAL phase cycle for every supported rule (at most
// 146097 days for DAILY with INTERVAL=1) plus the decisive-rule ablations.
const DefaultAnalyzeBudget = 5_000_000

// AnalyzeOption configures AnalyzeEmpty.
type AnalyzeOption func(*analyzeConfig)

type analyzeConfig struct {
	budget int
}

// AnalyzeBudget sets the work budget, measured in candidate day evaluations.
// A non-positive value selects DefaultAnalyzeBudget. When the budget is
// exhausted the result status is EmptyUnknown with BudgetExhausted set.
func AnalyzeBudget(days int) AnalyzeOption {
	return func(c *analyzeConfig) { c.budget = days }
}

// AnalyzeEmpty performs a read-only analysis of whether the recurrence rule
// can ever emit an occurrence.
//
// The analysis currently supports pure date semantics in UTC for
// DAILY/WEEKLY/MONTHLY/YEARLY with the library's date-level filters
// (BYMONTH, BYMONTHDAY, BYYEARDAY, BYWEEKNO, BYDAY with/without nth
// qualifiers and BYSETPOS). Intraday components (non-midnight DTSTART,
// BYHOUR/BYMINUTE/BYSECOND), non-UTC locations, HOURLY/MINUTELY/SECONDLY,
// BYEASTER and a date-time UNTIL yield EmptyUnknown.
//
// DTSTART and INTERVAL phase are preserved. The Gregorian 400-year cycle
// (146097 days, exactly 20871 weeks) is combined with INTERVAL via a finite
// phase-state space: only cycle/gcd(INTERVAL,cycle) distinct states exist.
// Emptiness over that finite space is exact, so a large INTERVAL (which may
// skip the first 400 future years entirely) is handled correctly instead of
// being declared empty by a naive 400-year forward scan.
func (r *RRule) AnalyzeEmpty(opts ...AnalyzeOption) EmptyResult {
	cfg := analyzeConfig{budget: DefaultAnalyzeBudget}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.budget <= 0 {
		cfg.budget = DefaultAnalyzeBudget
	}
	return (&emptyAnalyzer{r: r, budget: cfg.budget}).analyze()
}

// AnalyzeEmpty is the package-level convenience wrapper. It returns an error
// only when the rule options themselves are invalid.
func AnalyzeEmpty(options ROption, opts ...AnalyzeOption) (EmptyResult, error) {
	r, err := NewRRule(options)
	if err != nil {
		return EmptyResult{Status: EmptyUnknown, Why: "invalid rule options: " + err.Error()}, err
	}
	return r.AnalyzeEmpty(opts...), nil
}

type emptyAnalyzer struct {
	r      *RRule
	budget int
	spent  int
}

// charge accounts for n candidate day evaluations; false on over-budget.
func (a *emptyAnalyzer) charge(n int) bool {
	a.spent += n
	return a.spent <= a.budget
}

func (a *emptyAnalyzer) analyze() EmptyResult {
	r := a.r
	res := EmptyResult{CountLimited: r.count > 0}

	// Scope gate: pure date semantics in UTC for YEARLY..DAILY only.
	if r.freq < YEARLY || r.freq > DAILY {
		res.Why = "only DAILY/WEEKLY/MONTHLY/YEARLY date semantics are supported"
		return res
	}
	if loc := r.dtstart.Location(); loc != time.UTC {
		res.Why = "only UTC date semantics are supported; DTSTART location is " + loc.String()
		return res
	}
	if hh, mm, ss := r.dtstart.Clock(); hh != 0 || mm != 0 || ss != 0 || r.dtstart.Nanosecond() != 0 {
		res.Why = "intraday DTSTART is not supported by the date-only analyzer"
		return res
	}
	if len(r.OrigOptions.Byhour) != 0 || len(r.OrigOptions.Byminute) != 0 ||
		len(r.OrigOptions.Bysecond) != 0 {
		res.Why = "BYHOUR/BYMINUTE/BYSECOND are not supported by the date-only analyzer"
		return res
	}
	if len(r.byeaster) != 0 {
		res.Why = "BYEASTER is not supported by the date-only analyzer"
		return res
	}

	explicitUntil := !r.OrigOptions.Until.IsZero()
	if explicitUntil {
		if r.until.Location() != time.UTC {
			res.Why = "only UTC UNTIL is supported; got " + r.until.Location().String()
			return res
		}
		if hh, mm, ss := r.until.Clock(); hh != 0 || mm != 0 || ss != 0 || r.until.Nanosecond() != 0 {
			res.Why = "date-time UNTIL is not supported by the date-only analyzer"
			return res
		}
	}

	dy, dm, dd := r.dtstart.Date()
	startSerial := serial(dy, int(dm), dd)

	// Effective horizon. Explicit date UNTIL is used as-is. Without UNTIL
	// the library sets until = dtstart + MaxInt64 nanoseconds (~292 years);
	// the iterator stops emitting there, so that instant is the real
	// horizon. Round it up to the first whole date that can contain it.
	horizonTime := r.until
	if !explicitUntil {
		uy, um, ud := r.until.Date()
		horizonTime = time.Date(uy, um, ud, 0, 0, 0, 0, time.UTC)
		if hh, mm, ss := r.until.Clock(); hh != 0 || mm != 0 || ss != 0 || r.until.Nanosecond() != 0 {
			horizonTime = horizonTime.AddDate(0, 0, 1)
		}
	}
	hy, hm, hd := horizonTime.Date()
	horizon := serial(hy, int(hm), hd)

	if explicitUntil && horizon < startSerial {
		res.Status = EmptyProven
		res.Bounded = true
		res.Decisive = []DecisiveRule{{
			Item: "UNTIL=" + r.until.Format(DateFormat),
			Why:  "UNTIL precedes DTSTART, so no occurrence can be emitted",
		}}
		res.Why = "UNTIL is before DTSTART"
		return res
	}

	// COUNT=0 (or unset, or a negative value normalized to 0) means
	// unlimited in dateutil semantics; COUNT>0 merely truncates output and
	// cannot turn a non-empty rule empty, which is recorded in CountLimited.

	// Scan exactly one combined Gregorian(400y)/INTERVAL phase cycle,
	// ignoring the horizon: period 0 is clipped at DTSTART, then every
	// distinct phase state is visited once. The state before each period
	// repeats after lcm(INTERVAL,cycle), so this finite scan is exact and
	// decides mathematical emptiness even if the library's ~292-year
	// horizon would truncate it.
	phaseStates := a.phaseStates()
	witness, exhausted, _ := a.firstWitness(serial(9999, 12, 31), phaseStates+1)
	if exhausted {
		res.Status = EmptyUnknown
		res.BudgetExhausted = true
		res.Why = fmt.Sprintf("work budget of %d day evaluations exhausted at %d", a.budget, a.spent)
		return res
	}
	if !witness.IsZero() {
		if witness.After(horizonTime) {
			// Mathematically non-empty, but the sole/first survivor lies
			// beyond the iterator's effective horizon: the library emits
			// nothing. Report bounded emptiness, not a reachable witness.
			res.Status = EmptyProven
			res.Bounded = true
			decisive, dex := a.decisiveItems(true)
			if dex {
				res.Status = EmptyUnknown
				res.BudgetExhausted = true
				res.Bounded = false
				res.Why = fmt.Sprintf("work budget of %d day evaluations exhausted while isolating decisive rules at %d", a.budget, a.spent)
				return res
			}
			res.Decisive = decisive
			res.Why = fmt.Sprintf("the rule is non-empty in theory (first surviving candidate %s), but it lies beyond the effective horizon (%s) where the iterator stops",
				witness.Format(DateFormat), formatSerial(horizon))
			return res
		}
		res.Status = HasWitness
		res.Witness = witness
		res.Why = "a candidate surviving every filter exists; it is the first such date and matches the Iterator output"
		return res
	}

	// No witness over the complete finite phase cycle: mathematically empty.
	res.Status = EmptyProven
	decisive, dex := a.decisiveItems(true)
	if dex {
		res.Status = EmptyUnknown
		res.BudgetExhausted = true
		res.Bounded = false
		res.Why = fmt.Sprintf("work budget of %d day evaluations exhausted while isolating decisive rules at %d", a.budget, a.spent)
		return res
	}
	res.Decisive = decisive
	res.Why = fmt.Sprintf("no occurrence exists in any of the %d DTSTART/INTERVAL phase states of the Gregorian 400-year cycle",
		phaseStates)
	return res
}

// phaseStates returns the number of distinct DTSTART/INTERVAL phase states
// within one Gregorian 400-year cycle.
func (a *emptyAnalyzer) phaseStates() int {
	switch a.r.freq {
	case YEARLY:
		return 400 / gcd(a.r.interval, 400)
	case MONTHLY:
		return 4800 / gcd(a.r.interval, 4800) // 4800 months in 400 years
	case WEEKLY:
		return 20871 / gcd(a.r.interval, 20871) // 146097 days / 7
	default: // DAILY
		return 146097 / gcd(a.r.interval, 146097)
	}
}

// emptyCursor tracks the start of the current iteration period using the same
// bookkeeping as rIterator.
type emptyCursor struct {
	year    int
	month   int
	day     int
	weekday int
}

// firstWitness scans iteration periods chronologically and returns the first
// surviving occurrence on/after DTSTART and on/before upper. The scan examines
// at most maxPeriods periods, and returns the number of periods examined.
//
// Period 0 is clipped at DTSTART exactly like the iterator's first period.
// A caller compares the returned count with maxPeriods to tell whether the
// horizon cut the phase cycle short.
func (a *emptyAnalyzer) firstWitness(upper int, maxPeriods int) (w time.Time, exhausted bool, periods int) {
	r := a.r
	cur := emptyCursor{
		year:    r.dtstart.Year(),
		month:   int(r.dtstart.Month()),
		day:     r.dtstart.Day(),
		weekday: toPyWeekday(r.dtstart.Weekday()),
	}
	ii := iterInfo{rrule: r}
	ii.rebuild(cur.year, time.Month(cur.month))
	startSerial := serial(cur.year, cur.month, cur.day)
	maxSerial := serial(30000, 12, 31)

	for k := 0; k < maxPeriods; k++ {
		periodStart := serial(cur.year, cur.month, cur.day)
		if periodStart > upper || periodStart > maxSerial {
			return time.Time{}, false, k
		}
		// rebuild() populates yearlen/mask slices used by calcDaySet and the
		// filters; it must run before each period.
		ii.rebuild(cur.year, time.Month(cur.month))
		setStart, setEnd := ii.calcDaySet(r.freq, cur.year, time.Month(cur.month), cur.day)

		survivors, ex := a.collectSurvivors(&ii, setStart, setEnd)
		if ex {
			return time.Time{}, true, k
		}

		lo := setStart
		if k == 0 {
			lo = startSerial
		}
		var cand []int
		if len(r.bysetpos) != 0 {
			cand = a.applySetpos(survivors)
		} else {
			for _, z := range survivors {
				if z >= lo && z <= upper {
					cand = append(cand, z)
				}
			}
		}
		for _, z := range cand {
			if z < lo {
				continue
			}
			if z > upper {
				break
			}
			return serialTime(z, r.dtstart.Location()), false, k + 1
		}

		a.advance(&cur, &ii)
		if cur.year > MAXYEAR {
			// YEARLY iteration beyond year 9999: keep scanning for the
			// mathematical proof (serial/time arithmetic supports this),
			// since a phase cycle may extend past the library's year limit.
			if r.freq != YEARLY || cur.year > 30000 {
				return time.Time{}, false, k + 1
			}
		}
	}
	return time.Time{}, false, maxPeriods
}

// advance moves the cursor forward by INTERVAL periods, reproducing the
// post-output steps in rIterator.generate().
func (a *emptyAnalyzer) advance(cur *emptyCursor, ii *iterInfo) {
	r := a.r
	fixday := false
	switch r.freq {
	case YEARLY:
		cur.year += r.interval
		if cur.year <= 30000 {
			ii.rebuild(cur.year, time.Month(cur.month))
		}
		return
	case MONTHLY:
		cur.month += r.interval
		if cur.month > 12 {
			div, mod := divmod(cur.month, 12)
			cur.month = mod
			cur.year += div
			if cur.month == 0 {
				cur.month = 12
				cur.year--
			}
			if cur.year > MAXYEAR {
				return
			}
		}
		ii.rebuild(cur.year, time.Month(cur.month))
		return
	case WEEKLY:
		if r.wkst > cur.weekday {
			cur.day += -(cur.weekday + 1 + (6 - r.wkst)) + r.interval*7
		} else {
			cur.day += -(cur.weekday - r.wkst) + r.interval*7
		}
		cur.weekday = r.wkst
		fixday = true
	case DAILY:
		cur.day += r.interval
		fixday = true
	}
	if fixday && cur.day > 28 {
		dim := daysIn(time.Month(cur.month), cur.year)
		if cur.day > dim {
			for cur.day > dim {
				cur.day -= dim
				cur.month++
				if cur.month == 13 {
					cur.month = 1
					cur.year++
					if cur.year > MAXYEAR {
						return
					}
				}
				dim = daysIn(time.Month(cur.month), cur.year)
			}
			ii.rebuild(cur.year, time.Month(cur.month))
		}
	}
}

// collectSurvivors runs the exact filter chain from rIterator.generate over
// the period's dayset, returning surviving absolute serial day numbers.
func (a *emptyAnalyzer) collectSurvivors(ii *iterInfo, setStart, setEnd int) ([]int, bool) {
	r := a.r
	n := setEnd - setStart
	if n <= 0 {
		return nil, false
	}
	if !a.charge(n) {
		return nil, true
	}
	out := make([]int, 0, n)
	for i := setStart; i < setEnd; i++ {
		if len(r.bymonth) != 0 && !contains(r.bymonth, ii.mmask[i]) ||
			len(r.byweekno) != 0 && ii.wnomask[i] == 0 ||
			len(r.byweekday) != 0 && !contains(r.byweekday, ii.wdaymask[i]) ||
			len(ii.nwdaymask) != 0 && ii.nwdaymask[i] == 0 ||
			len(r.byeaster) != 0 && ii.eastermask[i] == 0 ||
			(len(r.bymonthday) != 0 || len(r.bynmonthday) != 0) &&
				!contains(r.bymonthday, ii.mdaymask[i]) &&
				!contains(r.bynmonthday, ii.nmdaymask[i]) ||
			len(r.byyearday) != 0 &&
				(i < ii.yearlen &&
					!contains(r.byyearday, i+1) &&
					!contains(r.byyearday, -ii.yearlen+i) ||
					i >= ii.yearlen &&
						!contains(r.byyearday, i+1-ii.yearlen) &&
						!contains(r.byyearday, -ii.nextyearlen+i-ii.yearlen)) {
			continue
		}
		yy, mm, dd := ii.firstyday.AddDate(0, 0, i).Date()
		out = append(out, serial(yy, int(mm), dd))
	}
	return out, false
}

// applySetpos mirrors the BYSETPOS selection in rIterator.generate for the
// date-only case, where the timeset has exactly one entry (timepos always 0).
func (a *emptyAnalyzer) applySetpos(survivors []int) []int {
	r := a.r
	if len(survivors) == 0 {
		return nil
	}
	var cand []int
	seen := map[int]bool{}
	for _, pos := range r.bysetpos {
		daypos := pos - 1
		if pos < 0 {
			daypos = pos
		}
		idx := daypos
		if idx < 0 {
			idx += len(survivors)
		}
		if idx < 0 || idx >= len(survivors) {
			continue
		}
		if z := survivors[idx]; !seen[z] {
			seen[z] = true
			cand = append(cand, z)
		}
	}
	// BYSETPOS positions may arrive out of order; emit chronologically.
	sortInts(cand)
	return cand
}

func sortInts(s []int) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

// serial converts a proleptic-Gregorian date to a serial day number
// (Rata Die; 1970-01-01 maps to 719162). Algorithm by Howard Hinnant.
func serial(year, month, day int) int {
	y := year
	if month <= 2 {
		y--
	}
	era := y / 400
	if y < 0 {
		era = (y - 399) / 400
	}
	yoe := y - era*400
	adj := 9
	if month > 2 {
		adj = -3
	}
	doy := (153*(month+adj)+2)/5 + day - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

// civil is the inverse of serial.
func civil(z int) (year, month, day int) {
	zz := z + 719468
	era := zz / 146097
	if zz < 0 {
		era = (zz - 146096) / 146097
	}
	doe := zz - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	day = doy - (153*mp+2)/5 + 1
	month = mp + 3
	year = y
	if month > 12 {
		month -= 12
	}
	if month <= 2 {
		year++
	}
	return year, month, day
}

func serialTime(z int, loc *time.Location) time.Time {
	y, m, d := civil(z)
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, loc)
}

func formatSerial(z int) string {
	return serialTime(z, time.UTC).Format(DateFormat)
}

// filterGroup identifies a user-supplied date filter relaxable in ablation.
type filterGroup struct {
	token string
	clear func(*ROption)
}

// presentedGroups lists only explicitly supplied filters. Filters implied
// solely by DTSTART (the library's defaults) are excluded because they alone
// never force a contradiction.
func (a *emptyAnalyzer) presentedGroups() []filterGroup {
	o := a.r.OrigOptions
	var groups []filterGroup
	add := func(tok string, clear func(*ROption)) {
		groups = append(groups, filterGroup{token: tok, clear: clear})
	}
	if len(o.Bymonth) != 0 {
		add(intsToken("BYMONTH", o.Bymonth), func(o *ROption) { o.Bymonth = nil })
	}
	if len(o.Bymonthday) != 0 {
		add(intsToken("BYMONTHDAY", o.Bymonthday), func(o *ROption) { o.Bymonthday = nil })
	}
	if len(o.Byyearday) != 0 {
		add(intsToken("BYYEARDAY", o.Byyearday), func(o *ROption) { o.Byyearday = nil })
	}
	if len(o.Byweekno) != 0 {
		add(intsToken("BYWEEKNO", o.Byweekno), func(o *ROption) { o.Byweekno = nil })
	}
	if len(o.Byweekday) != 0 {
		add(weekdaysToken(o.Byweekday), func(o *ROption) { o.Byweekday = nil })
	}
	if len(o.Bysetpos) != 0 {
		add(intsToken("BYSETPOS", o.Bysetpos), func(o *ROption) { o.Bysetpos = nil })
	}
	return groups
}

func intsToken(name string, values []int) string {
	out := name + "="
	for i, v := range values {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("%d", v)
	}
	return out
}

func weekdaysToken(days []Weekday) string {
	out := "BYDAY="
	for i, w := range days {
		if i > 0 {
			out += ","
		}
		out += w.String()
	}
	return out
}

// trialRRule builds a rule from mutated options for an ablation trial.
func trialRRule(base *RRule, mutate func(*ROption)) *RRule {
	o := base.OrigOptions
	mutate(&o)
	r := buildRRule(o)
	return &r
}

// decisiveItems isolates a minimal conjunction of rule components forcing the
// emptiness, via ablation: a component is decisive when relaxing it (alone)
// produces a witness over a full phase cycle. The second result signals that
// the budget was exhausted.
func (a *emptyAnalyzer) decisiveItems(completeCycle bool) ([]DecisiveRule, bool) {
	r := a.r

	// Greedy elimination over presented filters: repeatedly try dropping a
	// group from the current conjunction; if the rule stays empty the group
	// was not necessary and stays dropped. What remains is irreducible.
	groups := a.presentedGroups()
	kept := append([]filterGroup(nil), groups...)
	var decisive []DecisiveRule

	for _, candidate := range groups {
		trial := trialRRule(r, func(o *ROption) {
			for _, g := range kept {
				if g.token == candidate.token {
					g.clear(o)
				}
			}
		})
		ta := &emptyAnalyzer{r: trial, budget: a.budget, spent: a.spent}
		wit, exhausted, _ := ta.firstWitness(serial(9999, 12, 31), ta.phaseStates()+1)
		a.spent = ta.spent
		if exhausted {
			return nil, true
		}
		if !wit.IsZero() {
			decisive = append(decisive, DecisiveRule{
				Item:            candidate.token,
				Why:             "relaxing this filter alone yields an occurrence, so it is required for the empty intersection",
				RemovingWitness: wit,
			})
			continue
		}
		// Not necessary: remove it from the irreducible conjunction.
		next := kept[:0]
		for _, g := range kept {
			if g.token != candidate.token {
				next = append(next, g)
			}
		}
		kept = next
	}

	// INTERVAL phase: relax to INTERVAL=1 while keeping every filter.
	if r.interval > 1 {
		trial := trialRRule(r, func(o *ROption) { o.Interval = 1 })
		ta := &emptyAnalyzer{r: trial, budget: a.budget, spent: a.spent}
		wit, exhausted, _ := ta.firstWitness(serial(9999, 12, 31), ta.phaseStates()+1)
		a.spent = ta.spent
		if exhausted {
			return nil, true
		}
		if !wit.IsZero() {
			decisive = append(decisive, DecisiveRule{
				Item:            fmt.Sprintf("INTERVAL=%d", r.interval),
				Why:             fmt.Sprintf("the DTSTART/INTERVAL phase visits %d of the Gregorian states; INTERVAL=1 reaches an occurrence", a.phaseStates()),
				RemovingWitness: wit,
			})
		}
	}

	// Explicit UNTIL: relax it entirely.
	explicitUntil := !r.OrigOptions.Until.IsZero()
	if explicitUntil {
		trial := trialRRule(r, func(o *ROption) { o.Until = time.Time{} })
		ta := &emptyAnalyzer{r: trial, budget: a.budget, spent: a.spent}
		wit, exhausted, _ := ta.firstWitness(serial(9999, 12, 31), ta.phaseStates()+1)
		a.spent = ta.spent
		if exhausted {
			return nil, true
		}
		if !wit.IsZero() {
			decisive = append(decisive, DecisiveRule{
				Item:            "UNTIL=" + r.until.Format(DateFormat),
				Why:             "UNTIL cuts off before the first surviving candidate",
				RemovingWitness: wit,
			})
		}
	}

	// Fallback: name the boundary if ablation could not isolate a component
	// (this happens for horizon-bounded emptiness with no single culprit).
	if len(decisive) == 0 {
		if explicitUntil {
			decisive = append(decisive, DecisiveRule{
				Item: "UNTIL=" + r.until.Format(DateFormat),
				Why:  "no candidate survives before UNTIL",
			})
		} else if completeCycle {
			decisive = append(decisive, DecisiveRule{
				Item: "FREQ=" + r.freq.String(),
				Why:  "the filter conjunction admits no date in any visited phase state",
			})
		} else {
			decisive = append(decisive, DecisiveRule{
				Item: "FREQ=" + r.freq.String(),
				Why:  "no candidate survives in any visited period within the effective horizon",
			})
		}
	}
	return decisive, false
}
