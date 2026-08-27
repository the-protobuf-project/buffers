package bufir

import (
	"fmt"
	"sort"
	"strings"
)

// diag.go carries recoverable problems out of the build, with a per-rule
// severity the caller sets.
//
// The severity spec is spelled exactly like protokit.Options.Strict, and for the
// same reason a plugin has one at all: whether an unreserved field-number gap is
// a warning or a build failure is a property of the repository, not of the
// finding. A team that has never shipped a .capnp wants a warning. A team whose
// consumers are already deployed wants the build to stop.

// Severity is what the caller decided a rule means.
type Severity uint8

const (
	// SeverityWarn reports the problem and continues.
	SeverityWarn Severity = iota
	// SeverityError fails the build.
	SeverityError
)

// Rule names a class of recoverable problem. The set is closed, because the
// severity spec is written by hand and a typo'd rule name that silently matched
// nothing would look exactly like a rule that never fired.
type Rule string

const (
	// RuleOrdinal covers slots: an unreserved gap in field numbers, a pinned
	// ordinal that disagrees with the ledger, a value the ledger has never seen.
	//
	// This is the rule to set to error in a repository whose schemas are already
	// published. Everything under it is a statement about whether a consumer
	// compiled against last week's schema can still read this week's payload.
	RuleOrdinal Rule = "ordinal"

	// RuleLayout covers a message that asked for a packed layout and does not
	// qualify, or one whose inferred layout changed between runs.
	RuleLayout Rule = "layout"

	// RuleTarget covers a construct a specific target cannot express: a map in
	// ROS, a oneof in ROS, a 64-bit enum in FlatBuffers.
	RuleTarget Rule = "target"

	// RuleLint covers everything advisory: a missing doc comment on an exported
	// type, an unbounded sequence in a message destined for ROS.
	RuleLint Rule = "lint"
)

// allRules is the closed set, used to reject an unknown rule in a spec.
var allRules = []Rule{RuleOrdinal, RuleLayout, RuleTarget, RuleLint}

// Diagnostic is one recoverable problem.
type Diagnostic struct {
	// Rule is the class of problem, which decides the severity applied to it.
	Rule Rule

	// Node is what the problem is about, which is what makes a diagnostic
	// actionable: "sensors.v1.Sensor.rate_hz", not "a field".
	Node NodeID

	// Message is the problem, phrased as what is wrong rather than what rule
	// fired. The rule name is already carried separately.
	Message string

	// Hint is the fix, when there is a specific one. A diagnostic about an
	// unreserved gap can say exactly which `reserved` line to add, and one that
	// does is worth several that describe the situation.
	Hint string
}

// String renders the diagnostic for a plugin's stderr.
func (d Diagnostic) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s: %s", d.Rule, d.Node, d.Message)
	if d.Hint != "" {
		fmt.Fprintf(&b, "\n    fix: %s", d.Hint)
	}
	return b.String()
}

// Strictness resolves a rule to a severity.
type Strictness struct {
	// def applies to any rule the spec did not name.
	def Severity
	// byID holds the severities the spec named explicitly.
	byID map[Rule]Severity
}

// ParseStrict reads protokit's severity spec:
//
//	""                        every rule warns (the default)
//	"true"                    every rule is an error
//	"ordinal:error,lint:warn" per-rule severity, unnamed rules warn
//
// An unknown rule name is an error rather than a no-op. The spec is a hand-typed
// string in a buf.gen.yaml, which is the shape of configuration where "ordnial:
// error" would otherwise silently do nothing for a year.
func ParseStrict(spec string) (Strictness, error) {
	s := Strictness{def: SeverityWarn, byID: map[Rule]Severity{}}
	spec = strings.TrimSpace(spec)
	switch spec {
	case "":
		return s, nil
	case "true":
		s.def = SeverityError
		return s, nil
	case "false":
		return s, nil
	}

	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, level, ok := strings.Cut(part, ":")
		if !ok {
			return s, fmt.Errorf("strict: %q is not \"rule:severity\" (valid rules: %s)", part, ruleList())
		}
		rule := Rule(strings.TrimSpace(name))
		if !knownRule(rule) {
			return s, fmt.Errorf("strict: unknown rule %q (valid rules: %s)", rule, ruleList())
		}
		switch strings.TrimSpace(level) {
		case "error":
			s.byID[rule] = SeverityError
		case "warn":
			s.byID[rule] = SeverityWarn
		default:
			return s, fmt.Errorf("strict: %q is not a severity for rule %q (valid: error, warn)", level, rule)
		}
	}
	return s, nil
}

// Of returns the severity for a rule.
func (s Strictness) Of(r Rule) Severity {
	if got, ok := s.byID[r]; ok {
		return got
	}
	return s.def
}

// Partition splits diagnostics into those that fail the build and those that
// merely report, so a caller can print both and then decide whether to stop.
func (s Strictness) Partition(diags []Diagnostic) (errs, warns []Diagnostic) {
	for _, d := range diags {
		if s.Of(d.Rule) == SeverityError {
			errs = append(errs, d)
		} else {
			warns = append(warns, d)
		}
	}
	return errs, warns
}

// knownRule reports whether a rule name is one of the closed set.
func knownRule(r Rule) bool {
	for _, got := range allRules {
		if got == r {
			return true
		}
	}
	return false
}

// ruleList renders the valid rule names, for an error that tells the caller
// what they could have typed.
func ruleList() string {
	names := make([]string, len(allRules))
	for i, r := range allRules {
		names[i] = string(r)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
