package pkg

import (
	"Coin/pkg/block"
	"fmt"
)

// CheckBlockSyntax validates a block's
// syntactical structure.
// To be valid syntactically:
// The block's first transaction must be of type Coinbase.
// Inputs:
// b *block.Block the block to be checked for validity
// Returns:
// bool True if the block is syntactically valid. false
// otherwise
func CheckBlockSyntax(b *block.Block) bool {
	if b.Transactions == nil || len(b.Transactions) == 0 {
		fmt.Printf("{Validation.ChkBlkSyn} ERROR: transactions were" +
			" either nil or had zero length.\n")
		return false
	}
	return b.Transactions[0].IsCoinbase() && b.Transactions[0].SumOutputs() > 0
}

// CheckBlockSemantics validates a block's
// semantics.
// To be valid semantically:
// The merkle root of the transactions on the block must be less
// than the difficulty target.
// Inputs:
// b *block.Block the block to be checked for validity
// Returns:
// bool True if the block is semantically valid. false
// otherwise
func CheckBlockSemantics(b *block.Block) bool {
	// This function was removed so that students could implement it
	// in miner.CalculateNonce
	return true
}

// CheckBlockConfiguration validates that certain
// aspects of the block abide by the node's own configuration.
// To be valid "configurally":
// The block's size must be less than or equal to the largest
// allowed block size.
// Inputs:
// b *block.Block the block to be checked for validity
// Returns:
// bool True if the block is configurally valid. false
// otherwise
func (n *Node) CheckBlockConfiguration(b *block.Block) bool {
	return b.Size() <= n.Config.MaxBlockSize
}

// CheckBlock validates a block based on multiple
// conditions.
// To be valid:
// The block must be syntactically (ChkBlkSyn), semantically
// (ChkBlkSem), and configurally (ChkBlkConf) valid.
// Each transaction on the block must be syntactically (ChkTxSyn),
// semantically (ChkTxSem), and configurally (ChkTxConf) valid.
// Each transaction on the block must reference UTXO on the same
// chain (main or forked chain) and not be a double spend on that
// chain.
// Inputs:
// b *block.Block the block to be checked for validity
// Returns:
// bool True if the block is valid. false
// otherwise
func (n *Node) CheckBlock(b *block.Block) bool {
	if b == nil {
		fmt.Printf("{Validation.ChkBlk} ERROR: block was nil.\n")
		return false
	}
	//if !(CheckBlockSyntax(b) && CheckBlockSemantics(b) && n.CheckBlockConfiguration(b)) {
	//	return false
	//}
	//for i := 1; i < len(b.Transactions); i++ {
	//	t := b.Transactions[i]
	//	if !(CheckTransactionSyntax(t) && n.CheckTransactionSemantics(t) && n.CheckTransactionConfiguration(t)) {
	//		return false
	//	}
	//}
	return n.BlockChain.CoinDB.ValidateBlock(b.Transactions)
}

// CheckTransactionSyntax validates a transaction
// syntactically.
// To be valid:
// The transaction's inputs and outputs must not be empty.
// The transaction's output amounts must be larger than 0.
// Inputs:
// t *block.Transaction the transaction to be checked for validity
// Returns:
// bool True if the transaction is syntactically valid. false
// otherwise
func CheckTransactionSyntax(t *block.Transaction) bool {
	a := len(t.Inputs) > 0 && len(t.Outputs) > 0
	for _, txo := range t.Outputs {
		if txo.Amount <= 0 {
			return false
		}
	}
	return a
}

// CheckTransactionSemantics validates a
// a transaction semantically.
// To be valid:
// The sum of the transaction's inputs must be larger
// than the sum of the transaction's outputs.
// Inputs:
// t *block.Transaction the transaction to be checked for validity
// Returns:
// bool True if the transaction is semantically valid. false
// otherwise
func (n *Node) CheckTransactionSemantics(tx *block.Transaction) bool {
	return false
}

// CheckNonOrphanSemantically validates
// a transaction semantically with the guarantee that
// the transaction is not an orphan.
// To be valid:
// The transaction must not double spend any UTXO.
// The unlocking script on each of the transaction's
// inputs must successfully unlock each of the corresponding
// UTXO.
// Inputs:
// t *block.Transaction the transaction to be checked for validity
// Returns:
// bool True if the transaction is semantically valid. false
// otherwise
func (n *Node) CheckNonOrphanSemantically(t *block.Transaction) bool {
	//for _, inp := range t.Inputs {
	//	if n.Chain.IsInvalidInput(inp) {
	//		return false
	//	}
	//}
	//for _, inp := range t.Inputs {
	//	utxo := n.Chain.GetUTXO(inp)
	//	if utxo == nil || !utxo.IsUnlckd(inp.UnlockingScript) {
	//		return false
	//	}
	//}
	return true
}

// CheckTransactionConfiguration validates
// that certain aspects of the transaction meet the
// required configuration settings of the node.
// To be valid:
// The transaction must not be larger than the
// maximum allowed block size.
// Inputs:
// t *block.Transaction the transaction to be checked for validity
// Returns:
// bool True if the transaction is configurally valid. false
// otherwise
func (n *Node) CheckTransactionConfiguration(t *block.Transaction) bool {
	return t.Size() <= n.Config.MaxBlockSize
}

// CheckTransaction validates a transaction
// syntactically (ChkTxSyn), semantically (ChkTxSem),
// and configurally (ChkTxConf). If the transaction
// can be classified as an orphan, there are more
// validity checks that have to be done (ChkNonOrfSem).
// Inputs:
// t *block.Transaction the transaction to be checked for validity
// Returns:
// bool True if the transaction is syntactically valid. false
// otherwise
func (n *Node) CheckTransaction(tx *block.Transaction) bool {
	return true
	//valid := CheckTransactionSyntax(tx) && n.CheckTransactionSemantics(tx) && n.CheckTransactionConfiguration(tx)
	//if err := n.BlockChain.CoinDB.ValidateTransaction(tx); err != nil {
	//	return false
	//}
	//return valid
}
