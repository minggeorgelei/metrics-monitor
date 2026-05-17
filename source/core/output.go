package core

// Output writes batches of metrics somewhere (file, stdout, http, ws,
// influxdb, ...). Lifecycle:
//
//   Connect()    // once at startup
//   Write(...)   // many times, called by the agent's output runner
//   Close()      // once at shutdown
//
// Connect/Close exist because some outputs hold long-lived resources
// (files, sockets, db sessions). For trivial outputs both can be no-ops.
type Output interface {
	Name() string

	// Connect is called once before the first Write. Implementations
	// should open files, dial connections, etc. here.
	Connect() error

	// Close releases any resources held by the output. Called once at
	// shutdown; Write will not be invoked after Close returns.
	Close() error

	// Write delivers a batch of metrics to the output. The agent
	// determines batch size and timing. A returned error tells the
	// caller "treat the batch as failed" — typically the agent will
	// then mark the metrics as Keep (re-queued) via the Transaction
	// protocol, depending on output semantics.
	Write(metrics []*Metric) error
}
