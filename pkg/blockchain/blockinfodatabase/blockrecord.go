package blockinfodatabase

import (
	"Coin/pkg/block"
	"Coin/pkg/pro"
)

// BlockRecord contains information about where a Block
// and its UndoBlock are stored on Disk.
// Header is the Block's Header.
// Height is the height of the Block.
// NumberOfTransactions is the number of Transactions in the Block.
// BlockFile is the name of the file where the Block is stored.
// BlockStartOffset is the starting offset of the Block within the
// BlockFile.
// BlockEndOffset is the ending offset of the Block within
// the BlockFile.
// UndoFile is the name of the file where the UndoBlock is stored.
// UndoStartOffset is the starting offset of the UndoBlock within
// the UndoFile.
// UndoEndOffset is the ending offset of the UndoBlock within the
// UndoFile.
type BlockRecord struct {
	Header               *block.Header
	Height               uint32
	NumberOfTransactions uint32

	BlockFile        string
	BlockStartOffset uint32
	BlockEndOffset   uint32

	UndoFile        string
	UndoStartOffset uint32
	UndoEndOffset   uint32
}

// EncodeBlockRecord returns a pro.BlockRecord given a BlockRecord.
func EncodeBlockRecord(br *BlockRecord) *pro.BlockRecord {
	return &pro.BlockRecord{
		Header:               block.EncodeHeader(br.Header),
		Height:               br.Height,
		NumberOfTransactions: br.NumberOfTransactions,
		BlockFile:            br.BlockFile,
		BlockStartOffset:     br.BlockStartOffset,
		BlockEndOffset:       br.BlockEndOffset,
		UndoFile:             br.UndoFile,
		UndoStartOffset:      br.UndoStartOffset,
		UndoEndOffset:        br.UndoEndOffset,
	}
}

// DecodeBlockRecord returns a BlockRecord given a pro.BlockRecord.
func DecodeBlockRecord(pbr *pro.BlockRecord) *BlockRecord {
	return &BlockRecord{
		Header:               block.DecodeHeader(pbr.GetHeader()),
		Height:               pbr.GetHeight(),
		NumberOfTransactions: pbr.GetNumberOfTransactions(),
		BlockFile:            pbr.GetBlockFile(),
		BlockStartOffset:     pbr.GetBlockStartOffset(),
		BlockEndOffset:       pbr.GetBlockEndOffset(),
		UndoFile:             pbr.GetUndoFile(),
		UndoStartOffset:      pbr.GetUndoStartOffset(),
		UndoEndOffset:        pbr.GetUndoEndOffset(),
	}
}
