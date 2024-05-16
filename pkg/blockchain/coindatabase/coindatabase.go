package coindatabase

import (
	"Coin/pkg/block"
	"Coin/pkg/blockchain/chainwriter"
	"Coin/pkg/pro"
	"Coin/pkg/utils"
	"bytes"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"google.golang.org/protobuf/proto"
)

// CoinDatabase keeps track of Coins.
// db is a levelDB for persistent storage.
// mainCache stores as many Coins as possible for rapid validation.
// mainCacheSize is how many Coins are currently in the mainCache.
// mainCacheCapacity is the maximum number of Coins that the mainCache
// can store before it must flush.
type CoinDatabase struct {
	db                *leveldb.DB
	mainCache         map[CoinLocator]*Coin
	mainCacheSize     uint32
	mainCacheCapacity uint32
}

// New returns a CoinDatabase given a Config.
func New(config *Config) *CoinDatabase {
	db, err := leveldb.OpenFile(config.DatabasePath, nil)
	if err != nil {
		utils.Debug.Printf("Unable to initialize BlockInfoDatabase with path {%v}", config.DatabasePath)
	}
	return &CoinDatabase{
		db:                db,
		mainCache:         make(map[CoinLocator]*Coin),
		mainCacheSize:     0,
		mainCacheCapacity: config.MainCacheCapacity,
	}
}

// ValidateBlock returns whether a Block's Transactions are valid.
func (coinDB *CoinDatabase) ValidateBlock(transactions []*block.Transaction) bool {
	for _, tx := range transactions {
		if err := coinDB.ValidateTransaction(tx); err != nil {
			utils.Debug.Printf("%v", err)
			return false
		}
	}
	return true
}

// ValidateTransaction checks whether a Transaction's inputs are valid Coins.
// If the Coins have already been spent or do not exist, validateTransaction
// returns an error.
func (coinDB *CoinDatabase) ValidateTransaction(transaction *block.Transaction) error {
	for _, txi := range transaction.Inputs {
		key := makeCoinLocator(txi)
		if coin, ok := coinDB.mainCache[key]; ok {
			if coin.IsSpent {
				return fmt.Errorf("[validateTransaction] coin already spent")
			}
			continue
		}
		if data, err := coinDB.db.Get([]byte(txi.ReferenceTransactionHash), nil); err != nil {
			return fmt.Errorf("[validateTransaction] coin not in leveldb")
		} else {
			pcr := &pro.CoinRecord{}
			if err2 := proto.Unmarshal(data, pcr); err2 != nil {
				utils.Debug.Printf("Failed to unmarshal record from hash {%v}:", txi.ReferenceTransactionHash, err)
			}
			cr := DecodeCoinRecord(pcr)
			if !contains(cr.OutputIndexes, txi.OutputIndex) {
				return fmt.Errorf("[validateTransaction] coinRecord did not contain Coin")
			}
		}
	}
	return nil
}

// UndoCoins handles reverting a Block.
// blocks are the blocks that the coinDB must handle. We use these to get rid of
// created outputs.
// undoBlocks are the undo blocks that the coinDB must handle. We use these to
// reestablish the "used" coins (for inputs) as usable.
//
// At a high level, this function:
// (1) loops through all the block/undoBlock pairings
// (2) erases the coins and coin records created by the block's transaction.
// (3) re-establishes the inputs as usable.
// Note: Students must fill out this function for their project.
func (coinDB *CoinDatabase) UndoCoins(blocks []*block.Block, undoBlocks []*chainwriter.UndoBlock) {
	// loop through all the block/undoBlock pairings || len(blocks) = len(undoBlocks)
	for i := 0; i < len(blocks); i++ {
		// (1) deal with Blocks: erase the coins and the coin record
		for _, tx := range blocks[i].Transactions {
			// delete all the coins created by this block
			for j := 0; j < len(tx.Outputs); j++ {
				cl := CoinLocator{
					ReferenceTransactionHash: tx.Hash(),
					OutputIndex:              uint32(j),
				}
				delete(coinDB.mainCache, cl)
				coinDB.mainCacheSize--
			}
			// delete the coin record
			if err := coinDB.db.Delete([]byte(tx.Hash()), nil); err != nil {
				utils.Debug.Printf("[coinDb.UndoCoins] Error while deleting coin record for hash: %v", tx.Hash())
			}
		}
		// (2) deal with UndoBlocks: re-establish inputs as usable
		for j := 0; j < len(undoBlocks[i].TransactionInputHashes); j++ {
			txHash := undoBlocks[i].TransactionInputHashes[j]
			// mark coin in mainCache as usable once again
			cl := CoinLocator{
				ReferenceTransactionHash: undoBlocks[i].TransactionInputHashes[j],
				OutputIndex:              undoBlocks[i].OutputIndexes[j],
			}
			if coin, ok := coinDB.mainCache[cl]; ok {
				coin.IsSpent = false
			}
			// retrieve coin record from db
			cr := coinDB.getCoinRecordFromDB(txHash)
			//
			if cr != nil {
				// Add coins to record. This is the reestablishing part.
				cr = coinDB.addCoinToRecord(cr, undoBlocks[i], j)
			} else {
				// if there was no coin record to get from the db, we
				// need to make a new one with all the coins from
				// the undoBlock
				cr = &CoinRecord{
					Version:        0,
					OutputIndexes:  undoBlocks[i].OutputIndexes,
					Amounts:        undoBlocks[i].Amounts,
					LockingScripts: undoBlocks[i].LockingScripts,
				}
			}
			// put the updated record back in the db.
			coinDB.putRecordInDB(txHash, cr)
		}
	}
}

