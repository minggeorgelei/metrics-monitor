package core

// Input is implemented by every metric source (cpu, mem, disk, ...).
// Gather is invoked by the agent on the configured interval; the input
// should produce its metrics by calling acc.AddFields / AddGauge /
// AddCounter rather than returning them.
type Input interface {
	// Name returns the plugin's short identifier (e.g. "cpu"). Used
	// in logs and as the registry key.
	Name() string

	// Gather is called every interval. Implementations should keep
	// their work bounded: long-running sampling (like cpu.Percent
	// with a 500ms window) is fine, but anything async should be
	// modelled as a ServiceInput in the future.
	Gather(acc Accumulator) error
}
