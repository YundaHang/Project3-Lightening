package miner

import (
	"Coin/pkg/block"
	"fmt"
	"go.uber.org/atomic"
	"sync"
)

// TxPool represents all the valid transactions
// that the miner can mine.
// CurrentPriority is the current cumulative priority of
// all the transactions.
// PriorityLimit is the cumulative priority threshold
// needed to surpass in order to start mining.
// TxQ is the transaction maximum priority queue
// that the transactions are stored in.
// Ct is the current count of the transactions
// in the pool.
// Cap is the maximum amount of allowed
// transactions to store in the pool.
type TxPool struct {
	CurrentPriority *atomic.Uint32
	PriorityLimit   uint32

	TxQ      *block.Heap
	Count    *atomic.Uint32
	Capacity uint32

	Mutex sync.Mutex
}

// Length returns the count of transactions
// currently in the pool.
// Returns:
// uint32 the count (Ct) of the pool
func (tp *TxPool) Length() uint32 {
	return tp.Count.Load()
}

// NewTxPool constructs a transaction pool.
func NewTxPool(c *Config) *TxPool {
	return &TxPool{
		CurrentPriority: atomic.NewUint32(0),
		PriorityLimit:   c.PriorityLimit,
		TxQ:             block.NewTransactionHeap(),
		Count:           atomic.NewUint32(0),
		Capacity:        c.TransactionPoolCapacity,
	}
}

// PriorityMet checks to see
// if the transaction pool has enough
// cumulative priority to start mining.
func (tp *TxPool) PriorityMet() bool {
	return tp.CurrentPriority.Load() >= tp.PriorityLimit
}

// CalculatePriority calculates the
// priority of a transaction by dividing the
// fees (inputs - outputs) by the size of the
// transaction.
func CalculatePriority(t *block.Transaction, sumInputs uint32) uint32 {
	if t == nil {
		fmt.Printf("ERROR {TransactionPool.CalcPri}: The" +
			"inputted transaction was nil.\n")
		return 0
	}
	fees := sumInputs - t.SumOutputs()
	sz := t.Size()
	var factor uint32 = 100
	pri := fees * factor / sz
	if pri == 0 {
		return 1
	} else {
		return pri
	}
}

// Add adds a transaction to the transaction pool.
// If the transaction pool is full, the transaction
// will not be added. Otherwise, the cumulative
// priority level is updated, the counter is
// incremented, and the transaction is added to the
// heap.
func (tp *TxPool) Add(t *block.Transaction, sumInputs uint32) {
	if t == nil {
		fmt.Printf("ERROR {TransactionPool.Add}: The" +
			"inputted transaction was nil.\n")
		return
	}
	if tp.Count.Load() >= tp.Capacity {
		return
	}
	pri := CalculatePriority(t, sumInputs)
	tp.CurrentPriority.Add(pri)
	tp.Mutex.Lock()
	tp.TxQ.Add(pri, t)
	tp.Mutex.Unlock()
	tp.Count.Inc()
}

// CheckTransactions checks for any duplicate
// transactions in the heap and removes them.
func (tp *TxPool) CheckTransactions(txs []*block.Transaction) {
	tp.Mutex.Lock()
	amtRem, totalPriority := tp.TxQ.Remove(txs)
	tp.Mutex.Unlock()
	tp.Count.Sub(uint32(len(amtRem)))
	tp.CurrentPriority.Sub(totalPriority)
}
