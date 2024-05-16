package pro

import (
	"unsafe"
)

// SizeOfTransaction returns the
// size of the transaction in bytes.
func SizeOfTransaction(t *Transaction) uint32 {
	var sz uint32
	for _, txi := range t.Inputs {
		sz += uint32(unsafe.Sizeof(txi.ReferenceTransactionHash))
		sz += uint32(unsafe.Sizeof(txi.UnlockingScript))
		sz += uint32(unsafe.Sizeof(txi.OutputIndex))
	}
	for _, txo := range t.Outputs {
		sz += uint32(unsafe.Sizeof(txo.Amount))
		sz += uint32(unsafe.Sizeof(txo.LockingScript))
	}
	sz += uint32(unsafe.Sizeof(t.LockTime))
	sz += uint32(unsafe.Sizeof(t.Version))
	return sz
}

// NewTransaction returns a new protobuf transaction.
func NewTransaction(version uint32, inputs []*TransactionInput,
	outputs []*TransactionOutput, lockTime uint32) *Transaction {
	return &Transaction{
		Version:  version,
		Inputs:   inputs,
		Outputs:  outputs,
		LockTime: lockTime,
	}
}

// SizeOfBlock returns the
// size of the block in bytes
// Returns:
// uint32 the number of bytes the object
// takes up
func SizeOfBlock(b *Block) uint32 {
	var sz uint32
	for _, t := range b.Transactions {
		sz += SizeOfTransaction(t)
	}
	sz += SizeOfHeader(b.Header)
	return sz
}

// SizeOfHeader returns
// the size of the block header in bytes
// Returns:
// uint32 the number of bytes in the block
// header
func SizeOfHeader(h *Header) uint32 {
	var sz uint32
	sz += uint32(unsafe.Sizeof(h.PreviousHash))
	sz += uint32(unsafe.Sizeof(h.Version))
	sz += uint32(unsafe.Sizeof(h.DifficultyTarget))
	sz += uint32(unsafe.Sizeof(h.Nonce))
	sz += uint32(unsafe.Sizeof(h.Timestamp))
	return sz
}

// NewTransactionInput returns
// a new protobuf transaction input.

func NewTransactionInput(hash string, outputIndex uint32, unlockingScript []byte) *TransactionInput {
	return &TransactionInput{
		ReferenceTransactionHash: hash,
		OutputIndex:              outputIndex,
		UnlockingScript:          unlockingScript,
	}
}

// NewTransactionOutput returns
// a new protobuf transaction output.
func NewTransactionOutput(amount uint32, lockingScript []byte) *TransactionOutput {
	return &TransactionOutput{
		Amount:        amount,
		LockingScript: lockingScript,
	}
}
