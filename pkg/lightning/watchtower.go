package lightning

import (
	"Coin/pkg/block"
	"Coin/pkg/id"
)

type WatchTower struct {
	Id id.ID
	// do we want to make this a database? It could theoretically be very large (numChannels * numKeys)
	RevocationKeys map[string]*RevocationInfo
	// Channel to send a "caught" transaction to the node (and then to the wallet)
	RevokedTransactions chan *RevocationInfo
}

// HandleBlock handles a block and figures out if we need to revoke a transaction
func (w *WatchTower) HandleBlock(block *block.Block) *RevocationInfo {
	// TODO
	for _, tx := range block.Transactions {
		if revInfo, exists := w.RevocationKeys[tx.Hash()]; exists {
			go func() {
				w.RevokedTransactions <- revInfo
			}()
			delete(w.RevocationKeys, tx.Hash())
			return revInfo
		}
	}
	return nil
}
