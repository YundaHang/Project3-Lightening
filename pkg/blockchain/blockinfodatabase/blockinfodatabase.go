package blockinfodatabase

import (
	"Coin/pkg/pro"
	"Coin/pkg/utils"
	"github.com/syndtr/goleveldb/leveldb"
	"google.golang.org/protobuf/proto"
)

// BlockInfoDatabase is a wrapper for a levelDB
type BlockInfoDatabase struct {
	db *leveldb.DB
}

// New returns a BlockInfoDatabase given a Config
func New(config *Config) *BlockInfoDatabase {
	db, err := leveldb.OpenFile(config.DatabasePath, nil)
	if err != nil {
		utils.Debug.Printf("Unable to initialize BlockInfoDatabase with path {%v}", config.DatabasePath)
	}
	return &BlockInfoDatabase{db: db}
}

// StoreBlockRecord stores a BlockRecord in the BlockInfoDatabase.
// hash is the hash of the block, and the key for the blockRecord.
// blockRecord is the value we're storing in the database
//
// At a high level, here's what this function is doing:
// (1) converting a blockRecord to a protobuf version (for more effective storage)
// (2) converting the protobuf to bytes
// (3) storing the byte version of the blockRecord in our database
func (blockInfoDB *BlockInfoDatabase) StoreBlockRecord(hash string, blockRecord *BlockRecord) {
	// convert the blockRecord to a proto version
	protoRecord := EncodeBlockRecord(blockRecord)
	// marshaling (serializing) the proto record to bytes
	bytes, err := proto.Marshal(protoRecord)
	// checking that the marshalling process didn't throw an error
	if err != nil {
		utils.Debug.Printf("Failed to marshal protoRecord:", err)
	}
	// attempting to store the bytes in our database AND checking to make
	// sure that the storing process doesn't fail. The Put(key, value, writeOptions)
	// function is levelDB's.
	if err = blockInfoDB.db.Put([]byte(hash), bytes, nil); err != nil {
		utils.Debug.Printf("Unable to store block protoRecord for hash {%v}", hash)
	}
}

// GetBlockRecord returns a BlockRecord from the BlockInfoDatabase given
// the relevant block's hash.
// hash is the hash of the block, and the key for the blockRecord.
//
// At a high level, here's what this function is doing:
// (1) retrieving the byte version of the protobuf record.
// (2) converting the bytes to protobuf
// (3) converting the protobuf to blockRecord and returning that.
func (blockInfoDB *BlockInfoDatabase) GetBlockRecord(hash string) *BlockRecord {
	// attempting to retrieve the byte-version of the protobuf record
	// from our database AND checking that the value is retrieved successfully.
	// The Get(key, writeOptions) function is levelDB's.
	data, err := blockInfoDB.db.Get([]byte(hash), nil)
	if err != nil {
		utils.Debug.Printf("Unable to get block record for hash {%v}", hash)
	}
	// creating a protobuf blockRecord object to fill
	protoRecord := &pro.BlockRecord{}
	// unmarshalling (deserializing) the bytes stored in the database into the
	// protobuf object created on line 66. Checking that the conversion process
	// from bytes to protobuf object succeeds.
	if err = proto.Unmarshal(data, protoRecord); err != nil {
		utils.Debug.Printf("Failed to unmarshal record from hash {%v}:", hash, err)
	}
	// convert the protobuf record to a normal blockRecord and returning that.
	return DecodeBlockRecord(protoRecord)
}

// Close is used to actually shut down the db (for testing purposes)
func (blockInfoDB *BlockInfoDatabase) Close() {
	blockInfoDB.db.Close()
}
