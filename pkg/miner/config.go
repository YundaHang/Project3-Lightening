package miner

import (
	"Coin/pkg/utils"
	"math"
)

// Config represents the settings for the
// Miner.
// HasMiner defines whether
// the node will have a miner.
// Version defines the software version
// of the node.
// DefineLockTime defines the lock
// time that should be on the coinbase transaction.
// TransactionPoolCapacity defines the maximum number of
// transactions allowed in the transaction pool.
// PriorityLimit defines the priority threshold that
// must be met for the miner to start mining a
// group of transactions
// BlockSize defines the maximum size a block can be.
// NonceLimit defines the maximum nonce that miners
// are willing to mine to.
// InitialSubsidy defines the initial subsidy given
// to miners for the minting reward before any
// halvings.
// SubsidyHalvingRate defines the rate (in terms of
// blocks added) at which the initial subsidy
// is halved.
// MaxHalvings defines the maximum number of
// halvings that are allowed before the subsidy
// becomes 0.
// InitialPOWDifficulty represents the inital proof of
// work difficulty. This is helpful in the
// config because now some nodes can be toggled
// to have a higher proof of work than others,
// which is essentially adjusting the speeds of miners
// on the network.
type Config struct {
	HasMiner bool

	Version        uint32
	DefineLockTime uint32

	TransactionPoolCapacity uint32
	PriorityLimit           uint32

	BlockSize  uint32
	NonceLimit uint32

	InitialSubsidy       uint32
	SubsidyHalvingRate   uint32
	MaxHalvings          uint32
	InitialPOWDifficulty []byte
}

// DefaultConfig returns the default settings
// for the Miner.
func DefaultConfig(powdNumZeros int) *Config {
	return &Config{
		HasMiner:                true,
		Version:                 0,
		DefineLockTime:          0,
		TransactionPoolCapacity: 50,
		PriorityLimit:           10,
		BlockSize:               1000,
		NonceLimit:              uint32(math.Pow(2, 20)),
		InitialSubsidy:          50,
		SubsidyHalvingRate:      10,
		MaxHalvings:             10,
		InitialPOWDifficulty:    utils.CalcPOWD(powdNumZeros),
	}
}
