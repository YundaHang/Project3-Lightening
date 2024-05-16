package coindatabase

// Config is the CoinDatabase's configuration options.
type Config struct {
	DatabasePath      string
	MainCacheCapacity uint32
}

// DefaultConfig returns the CoinDatabase's default Config.
func DefaultConfig() *Config {
	return &Config{
		DatabasePath:      "coindata",
		MainCacheCapacity: 30,
	}
}
