package chainwriter

import (
	"Coin/pkg/block"
	"Coin/pkg/blockchain/blockinfodatabase"
	"Coin/pkg/pro"
	"Coin/pkg/utils"
	"google.golang.org/protobuf/proto"
	"log"
	"os"
	"strconv"
)

// ChainWriter handles all I/O for the BlockChain. It stores and retrieves
// Blocks and UndoBlocks.
// See config.go for more information on its fields.
// Block files are of the format:
// "DataDirectory/BlockFileName_CurrentBlockFileNumber.FileExtension"
// Ex: "data/block_0.txt"
// UndoBlock files are of the format:
// "DataDirectory/UndoFileName_CurrentUndoFileNumber.FileExtension"
// Ex: "data/undo_0.txt"
type ChainWriter struct {
	// data storage information
	FileExtension string
	DataDirectory string

	// block information
	BlockFileName          string
	CurrentBlockFileNumber uint32
	CurrentBlockOffset     uint32
	MaxBlockFileSize       uint32

	// undo block information
	UndoFileName          string
	CurrentUndoFileNumber uint32
	CurrentUndoOffset     uint32
	MaxUndoFileSize       uint32
}

// New returns a ChainWriter given a Config.
func New(config *Config) *ChainWriter {
	if err := os.Mkdir(config.DataDirectory, 0700); err != nil {
		log.Fatalf("Could not create ChainWriter's data directory")
	}
	return &ChainWriter{
		FileExtension:          config.FileExtension,
		DataDirectory:          config.DataDirectory,
		BlockFileName:          config.BlockFileName,
		CurrentBlockFileNumber: 0,
		CurrentBlockOffset:     0,
		MaxBlockFileSize:       config.MaxBlockFileSize,
		UndoFileName:           config.UndoFileName,
		CurrentUndoFileNumber:  0,
		CurrentUndoOffset:      0,
		MaxUndoFileSize:        config.MaxUndoFileSize,
	}
}

// StoreBlock stores a Block and its corresponding UndoBlock to Disk,
// returning a BlockRecord that contains information for later retrieval.
func (cw *ChainWriter) StoreBlock(bl *block.Block, undoBlock *UndoBlock, height uint32) *blockinfodatabase.BlockRecord {
	// serialize block
	b := block.EncodeBlock(bl)
	serializedBlock, err := proto.Marshal(b)
	if err != nil {
		utils.Debug.Printf("Failed to marshal block")
	}
	// serialize undo block
	ub := EncodeUndoBlock(undoBlock)
	serializedUndoBlock, err := proto.Marshal(ub)
	if err != nil {
		utils.Debug.Printf("Failed to marshal undo block")
	}
	// write block to disk
	bfi := cw.WriteBlock(serializedBlock)
	// create an empty file info, which we will update if the function is passed an undo block.
	ufi := &FileInfo{}
	if undoBlock.Amounts != nil {
		ufi = cw.WriteUndoBlock(serializedUndoBlock)
	}

	return &blockinfodatabase.BlockRecord{
		Header:               bl.Header,
		Height:               height,
		NumberOfTransactions: uint32(len(bl.Transactions)),
		BlockFile:            bfi.FileName,
		BlockStartOffset:     bfi.StartOffset,
		BlockEndOffset:       bfi.EndOffset,
		UndoFile:             ufi.FileName,
		UndoStartOffset:      ufi.StartOffset,
		UndoEndOffset:        ufi.EndOffset,
	}
}

