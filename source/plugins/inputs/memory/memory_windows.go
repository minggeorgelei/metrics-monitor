package memory

import (
	"github.com/shirou/gopsutil/v4/mem"
)

const extendedMemorySupported = true

func getExtendedMemory() (map[string]any, error) {
	extendedMemory, err := mem.NewExWindows().VirtualMemory()
	return map[string]any{
		"commit_limit":    extendedMemory.CommitLimit,
		"commit_total":    extendedMemory.CommitTotal,
		"page_file_avail": extendedMemory.PageFileAvail,
		"page_file_total": extendedMemory.PageFileTotal,
		"physical_avail":  extendedMemory.PhysAvail,
		"physical_total":  extendedMemory.PhysTotal,
		"virtual_avail":   extendedMemory.VirtualAvail,
		"virtual_total":   extendedMemory.VirtualTotal,
	}, err
}