// addCoinToRecord adds a Coin to a CoinRecord given an UndoBlock and index,
// returning the updated CoinRecord.
func (coinDB *CoinDatabase) addCoinToRecord(cr *CoinRecord, ub *chainwriter.UndoBlock, index int) *CoinRecord {
	cr.OutputIndexes = append(cr.OutputIndexes, ub.OutputIndexes[index])
	cr.Amounts = append(cr.Amounts, ub.Amounts[index])
	cr.LockingScripts = append(cr.LockingScripts, ub.LockingScripts[index])
	return cr
}

// FlushMainCache flushes the mainCache to the db.
func (coinDB *CoinDatabase) FlushMainCache() {
	// update coin records
	updatedCoinRecords := make(map[string]*CoinRecord)
	for cl, coin := range coinDB.mainCache {
		// don't need to update the coin record if the coin isn't spent
		if !coin.IsSpent {
			continue
		}
		// check whether we already updated this record
		var cr *CoinRecord

		// (1) get our coin record
		// first check our map, in case we already updated the coin record given
		// a previous coin
		if cr2, ok := updatedCoinRecords[cl.ReferenceTransactionHash]; ok {
			cr = cr2
		} else {
			// if we haven't already update this coin record, retrieve from db
			data, err := coinDB.db.Get([]byte(cl.ReferenceTransactionHash), nil)
			if err != nil {
				utils.Debug.Printf("[FlushMainCache] coin record not in leveldb")
			}
			pcr := &pro.CoinRecord{}
			if err = proto.Unmarshal(data, pcr); err != nil {
				utils.Debug.Printf("Failed to unmarshal record from hash {%v}:%v", cl.ReferenceTransactionHash, err)
			}
			cr = DecodeCoinRecord(pcr)
		}
		// (2) we know that the coin is spent given our first check, so we should remove it from the record
		cr = coinDB.removeCoinFromRecord(cr, cl.OutputIndex)
		// add the updated coin record and remove the coin from the cache
		updatedCoinRecords[cl.ReferenceTransactionHash] = cr
		delete(coinDB.mainCache, cl)
	}
	coinDB.mainCacheSize = 0
	// write the new records
	for key, cr := range updatedCoinRecords {
		if len(cr.OutputIndexes) == 0 {
			err := coinDB.db.Delete([]byte(key), nil)
			if err != nil {
				utils.Debug.Printf("[FlushMainCache] failed to delete key {%v}", key)
			}
		} else {
			coinDB.putRecordInDB(key, cr)
		}
	}
}

// StoreBlock handles storing a newly minted Block. It:
// (1) removes spent TransactionOutputs
// (2) stores new TransactionOutputs as Coins in the mainCache
// (3) stores CoinRecords for the Transactions in the db.
//
// Important note: students do NOT have these helper functions. We created them to
// make our lives easier. You should PUSH students to do the same, but they don't
// have to.
func (coinDB *CoinDatabase) StoreBlock(transactions []*block.Transaction) {
	coinDB.updateSpentCoins(transactions)
	coinDB.storeTransactionsInMainCache(transactions)
	coinDB.storeTransactionsInDB(transactions)
}

