//go:build !linux && !windows

package memory

const extendedMemorySupported = false

func getExtendedMemory() (map[string]any, error) {
	return nil, nil
}