// WriteBlock writes a serialized Block to Disk and returns
// a FileInfo for storage information.
//
// At a high level, here's what this function is doing:
// (1) checking to make sure we still have space for this
// block in our current file, and updating the file if necessary.
// (2) opening a path to the file
// (3) writing our serialized block to that file, making use
// of our helper function writeToDisk(fileName, data)
// (4) creating a file info that returns the file name,
// starting offset in the file, and the ending offset in the file.
// (5) updating our offset fo the next write.
// (6) returning the FileInfo, which will later be used by the
// BlockInfoDB when filling out a BlockRecord.
func (cw *ChainWriter) WriteBlock(serializedBlock []byte) *FileInfo {
	// need to know the length of the block
	length := uint32(len(serializedBlock))
	// if we don't have enough space for this block in the current file,
	// we have to update our file by changing the current file number
	// and resetting the start offset to zero (so we write at the beginning
	// of the file again.
	// (recall format from above: "data/block_0.txt")
	if cw.CurrentBlockOffset+length >= cw.MaxBlockFileSize {
		cw.CurrentBlockOffset = 0
		cw.CurrentBlockFileNumber++
	}
	// create path to correct file, following format
	// "DataDirectory/BlockFileName_CurrentBlockFileNumber.FileExtension"
	// Ex: "data/block_0.txt"
	fileName := cw.DataDirectory + "/" + cw.BlockFileName + "_" + strconv.Itoa(int(cw.CurrentBlockFileNumber)) + cw.FileExtension
	// write serialized block to disk
	writeToDisk(fileName, serializedBlock)
	// create a file info object with the starting and ending offsets of the serialized block
	fi := &FileInfo{
		FileName:    fileName,
		StartOffset: cw.CurrentBlockOffset,
		EndOffset:   cw.CurrentBlockOffset + length,
	}
	// update offset for next write
	cw.CurrentBlockOffset += length
	// return the file info
	return fi
}

// WriteUndoBlock writes a serialized UndoBlock to Disk and returns
// a FileInfo for storage information.
//
// The only difference between this and WriteBlock() is the fields
// we're updating when writing an UndoBlock.
//
// At a high level, here's what this function is doing:
// (1) checking to make sure we still have space for this
// undo block in our current undo file, and updating the undo file
// if necessary.
// (2) opening a path to the undo file
// (3) writing our serialized undo block to that undo file, making use
// of our helper function writeToDisk(fileName, data)
// (4) creating a file info that returns the undo file name,
// starting offset in the undo file, and the ending undo offset in the
// undo file.
// (5) updating our undo offset fo the next write.
// (6) returning the FileInfo, which will later be used by the
// BlockInfoDB when filling out a BlockRecord.
func (cw *ChainWriter) WriteUndoBlock(serializedUndoBlock []byte) *FileInfo {
	// need to know the length of the block
	length := uint32(len(serializedUndoBlock))
	// if we don't have enough space for this undo block in the current undo file,
	// we have to update our undo file by changing the current undo file number
	// and resetting the start undo offset to zero (so we write at the beginning
	// of the undo file again.
	// (recall format from above: "data/undo_0.txt")
	if cw.CurrentUndoOffset+length >= cw.MaxUndoFileSize {
		cw.CurrentUndoOffset = 0
		cw.CurrentUndoFileNumber++
	}
	// create path to correct file, following format
	// "DataDirectory/BlockFileName_CurrentBlockFileNumber.FileExtension"
	// Ex: "data/undo_0.txt"
	fileName := cw.DataDirectory + "/" + cw.UndoFileName + "_" + strconv.Itoa(int(cw.CurrentUndoFileNumber)) + cw.FileExtension
	// write serialized undo block to disk
	writeToDisk(fileName, serializedUndoBlock)
	// create a file info object with the starting and ending undo offsets of the serialized
	// undo block
	fi := &FileInfo{
		FileName:    fileName,
		StartOffset: cw.CurrentUndoOffset,
		EndOffset:   cw.CurrentUndoOffset + length,
	}
	// update offset for next write
	cw.CurrentUndoOffset += length
	// return the file info
	return fi
}

// ReadBlock returns a Block given a FileInfo.
func (cw *ChainWriter) ReadBlock(fi *FileInfo) *block.Block {
	bytes := readFromDisk(fi)
	pb := &pro.Block{}
	if err := proto.Unmarshal(bytes, pb); err != nil {
		utils.Debug.Printf("failed to unmarshal block from file info {%v}", fi)
	}
	return block.DecodeBlock(pb)
}

// ReadUndoBlock returns an UndoBlock given a FileInfo.
func (cw *ChainWriter) ReadUndoBlock(fi *FileInfo) *UndoBlock {
	bytes := readFromDisk(fi)
	pub := &pro.UndoBlock{}
	if err := proto.Unmarshal(bytes, pub); err != nil {
		utils.Debug.Printf("failed to unmarshal undo block from file info {%v}", fi)
	}
	return DecodeUndoBlock(pub)
}
