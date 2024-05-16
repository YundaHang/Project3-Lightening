package blockchain

import (
	"Coin/pkg/block"
	"Coin/pkg/blockchain/blockinfodatabase"
	"Coin/pkg/blockchain/chainwriter"
	"Coin/pkg/blockchain/coindatabase"
	"Coin/pkg/utils"
	"math"
)

// BlockChain is the main type of this project.
// Length is the length of the active chain.
// LastBlock is the last block of the active chain.
// LastHash is the hash of the last block of the active chain.
// UnsafeHashes are the hashes of the "unsafe" blocks on the
// active chain. These "unsafe" blocks may be reverted during a
// fork.
// maxHashes is the number of unsafe hashes that the chain keeps track of.
// BlockInfoDB is a pointer to a block info database
// ChainWriter is a pointer to a chain writer.
// CoinDB is a pointer to a coin database.
type BlockChain struct {
	Address      string
	Length       uint32
	LastBlock    *block.Block
	LastHash     string
	UnsafeHashes []string
	maxHashes    int
	ConfirmBlock chan *block.Block

	BlockInfoDB *blockinfodatabase.BlockInfoDatabase
	ChainWriter *chainwriter.ChainWriter
	CoinDB      *coindatabase.CoinDatabase
}

// New returns a blockchain given a Config.
func New(config *Config) *BlockChain {
	genBlock := GenesisBlock(config)
	hash := genBlock.Hash()
	// set up db paths
	blockInfoDBConfig := blockinfodatabase.DefaultConfig()
	blockInfoDBConfig.DatabasePath = config.BlockInfoDBPath

	chainWriterConfig := chainwriter.DefaultConfig()
	chainWriterConfig.DataDirectory = config.ChainWriterDBPath

	coinDBConfig := coindatabase.DefaultConfig()
	coinDBConfig.DatabasePath = config.CoinDBPath

	bc := &BlockChain{
		Length:       1,
		LastBlock:    genBlock,
		LastHash:     hash,
		UnsafeHashes: []string{hash},
		maxHashes:    6,
		BlockInfoDB:  blockinfodatabase.New(blockInfoDBConfig),
		ChainWriter:  chainwriter.New(chainWriterConfig),
		CoinDB:       coindatabase.New(coinDBConfig),
	}
	// have to store the genesis block
	bc.CoinDB.StoreBlock(genBlock.Transactions)
	ub := &chainwriter.UndoBlock{}
	br := bc.ChainWriter.StoreBlock(genBlock, ub, 1)
	bc.BlockInfoDB.StoreBlockRecord(hash, br)
	return bc
}

