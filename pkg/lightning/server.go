package lightning

import (
	"Coin/pkg/address"
	"Coin/pkg/block"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
	"Coin/pkg/script"
	"context"
	"errors"
	"strconv"
	"time"
)

// Version was copied directly from pkg/server.go. Only changed the function receiver and types
func (ln *LightningNode) Version(ctx context.Context, in *pro.VersionRequest) (*pro.Empty, error) {
	// Reject all outdated versions (this is not true to Satoshi Client)
	if in.Version != ln.Config.Version {
		return &pro.Empty{}, nil
	}
	// If addr map is full or does not contain addr of ver, reject
	newAddr := address.New(in.AddrMe, uint32(time.Now().UnixNano()))
	if ln.AddressDB.Get(newAddr.Addr) != nil {
		err := ln.AddressDB.UpdateLastSeen(newAddr.Addr, newAddr.LastSeen)
		if err != nil {
			return &pro.Empty{}, nil
		}
	} else if err := ln.AddressDB.Add(newAddr); err != nil {
		return &pro.Empty{}, nil
	}
	newPeer := peer.New(ln.AddressDB.Get(newAddr.Addr), in.Version, in.BestHeight)
	// Check if we are waiting for a ver in response to a ver, do not respond if this is a confirmation of peering
	pendingVer := newPeer.Addr.SentVer != time.Time{} && newPeer.Addr.SentVer.Add(ln.Config.VersionTimeout).After(time.Now())
	if ln.PeerDb.Add(newPeer) && !pendingVer {
		newPeer.Addr.SentVer = time.Now()
		_, err := newAddr.VersionRPC(&pro.VersionRequest{
			Version:    ln.Config.Version,
			AddrYou:    in.AddrYou,
			AddrMe:     ln.Address,
			BestHeight: ln.BlockHeight,
		})
		if err != nil {
			return &pro.Empty{}, err
		}
	}
	return &pro.Empty{}, nil
}

// OpenChannel is called by another lightning node that wants to open a channel with us
func (ln *LightningNode) OpenChannel(ctx context.Context, in *pro.OpenChannelRequest) (*pro.OpenChannelResponse, error) {
	// TODO

	// check if the req is sent from our peer and if the channel already exists
	peer := ln.PeerDb.Get(in.Address)
	if peer == nil {
		return nil, errors.New("unknown peer")
	}

	if _, exists := ln.Channels[peer]; exists {
		return nil, errors.New("channel already exists")
	}

	// validate and sign funding and refund txn
	fundingTx := block.DecodeTransaction(in.GetFundingTransaction())
	refundTx := block.DecodeTransaction(in.GetRefundTransaction())

	err := ln.ValidateAndSign(fundingTx)
	if err != nil {
		return nil, errors.New("can not validate or sign the funding transaction")
	}
	err = ln.ValidateAndSign(refundTx)
	if err != nil {
		return nil, errors.New("can not validate or sign the refund transaction")
	}

	// open the channel with this peer
	pubRevKey, privRevKey := GenerateRevocationKey()

	ln.Channels[peer] = &Channel{
		Funder:             false,
		FundingTransaction: fundingTx,
		State:              0,
		CounterPartyPubKey: in.GetPublicKey(),
		MyTransactions:     []*block.Transaction{refundTx},
		TheirTransactions:  []*block.Transaction{refundTx},
		MyRevocationKeys: map[string][]byte{
			string(pubRevKey): privRevKey,
		},
	}

	resp := &pro.OpenChannelResponse{
		PublicKey:                ln.Id.GetPublicKeyBytes(),
		SignedFundingTransaction: block.EncodeTransaction(fundingTx),
		SignedRefundTransaction:  block.EncodeTransaction(refundTx),
	}

	return resp, nil
}

func (ln *LightningNode) GetUpdatedTransactions(ctx context.Context, in *pro.TransactionWithAddress) (*pro.UpdatedTransactions, error) {
	// TODO
	peer := ln.PeerDb.Get(in.Address)
	if peer == nil {
		return nil, errors.New("unknown incoming address")
	}

	theirTx := block.DecodeTransaction(in.GetTransaction())
	ln.SignTransaction(theirTx)

	pubRevKey, privRevKey := GenerateRevocationKey()
	ourTx := ln.generateTransactionWithCorrectScripts(peer, theirTx, pubRevKey)

	ln.Channels[peer].TheirTransactions = append(ln.Channels[peer].TheirTransactions, theirTx)
	ln.Channels[peer].MyRevocationKeys[strconv.Itoa(ln.Channels[peer].State)] = privRevKey

	updatedTransactions := &pro.UpdatedTransactions{
		SignedTransaction:   block.EncodeTransaction(theirTx),
		UnsignedTransaction: block.EncodeTransaction(ourTx),
	}

	return updatedTransactions, nil
}

func (ln *LightningNode) GetRevocationKey(ctx context.Context, in *pro.SignedTransactionWithKey) (*pro.RevocationKey, error) {
	// TODO
	peer := ln.PeerDb.Get(in.Address)
	if peer == nil {
		return nil, errors.New("unknown incoming address")
	}

	newTx := block.DecodeTransaction(in.GetSignedTransaction())
	ln.Channels[peer].MyTransactions = append(ln.Channels[peer].MyTransactions, newTx)

	scriptType, err := script.DetermineScriptType(newTx.Outputs[0].LockingScript)

	if err != nil {
		return nil, errors.New("can not determine the script type")
	}

	// If I am the funder
	outputIndex := 1
	// If they are the funder
	if ln.Channels[peer].Funder {
		outputIndex = 0
	}

	revocationInfo := &RevocationInfo{
		RevKey:            in.GetRevocationKey(),
		TransactionOutput: newTx.Outputs[outputIndex],
		OutputIndex:       uint32(outputIndex),
		TransactionHash:   string(newTx.Hash()),
		ScriptType:        scriptType,
	}

	if ln.Channels[peer].TheirRevocationKeys == nil {
		ln.Channels[peer].TheirRevocationKeys = make(map[string]*RevocationInfo)
	}
	ln.Channels[peer].TheirRevocationKeys[string(newTx.Hash())] = revocationInfo
	myRevKey := ln.Channels[peer].MyRevocationKeys[string(newTx.Hash())]
	ln.Channels[peer].State += 1

	return &pro.RevocationKey{
		Key: myRevKey,
	}, nil
}
