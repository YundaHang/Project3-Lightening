package blockinfodatabase

// Config is the BlockInfoDatabase's configuration options.
type Config struct {
	DatabasePath string
}

// DefaultConfig returns the default configuration for the
// BlockInfoDatabase.
func DefaultConfig() *Config {
	return &Config{DatabasePath: "blockinfodata"}
}
