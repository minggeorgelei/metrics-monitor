package memory

import (
	"github.com/shirou/gopsutil/v4/mem"
)

const extendedMemorySupported = true

func getExtendedMemory() (map[string]any, error) {
	extendedMemory, err := mem.NewExLinux().VirtualMemory()
	return map[string]any{
		"active_file":   extendedMemory.ActiveFile,
		"inactive_file": extendedMemory.InactiveFile,
		"active_Anon":   extendedMemory.ActiveAnon,
		"inactive_Anon": extendedMemory.InactiveAnon,
		"unevictable":   extendedMemory.Unevictable,
		"percpu":        extendedMemory.Percpu,
		"kernel_stack":  extendedMemory.KernelStack,
	}, err
}
