package rrule

import (
	"fmt"
	"math"
	"math/bits"
	"sort"
	"strings"
	"time"
)

const (
	gregorianDays        = 146_097
	gregorianYears       = 400
	gregorianMonths      = 4_800
	gregorianWeeks       = 20_871
	defaultAnalyzeBudget = 500_000
)

type AnalyzeEmptyStatus int

const (
	// AnalyzeEmptyUnknown means the analyzer cannot prove either emptiness or existence within its supported semantics/budget.
	AnalyzeEmptyUnknown AnalyzeEmptyStatus = iota
	// AnalyzeEmptyConfirmed means the rule provably has no occurrence.
	AnalyzeEmptyConfirmed
	// AnalyzeEmptyNonEmpty means the rule has at least one iterator-visible occurrence.
	AnalyzeEmptyNonEmpty
)

// EmptyAnalysis is the read-only result of an RRULE emptiness analysis.
type EmptyAnalysis struct {
	Status          AnalyzeEmptyStatus
	Witness         time.Time
	DecisiveRules   []string
	BudgetExhausted bool
}

func (status AnalyzeEmptyStatus) String() string {
	switch status {
	case AnalyzeEmptyConfirmed:
		return "empty"
	case AnalyzeEmptyNonEmpty:
		return "non-empty"
	default:
		return "unknown"
	}
}

type emptyAnalyzer struct {
	r       *RRule
	budget  int
	charges int
}

type analyzeWindow struct {
	repStart        civilDate
	actualDay       int64
	actualAvailable bool
	startIndex      int
	length          int
	firstWindow     bool
}

type civilDate struct {
	year  int
	month time.Month
	day   int
}

// AnalyzeEmpty analyzes whether a date-only UTC rule can produce an occurrence.
// The optional budget limits the number of examined candidate days; exhaustion returns AnalyzeEmptyUnknown.
func (r *RRule) AnalyzeEmpty(budgets ...int) EmptyAnalysis {
	budget := defaultAnalyzeBudget
	if len(budgets) > 0 {
		budget = budgets[0]
	}
	if budget < 0 {
		return EmptyAnalysis{Status: AnalyzeEmptyUnknown}
	}

	analyzer := emptyAnalyzer{r: r, budget: budget}
	if !analyzer.supported() {
		return EmptyAnalysis{Status: AnalyzeEmptyUnknown}
	}

	first, stopped := analyzer.firstOccurrence()
	if analyzer.budgetExhausted() {
		return EmptyAnalysis{Status: AnalyzeEmptyUnknown, BudgetExhausted: true}
	}

	if first.found {
		return EmptyAnalysis{
			Status:  AnalyzeEmptyNonEmpty,
			Witness: first.witness,
		}
	}
	if stopped == stoppedMaxYear {
		return EmptyAnalysis{Status: AnalyzeEmptyUnknown}
	}

	analysis := EmptyAnalysis{Status: AnalyzeEmptyConfirmed}
	if analyzer.hasUntil() {
		analysis.DecisiveRules = analyzer.decisiveRules(true)
		if analyzer.budgetExhausted() {
			return EmptyAnalysis{Status: AnalyzeEmptyUnknown, BudgetExhausted: true}
		}
		return analysis
	}

	analysis.DecisiveRules = analyzer.decisiveRules(false)
	if analyzer.budgetExhausted() {
		return EmptyAnalysis{Status: AnalyzeEmptyUnknown, BudgetExhausted: true}
	}
	return analysis
}

type stopReason int

const (
	stoppedComplete stopReason = iota
	stoppedUntil
	stoppedMaxYear
)

type occurrenceResult struct {
	found   bool
	witness time.Time
}

type evalResult struct {
	witness     time.Time
	found       bool
	maxExceeded bool
}