// GenesisBlock creates the genesis Block, using the Config's
// InitialSubsidy and GenesisPublicKey.
func GenesisBlock(config *Config) *block.Block {
	txo := &block.TransactionOutput{
		Amount:        config.InitialSubsidy,
		LockingScript: config.GenesisPublicKey,
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

// HandleBlock handles a new Block. At a high level, it:
// (1) Validates and stores the Block.
// (2) Stores the Block and resulting Undoblock to Disk.
// (3) Stores the BlockRecord in the BlockInfoDatabase.
// (4) Handles a fork, if necessary.
// (5) Updates the BlockChain's fields.
func (bc *BlockChain) HandleBlock(b *block.Block) {
	appends := bc.appendsToActiveChain(b)
	blockHash := b.Hash()

	// 1. Validate Block
	if appends && !bc.CoinDB.ValidateBlock(b.Transactions) {
		return
	}

	// 2. Make Undo Block
	ub := bc.makeUndoBlock(b.Transactions)

	// 4. Get BlockRecord for previous Block
	previousBr := bc.BlockInfoDB.GetBlockRecord(b.Header.PreviousHash)

	// 5. Store UndoBlock and Block to Disk
	height := previousBr.Height + 1
	br := bc.ChainWriter.StoreBlock(b, ub, height)

	// 6. Store BlockRecord to BlockInfoDatabase
	bc.BlockInfoDB.StoreBlockRecord(blockHash, br)

	if appends {
		// 7. Handle appending Block
		bc.CoinDB.StoreBlock(b.Transactions)
		bc.Length++
		bc.LastBlock = b
		bc.LastHash = blockHash
		if len(bc.UnsafeHashes) >= 6 {
			bc.UnsafeHashes = bc.UnsafeHashes[1:]
		}
		bc.UnsafeHashes = append(bc.UnsafeHashes, blockHash)
	} else if height > bc.Length {
		// 8. Handle fork
		bc.handleFork(b, height)
	}
}

// handleFork updates the BlockChain when a fork occurs. First, it
// finds the Blocks the BlockChain must revert. Once found, it uses
// those Blocks to update the CoinDatabase. Lastly, it updates the
// BlockChain's fields to reflect the fork.
func (bc *BlockChain) handleFork(b *block.Block, height uint32) {
	// (1) Make sure that this is a valid fork
	forkLength, ancestorHash := bc.getForkLengthAndAncestor(b.Hash())
	if forkLength < 0 {
		utils.Debug.Printf("[blockchain.handleFork] fork was invalid")
		return
	}

	// (2) retrieve the blocks on the existing main chain
	blocks, undoBlocks := bc.getBlocksAndUndoBlocks(forkLength, bc.LastHash)

	// (3) update unsafe hashes
	min := int(math.Min(float64(6), float64(forkLength)))
	// delete unsafe hashes up until the common ancestor
	for i := min - 1; i > 0; i-- {
		if bc.UnsafeHashes[i] == ancestorHash {
			break
		} else {
			bc.UnsafeHashes = bc.UnsafeHashes[:len(bc.UnsafeHashes)-1]
		}
	}
	// add in the new hashes (reverse order because that's how getBlocksAndUndoBlocks
	// returns them)
	for i := len(blocks) - 1; i >= 0; i-- {
		bc.UnsafeHashes = append(bc.UnsafeHashes, blocks[i].Hash())
	}

	// (4) Reflect changes in coinDB
	bc.CoinDB.UndoCoins(blocks, undoBlocks)

	// (5) Store our new blocks in the coinDB!
	for _, bl := range blocks {
		if !bc.CoinDB.ValidateBlock(bl.Transactions) {
			utils.Debug.Printf("Validation failed for forked block {%v}", b.Hash())
		}
		bc.CoinDB.StoreBlock(bl.Transactions)
	}

	// (5) Update blockchain fields
	bc.LastBlock = b
	bc.LastHash = b.Hash()
	bc.Length = height
}

// makeUndoBlock returns an UndoBlock given a slice of Transactions.
func (bc *BlockChain) makeUndoBlock(txs []*block.Transaction) *chainwriter.UndoBlock {
	var transactionHashes []string
	var outputIndexes []uint32
	var amounts []uint32
	var lockingScripts [][]byte
	for _, tx := range txs {
		for _, txi := range tx.Inputs {
			cl := coindatabase.CoinLocator{
				ReferenceTransactionHash: txi.ReferenceTransactionHash,
				OutputIndex:              txi.OutputIndex,
			}
			coin := bc.CoinDB.GetCoin(cl)
			// if the coin is nil it means this isn't even a possible fork
			if coin == nil {
				return &chainwriter.UndoBlock{
					TransactionInputHashes: nil,
					OutputIndexes:          nil,
					Amounts:                nil,
					LockingScripts:         nil,
				}
			}
			transactionHashes = append(transactionHashes, txi.ReferenceTransactionHash)
			outputIndexes = append(outputIndexes, txi.OutputIndex)
			amounts = append(amounts, coin.TransactionOutput.Amount)
			lockingScripts = append(lockingScripts, coin.TransactionOutput.LockingScript)
		}
	}
	return &chainwriter.UndoBlock{
		TransactionInputHashes: transactionHashes,
		OutputIndexes:          outputIndexes,
		Amounts:                amounts,
		LockingScripts:         lockingScripts,
	}
}

// GetBlock uses the ChainWriter to retrieve a Block from Disk
// given that Block's hash
func (bc *BlockChain) GetBlock(blockHash string) *block.Block {
	br := bc.BlockInfoDB.GetBlockRecord(blockHash)
	fi := &chainwriter.FileInfo{
		FileName:    br.BlockFile,
		StartOffset: br.BlockStartOffset,
		EndOffset:   br.BlockEndOffset,
	}
	return bc.ChainWriter.ReadBlock(fi)
}

// getUndoBlock uses the ChainWriter to retrieve an UndoBlock
// from Disk given the corresponding Block's hash
func (bc *BlockChain) getUndoBlock(blockHash string) *chainwriter.UndoBlock {
	br := bc.BlockInfoDB.GetBlockRecord(blockHash)
	fi := &chainwriter.FileInfo{
		FileName:    br.UndoFile,
		StartOffset: br.UndoStartOffset,
		EndOffset:   br.UndoEndOffset,
	}
	return bc.ChainWriter.ReadUndoBlock(fi)
}

// GetBlocks retrieves a slice of blocks from the main chain given a
// starting and ending height, inclusive. Given a chain of length 50,
// GetBlocks(10, 20) returns blocks 10 through 20.
func (bc *BlockChain) GetBlocks(start, end uint32) []*block.Block {
	if start >= end || end <= 0 || start <= 0 || end > bc.Length {
		utils.Debug.Printf("cannot get chain blocks with values start: %v end: %v", start, end)
	}

	var blocks []*block.Block
	currentHeight := bc.Length
	nextHash := bc.LastBlock.Hash()

	for currentHeight >= start {
		br := bc.BlockInfoDB.GetBlockRecord(nextHash)
		fi := &chainwriter.FileInfo{
			FileName:    br.BlockFile,
			StartOffset: br.BlockStartOffset,
			EndOffset:   br.BlockEndOffset,
		}
		if currentHeight <= end {
			nextBlock := bc.ChainWriter.ReadBlock(fi)
			blocks = append(blocks, nextBlock)
		}
		nextHash = br.Header.PreviousHash
		currentHeight--
	}
	return reverseBlocks(blocks)
}

// GetHashes retrieves a slice of hashes from the main chain given a
// starting and ending height, inclusive. Given a BlockChain of length
// 50, GetHashes(10, 20) returns the hashes of Blocks 10 through 20.
func (bc *BlockChain) GetHashes(start, end uint32) []string {
	if start >= end || end <= 0 || start <= 0 || end > bc.Length {
		utils.Debug.Printf("cannot get chain blocks with values start: %v end: %v", start, end)
	}

	var hashes []string
	currentHeight := bc.Length
	nextHash := bc.LastBlock.Hash()

	for currentHeight >= start {
		br := bc.BlockInfoDB.GetBlockRecord(nextHash)
		if currentHeight <= end {
			hashes = append(hashes, nextHash)
		}
		nextHash = br.Header.PreviousHash
		currentHeight--
	}
	return reverseHashes(hashes)
}

// appendsToActiveChain returns whether a Block appends to the
// BlockChain's active chain or not.
func (bc *BlockChain) appendsToActiveChain(b *block.Block) bool {
	return bc.LastBlock.Hash() == b.Header.PreviousHash
}

// getForkLength returns the length of a fork, exclusive of the
// common ancestor with the main chain. If it returns -1, the fork is
// invalid
func (bc *BlockChain) getForkLengthAndAncestor(startHash string) (int, string) {
	// turn unsafeHashes into map for easier checking
	unsafeHashes := make(map[string]bool)
	for _, h := range bc.UnsafeHashes {
		unsafeHashes[h] = true
	}
	forkLength := 0
	nextHash := startHash
	// keep going backwards on the forked chain until we find the common ancestor
	// with the main chain. Update forkLength as we go
	for i := 0; i < len(bc.UnsafeHashes)+1; i++ {
		if _, ok := unsafeHashes[nextHash]; ok {
			return forkLength, nextHash
		}
		br := bc.BlockInfoDB.GetBlockRecord(nextHash)
		nextHash = br.Header.PreviousHash
		forkLength++
	}
	// Couldn't find an ancestor hash, so invalid fork
	return -1, ""
}

// getBlocksAndUndoBlocks returns a slice of n Blocks with a
// corresponding slice of n UndoBlocks. They are returned in reverse order:
// given block heights of 1, 2, and 3, this function will return the blocks
// as [3, 2, 1] (to make undoing easier)
func (bc *BlockChain) getBlocksAndUndoBlocks(n int, hash string) ([]*block.Block, []*chainwriter.UndoBlock) {
	var blocks []*block.Block
	var undoBlocks []*chainwriter.UndoBlock
	nextHash := hash
	for i := 0; i < n; i++ {
		b := bc.GetBlock(nextHash)
		ub := bc.getUndoBlock(nextHash)
		blocks = append(blocks, b)
		undoBlocks = append(undoBlocks, ub)
		nextHash = b.Header.PreviousHash
	}
	return blocks, undoBlocks
}

// reverseBlocks returns a reversed slice of Blocks.
func reverseBlocks(s []*block.Block) []*block.Block {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// reverseHashes returns a reversed slice of hashes.
func reverseHashes(s []string) []string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func (bc *BlockChain) SetAddress(address string) {
	bc.Address = address
}

func (bc *BlockChain) GetBalance(pk []byte) uint32 {
	return bc.CoinDB.GetBalance(pk)
}

func (bc *BlockChain) List() []*block.Block {
	return bc.GetBlocks(1, bc.Length)
}

// GetInputSums returns a slice of summed transaction input totals, given a slice of transactions.
// The indexes of the slice of totals correspond to the indexes of the transactions.
// In other words, the sum of the inputs for txs[3] is sums[3]
func (bc *BlockChain) GetInputSums(txs []*block.Transaction) []uint32 {
	var sums []uint32
	for _, tx := range txs {
		sum := uint32(0)
		for _, txi := range tx.Inputs {
			cl := coindatabase.CoinLocator{
				ReferenceTransactionHash: txi.ReferenceTransactionHash,
				OutputIndex:              txi.OutputIndex,
			}
			coin := bc.CoinDB.GetCoin(cl)
			if coin == nil {
				utils.Debug.Printf("[blockchain.GetCoins] Error: could not find coin")
			} else {
				sum += coin.TransactionOutput.Amount
			}
		}
		sums = append(sums, sum)
	}
	return sums
}
