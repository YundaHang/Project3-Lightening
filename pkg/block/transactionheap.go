package block

import (
	"container/heap"
	"fmt"
)

// HeapNode (TransactionHeapNode) represents
// a node within a heap or a priority queue.
// Priority	uint32	the priority of the transaction.
// Transaction	*Transaction	pointer to a transaction.
type HeapNode struct {
	Priority    uint32
	Transaction *Transaction
}

// Heap represents a
// heap or priority queue for storing transactions.
// It is a synonymous type with an array of
// *TxHeapNodes since the heap is implemented
// using an array.
type Heap []*HeapNode

// NewTransactionHeap creates
// a new empty transaction heap.
// Returns:
// *TxHeap a pointer to the transaction heap.
func NewTransactionHeap() *Heap {
	var queue Heap
	heap.Init(&queue)
	return &queue
}

// Add adds a transaction with a particular
// priority to the transaction heap.
// Inputs:
// p	uint32 the priority of the new transaction.
// t	*Transaction the new transaction.
func (h *Heap) Add(p uint32, t *Transaction) {
	if t == nil {
		fmt.Printf("ERROR {TxHeap.Add}: " +
			"received a nil transaction.\n")
		return
	}
	n := &HeapNode{Priority: p, Transaction: t}
	heap.Push(h, n)
}

// GetFirst gets the first transaction in the heap.
func (h *Heap) GetFirst() *HeapNode {
	return (*h)[0]
}

// IncrementAll increments
// the priority of every transaction
// stored within the heap.
func (h *Heap) IncrementAll() {
	for i := 0; i < h.Len(); i++ {
		(*h)[i].Priority++
	}
}

// Len returns the length
// of the heap. This method is required
// by the heap interface.
// Returns:
// int	the length of the heap
func (h Heap) Len() int {
	return len(h)
}

// Less returns whether one
// transaction is greater than another.
// This method is required by the heap interface,
// which is why it has a misleading name.
// Inputs:
// i	int idx of a particular transaction.
// j	int	idx of a particular transaction.
// Returns:
// bool	True if transaction at index i
// has a larger priority than transaction
// at index j. False otherwise.
func (h Heap) Less(i, j int) bool {
	return h[i].Priority > h[j].Priority
}

// Swap swaps two transactions in the
// heap. This method is required by the
// heap interface.
// Inputs:
// i	int idx of a particular transaction.
// j	int	idx of a particular transaction.
func (h Heap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push adds an element to the heap.
// This method is required by the
// heap interface.
func (h *Heap) Push(x interface{}) {
	*h = append(*h, x.(*HeapNode))
}

// Pop removes the top element of the heap.
// This method is required by the
// heap interface.
func (h *Heap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// Peek returns the top element of the heap.
// This method is required by the
// heap interface.
func (h *Heap) Peek() interface{} {
	old := *h
	n := len(*h)
	return old[n-1]
}

// Remove removes any transactions stored
// in the heap that are also in the inputted
// array.
// Inputs:
// ts	[]*Transaction	an array of pointers to
// transactions that should be removed from the
// heap.
// Returns:
// []*Transaction	all the transactions that
// were removed from the heap and the total priority removed
func (h *Heap) Remove(ts []*Transaction) ([]*Transaction, uint32) {
	var r = make([]*Transaction, 0)
	totalPriority := uint32(0)
	for _, t := range ts {
		if t == nil {
			fmt.Printf("ERROR {TxHeap.Rmv}: " +
				"received a nil transaction.\n")
			return nil, totalPriority
		}
		i, isIn := h.GetIndex(t)
		if isIn {
			priority := (*h)[i].Priority
			(*h)[i], (*h)[len(*h)-1] = (*h)[len(*h)-1], (*h)[i]
			*h = (*h)[:len(*h)-1]
			r = append(r, t)
			totalPriority += priority
		}
	}
	heap.Init(h)
	return r, totalPriority
}

// GetIndex gets the index into the heap of a
// particular transaction.
// Inputs:
// t	*Transaction	the transaction in which
// the corresponding index is desired
// Returns:
// int	the index of the inputted transaction
// bool True if the index could be found, false
// otherwise.
func (h *Heap) GetIndex(t *Transaction) (int, bool) {
	if t == nil {
		fmt.Printf("ERROR {TxHeap.GetIndex}: " +
			"received a nil transaction.\n")
		return 0, false
	}
	for i, v := range *h {
		if v.Transaction.Hash() == t.Hash() {
			return i, true
		}
	}
	return 0, false
}

// Has returns true if the inputted
// transaction is in the heap.
// Inputs:
// t *Transaction a transaction that could
// be in the heap
// Returns:
// bool	True if the transaction is in the
// heap, false otherwise.
func (h *Heap) Has(t *Transaction) bool {
	if t == nil {
		fmt.Printf("ERROR {TxHeap.Has}: " +
			"received a nil transaction.\n")
		return false
	}
	for _, tx := range *h {
		if tx.Transaction.Hash() == t.Hash() {
			return true
		}
	}
	return false
}

// RemoveAboveThreshold removes all transactions
// in the heap that are above a certain priority.
// Inputs:
// thresh	uint32	the threshold that dictates
// the minimum priority needed in order to be
// removed from the heap
// Returns:
// []*Transaction all transactions that were
// removed.
func (h *Heap) RemoveAboveThreshold(threshold uint32) []*Transaction {
	var rem []*Transaction
	for i := 0; i < h.Len(); i++ {
		val := (*h)[i]
		if val.Priority >= threshold {
			rem = append(rem, val.Transaction)
		}
	}
	h.Remove(rem)
	return rem
}