func (a *emptyAnalyzer) supported() bool {
	if a.r.freq > DAILY {
		return false
	}
	if a.r.dtstart.Location() != time.UTC && a.r.dtstart.Location().String() != "UTC" {
		return false
	}
	if !a.r.until.IsZero() && a.r.until.Location() != time.UTC && a.r.until.Location().String() != "UTC" {
		return false
	}
	if len(a.r.OrigOptions.Byhour) != 0 || len(a.r.OrigOptions.Byminute) != 0 || len(a.r.OrigOptions.Bysecond) != 0 {
		return false
	}
	if a.r.dtstart.Hour() != 0 || a.r.dtstart.Minute() != 0 || a.r.dtstart.Second() != 0 || a.r.dtstart.Nanosecond() != 0 {
		return false
	}
	for _, offset := range a.r.byeaster {
		if offset < -115 || offset > 291 {
			return false
		}
	}
	for _, pos := range a.r.bysetpos {
		if pos == 0 || pos < -366 || pos > 366 {
			return false
		}
	}
	return a.r.dtstart.Year() >= 1
}

func (a *emptyAnalyzer) hasUntil() bool {
	return !a.r.OrigOptions.Until.IsZero()
}

func (a *emptyAnalyzer) charge(amount int) bool {
	if a.charges > a.budget-amount {
		a.charges = a.budget + 1
		return false
	}
	a.charges += amount
	return true
}

func (a *emptyAnalyzer) budgetExhausted() bool {
	return a.charges > a.budget
}

func analyzeTimeValue(t time.Time) string {
	return t.UTC().Format(DateTimeFormat)
}

func civilDateFromTime(t time.Time) civilDate {
	year, month, day := t.Date()
	return civilDate{year: year, month: month, day: day}
}

func daysFromCivil(date civilDate) int {
	year := int64(date.year)
	if date.month <= time.February {
		year--
	}
	era := 0
	if year >= 0 {
		era = int(year / 400)
	} else {
		era = int((year - 399) / 400)
	}
	yoe := int(year - int64(era)*400)
	month := int(date.month)
	doy := (153*(month+(ifMonth(month > 2, -3, 9)))+2)/5 + date.day - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*gregorianDays + doe - 719162
}

func ifMonth(condition bool, whenTrue, whenFalse int) int {
	if condition {
		return whenTrue
	}
	return whenFalse
}

func civilFromDays(days int64) civilDate {
	z := days + 719162
	era := 0
	if z >= 0 {
		era = int(z / gregorianDays)
	} else {
		era = int((z - gregorianDays + 1) / gregorianDays)
	}
	doe := int(z - int64(era)*gregorianDays)
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := int64(yoe) + int64(era)*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	day := doy - (153*mp+2)/5 + 1
	month := time.Month(mp + ifMonth(mp < 10, 3, -9))
	if month <= time.February {
		y++
	}
	return civilDate{year: int(y), month: month, day: day}
}

func gcd(left, right int) int {
	for right != 0 {
		left, right = right, left%right
	}
	return left
}

func analyzeInts(values []int) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprintf("%d", value)
	}
	return strings.Join(parts, ",")
}

func analyzeWeekdays(values []Weekday) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = value.String()
	}
	return strings.Join(parts, ",")
}

func (a *emptyAnalyzer) firstOccurrence() (occurrenceResult, stopReason) {
	dtstart := civilDateFromTime(a.r.dtstart)
	dtstartDay := int64(daysFromCivil(dtstart))
	untilDay := int64(math.MaxInt64)
	if a.hasUntil() {
		until := a.r.until.UTC()
		untilDate := time.Date(until.Year(), until.Month(), until.Day(), 0, 0, 0, 0, time.UTC)
		untilDay = int64(daysFromCivil(civilDateFromTime(untilDate)))
	}

	maxDay := int64(daysFromCivil(civilDate{year: MAXYEAR, month: time.December, day: 31}))
	periods := a.periodCount()
	beyondMax := false
	for index := 0; index < periods; index++ {
		window := a.windowAt(dtstart, index)
		if window.actualAvailable && window.actualDay > untilDay {
			return occurrenceResult{}, stoppedUntil
		}
		if !window.actualAvailable {
			beyondMax = true
		}

		result := a.evaluateWindow(window, dtstartDay, untilDay, maxDay)
		if result.found {
			return occurrenceResult{found: true, witness: result.witness}, stoppedComplete
		}
		beyondMax = beyondMax || result.maxExceeded
	}
	if beyondMax {
		return occurrenceResult{}, stoppedMaxYear
	}
	return occurrenceResult{}, stoppedComplete
}

