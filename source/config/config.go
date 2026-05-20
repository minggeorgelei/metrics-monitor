// Package config loads a TOML configuration file into a fully-wired
// set of RunningInputs and RunningOutputs ready to hand to the agent.
//
// File format:
//
//	[agent]
//	metric_channel_size = 1000
//	log_level           = "info"
//
//	[[inputs.cpu]]
//	interval         = "1s"
//	percpu           = true
//	totalcpu         = true
//	collect_cpu_time = true
//	collect_usage    = true
//	collect_cpu_info = true
//
//	[[outputs.file]]
//	path           = "stdout"
//	flush_interval = "1s"
//	batch_size     = 1000
//	buffer_limit   = 10000
//
// Two-phase TOML decode:
//  1. Top-level decode into `raw` with [inputs|outputs].<name> kept
//     as toml.Primitive — opaque blobs we can decode later.
//  2. For each block, look up the Creator in the plugin registry,
//     create a fresh instance, then PrimitiveDecode the block into
//     it. Plugin-specific TOML fields (percpu, path, ...) populate
//     the plugin struct; common fields (interval, flush_interval)
//     populate a small commonConfig struct alongside.
//
// This keeps plugin code free of agent-level concerns and lets the
// config loader stay generic over any number of plugin types.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/minggeorgelei/metrics-monitor/source/agent"
	"github.com/minggeorgelei/metrics-monitor/source/core"
	"github.com/minggeorgelei/metrics-monitor/source/models"
	"github.com/minggeorgelei/metrics-monitor/source/plugins/aggregators"
	"github.com/minggeorgelei/metrics-monitor/source/plugins/inputs"
	"github.com/minggeorgelei/metrics-monitor/source/plugins/outputs"
	"github.com/minggeorgelei/metrics-monitor/source/plugins/processors"
)

// Config is the result of loading a config file. It's already split
// into the shape the agent constructor expects.
type Config struct {
	Agent         agent.Config
	Inputs        []*models.RunningInput
	Processors    []*models.RunningProcessor
	Aggregators   []*models.RunningAggregator
	AggProcessors []*models.RunningProcessor // run after aggregators (between Push and outputs)
	Outputs       []*models.RunningOutput

	// LogLevel / LogFormat / LogFile mirror the agent.log_* TOML
	// fields so the CLI can configure logging before constructing
	// the agent.
	LogLevel  string
	LogFormat string
	LogFile   string
}

// --- on-disk schema (TOML decode targets) ---

type fileSchema struct {
	Agent                agentSchema                 `toml:"agent"`
	Inputs               map[string][]toml.Primitive `toml:"inputs"`
	Processors           map[string][]toml.Primitive `toml:"processors"`
	Aggregators          map[string][]toml.Primitive `toml:"aggregators"`
	AggregatorProcessors map[string][]toml.Primitive `toml:"aggregator_processors"`
	Outputs              map[string][]toml.Primitive `toml:"outputs"`
}

type agentSchema struct {
	MetricChannelSize              int    `toml:"metric_channel_size"`
	LogLevel                       string `toml:"log_level"`
	LogFormat                      string `toml:"log_format"`
	LogFile                        string `toml:"log_file"`
	SkipProcessorsAfterAggregators bool   `toml:"skip_processors_after_aggregators"`
}

// filterSchema is the TOML view of a per-plugin Filter. Embedded into
// each common*Config below so every plugin block (inputs / processors /
// aggregators / outputs) supports the same filter knobs.
//
// TagPass / TagDrop use TOML inline-table semantics:
//
//	[[outputs.file]]
//	  namepass = ["cpu", "mem"]
//	  [outputs.file.tagpass]
//	    cpu = ["cpu-total"]
type filterSchema struct {
	NamePass []string `toml:"namepass"`
	NameDrop []string `toml:"namedrop"`

	TagPass map[string][]string `toml:"tagpass"`
	TagDrop map[string][]string `toml:"tagdrop"`

	FieldInclude []string `toml:"fieldinclude"`
	FieldExclude []string `toml:"fieldexclude"`
	TagInclude   []string `toml:"taginclude"`
	TagExclude   []string `toml:"tagexclude"`
}

// build constructs and Compile()s a models.Filter from the parsed
// TOML view. Returns the value (not a pointer) so callers can embed
// it directly into a RunningXxxConfig.
func (s *filterSchema) build() (models.Filter, error) {
	f := models.Filter{
		NamePass:     s.NamePass,
		NameDrop:     s.NameDrop,
		FieldInclude: s.FieldInclude,
		FieldExclude: s.FieldExclude,
		TagInclude:   s.TagInclude,
		TagExclude:   s.TagExclude,
	}
	for name, values := range s.TagPass {
		f.TagPassFilters = append(f.TagPassFilters, models.TagFilter{Name: name, Values: values})
	}
	for name, values := range s.TagDrop {
		f.TagDropFilters = append(f.TagDropFilters, models.TagFilter{Name: name, Values: values})
	}
	if err := f.Compile(); err != nil {
		return models.Filter{}, err
	}
	return f, nil
}