// updateSpentCoins marks Coins in the mainCache as spent and removes
// Coins from their CoinRecords if they are not in the mainCache.
//
// Note: NOT included in the stencil.
func (coinDB *CoinDatabase) updateSpentCoins(transactions []*block.Transaction) {
	// loop through all the transactions from the block,
	// marking the coins used to create the inputs as spent.
	for _, tx := range transactions {
		for _, txi := range tx.Inputs {
			// get the coin locator for the input
			cl := makeCoinLocator(txi)
			// mark coins in the main cache as spent
			if coin, ok := coinDB.mainCache[cl]; ok {
				coin.IsSpent = true
				coinDB.mainCache[cl] = coin
			} else {
				// if the coin is not in the cache,
				// we have to remove the coin from the
				// database.
				txHash := tx.Hash()
				// remove the spent coin from the db
				coinDB.removeCoinFromDB(txHash, cl)
			}
		}
	}
}

// removeCoinFromDB removes a Coin from a CoinRecord, deleting the CoinRecord
// from the db entirely if it is the last remaining Coin in the CoinRecord.
func (coinDB *CoinDatabase) removeCoinFromDB(txHash string, cl CoinLocator) {
	cr := coinDB.getCoinRecordFromDB(txHash)
	switch {
	case cr == nil:
		return
	case len(cr.Amounts) <= 1:
		if err := coinDB.db.Delete([]byte(txHash), nil); err != nil {
			utils.Debug.Printf("[removeCoinFromDB] failed to remove {%v} from db", txHash)
		}
	default:
		cr = coinDB.removeCoinFromRecord(cr, cl.OutputIndex)
		coinDB.putRecordInDB(txHash, cr)
	}
}

// putRecordInDB puts a CoinRecord into the db.
func (coinDB *CoinDatabase) putRecordInDB(txHash string, cr *CoinRecord) {
	record := EncodeCoinRecord(cr)
	bytes, err := proto.Marshal(record)
	if err != nil {
		utils.Debug.Printf("[coindatabase.putRecordInDB] Unable to marshal coin record for key {%v}", txHash)
	}
	if err2 := coinDB.db.Put([]byte(txHash), bytes, nil); err2 != nil {
		utils.Debug.Printf("Unable to store coin record for key {%v}", txHash)
	}
}

// removeCoinFromRecord returns an updated CoinRecord. It removes the Coin
// with the given outputIndex, if the Coin exists in the CoinRecord.
func (coinDB *CoinDatabase) removeCoinFromRecord(cr *CoinRecord, outputIndex uint32) *CoinRecord {
	index := indexOf(cr.OutputIndexes, outputIndex)
	if index < 0 {
		return cr
	}
	cr.OutputIndexes = append(cr.OutputIndexes[:index], cr.OutputIndexes[index+1:]...)
	cr.Amounts = append(cr.Amounts[:index], cr.Amounts[index+1:]...)
	cr.LockingScripts = append(cr.LockingScripts[:index], cr.LockingScripts[index+1:]...)
	return cr
}

// storeTransactionsInMainCache generates Coins from a slice of Transactions
// and stores them in the CoinDatabase's mainCache. It flushes the mainCache
// if it reaches mainCacheCapacity.
//
// At a high level, this function:
// (1) loops through the newly created transaction outputs from the Block's
// transactions.
// (2) flushes our cache if we reach capacity
// (3) creates a coin (value) and coin locator (key) for each output,
// adding them to the main cache.
//
// Note: NOT included in the stencil.
func (coinDB *CoinDatabase) storeTransactionsInMainCache(transactions []*block.Transaction) {
	for _, tx := range transactions {
		// get hash now, which we will use in creating coin locators
		// for each output later
		txHash := tx.Hash()
		for i, txo := range tx.Outputs {
			// check whether we're approaching our capacity and flush if we are
			if coinDB.mainCacheSize+uint32(len(tx.Outputs)) >= coinDB.mainCacheCapacity {
				coinDB.FlushMainCache()
			}
			// actually create the coin
			coin := &Coin{
				TransactionOutput: txo,
				IsSpent:           false,
			}
			// create the coin locator, which is they key to the coin
			cl := CoinLocator{
				ReferenceTransactionHash: txHash,
				OutputIndex:              uint32(i),
			}
			// add the coin to main cach and increment the size of the main cache.
			coinDB.mainCache[cl] = coin
			coinDB.mainCacheSize++
		}
	}
}

