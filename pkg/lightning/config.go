package lightning

import (
	"Coin/pkg/id"
	"time"
)

type Config struct {
	IdConfig         *id.Config
	LockTime         uint32
	AdditionalBlocks uint32
	Version          uint32

	Port           int
	VersionTimeout time.Duration
}

func DefaultConfig(port int) *Config {
	return &Config{
		IdConfig:       id.DefaultConfig(),
		LockTime:       10,
		Version:        0,
		Port:           port,
		VersionTimeout: time.Second * 2,
	}
}
