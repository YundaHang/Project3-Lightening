package pkg

import (
	"Coin/pkg/blockchain"
	"Coin/pkg/id"
	"Coin/pkg/lightning"
	"Coin/pkg/miner"
	"Coin/pkg/wallet"
	"time"
)

// Config is the configuration for the node.
// IdConf is the configuration for the id,
// MinerConfig is the configuration for the miner,
// WalletConfig is the configuration for the wallet,
// ChainConfig is the configuration for the blockchain,
// Version is the version that the node is (used for
// software updates),
// PeerLimit is the maximum amount of peers the node
// is allowed to have,
// AddressLimit is the maximum amount of addresses the
// node is allowed to keep track of.
// Port is the port that the node should run on,
// MaxBlockSize is the maximum allowed block size,
type Config struct {
	IdConfig        *id.Config
	MinerConfig     *miner.Config
	WalletConfig    *wallet.Config
	ChainConfig     *blockchain.Config
	LightningConfig *lightning.Config

	HasCustomId bool
	CustomID    id.ID

	Version        int
	PeerLimit      int
	AddressLimit   int
	Port           int
	VersionTimeout time.Duration

	MaxBlockSize uint32
}

// DefaultConfig creates a Config object that
// contains basic/standard configurations for
// the node. To do this, it also calls the default
// config methods for the other more specific
// configs (such as configs for id, miner, wallet,
// and Chain).
// Inputs:
// port int the port that the node should start
// on
func DefaultConfig(port int) *Config {
	c := &Config{
		IdConfig:        id.DefaultConfig(),
		MinerConfig:     miner.DefaultConfig(-1),
		WalletConfig:    wallet.DefaultConfig(),
		ChainConfig:     blockchain.DefaultConfig(),
		LightningConfig: lightning.DefaultConfig(port + 40),
		Version:         0,
		PeerLimit:       20,
		AddressLimit:    1000,
		Port:            port,
		VersionTimeout:  time.Second * 2,
		MaxBlockSize:    10000000,
	}
	return c
}

func TestingConfig(port int) *Config {
	c := &Config{
		IdConfig:       id.DefaultConfig(),
		MinerConfig:    miner.DefaultConfig(-1),
		WalletConfig:   wallet.DefaultConfig(),
		ChainConfig:    blockchain.DefaultConfig(),
		Version:        0,
		PeerLimit:      20,
		AddressLimit:   1000,
		Port:           port,
		VersionTimeout: time.Second * 2,
		MaxBlockSize:   10000000,
	}
	return c
}
