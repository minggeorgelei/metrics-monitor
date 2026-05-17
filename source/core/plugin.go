package core

// Initializer is an optional interface plugins may implement to do
// post-config setup: the config loader will call Init() after TOML
// decoding has populated the struct, before the agent starts using
// the plugin. Init failures cause config loading to fail before any
// Gather/Write happens.
//
// Use Init for things that depend on config values (compile a regex,
// validate a path, allocate state-keeping maps). Pure default values
// belong in the Creator factory function instead.
type Initializer interface {
	Init() error
}