// commonInputConfig holds the agent-level knobs that apply to every
// input regardless of plugin type. Decoded from the same TOML block
// as the plugin-specific fields; the plugin ignores keys it doesn't
// know about (and vice versa).
type commonInputConfig struct {
	Interval     duration `toml:"interval"`
	filterSchema `toml:",inline"`
}

// commonOutputConfig is the equivalent for outputs.
type commonOutputConfig struct {
	FlushInterval duration `toml:"flush_interval"`
	BatchSize     int      `toml:"batch_size"`
	BufferLimit   int      `toml:"buffer_limit"`
	filterSchema  `toml:",inline"`
}

// commonProcessorConfig holds the chain-ordering knob plus filter rules.
type commonProcessorConfig struct {
	Order        int `toml:"order"`
	filterSchema `toml:",inline"`
}

// commonAggregatorConfig holds the agent-level knobs for an aggregator.
type commonAggregatorConfig struct {
	Period       duration `toml:"period"`
	DropOriginal bool     `toml:"drop_original"`
	// Grace / Delay widen the [periodStart, periodEnd] window that
	// Add() will accept metrics for. See RunningAggregatorConfig.
	Grace        duration `toml:"grace"`
	Delay        duration `toml:"delay"`
	filterSchema `toml:",inline"`
}

// duration is a tiny wrapper that lets TOML parse "1s" / "500ms" /
// "2m" strings directly into a time.Duration field. BurntSushi/toml
// doesn't do this conversion natively for time.Duration.
type duration time.Duration

func (d *duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = duration(parsed)
	return nil
}

// Load reads the file at path, validates it against the registry of
// known plugin names, and returns a ready-to-run Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var raw fileSchema
	md, err := toml.Decode(string(data), &raw)
	if err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	// Catch typos: any unknown top-level keys are a likely bug, not
	// a feature, so refuse to start rather than silently ignore.
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		// Filter out the primitives that are "undecoded" by design —
		// we'll decode them in a second pass once we know which
		// plugin to dispatch to.
		var truly []string
		for _, key := range undecoded {
			s := key.String()
			if !strings.HasPrefix(s, "inputs.") &&
				!strings.HasPrefix(s, "processors.") &&
				!strings.HasPrefix(s, "aggregators.") &&
				!strings.HasPrefix(s, "aggregator_processors.") &&
				!strings.HasPrefix(s, "outputs.") {
				truly = append(truly, s)
			}
		}
		if len(truly) > 0 {
			return nil, fmt.Errorf("config %q has unknown keys: %v", path, truly)
		}
	}

	cfg := &Config{
		Agent: agent.Config{
			MetricChannelSize:              raw.Agent.MetricChannelSize,
			SkipProcessorsAfterAggregators: raw.Agent.SkipProcessorsAfterAggregators,
		},
		LogLevel:  raw.Agent.LogLevel,
		LogFormat: raw.Agent.LogFormat,
		LogFile:   raw.Agent.LogFile,
	}

	for name, blocks := range raw.Inputs {
		for i, blk := range blocks {
			ri, err := buildInput(md, name, blk)
			if err != nil {
				return nil, fmt.Errorf("inputs.%s[%d]: %w", name, i, err)
			}
			cfg.Inputs = append(cfg.Inputs, ri)
		}
	}

	for name, blocks := range raw.Processors {
		for i, blk := range blocks {
			rp, err := buildProcessor(md, name, blk)
			if err != nil {
				return nil, fmt.Errorf("processors.%s[%d]: %w", name, i, err)
			}
			cfg.Processors = append(cfg.Processors, rp)
		}
	}

	for name, blocks := range raw.AggregatorProcessors {
		for i, blk := range blocks {
			rp, err := buildProcessor(md, name, blk)
			if err != nil {
				return nil, fmt.Errorf("aggregator_processors.%s[%d]: %w", name, i, err)
			}
			cfg.AggProcessors = append(cfg.AggProcessors, rp)
		}
	}

	for name, blocks := range raw.Aggregators {
		for i, blk := range blocks {
			ra, err := buildAggregator(md, name, blk)
			if err != nil {
				return nil, fmt.Errorf("aggregators.%s[%d]: %w", name, i, err)
			}
			cfg.Aggregators = append(cfg.Aggregators, ra)
		}
	}

	for name, blocks := range raw.Outputs {
		for i, blk := range blocks {
			ro, err := buildOutput(md, name, blk)
			if err != nil {
				return nil, fmt.Errorf("outputs.%s[%d]: %w", name, i, err)
			}
			cfg.Outputs = append(cfg.Outputs, ro)
		}
	}

	if len(cfg.Inputs) == 0 {
		return nil, fmt.Errorf("config %q defines no inputs", path)
	}
	if len(cfg.Outputs) == 0 {
		return nil, fmt.Errorf("config %q defines no outputs", path)
	}
	return cfg, nil
}

