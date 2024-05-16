package miner

import "Coin/pkg/block"

// MiningPool is the list of transactions
// that the miner is currently mining.
type MiningPool []*block.Transaction

// NewMiningPool selects the highest priority
// transactions from the transaction pool.
func (m *Miner) NewMiningPool() MiningPool {
	var txs []*block.Transaction
	var blkSz uint32 = 100 // assume coinbase
	var rankings = *m.TxPool.TxQ
	for i := 0; i < len(rankings); i++ {
		blkSz += rankings[i].Transaction.Size()
		if blkSz < m.Config.BlockSize {
			txs = append(txs, rankings[i].Transaction)
		} else {
			break
		}
	}
	return txs
}
