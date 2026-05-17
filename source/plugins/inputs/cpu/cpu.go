package cpu

import (
	"errors"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"

	"github.com/minggeorgelei/metrics-monitor/source/core"
	"github.com/minggeorgelei/metrics-monitor/source/plugins/inputs"
)

// CPU collects per-CPU metrics. All flags are user-configurable via
// the TOML [[inputs.cpu]] block.
type CPU struct {
	PerCPU         bool `toml:"percpu"`           // emit one metric per logical CPU
	TotalCPU       bool `toml:"totalcpu"`         // emit one aggregate metric (cpu="cpu-total")
	CollectCPUTime bool `toml:"collect_cpu_time"` // emit time_* counters
	CollectUsage   bool `toml:"collect_usage"`    // emit usage_* gauges
	CollectCPUInfo bool `toml:"collect_cpu_info"` // emit cpu_info gauge with model/vendor/cores

	// Internal state — not configurable via TOML. Allocated by Init.
	lastStats map[string]cpu.TimesStat
}

func (*CPU) Name() string { return "cpu" }

// Init runs after TOML decoding has populated the struct fields and
// before the first Gather. We allocate the delta-tracking map here
// rather than in the Creator so it's clearly a post-config setup
func (c *CPU) Init() error {
	c.lastStats = make(map[string]cpu.TimesStat)

	// Sanity check: if a user disables every output category they'd
	// end up with a no-op plugin. Fail loudly so the misconfig is
	// caught at startup.
	if !c.CollectCPUTime && !c.CollectUsage && !c.CollectCPUInfo {
		return errors.New("cpu input has every collect_* flag false — nothing to emit")
	}
	return nil
}

func (c *CPU) Gather(acc core.Accumulator) error {
	now := time.Now()
	var times []cpu.TimesStat

	if c.PerCPU {
		per, err := cpu.Times(true)
		if err != nil {
			return fmt.Errorf("cpu times (per-cpu): %w", err)
		}
		times = append(times, per...)
	}
	if c.TotalCPU {
		total, err := cpu.Times(false)
		if err != nil {
			return fmt.Errorf("cpu times (total): %w", err)
		}
		times = append(times, total...)
	}

	for _, t := range times {
		tags := map[string]string{"cpu": t.CPU}

		if c.CollectCPUTime {
			acc.AddCounter("cpu", map[string]any{
				"time_user":       t.User,
				"time_system":     t.System,
				"time_idle":       t.Idle,
				"time_nice":       t.Nice,
				"time_iowait":     t.Iowait,
				"time_irq":        t.Irq,
				"time_softirq":    t.Softirq,
				"time_steal":      t.Steal,
				"time_guest":      t.Guest,
				"time_guest_nice": t.GuestNice,
			}, tags, now)
		}

		// Derive percentages from the delta with our previous sample.
		// The first Gather has no previous sample so we skip;
		// subsequent gathers always emit (assuming CollectUsage).
		if c.CollectUsage {
			last, ok := c.lastStats[t.CPU]
			if !ok {
				continue
			}
			totalDelta := total(t) - total(last)
			if totalDelta < 0 {
				return errors.New("cpu time went backwards (clock adjustment?)")
			}
			if totalDelta == 0 {
				continue
			}
			acc.AddGauge("cpu", map[string]any{
				"usage_user":       100 * (t.User - last.User - (t.Guest - last.Guest)) / totalDelta,
				"usage_system":     100 * (t.System - last.System) / totalDelta,
				"usage_idle":       100 * (t.Idle - last.Idle) / totalDelta,
				"usage_nice":       100 * (t.Nice - last.Nice - (t.GuestNice - last.GuestNice)) / totalDelta,
				"usage_iowait":     100 * (t.Iowait - last.Iowait) / totalDelta,
				"usage_irq":        100 * (t.Irq - last.Irq) / totalDelta,
				"usage_softirq":    100 * (t.Softirq - last.Softirq) / totalDelta,
				"usage_steal":      100 * (t.Steal - last.Steal) / totalDelta,
				"usage_guest":      100 * (t.Guest - last.Guest) / totalDelta,
				"usage_guest_nice": 100 * (t.GuestNice - last.GuestNice) / totalDelta,
			}, tags, now)
		}
	}

	// Stash current values for next Gather's delta.
	for _, t := range times {
		c.lastStats[t.CPU] = t
	}

	if c.CollectCPUInfo {
		c.emitInfo(acc, now)
	}
	return nil
}

// emitInfo publishes one "cpu_info" metric per Gather. Static fields
// (model, vendor, family) live in tags so consumers can filter by them;
// numeric fields (cores, mhz, cache) live in fields. The actual
// data is read fresh on every Gather — cpu.Info() is microsecond-fast
// on every supported platform.
func (c *CPU) emitInfo(acc core.Accumulator, now time.Time) {
	info, err := cpu.Info()
	if err != nil || len(info) == 0 {
		acc.AddError(fmt.Errorf("cpu info: %w", err))
		return
	}
	logical, _ := cpu.Counts(true)
	physical, _ := cpu.Counts(false)

	head := info[0]
	acc.AddGauge("cpu_info", map[string]any{
		"cores_physical": physical,
		"cores_logical":  logical,
		"mhz":            head.Mhz,
		"cache_size_kb":  int64(head.CacheSize),
	}, map[string]string{
		"model_name": head.ModelName,
		"vendor_id":  head.VendorID,
		"family":     head.Family,
		"model":      head.Model,
	}, now)
}

// total returns the sum of all CPU time fields except guest/guest_nice
// (those are already included in user/nice on Linux per the kernel
// docs — matching Telegraf's helper to avoid double counting).
func total(t cpu.TimesStat) float64 {
	return t.User + t.System + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal + t.Idle
}

// init self-registers the plugin with the inputs registry. Imported
// indirectly from main.go via a blank import; runs before main().
func init() {
	inputs.Add("cpu", func() core.Input {
		return &CPU{
			PerCPU:         true,
			TotalCPU:       true,
			CollectCPUTime: true,
			CollectUsage:   true,
			CollectCPUInfo: true,
		}
	})
}
