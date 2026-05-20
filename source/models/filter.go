package models

import (
	"fmt"

	"github.com/minggeorgelei/metrics-monitor/source/core"
	"github.com/minggeorgelei/metrics-monitor/source/filter"
)

// TagFilter matches metric tag VALUES under a specific tag KEY.
// Example: TagFilter{Name: "cpu", Values: []string{"cpu-total"}} —
// the metric passes if it has a tag "cpu" with value "cpu-total".
type TagFilter struct {
	Name   string
	Values []string

	matcher filter.Filter
}

func (tf *TagFilter) compile() error {
	m, err := filter.Compile(tf.Values)
	if err != nil {
		return err
	}
	tf.matcher = m
	return nil
}

// Filter is a per-plugin filtering policy. It bundles the standard
// six selectors (namepass/drop, tagpass/drop) and four modifiers
// (fieldinclude/exclude, taginclude/exclude) into one type so each
// running wrapper holds one Filter regardless of how many rules the
// user configured.
//
// Lifecycle:
//
//	1. config.Load populates the slice fields from TOML
//	2. config.Load calls Compile() — this builds the underlying
//	   filter.Filter handles and toggles isActive/selectActive/
//	   modifyActive flags. After Compile() returns, the Filter is
//	   ready for use from any goroutine (no further mutation).
//	3. Running wrappers call Select() (returns "should this metric
//	   pass through?") and Modify() (removes fields/tags per the
//	   include/exclude rules).
//
// Empty Filter is a no-op — Select returns true unconditionally,
// Modify does nothing. IsActive() is the cheap pre-check callers
// should use to skip the work entirely.
//
// Mirrors influxdata/telegraf's models.Filter, minus the CEL-based
// MetricPass — see filter package doc for the omission rationale.
type Filter struct {
	NamePass []string
	NameDrop []string

	TagPassFilters []TagFilter
	TagDropFilters []TagFilter

	FieldInclude []string
	FieldExclude []string
	TagInclude   []string
	TagExclude   []string

	// --- compiled state (populated by Compile()) ---

	namePassFilter filter.Filter
	nameDropFilter filter.Filter

	fieldIncludeFilter filter.Filter
	fieldExcludeFilter filter.Filter
	tagIncludeFilter   filter.Filter
	tagExcludeFilter   filter.Filter

	selectActive bool
	modifyActive bool
	isActive     bool
}

// Compile prepares the filter for use. Idempotent — safe to call more
// than once during config loading.
func (f *Filter) Compile() error {
	f.selectActive = len(f.NamePass) > 0 ||
		len(f.NameDrop) > 0 ||
		len(f.TagPassFilters) > 0 ||
		len(f.TagDropFilters) > 0
	f.modifyActive = len(f.FieldInclude) > 0 ||
		len(f.FieldExclude) > 0 ||
		len(f.TagInclude) > 0 ||
		len(f.TagExclude) > 0
	f.isActive = f.selectActive || f.modifyActive

	if !f.isActive {
		return nil
	}

	var err error
	if f.selectActive {
		f.namePassFilter, err = filter.Compile(f.NamePass)
		if err != nil {
			return fmt.Errorf("compile namepass: %w", err)
		}
		f.nameDropFilter, err = filter.Compile(f.NameDrop)
		if err != nil {
			return fmt.Errorf("compile namedrop: %w", err)
		}
		for i := range f.TagPassFilters {
			if err := f.TagPassFilters[i].compile(); err != nil {
				return fmt.Errorf("compile tagpass[%d]: %w", i, err)
			}
		}
		for i := range f.TagDropFilters {
			if err := f.TagDropFilters[i].compile(); err != nil {
				return fmt.Errorf("compile tagdrop[%d]: %w", i, err)
			}
		}
	}

	if f.modifyActive {
		f.fieldIncludeFilter, err = filter.Compile(f.FieldInclude)
		if err != nil {
			return fmt.Errorf("compile fieldinclude: %w", err)
		}
		f.fieldExcludeFilter, err = filter.Compile(f.FieldExclude)
		if err != nil {
			return fmt.Errorf("compile fieldexclude: %w", err)
		}
		f.tagIncludeFilter, err = filter.Compile(f.TagInclude)
		if err != nil {
			return fmt.Errorf("compile taginclude: %w", err)
		}
		f.tagExcludeFilter, err = filter.Compile(f.TagExclude)
		if err != nil {
			return fmt.Errorf("compile tagexclude: %w", err)
		}
	}
	return nil
}

// IsActive returns true if any selector or modifier rule is set. Use
// it as a fast guard to skip Select/Modify entirely when the filter
// is a no-op (the common case).
func (f *Filter) IsActive() bool {
	return f.isActive
}

// Select returns true if the metric passes the namepass/namedrop and
// tagpass/tagdrop rules. The metric is not modified.
func (f *Filter) Select(m *core.Metric) bool {
	if !f.selectActive {
		return true
	}
	if !shouldNamePass(f.namePassFilter, f.nameDropFilter, m.Name) {
		return false
	}
	if !shouldTagsPass(f.TagPassFilters, f.TagDropFilters, m.Tags) {
		return false
	}
	return true
}

// Modify mutates the metric in place: removes fields not matching
// fieldinclude / matching fieldexclude, and removes tags not matching
// taginclude / matching tagexclude.
//
// Callers are responsible for ensuring the metric is theirs to mutate
// (i.e. they've Copy()'d it if other consumers share the pointer).
func (f *Filter) Modify(m *core.Metric) {
	if !f.modifyActive {
		return
	}
	if f.fieldIncludeFilter != nil || f.fieldExcludeFilter != nil {
		for k := range m.Fields {
			if !filter.ShouldPassFilters(f.fieldIncludeFilter, f.fieldExcludeFilter, k) {
				delete(m.Fields, k)
			}
		}
	}
	if f.tagIncludeFilter != nil || f.tagExcludeFilter != nil {
		for k := range m.Tags {
			if !filter.ShouldPassFilters(f.tagIncludeFilter, f.tagExcludeFilter, k) {
				delete(m.Tags, k)
			}
		}
	}
}

// --- internal helpers ---

func shouldNamePass(pass, drop filter.Filter, name string) bool {
	if pass != nil && !pass.Match(name) {
		return false
	}
	if drop != nil && drop.Match(name) {
		return false
	}
	return true
}

// shouldTagsPass applies tagpass / tagdrop. Telegraf's combined rule
// when both are set: pass must match AND drop must NOT match (a tag
// listed in both is dropped).
func shouldTagsPass(passFilters, dropFilters []TagFilter, tags map[string]string) bool {
	tagMatches := func(filters []TagFilter) bool {
		for _, tf := range filters {
			if tf.matcher == nil {
				continue
			}
			if v, ok := tags[tf.Name]; ok && tf.matcher.Match(v) {
				return true
			}
		}
		return false
	}

	hasPass := len(passFilters) > 0
	hasDrop := len(dropFilters) > 0

	switch {
	case hasPass && hasDrop:
		return tagMatches(passFilters) && !tagMatches(dropFilters)
	case hasPass:
		return tagMatches(passFilters)
	case hasDrop:
		return !tagMatches(dropFilters)
	default:
		return true
	}
}
