package memory

import (
	"fmt"
	"maps"
	"runtime"
	"time"

	"github.com/minggeorgelei/metrics-monitor/source/core"
	"github.com/minggeorgelei/metrics-monitor/source/plugins/inputs"
	memory "github.com/shirou/gopsutil/v4/mem"
)

type Memory struct {
	CollectExtended bool `toml:"collect_extended"` // emit extended memory
	platform        string
}

func (mem *Memory) Init() error {
	mem.platform = runtime.GOOS
	if mem.CollectExtended && !extendedMemorySupported {
		mem.CollectExtended = false
	}
	return nil
}

// Gather implements core.Input.
func (mem *Memory) Gather(acc core.Accumulator) error {
	vm, err := memory.VirtualMemory()
	if err != nil {
		return fmt.Errorf("error getting virtual memory info: %w", err)
	}

	fields := map[string]any{
		"total":             vm.Total,
		"available":         vm.Available,
		"used":              vm.Used,
		"available_percent": 100 * float64(vm.Available) / float64(vm.Total),
		"used_percent":      vm.UsedPercent,
	}

	switch mem.platform {
	case "darwin":
		fields["active"] = vm.Active
		fields["inactive"] = vm.Inactive
		fields["free"] = vm.Free
		fields["wired"] = vm.Wired
	case "openbsd":
		fields["active"] = vm.Active
		fields["inactive"] = vm.Inactive
		fields["free"] = vm.Free
		fields["buffered"] = vm.Buffers
		fields["wired"] = vm.Wired
	case "freebsd":
		fields["active"] = vm.Active
		fields["inactive"] = vm.Inactive
		fields["free"] = vm.Free
		fields["buffered"] = vm.Buffers
		fields["cached"] = vm.Cached
		fields["laundry"] = vm.Laundry
		fields["wired"] = vm.Wired
	case "linux":
		fields["active"] = vm.Active
		fields["inactive"] = vm.Inactive
		fields["free"] = vm.Free
		fields["buffered"] = vm.Buffers
		fields["cached"] = vm.Cached
		fields["commit_limit"] = vm.CommitLimit
		fields["committed_as"] = vm.CommittedAS
		fields["dirty"] = vm.Dirty
		fields["high_free"] = vm.HighFree
		fields["high_total"] = vm.HighTotal
		fields["huge_pages_free"] = vm.HugePagesFree
		fields["huge_page_size"] = vm.HugePageSize
		fields["huge_pages_total"] = vm.HugePagesTotal
		fields["low_free"] = vm.LowFree
		fields["low_total"] = vm.LowTotal
		fields["mapped"] = vm.Mapped
		fields["page_tables"] = vm.PageTables
		fields["shared"] = vm.Shared
		fields["slab"] = vm.Slab
		fields["sreclaimable"] = vm.Sreclaimable
		fields["sunreclaim"] = vm.Sunreclaim
		fields["swap_cached"] = vm.SwapCached
		fields["swap_free"] = vm.SwapFree
		fields["swap_total"] = vm.SwapTotal
		fields["vmalloc_chunk"] = vm.VmallocChunk
		fields["vmalloc_total"] = vm.VmallocTotal
		fields["vmalloc_used"] = vm.VmallocUsed
		fields["write_back_tmp"] = vm.WriteBackTmp
		fields["write_back"] = vm.WriteBack
	}
	if mem.CollectExtended {
		extendedFields, err := getExtendedMemory()
		if err != nil {
			acc.AddError(fmt.Errorf("error getting extended memory info: %w", err))
		} else {
			maps.Copy(fields, extendedFields)
		}
	}
	acc.AddGauge("memory", fields, nil, time.Now())
	return nil
}

func (*Memory) Name() string { return "memory" }

func init() {
	inputs.Add("memory", func() core.Input {
		return &Memory{
			CollectExtended: true,
		}
	})
}
