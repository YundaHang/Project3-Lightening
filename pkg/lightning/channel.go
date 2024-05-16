package lightning

import (
	"Coin/pkg/block"
	"Coin/pkg/id"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
	"Coin/pkg/script"
	"fmt"
	"strconv"
)

// Channel is our node's view of a channel
// Funder is whether we are the channel's funder
// FundingTransaction is the channel's funding transaction
// CounterPartyPubKey is the other node's public key
// State is the current state that we are at. On instantiation,
// the refund transaction is the transaction for state 0
// Transactions is the slice of transactions, indexed by state
// MyRevocationKeys is a mapping of my private revocation keys
// TheirRevocationKeys is a mapping of their private revocation keys
type Channel struct {
	Funder             bool
	FundingTransaction *block.Transaction
	State              int
	CounterPartyPubKey []byte

	MyTransactions    []*block.Transaction
	TheirTransactions []*block.Transaction

	MyRevocationKeys    map[string][]byte
	TheirRevocationKeys map[string]*RevocationInfo
}

type RevocationInfo struct {
	RevKey            []byte
	TransactionOutput *block.TransactionOutput
	OutputIndex       uint32
	TransactionHash   string
	ScriptType        int
}

// GenerateRevocationKey returns a new public, private key pair
func GenerateRevocationKey() ([]byte, []byte) {
	i, _ := id.CreateSimpleID()
	return i.GetPublicKeyBytes(), i.GetPrivateKeyBytes()
}

// CreateChannel creates a channel with another lightning node
// fee must be enough to cover two transactions! You will get back change from first
func (ln *LightningNode) CreateChannel(peer *peer.Peer, theirPubKey []byte, amount uint32, fee uint32) {
	// TODO
	newChannel := &Channel{
		Funder:              true,
		FundingTransaction:  nil,
		State:               0,
		CounterPartyPubKey:  theirPubKey,
		MyTransactions:      make([]*block.Transaction, 0),
		TheirTransactions:   make([]*block.Transaction, 0),
		MyRevocationKeys:    make(map[string][]byte),
		TheirRevocationKeys: make(map[string]*RevocationInfo),
	}
	ln.Channels[peer] = newChannel

	walletRequest := WalletRequest{
		Amount:             amount,
		Fee:                2 * fee,
		CounterPartyPubKey: theirPubKey,
	}

	fundingTx := ln.generateFundingTransaction(walletRequest)
	pubRevKey, privRevKey := GenerateRevocationKey()
	refundTx := ln.generateRefundTransaction(theirPubKey, fundingTx, fee, pubRevKey)
	newChannel.MyRevocationKeys[strconv.Itoa(newChannel.State)] = privRevKey

	req := &pro.OpenChannelRequest{
		Address:            ln.Address,
		PublicKey:          ln.Id.GetPublicKeyBytes(),
		FundingTransaction: block.EncodeTransaction(fundingTx),
		RefundTransaction:  block.EncodeTransaction(refundTx),
	}

	resp, err := peer.Addr.OpenChannelRPC(req)
	if err != nil {
		fmt.Println("error when opening channel")
	}

	signedFundingTx := block.DecodeTransaction(resp.SignedFundingTransaction)
	signedRefundTx := block.DecodeTransaction(resp.SignedRefundTransaction)

	newChannel.FundingTransaction = signedFundingTx
	newChannel.TheirTransactions = append(newChannel.TheirTransactions, signedRefundTx)
	newChannel.MyTransactions = append(newChannel.MyTransactions, signedRefundTx)

	ln.SignTransaction(fundingTx)
	ln.BroadcastTransaction <- fundingTx
}

// UpdateState is called to update the state of a channel.
func (ln *LightningNode) UpdateState(peer *peer.Peer, tx *block.Transaction) {
	// TODO
	txWithAddress := &pro.TransactionWithAddress{
		Transaction: block.EncodeTransaction(tx),
		Address:     ln.Address,
	}

	// Get Updated Txn
	updatedTx, err := peer.Addr.GetUpdatedTransactionsRPC(txWithAddress)
	if err != nil {
		fmt.Println("error when getting updated transactions")
	}

	myTx := block.DecodeTransaction(updatedTx.SignedTransaction)
	theirTx := block.DecodeTransaction(updatedTx.UnsignedTransaction)

	channel := ln.Channels[peer]
	channel.MyTransactions = append(channel.MyTransactions, myTx)
	ln.SignTransaction(theirTx)
	channel.TheirTransactions = append(channel.TheirTransactions, theirTx)

	// Get Their Revocation Key
	signedTxWithKey := &pro.SignedTransactionWithKey{
		SignedTransaction: block.EncodeTransaction(theirTx),
		RevocationKey:     channel.MyRevocationKeys[strconv.Itoa(channel.State)],
		Address:           ln.Address,
	}
	theirRevKey, err := peer.Addr.GetRevocationKeyRPC(signedTxWithKey)
	if err != nil {
		fmt.Println("can not get their revocation key")
		fmt.Println("0000")
		fmt.Println(err)
	}

	scriptType, err := script.DetermineScriptType(theirTx.Outputs[0].LockingScript)
	if err != nil {
		fmt.Println("can not determine the script type")
	}

	// Increment Out State
	channel.State += 1
	revocationInfo := &RevocationInfo{
		RevKey:            theirRevKey.Key,
		TransactionOutput: theirTx.Outputs[0],
		OutputIndex:       0,
		TransactionHash:   theirTx.Hash(),
		ScriptType:        scriptType,
	}
	channel.TheirRevocationKeys[string(theirTx.Hash())] = revocationInfo
}