func (a *emptyAnalyzer) periodCount() int {
	switch a.r.freq {
	case YEARLY:
		return gregorianYears / gcd(gregorianYears, a.r.interval)
	case MONTHLY:
		return gregorianMonths / gcd(gregorianMonths, a.r.interval)
	case WEEKLY:
		return 1 + gregorianWeeks/gcd(gregorianWeeks, a.r.interval)
	default:
		return gregorianDays / gcd(gregorianDays, a.r.interval)
	}
}

func (a *emptyAnalyzer) windowAt(dtstart civilDate, index int) analyzeWindow {
	switch a.r.freq {
	case YEARLY:
		return a.yearlyWindow(dtstart, index)
	case MONTHLY:
		return a.monthlyWindow(dtstart, index)
	case WEEKLY:
		return a.weeklyWindow(dtstart, index)
	default:
		return a.dailyWindow(dtstart, index)
	}
}

func (a *emptyAnalyzer) yearlyWindow(dtstart civilDate, index int) analyzeWindow {
	step := mulMod(index, a.r.interval, gregorianYears)
	repYear := positiveMod(dtstart.year+step-1, gregorianYears) + 1
	repStart := civilDate{year: repYear, month: time.January, day: 1}
	actualDay, available := a.yearlyActualDay(dtstart, index)
	return a.dateWindow(repStart, actualDay, available, 0, 365+isLeap(repYear), index == 0)
}

func (a *emptyAnalyzer) yearlyActualDay(dtstart civilDate, index int) (int64, bool) {
	step, ok := checkedMul(int64(index), int64(a.r.interval))
	if ok {
		if step <= math.MaxInt64-int64(dtstart.year) {
			year := dtstart.year + int(step)
			if year >= 1 && year <= MAXYEAR {
				return int64(daysFromCivil(civilDate{year: year, month: time.January, day: 1})), true
			}
		}
	}
	return math.MaxInt64, false
}

func (a *emptyAnalyzer) monthlyWindow(dtstart civilDate, index int) analyzeWindow {
	step := mulMod(index, a.r.interval, gregorianMonths)
	repMonthIndex := positiveMod((dtstart.year-1)*12+int(dtstart.month-1)+step, 12)
	repYearOffset := positiveMod((dtstart.year-1)*12+int(dtstart.month-1)+step, gregorianMonths) / 12
	repYear := repYearOffset + 1
	month := time.Month(repMonthIndex + 1)
	repStart := civilDate{year: repYear, month: month, day: 1}
	actualDay, available := a.monthlyActualDay(dtstart, index)
	return a.dateWindow(repStart, actualDay, available, 0, daysIn(month, repYear), index == 0)
}

func (a *emptyAnalyzer) monthlyActualDay(dtstart civilDate, index int) (int64, bool) {
	step, ok := checkedMul(int64(index), int64(a.r.interval))
	if !ok {
		return math.MaxInt64, false
	}
	base := int64((dtstart.year-1)*12 + int(dtstart.month-1))
	if step > math.MaxInt64-base {
		return math.MaxInt64, false
	}
	monthIndex := base + step
	year := int(monthIndex/12 + 1)
	month := time.Month(int(monthIndex%12 + 1))
	if year < 1 || year > MAXYEAR {
		return math.MaxInt64, false
	}
	return int64(daysFromCivil(civilDate{year: year, month: month, day: 1})), true
}

func (a *emptyAnalyzer) dailyWindow(dtstart civilDate, index int) analyzeWindow {
	step := mulMod(index, a.r.interval, gregorianDays)
	rep := civilFromDays(daysMod(int64(daysFromCivil(dtstart) + step)))
	actualDay, available := checkedAdd(int64(daysFromCivil(dtstart)), checkedMulOrMax(int64(index), int64(a.r.interval)))
	return a.singleDayWindow(rep, actualDay, available, index == 0)
}