// storeTransactionsInDB generates CoinRecords from a slice of Transactions and
// stores them in the CoinDatabase's db.
//
// At a high level, this function:
// (1) creates coin records for the block's transactions
// (2) stores those coin records in the db
//
// Note: NOT included in the stencil.
func (coinDB *CoinDatabase) storeTransactionsInDB(transactions []*block.Transaction) {
	for _, tx := range transactions {
		cr := coinDB.createCoinRecord(tx)
		txHash := tx.Hash()
		coinDB.putRecordInDB(txHash, cr)
	}
}

// createCoinRecord returns a CoinRecord for the provided Transaction.
func (coinDB *CoinDatabase) createCoinRecord(tx *block.Transaction) *CoinRecord {
	var outputIndexes []uint32
	var amounts []uint32
	var LockingScripts [][]byte
	for i, txo := range tx.Outputs {
		outputIndexes = append(outputIndexes, uint32(i))
		amounts = append(amounts, txo.Amount)
		LockingScripts = append(LockingScripts, txo.LockingScript)
	}
	cr := &CoinRecord{
		Version:        0,
		OutputIndexes:  outputIndexes,
		Amounts:        amounts,
		LockingScripts: LockingScripts,
	}
	return cr
}

// getCoinRecordFromDB returns a CoinRecord from the db given a hash.
func (coinDB *CoinDatabase) getCoinRecordFromDB(txHash string) *CoinRecord {
	if data, err := coinDB.db.Get([]byte(txHash), nil); err != nil {
		utils.Debug.Printf("[getCoinRecordFromDB] coin not in leveldb")
		return nil
	} else {
		pcr := &pro.CoinRecord{}
		if err = proto.Unmarshal(data, pcr); err != nil {
			utils.Debug.Printf("Failed to unmarshal record from hash {%v}:", txHash, err)
		}
		cr := DecodeCoinRecord(pcr)
		return cr
	}
}

// GetCoin returns a Coin given a CoinLocator. It first checks the
// mainCache, then checks the db. If the Coin doesn't exist,
// it returns nil.
func (coinDB *CoinDatabase) GetCoin(cl CoinLocator) *Coin {
	if coin, ok := coinDB.mainCache[cl]; ok {
		return coin
	}
	cr := coinDB.getCoinRecordFromDB(cl.ReferenceTransactionHash)
	if cr == nil {
		return nil
	}
	index := indexOf(cr.OutputIndexes, cl.OutputIndex)
	if index < 0 {
		return nil
	}
	return &Coin{
		TransactionOutput: &block.TransactionOutput{
			Amount:        cr.Amounts[index],
			LockingScript: cr.LockingScripts[index],
		},
		IsSpent: false,
	}
}

//GetBalance returns the current balance of the publicKey
func (coinDB *CoinDatabase) GetBalance(publicKey []byte) uint32 {
	coinDB.FlushMainCache()
	balance := uint32(0)
	iterator := coinDB.db.NewIterator(nil, nil)
	for iterator.Next() {
		value := iterator.Value()
		pcr := &pro.CoinRecord{}
		if err := proto.Unmarshal(value, pcr); err != nil {
			utils.Debug.Printf("[GetBalance] Failed to unmarshal record from coinDB iterator:")
		}
		cr := DecodeCoinRecord(pcr)
		for i, pK := range cr.LockingScripts {
			if bytes.Equal(pK, publicKey) {
				balance += cr.Amounts[i]
			}
		}
	}
	iterator.Release()
	return balance
}

// contains returns true if an int slice s contains element e, false if it does not.
func contains(s []uint32, e uint32) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

//indexOf returns the index of element e in int slice s, -1 if the element does not exist.
func indexOf(s []uint32, e uint32) int {
	for i, a := range s {
		if a == e {
			return i
		}
	}
	return -1
}

// Close is used to actually shut down the db (for testing purposes)
func (coinDB *CoinDatabase) Close() {
	coinDB.db.Close()
}