func buildInput(md toml.MetaData, name string, blk toml.Primitive) (*models.RunningInput, error) {
	creator, ok := inputs.Inputs[name]
	if !ok {
		return nil, fmt.Errorf("unknown input plugin %q (registered: %v)", name, registeredInputNames())
	}

	// Plugin-specific decode.
	plugin := creator()
	if err := md.PrimitiveDecode(blk, plugin); err != nil {
		return nil, fmt.Errorf("decode plugin fields: %w", err)
	}

	// Common decode (interval etc.). Same block, different target.
	var common commonInputConfig
	if err := md.PrimitiveDecode(blk, &common); err != nil {
		return nil, fmt.Errorf("decode common fields: %w", err)
	}

	filter, err := common.filterSchema.build()
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}

	// Run optional Init now so config errors surface at startup,
	// not at first Gather.
	if init, ok := plugin.(core.Initializer); ok {
		if err := init.Init(); err != nil {
			return nil, fmt.Errorf("init: %w", err)
		}
	}

	return models.NewRunningInput(plugin, models.RunningInputConfig{
		Interval: time.Duration(common.Interval),
		Filter:   filter,
	}), nil
}

func buildProcessor(md toml.MetaData, name string, blk toml.Primitive) (*models.RunningProcessor, error) {
	creator, ok := processors.Processors[name]
	if !ok {
		return nil, fmt.Errorf("unknown processor plugin %q (registered: %v)", name, registeredProcessorNames())
	}

	plugin := creator()
	if err := md.PrimitiveDecode(blk, plugin); err != nil {
		return nil, fmt.Errorf("decode plugin fields: %w", err)
	}

	var common commonProcessorConfig
	if err := md.PrimitiveDecode(blk, &common); err != nil {
		return nil, fmt.Errorf("decode common fields: %w", err)
	}

	if init, ok := plugin.(core.Initializer); ok {
		if err := init.Init(); err != nil {
			return nil, fmt.Errorf("init: %w", err)
		}
	}

	filter, err := common.filterSchema.build()
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}

	return models.NewRunningProcessor(plugin, models.RunningProcessorConfig{
		Order:  common.Order,
		Filter: filter,
	}), nil
}

func buildAggregator(md toml.MetaData, name string, blk toml.Primitive) (*models.RunningAggregator, error) {
	creator, ok := aggregators.Aggregators[name]
	if !ok {
		return nil, fmt.Errorf("unknown aggregator plugin %q (registered: %v)", name, registeredAggregatorNames())
	}

	plugin := creator()
	if err := md.PrimitiveDecode(blk, plugin); err != nil {
		return nil, fmt.Errorf("decode plugin fields: %w", err)
	}

	var common commonAggregatorConfig
	if err := md.PrimitiveDecode(blk, &common); err != nil {
		return nil, fmt.Errorf("decode common fields: %w", err)
	}

	if init, ok := plugin.(core.Initializer); ok {
		if err := init.Init(); err != nil {
			return nil, fmt.Errorf("init: %w", err)
		}
	}

	filter, err := common.filterSchema.build()
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}

	return models.NewRunningAggregator(plugin, models.RunningAggregatorConfig{
		Period:       time.Duration(common.Period),
		DropOriginal: common.DropOriginal,
		Grace:        time.Duration(common.Grace),
		Delay:        time.Duration(common.Delay),
		Filter:       filter,
	}), nil
}

func buildOutput(md toml.MetaData, name string, blk toml.Primitive) (*models.RunningOutput, error) {
	creator, ok := outputs.Outputs[name]
	if !ok {
		return nil, fmt.Errorf("unknown output plugin %q (registered: %v)", name, registeredOutputNames())
	}

	plugin := creator()
	if err := md.PrimitiveDecode(blk, plugin); err != nil {
		return nil, fmt.Errorf("decode plugin fields: %w", err)
	}

	var common commonOutputConfig
	if err := md.PrimitiveDecode(blk, &common); err != nil {
		return nil, fmt.Errorf("decode common fields: %w", err)
	}

	if init, ok := plugin.(core.Initializer); ok {
		if err := init.Init(); err != nil {
			return nil, fmt.Errorf("init: %w", err)
		}
	}

	filter, err := common.filterSchema.build()
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}

	return models.NewRunningOutput(plugin, models.RunningOutputConfig{
		FlushInterval:     time.Duration(common.FlushInterval),
		MetricBatchSize:   common.BatchSize,
		MetricBufferLimit: common.BufferLimit,
		Filter:            filter,
	}), nil
}

func registeredInputNames() []string {
	out := make([]string, 0, len(inputs.Inputs))
	for k := range inputs.Inputs {
		out = append(out, k)
	}
	return out
}

func registeredOutputNames() []string {
	out := make([]string, 0, len(outputs.Outputs))
	for k := range outputs.Outputs {
		out = append(out, k)
	}
	return out
}

func registeredProcessorNames() []string {
	out := make([]string, 0, len(processors.Processors))
	for k := range processors.Processors {
		out = append(out, k)
	}
	return out
}

func registeredAggregatorNames() []string {
	out := make([]string, 0, len(aggregators.Aggregators))
	for k := range aggregators.Aggregators {
		out = append(out, k)
	}
	return out
}