func (a *emptyAnalyzer) weeklyWindow(dtstart civilDate, index int) analyzeWindow {
	dtstartWeekday := toPyWeekday(time.Date(dtstart.year, dtstart.month, dtstart.day, 0, 0, 0, 0, time.UTC).Weekday())
	back := pymod(dtstartWeekday-a.r.wkst, 7)
	firstAnchorOrdinal := daysFromCivil(dtstart) - back
	firstAnchor := civilFromDays(int64(firstAnchorOrdinal))
	stepDays, _ := checkedMul(int64(a.r.interval), 7)
	if index == 0 {
		startIndex := firstAnchorOrdinal - daysFromCivil(civilDate{year: firstAnchor.year, month: time.January, day: 1})
		return a.dateWindow(firstAnchor, int64(firstAnchorOrdinal), true, startIndex, 7-back, true)
	}
	repStep := mulMod(index, int(stepDays), gregorianDays)
	repAnchor := civilFromDays(int64(firstAnchorOrdinal) + daysMod(int64(repStep)))
	indexStep, stepOK := checkedMul(int64(index), stepDays)
	actualDay, available := checkedAdd(int64(firstAnchorOrdinal), indexStep)
	if !stepOK {
		available = false
	}
	startIndex := daysFromCivil(repAnchor) - daysFromCivil(civilDate{year: repAnchor.year, month: time.January, day: 1})
	return a.dateWindow(repAnchor, actualDay, available, startIndex, 7, false)
}

func (a *emptyAnalyzer) singleDayWindow(rep civilDate, actualDay int64, available, firstWindow bool) analyzeWindow {
	yearStart := civilDate{year: rep.year, month: time.January, day: 1}
	index := daysFromCivil(rep) - daysFromCivil(yearStart)
	return a.dateWindow(rep, actualDay, available, index, 1, firstWindow)
}

func (a *emptyAnalyzer) dateWindow(repStart civilDate, actualDay int64, available bool, startIndex, length int, firstWindow bool) analyzeWindow {
	return analyzeWindow{
		repStart:        repStart,
		actualDay:       actualDay,
		actualAvailable: available,
		startIndex:      startIndex,
		length:          length,
		firstWindow:     firstWindow,
	}
}

func positiveMod(value, modulus int) int {
	result := value % modulus
	if result < 0 {
		result += modulus
	}
	return result
}

func daysMod(days int64) int64 {
	return ((days % gregorianDays) + gregorianDays) % gregorianDays
}

func mulMod(left, right, modulus int) int {
	hi, lo := bits.Mul64(uint64(left), uint64(right))
	_, rem := bits.Div64(hi, lo, uint64(modulus))
	return int(rem)
}

func checkedMul(left, right int64) (int64, bool) {
	if left != 0 && right != 0 {
		result := left * right
		if result/right != left {
			return math.MaxInt64, false
		}
	}
	return left * right, true
}

func checkedMulOrMax(left, right int64) int64 {
	result, ok := checkedMul(left, right)
	if !ok {
		return math.MaxInt64
	}
	return result
}

func checkedAdd(base, delta int64) (int64, bool) {
	if delta > 0 && base > math.MaxInt64-delta || delta < 0 && base < math.MinInt64-delta {
		return math.MaxInt64, false
	}
	return base + delta, true
}

