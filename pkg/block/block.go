package block

import (
	"Coin/pkg/pro"
	"Coin/pkg/utils"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"google.golang.org/protobuf/proto"
	"strconv"
	"strings"
)

// Header provides information about the Block.
// Version is the Block's version.
// PreviousHash is the hash of the previous Block.
// MerkleRoot is the hash of all the Block's Transactions.
// DifficultyTarget is the difficulty of achieving a winning Nonce.
// Nonce is a "number only used once" that satisfies the DifficultyTarget.
// Timestamp is when the Block was successfully mined.
type Header struct {
	Version          uint32
	PreviousHash     string
	MerkleRoot       string
	DifficultyTarget string
	Nonce            uint32
	Timestamp        uint32
}

// Block includes a Header and a slice of Transactions.
type Block struct {
	Header       *Header
	Transactions []*Transaction
}

// EncodeHeader returns a pro.Header given a Header.
func EncodeHeader(header *Header) *pro.Header {
	if header == nil {
		fmt.Printf("header was nil")
	}

	return &pro.Header{
		Version:          header.Version,
		PreviousHash:     header.PreviousHash,
		MerkleRoot:       header.MerkleRoot,
		DifficultyTarget: header.DifficultyTarget,
		Nonce:            header.Nonce,
		Timestamp:        header.Timestamp,
	}
}

// DecodeHeader returns a Header given a pro.Header.
func DecodeHeader(pheader *pro.Header) *Header {
	return &Header{
		Version:          pheader.GetVersion(),
		PreviousHash:     pheader.GetPreviousHash(),
		MerkleRoot:       pheader.GetMerkleRoot(),
		DifficultyTarget: pheader.GetDifficultyTarget(),
		Nonce:            pheader.GetNonce(),
		Timestamp:        pheader.GetTimestamp(),
	}
}

// EncodeBlock returns a pro.Block given a Block.
func EncodeBlock(b *Block) *pro.Block {
	var ptxs []*pro.Transaction
	for _, tx := range b.Transactions {
		ptxs = append(ptxs, EncodeTransaction(tx))
	}
	return &pro.Block{
		Header:       EncodeHeader(b.Header),
		Transactions: ptxs,
	}
}

// DecodeBlock returns a Block given a pro.Block.
func DecodeBlock(pb *pro.Block) *Block {
	var txs []*Transaction
	for _, ptx := range pb.GetTransactions() {
		txs = append(txs, DecodeTransaction(ptx))
	}
	return &Block{
		Header:       DecodeHeader(pb.GetHeader()),
		Transactions: txs,
	}
}

// Hash returns the hash of the block (which is done via the header)
func (b *Block) Hash() string {
	h := sha256.New()
	pb := EncodeHeader(b.Header)
	bytes, err := proto.Marshal(pb)
	if err != nil {
		utils.Debug.Printf("[block.Hash()] Unable to marshal block")
	}
	h.Write(bytes)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Size returns the size of the
// block in bytes
func (b *Block) Size() uint32 {
	return pro.SizeOfBlock(EncodeBlock(b))
}

func (b *Block) NameTag() string {
	i, _ := strconv.ParseInt(b.Hash()[:10], 16, 64)
	return fmt.Sprintf("%v", utils.Colorize(fmt.Sprintf("block-%v", b.Hash()[:8]), int(i)))
}

func New(previousHash string, txs []*Transaction, target string) *Block {
	return &Block{
		Header: &Header{
			Version:          0,
			PreviousHash:     previousHash,
			MerkleRoot:       CalculateMerkleRoot(txs),
			DifficultyTarget: target,
		},
		Transactions: txs,
	}

}

// CalculateMerkleRoot calculates
// the merkle root for a list of transactions.
// Look up merkle trees for further description.
// Input:
// txs	[]*block.Transaction a list of transactions
// that represent the leaves of the merkle tree.
// Returns:
// string	the root of the merkle tree represented
// as a hex string.
func CalculateMerkleRoot(txs []*Transaction) string {
	var hashes []string
	if len(txs) > 1 && len(txs)%2 != 0 {
		txs = append(txs, txs[len(txs)-1])
	}
	for _, t := range txs {
		hashes = append(hashes, t.Hash())
	}
	for len(hashes) != 1 {
		var newHashes []string
		if len(hashes)%2 != 0 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}
		for i := 0; i < len(hashes); i += 2 {
			bytes1, _ := hex.DecodeString(hashes[i])
			bytes2, _ := hex.DecodeString(hashes[i+1])
			bytes3 := append(bytes1[:], bytes2[:]...)
			newHsh := utils.Hash(bytes3)
			newHashes = append(newHashes, newHsh)
		}
		hashes = newHashes
	}
	if hashes == nil || len(hashes) < 1 {
		fmt.Printf("ERROR {block.CaclMrkRt}: function" +
			"ended up not being able to calculate a root.\n")
		return ""
	}
	return hashes[0]
}

func (b *Block) Summarize() string {
	txs := make([]string, 0)
	for _, t := range b.Transactions {
		txs = append(txs, t.NameTag())
	}
	i, _ := strconv.ParseInt(b.Header.PreviousHash[:10], 16, 64)
	prevNameTag := utils.Colorize(fmt.Sprintf("block-%v", b.Header.PreviousHash[:6]), int(i))
	return fmt.Sprintf("{prev: %v, txs: [%v]}", prevNameTag, strings.Join(txs, ", "))
}
