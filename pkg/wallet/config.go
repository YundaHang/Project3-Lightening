package wallet

// Config represents the configuration (settings)
// for the wallet.
// HasWt (HasWallet) defines whether the wallet
// should even exist on the node.
// TxRplyThresh (TransactionReplayThreshold)
// defines the time (represented by blocks seen)
// in which the wallet will resend a transaction
// to the network.
// SafeBlkAmt (SafeBlockAmount) defines the amount
// of blocks that need to be on top of the block
// that contains a transaction for that transaction
// to be considered valid by the wallet.
// TxVer (TransactionVersion) is the same as the
// software version of the node.
// DefLckTm (DefaultLockTime) is the default lock
// time (when the utxo can be spent)
type Config struct {
	HasWallet                  bool
	TransactionReplayThreshold uint32
	SafeBlockAmount            uint32
	TransactionVersion         uint32
	DefaultLockTime            uint32
	DefaultFee                 uint32
}

// DefaultConfig returns the standard/basic
// settings for the wallet
func DefaultConfig() *Config {
	return &Config{
		HasWallet:                  true,
		TransactionReplayThreshold: 3,
		SafeBlockAmount:            5,
		TransactionVersion:         0,
		DefaultLockTime:            0,
		DefaultFee:                 5,
	}
}