func (a *emptyAnalyzer) evaluateWindow(window analyzeWindow, dtstartDay, untilDay, maxDay int64) evalResult {
	if !a.charge(window.length) {
		return evalResult{}
	}

	info := iterInfo{rrule: a.r}
	info.rebuild(window.repStart.year, window.repStart.month)
	matched := make([]int, 0, window.length)

	for offset := 0; offset < window.length; offset++ {
		dayIndex := window.startIndex + offset
		if window.firstWindow && offset < int(dtstartDay-window.actualDay) {
			continue
		}
		if window.actualAvailable && a.hasUntil() && window.actualDay+int64(offset) > untilDay {
			continue
		}
		if !info.dayMatches(a.r, dayIndex) {
			continue
		}
		matched = append(matched, offset)
	}

	selected := matched
	if len(a.r.bysetpos) != 0 {
		selected = selected[:0]
		for _, pos := range a.r.bysetpos {
			daypos := pos - 1
			if pos < 0 {
				daypos = pos
			}
			index, err := pySubscript(matched, daypos)
			if err != nil {
				continue
			}
			if !containsInt(selected, index) {
				selected = append(selected, index)
			}
		}
		sort.Ints(selected)
	}

	result := evalResult{}
	for _, offset := range selected {
		actualDay := window.actualDay + int64(offset)
		if !window.actualAvailable || actualDay > maxDay {
			result.maxExceeded = true
			continue
		}
		date := civilFromDays(actualDay)
		witness := time.Date(date.year, date.month, date.day,
			a.r.dtstart.Hour(), a.r.dtstart.Minute(), a.r.dtstart.Second(),
			a.r.dtstart.Nanosecond(), time.UTC)
		if witness.Before(a.r.dtstart) {
			continue
		}
		if a.hasUntil() && witness.After(a.r.until) {
			continue
		}
		return evalResult{found: true, witness: witness}
	}
	return result
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (a *emptyAnalyzer) decisiveRules(bounded bool) []string {
	rules := []string{"FREQ=" + a.r.freq.String()}
	var firstWithoutInterval occurrenceResult
	if !bounded && a.r.interval != 1 {
		probe := *a.r
		probe.interval = 1
		probe.OrigOptions.Interval = 1
		probe.Options.Interval = 1
		probeAnalyzer := emptyAnalyzer{r: &probe, budget: a.remainingBudget()}
		var stop stopReason
		firstWithoutInterval, stop = probeAnalyzer.firstOccurrence()
		a.charges += probeAnalyzer.charges
		if a.budgetExhausted() {
			return nil
		}
		if firstWithoutInterval.found && stop != stoppedMaxYear {
			rules = append(rules, fmt.Sprintf("INTERVAL=%d", a.r.interval))
			if a.dtstartPhaseDecides(firstWithoutInterval.witness) {
				rules = append(rules, "DTSTART="+analyzeTimeValue(a.r.dtstart))
			}
		}
	} else if bounded {
		rules = append(rules, "DTSTART="+analyzeTimeValue(a.r.dtstart), "UNTIL="+analyzeTimeValue(a.r.until))
	}
	groups := a.removableDateGroups()
	if a.r.OrigOptions.Wkst != MO {
		groups = append([]dateRuleGroup{{
			name: "WKST=" + a.r.OrigOptions.Wkst.String(),
			clear: func(rule *RRule) {
				rule.wkst = 0
				rule.OrigOptions.Wkst = MO
				rule.Options.Wkst = MO
			},
		}}, groups...)
	}
	for _, group := range groups {
		probe := *a.r
		group.clear(&probe)
		if !bounded {
			probe.OrigOptions.Until = time.Time{}
			probe.Options.Until = time.Time{}
			probe.until = buildRRule(probe.OrigOptions).until
		}
		probeAnalyzer := emptyAnalyzer{r: &probe, budget: a.remainingBudget()}
		first, stop := probeAnalyzer.firstOccurrence()
		a.charges += probeAnalyzer.charges
		if a.budgetExhausted() {
			return nil
		}
		if !first.found && stop != stoppedMaxYear {
			continue
		}
		rules = append(rules, group.name)
	}
	sort.Strings(rules)
	return rules
}

func (a *emptyAnalyzer) dtstartPhaseDecides(witness time.Time) bool {
	probe := *a.r
	year, month, day := witness.Date()
	switch a.r.freq {
	case YEARLY:
		probe.dtstart = time.Date(year, a.r.dtstart.Month(), a.r.dtstart.Day(), 0, 0, 0, 0, time.UTC)
	case MONTHLY:
		probe.dtstart = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	case WEEKLY:
		probe.dtstart = time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	default:
		probe.dtstart = time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}
	probe.OrigOptions.Dtstart = probe.dtstart
	probe.Options.Dtstart = probe.dtstart
	probeAnalyzer := emptyAnalyzer{r: &probe, budget: a.remainingBudget()}
	first, stop := probeAnalyzer.firstOccurrence()
	a.charges += probeAnalyzer.charges
	return first.found && stop != stoppedMaxYear
}

type dateRuleGroup struct {
	name  string
	clear func(*RRule)
}

func (a *emptyAnalyzer) removableDateGroups() []dateRuleGroup {
	options := a.r.OrigOptions
	groups := make([]dateRuleGroup, 0)
	if a.r.freq == YEARLY && len(options.Bymonth) == 0 && len(a.r.bymonth) != 0 {
		name := "BYMONTH=" + analyzeInts(a.r.bymonth)
		groups = append(groups, dateRuleGroup{name: name, clear: func(rule *RRule) { rule.bymonth = nil }})
	}
	if (a.r.freq == YEARLY || a.r.freq == MONTHLY) && len(options.Bymonthday) == 0 &&
		(len(a.r.bymonthday) != 0 || len(a.r.bynmonthday) != 0) {
		values := append(append([]int{}, a.r.bymonthday...), a.r.bynmonthday...)
		groups = append(groups, dateRuleGroup{
			name:  "BYMONTHDAY=" + analyzeInts(values),
			clear: func(rule *RRule) { rule.bymonthday, rule.bynmonthday = nil, nil },
		})
	}
	if a.r.freq == WEEKLY && len(options.Byweekday) == 0 && len(a.r.byweekday) != 0 {
		weekdays := make([]Weekday, 0, len(a.r.byweekday))
		for _, weekday := range a.r.byweekday {
			weekdays = append(weekdays, Weekday{weekday: weekday})
		}
		groups = append(groups, dateRuleGroup{
			name:  "BYDAY=" + analyzeWeekdays(weekdays),
			clear: func(rule *RRule) { rule.byweekday = nil },
		})
	}
	if len(options.Bysetpos) != 0 {
		groups = append(groups, dateRuleGroup{
			name:  "BYSETPOS=" + analyzeInts(options.Bysetpos),
			clear: func(rule *RRule) { rule.bysetpos, rule.OrigOptions.Bysetpos, rule.Options.Bysetpos = nil, nil, nil },
		})
	}
	if len(options.Bymonth) != 0 {
		groups = append(groups, dateRuleGroup{
			name:  "BYMONTH=" + analyzeInts(options.Bymonth),
			clear: func(rule *RRule) { rule.bymonth, rule.OrigOptions.Bymonth, rule.Options.Bymonth = nil, nil, nil },
		})
	}
	if len(options.Bymonthday) != 0 {
		groups = append(groups, dateRuleGroup{
			name: "BYMONTHDAY=" + analyzeInts(options.Bymonthday),
			clear: func(rule *RRule) {
				rule.bymonthday, rule.bynmonthday, rule.OrigOptions.Bymonthday = nil, nil, nil
				rule.Options.Bymonthday = nil
			},
		})
	}
	if len(options.Byyearday) != 0 {
		groups = append(groups, dateRuleGroup{
			name:  "BYYEARDAY=" + analyzeInts(options.Byyearday),
			clear: func(rule *RRule) { rule.byyearday, rule.OrigOptions.Byyearday, rule.Options.Byyearday = nil, nil, nil },
		})
	}
	if len(options.Byweekno) != 0 {
		groups = append(groups, dateRuleGroup{
			name:  "BYWEEKNO=" + analyzeInts(options.Byweekno),
			clear: func(rule *RRule) { rule.byweekno, rule.OrigOptions.Byweekno, rule.Options.Byweekno = nil, nil, nil },
		})
	}
	if len(options.Byweekday) != 0 {
		groups = append(groups, dateRuleGroup{
			name: "BYDAY=" + analyzeWeekdays(options.Byweekday),
			clear: func(rule *RRule) {
				rule.byweekday, rule.bynweekday, rule.OrigOptions.Byweekday = nil, nil, nil
				rule.Options.Byweekday = nil
			},
		})
	}
	if len(options.Byeaster) != 0 {
		groups = append(groups, dateRuleGroup{
			name:  "BYEASTER=" + analyzeInts(options.Byeaster),
			clear: func(rule *RRule) { rule.byeaster, rule.OrigOptions.Byeaster, rule.Options.Byeaster = nil, nil, nil },
		})
	}
	return groups
}

func (a *emptyAnalyzer) remainingBudget() int {
	remaining := a.budget - a.charges
	if remaining < 0 {
		return 0
	}
	return remaining
}
