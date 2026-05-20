package filter

import "github.com/gobwas/glob"

// filterSingle is the cheapest implementation: one exact string match.
type filterSingle struct {
	s string
}

func (f *filterSingle) Match(s string) bool {
	return f.s == s
}

// filterNoGlob handles "multiple exact patterns, no wildcards" via a
// map lookup — O(1) per Match regardless of how many patterns.
type filterNoGlob struct {
	m map[string]struct{}
}

func newFilterNoGlob(patterns []string) Filter {
	out := &filterNoGlob{m: make(map[string]struct{}, len(patterns))}
	for _, p := range patterns {
		out.m[p] = struct{}{}
	}
	return out
}

func (f *filterNoGlob) Match(s string) bool {
	_, ok := f.m[s]
	return ok
}

// filterGlobMultiple wraps several compiled glob.Globs and matches if
// any one of them matches. Compile-time work is amortized; per-match
// cost is len(set) glob evaluations in the worst case.
type filterGlobMultiple struct {
	set []glob.Glob
}

func newFilterGlobMultiple(patterns []string, separators ...rune) (Filter, error) {
	f := &filterGlobMultiple{set: make([]glob.Glob, 0, len(patterns))}
	for _, p := range patterns {
		g, err := glob.Compile(p, separators...)
		if err != nil {
			return nil, err
		}
		f.set = append(f.set, g)
	}
	return f, nil
}

func (f *filterGlobMultiple) Match(s string) bool {
	for _, g := range f.set {
		if g.Match(s) {
			return true
		}
	}
	return false
}
