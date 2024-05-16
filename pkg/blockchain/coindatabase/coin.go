package coindatabase

import "Coin/pkg/block"

// Coin is used by the CoinDatabase to keep track of unspent
// TransactionOutputs.
// TransactionOutput is the underlying TransactionOutput.
// IsSpent is whether that TransactionOutput has been spent.
// Active is whether that TransactionOutput is one created by
// Blocks on the active Chain.
type Coin struct {
	TransactionOutput *block.TransactionOutput
	IsSpent           bool
}

// CoinLocator is a dumbed down TransactionInput, used
// as a key to Coins in the CoinDatabase's mainCache.
type CoinLocator struct {
	ReferenceTransactionHash string
	OutputIndex              uint32
}

// makeCoinLocator returns a CoinLocator given a TransactionInput.
func makeCoinLocator(txi *block.TransactionInput) CoinLocator {
	return CoinLocator{
		ReferenceTransactionHash: txi.ReferenceTransactionHash,
		OutputIndex:              txi.OutputIndex,
	}
}
