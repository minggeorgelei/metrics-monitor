// Package filter compiles a list of patterns into a fast Matcher for
// the namepass / namedrop / tagpass / tagdrop / fieldinclude /
// fieldexclude / taginclude / tagexclude rules in models.Filter.
//
// Mirrors influxdata/telegraf's `filter` package, scoped to what we
// need. The three Filter implementations behind Compile() are picked
// from cheapest to most general:
//
//	single pattern, no wildcards  → string == string (filterSingle)
//	multiple patterns, no wildcards → map lookup     (filterNoGlob)
//	single glob                    → gobwas/glob handle
//	multiple globs                 → slice of glob handles
//
// Compile returns nil for an empty pattern list — callers should treat
// nil as "no filter" (match anything).
package filter

import (
	"strings"

	"github.com/gobwas/glob"
)

// Filter matches a single string against the compiled pattern list.
type Filter interface {
	Match(string) bool
}

// Compile builds a Filter from a list of patterns. Optional separator
// runes restrict `*` from crossing those characters (useful for nested
// keys like cpu.usage.idle where `cpu.*` should match cpu.usage but
// not cpu.usage.idle when '.' is a separator).
//
// Returns nil with no error when the pattern list is empty.
func Compile(patterns []string, separators ...rune) (Filter, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	hasWildcards := len(separators) > 0
	for _, p := range patterns {
		if strings.ContainsAny(p, "*?[") {
			hasWildcards = true
			break
		}
	}

	switch {
	case !hasWildcards && len(patterns) == 1:
		return &filterSingle{s: patterns[0]}, nil
	case !hasWildcards:
		return newFilterNoGlob(patterns), nil
	case len(patterns) == 1:
		return glob.Compile(patterns[0], separators...)
	default:
		return newFilterGlobMultiple(patterns, separators...)
	}
}

// MustCompile is the Compile variant for tests and static config that
// panics on a malformed pattern.
func MustCompile(patterns []string, separators ...rune) Filter {
	f, err := Compile(patterns, separators...)
	if err != nil {
		panic(err)
	}
	return f
}

// ShouldPassFilters applies the standard include/exclude rule pair to
// a single key: include nil means "everything passes the include
// check"; exclude nil means "nothing is excluded".
func ShouldPassFilters(include, exclude Filter, key string) bool {
	if include != nil && !include.Match(key) {
		return false
	}
	if exclude != nil && exclude.Match(key) {
		return false
	}
	return true
}
