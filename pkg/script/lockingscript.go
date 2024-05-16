package script

import (
	"Coin/pkg/pro"
	"fmt"
	"google.golang.org/protobuf/proto"
)

// P2PK represents a PayToPublicKey script
const P2PK = 0

// MULTI represents a MultiParty script
const MULTI = 1

// HTLC represents a HashedTimeLock script
const HTLC = 2

// PayToPublicKey is the standard locking script, when we want to pay one person
type PayToPublicKey struct {
	ScriptType int
	PublicKey  []byte
}

// MultiParty is a locking script that requires signatures from two parties
type MultiParty struct {
	ScriptType       int
	MyPublicKey      []byte
	TheirPublicKey   []byte
	RevocationKey    []byte
	AdditionalBlocks uint32
}

// HashedTimeLock is an extended MultiParty script, which also includes hash lock fields
type HashedTimeLock struct {
	// These are the same as MultiParty
	ScriptType     int
	MyPublicKey    []byte
	TheirPublicKey []byte
	RevocationKey  []byte

	// Additional fields for a hash lock
	HashLock         string
	AdditionalBlocks uint32
	Fee              uint32
}

func EncodeMultiParty(multi *MultiParty) *pro.MultiParty {
	return &pro.MultiParty{
		ScriptType:       pro.ScriptType_MULTI,
		MyPublicKey:      multi.MyPublicKey,
		TheirPublicKey:   multi.TheirPublicKey,
		RevocationKey:    multi.RevocationKey,
		AdditionalBlocks: multi.AdditionalBlocks,
	}
}

func EncodePayToPublicKey(p2pk *PayToPublicKey) *pro.PayToPublicKey {
	return &pro.PayToPublicKey{
		ScriptType: pro.ScriptType_P2PK,
		PublicKey:  p2pk.PublicKey,
	}
}

func EncodeHashedTimeLock(htlc *HashedTimeLock) *pro.HashedTimeLock {
	return &pro.HashedTimeLock{
		ScriptType:       pro.ScriptType_HTLC,
		MyPublicKey:      htlc.MyPublicKey,
		TheirPublicKey:   htlc.TheirPublicKey,
		RevocationKey:    htlc.RevocationKey,
		HashLock:         htlc.HashLock,
		AdditionalBlocks: htlc.AdditionalBlocks,
		Fee:              htlc.Fee,
	}
}

func DecodePayToPublicKey(p2pk *pro.PayToPublicKey) *PayToPublicKey {
	return &PayToPublicKey{PublicKey: p2pk.GetPublicKey()}
}

func DecodeMultiParty(multi *pro.MultiParty) *MultiParty {
	return &MultiParty{
		MyPublicKey:      multi.GetMyPublicKey(),
		TheirPublicKey:   multi.GetTheirPublicKey(),
		RevocationKey:    multi.GetTheirPublicKey(),
		AdditionalBlocks: multi.GetAdditionalBlocks(),
	}
}

func DecodeHashedTimeLock(htlc *pro.HashedTimeLock) *HashedTimeLock {
	return &HashedTimeLock{
		MyPublicKey:      htlc.GetMyPublicKey(),
		TheirPublicKey:   htlc.GetTheirPublicKey(),
		RevocationKey:    htlc.GetRevocationKey(),
		HashLock:         htlc.GetHashLock(),
		AdditionalBlocks: htlc.GetAdditionalBlocks(),
		Fee:              htlc.GetFee(),
	}
}

func DetermineScriptType(b []byte) (int, error) {
	// since proto will unmarshal anything, we unmarshal
	// as a pay to public key and then we check the script type
	p2pk := &pro.PayToPublicKey{}
	err := proto.Unmarshal(b, p2pk)
	if err != nil {
		return -1, fmt.Errorf("unable to unmarshal script")
	}
	switch p2pk.ScriptType {
	case pro.ScriptType_P2PK:
		return P2PK, nil
	case pro.ScriptType_MULTI:
		return MULTI, nil
	case pro.ScriptType_HTLC:
		return HTLC, nil
	default:
		return -1, fmt.Errorf("unable to unmarshal script")
	}
}
