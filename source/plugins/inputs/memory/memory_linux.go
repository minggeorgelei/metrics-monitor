package memory

import (
	"github.com/shirou/gopsutil/v4/mem"
)

const extendedMemorySupported = true

func getExtendedMemory() (map[string]any, error) {
	extendedMemory, err := mem.NewExLinux().VirtualMemory()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"active_file":   extendedMemory.ActiveFile,
		"inactive_file": extendedMemory.InactiveFile,
		"active_anon":   extendedMemory.ActiveAnon,
		"inactive_anon": extendedMemory.InactiveAnon,
		"unevictable":   extendedMemory.Unevictable,
		"percpu":        extendedMemory.Percpu,
		"kernel_stack":  extendedMemory.KernelStack,
	}, nil
}
