package test

import (
	"Coin/pkg/block"
	"Coin/pkg/blockchain/blockinfodatabase"
	"Coin/pkg/blockchain/chainwriter"
	"Coin/pkg/pro"
	"Coin/pkg/wallet"
	"fmt"
	"google.golang.org/protobuf/proto"
)

// MockedHeader returns a mocked Header.
func MockedHeader() *block.Header {
	return &block.Header{
		Version:          0,
		PreviousHash:     "",
		MerkleRoot:       "",
		DifficultyTarget: "",
		Nonce:            0,
		Timestamp:        0,
	}
}

// MockedBlockRecord returns a mocked BlockRecord.
func MockedBlockRecord() *blockinfodatabase.BlockRecord {
	return &blockinfodatabase.BlockRecord{
		Header:               MockedHeader(),
		Height:               0,
		NumberOfTransactions: 0,
		BlockFile:            "blockinfodata/block_0",
		BlockStartOffset:     0,
		BlockEndOffset:       10,
		UndoFile:             "",
		UndoStartOffset:      0,
		UndoEndOffset:        0,
	}
}

// MockedTransactionInput returns a mocked TransactionInput.
func MockedTransactionInput() *block.TransactionInput {
	return &block.TransactionInput{
		ReferenceTransactionHash: "",
		OutputIndex:              0,
		UnlockingScript:          []byte{},
	}
}

// MockedTransactionOutput returns a mocked TransactionOutput.
func MockedTransactionOutput() *block.TransactionOutput {
	return &block.TransactionOutput{
		Amount:        0,
		LockingScript: []byte{},
	}
}

// MockedTransaction returns a mocked Transaction.
func MockedTransaction() *block.Transaction {
	return &block.Transaction{
		Segwit:    true,
		Version:   0,
		Inputs:    []*block.TransactionInput{MockedTransactionInput()},
		Outputs:   []*block.TransactionOutput{MockedTransactionOutput()},
		Witnesses: [][]byte{},
		LockTime:  0,
	}
}

// MockedBlock returns a mocked Block.
func MockedBlock() *block.Block {
	return &block.Block{
		Header:       MockedHeader(),
		Transactions: []*block.Transaction{MockedTransaction()},
	}
}

// MockedUndoBlock returns a mocked UndoBlock.
func MockedUndoBlock() *chainwriter.UndoBlock {
	return &chainwriter.UndoBlock{
		TransactionInputHashes: []string{""},
		OutputIndexes:          []uint32{1},
		Amounts:                []uint32{1},
		LockingScripts:         [][]byte{},
	}
}

//GenesisBlock returns the genesis block for testing purposes.
func GenesisBlock() *block.Block {
	txo := &block.TransactionOutput{
		Amount:        1_000_000_000,
		LockingScript: []byte{00, 11},
	}
	genTx := &block.Transaction{
		Version:  0,
		Inputs:   nil,
		Outputs:  []*block.TransactionOutput{txo},
		LockTime: 0,
	}
	return &block.Block{
		Header: &block.Header{
			Version:          0,
			PreviousHash:     "",
			MerkleRoot:       "",
			DifficultyTarget: "",
			Nonce:            0,
			Timestamp:        0,
		},
		Transactions: []*block.Transaction{genTx},
	}
}

// MakeBlockFromPrev creates a new Block from an existing Block,
// using the old Block's TransactionOutputs as TransactionInputs
// for the new Transaction.
func MakeBlockFromPrev(b *block.Block) *block.Block {
	newHeader := &block.Header{
		Version:          0,
		PreviousHash:     b.Hash(),
		MerkleRoot:       "",
		DifficultyTarget: "",
		Nonce:            0,
		Timestamp:        0,
	}

	var transactions []*block.Transaction

	for _, tx := range b.Transactions {
		txHash := tx.Hash()
		for i, txo := range tx.Outputs {
			txi := &block.TransactionInput{
				ReferenceTransactionHash: txHash,
				OutputIndex:              uint32(i),
				UnlockingScript:          []byte{},
			}
			txo1 := &block.TransactionOutput{
				Amount:        txo.Amount / 2,
				LockingScript: []byte{},
			}
			tx1 := &block.Transaction{
				Version:  uint32(i),
				Inputs:   []*block.TransactionInput{txi},
				Outputs:  []*block.TransactionOutput{txo1},
				LockTime: 0,
			}
			transactions = append(transactions, tx1)
		}
	}
	return &block.Block{
		Header:       newHeader,
		Transactions: transactions,
	}
}

// UndoBlockFromBlock creates an UndoBlock from a Block.
// This function only works because we're not using inputs from
// other Blocks. It also does not actually take care of amounts
// or public keys.
func UndoBlockFromBlock(b *block.Block) *chainwriter.UndoBlock {
	var transactionHashes []string
	var outputIndexes []uint32
	var amounts []uint32
	var lockingScripts [][]byte
	for _, tx := range b.Transactions {
		for _, txi := range tx.Inputs {
			transactionHashes = append(transactionHashes, txi.ReferenceTransactionHash)
			outputIndexes = append(outputIndexes, txi.OutputIndex)
			amounts = append(amounts, 0)
			lockingScripts = append(lockingScripts, []byte{})
		}
	}
	return &chainwriter.UndoBlock{
		TransactionInputHashes: transactionHashes,
		OutputIndexes:          outputIndexes,
		Amounts:                amounts,
		LockingScripts:         lockingScripts,
	}
}

func MockedBlockWithNCoins(w *wallet.Wallet, n uint32, amt uint32) *block.Block {
	var txs []*block.Transaction
	pK := &pro.PayToPublicKey{
		PublicKey: w.Id.GetPublicKeyBytes(),
	}
	script, err := proto.Marshal(pK)
	if err != nil {
		fmt.Printf("[MockedBlockWithNCoins] unable to marshal")
	}
	for i := uint32(0); i < n; i++ {
		tx := CreateMockedTransaction([]uint32{amt}, []uint32{amt})
		tx.Outputs[0].LockingScript = script
		txs = append(txs, tx)
	}
	b := MockedBlock()
	b.Transactions = txs
	return b
}
