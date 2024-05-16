package blockchain

import (
	"Coin/pkg/blockchain/blockinfodatabase"
	"Coin/pkg/blockchain/chainwriter"
	"Coin/pkg/blockchain/coindatabase"
	"encoding/hex"
)

// Config is the BlockChain's configuration options.
type Config struct {
	GenesisPublicKey  []byte
	InitialSubsidy    uint32
	HasChain          bool
	BlockInfoDBPath   string
	ChainWriterDBPath string
	CoinDBPath        string
}

// GENPK is the public key that was used
// for the genesis transaction on the
// genesis block.
var GENPK = "3059301306072a8648ce3d020106082a8648ce3d030107034200042418a20458559ae13a0d4bb6ac284c66a5cebb5689563d4cf573473d8c6d5abfa9a21a65dbb3ba2f2d930be7f763f940f9864abaf199a0f0d8d14bedda2dcad9"

// GENPVK is the public key that was used
// for the genesis transaction on the
// genesis block.
var GENPVK = "307702010104202456b0e8bed5c27dcadb044df1af8eaf714084b61a23d17359fb09f3c3f5fff5a00a06082a8648ce3d030107a144034200042418a20458559ae13a0d4bb6ac284c66a5cebb5689563d4cf573473d8c6d5abfa9a21a65dbb3ba2f2d930be7f763f940f9864abaf199a0f0d8d14bedda2dcad9"

// DefaultConfig returns the default configuration for the blockchain.
func DefaultConfig() *Config {
	pkB, _ := hex.DecodeString(GENPK)
	return &Config{
		GenesisPublicKey:  pkB,
		InitialSubsidy:    0,
		HasChain:          true,
		BlockInfoDBPath:   blockinfodatabase.DefaultConfig().DatabasePath,
		ChainWriterDBPath: chainwriter.DefaultConfig().DataDirectory,
		CoinDBPath:        coindatabase.DefaultConfig().DatabasePath,
	}
}
