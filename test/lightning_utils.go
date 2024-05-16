package test

import (
	"Coin/pkg/block"
	"Coin/pkg/lightning"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
	"Coin/pkg/utils"
	"crypto/ecdsa"
	"fmt"
	"google.golang.org/protobuf/proto"
	"testing"
)

func NewLightningNode() *lightning.LightningNode {
	return lightning.New(lightning.DefaultConfig(GetFreePort()))
}

// NewLightningCluster
// First node is always the genesis node
func NewLightningCluster(n int) []*lightning.LightningNode {
	var cluster []*lightning.LightningNode
	for i := 0; i < n; i++ {
		cluster = append(cluster, NewLightningNode())
	}
	return cluster
}

// ConnectLightningCluster connects a cluster of lightning nodes
func ConnectLightningCluster(c []*lightning.LightningNode) {
	for i := 0; i < len(c); i++ {
		for j := 0; j < len(c); j++ {
			if i == j {
				continue
			}
			c[i].ConnectToPeer(c[j].Address)
		}
	}
}

func StartLightningCluster(c []*lightning.LightningNode) {
	for _, node := range c {
		node.Start()
	}
}

func MakeLockingScript(pubRevKey []byte) []byte {
	multi := &pro.MultiParty{
		MyPublicKey:      []byte{},
		TheirPublicKey:   []byte{},
		RevocationKey:    pubRevKey,
		AdditionalBlocks: 0,
	}
	sB, err := proto.Marshal(multi)
	if err != nil {
		fmt.Printf("Unable to marshal multi lockScript")
	}
	return sB
}

func MakeRevocableTransaction(pubRevKey []byte, sK *ecdsa.PrivateKey) *block.Transaction {
	txo1 := MockedTransactionOutput()
	txo2 := MockedTransactionOutput()
	lockingScript := MakeLockingScript(pubRevKey)
	txo3 := &block.TransactionOutput{
		Amount:        100,
		LockingScript: lockingScript,
	}
	tx := &block.Transaction{
		Segwit:    true,
		Version:   0,
		Inputs:    []*block.TransactionInput{MockedTransactionInput()},
		Outputs:   []*block.TransactionOutput{txo1, txo2, txo3},
		Witnesses: [][]byte{},
		LockTime:  0,
	}
	sig, _ := utils.Sign(sK, []byte(tx.Hash()))
	tx.Witnesses = [][]byte{sig}
	return tx
}

func MockedLightningTransaction(ln *lightning.LightningNode) *block.Transaction {
	return MakeRevocableTransaction([]byte{}, ln.Id.GetPrivateKey())
}

// MakeUpdatedTransaction decrements amount from our transaction, and adds it to our peer's
func MakeUpdatedTransaction(t *testing.T, ln *lightning.LightningNode, peer *peer.Peer, amount uint32, isFirst bool) *block.Transaction {
	channel := ln.Channels[peer]
	tx := channel.MyTransactions[channel.State]
	if isFirst {
		// This isn't actually how it works, since we would use this transaction's outputs
		// as the inputs for our new one. But that's ok because we only care about the right output
		// amounts for testing purposes.
		tx = channel.FundingTransaction
	}
	var outputs []*block.TransactionOutput
	pubRev, secRev := lightning.GenerateRevocationKey()
	multi := &pro.MultiParty{
		MyPublicKey:      ln.Id.GetPublicKeyBytes(),
		TheirPublicKey:   channel.CounterPartyPubKey,
		RevocationKey:    pubRev,
		AdditionalBlocks: 0,
	}
	scriptB, err := proto.Marshal(multi)
	if err != nil {
		t.Errorf("[MakeUpdatedTransaction] Failed to make transaction")
	}
	myCoinIndex := 1
	if channel.Funder {
		myCoinIndex = 0
	}

	myCoin := &block.TransactionOutput{
		Amount:        tx.Outputs[myCoinIndex].Amount - amount,
		LockingScript: scriptB,
	}
	theirCoin := &block.TransactionOutput{
		Amount:        tx.Outputs[1-myCoinIndex].Amount + amount,
		LockingScript: nil,
	}
	outputs = []*block.TransactionOutput{theirCoin, myCoin}
	if channel.Funder {
		outputs = []*block.TransactionOutput{myCoin, theirCoin}
	}
	if len(tx.Outputs) == 3 {
		outputs = append(outputs, tx.Outputs[2])
	}
	updatedTx := &block.Transaction{
		Segwit:    tx.Segwit,
		Version:   tx.Version,
		Inputs:    tx.Inputs,
		Outputs:   outputs,
		Witnesses: [][]byte{},
		LockTime:  0,
	}
	// now that we have the transaction, we can add it to our revocation keys
	channel.MyRevocationKeys[updatedTx.Hash()] = secRev
	return updatedTx
}
