package lightning

import (
	"Coin/pkg/address"
	"Coin/pkg/address/addressdb"
	"Coin/pkg/block"
	"Coin/pkg/id"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
	"Coin/pkg/utils"
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type WalletRequest struct {
	Amount             uint32
	Fee                uint32
	CounterPartyPubKey []byte
}

// LightningNode is the main struct of this package--it's the lightning node
// Channels: key is peer's address
// BroadcastTransaction: a channel to send a transaction we want the
// Bitcoin node to broadcast
// GetTransactionFromWallet: a channel to get a transaction from the wallet
// ReceiveTransactionFromWallet: a channel to receive a transaction from the wallet
// RevocationKeys: channel to send revocationKeys to watchtower
type LightningNode struct {
	*pro.UnimplementedLightningServer
	Server *grpc.Server

	Config  *Config
	Address string
	Id      id.ID

	BlockHeight uint32
	Channels    map[*peer.Peer]*Channel

	BroadcastTransaction chan *block.Transaction

	GetTransactionFromWallet     chan WalletRequest
	ReceiveTransactionFromWallet chan *block.Transaction

	RevocationKeys chan *RevocationInfo

	AddressDB addressdb.AddressDb
	PeerDb    peer.PeerDb
}

func New(config *Config) *LightningNode {
	i, _ := id.New(config.IdConfig)
	return &LightningNode{
		Config:                       config,
		Id:                           i,
		AddressDB:                    addressdb.New(true, 1000),
		PeerDb:                       peer.NewDb(true, 200, ""),
		BroadcastTransaction:         make(chan *block.Transaction),
		GetTransactionFromWallet:     make(chan WalletRequest),
		ReceiveTransactionFromWallet: make(chan *block.Transaction),
		RevocationKeys:               make(chan *RevocationInfo),
		Channels:                     make(map[*peer.Peer]*Channel),
	}
}

// Start starts the lightning server so that we can hear from other
// Pretty much fully copied from node.go
func (ln *LightningNode) Start() {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	addr := fmt.Sprintf("%v:%v", hostname, ln.Config.Port)
	ln.Address = addr
	ln.PeerDb.SetAddr(addr)
	utils.Debug.Printf("Lightning %v started", utils.FmtAddr(ln.Address))
	ln.StartServer(addr)
	// don't think that we need to do any of the other stuff in node.go
}

// ConnectToPeer connects us to a peer
func (ln *LightningNode) ConnectToPeer(addr string) {
	a := address.New(addr, 0)
	_, err := a.LightningVersionRPC(&pro.VersionRequest{
		Version:    ln.Config.Version,
		AddrYou:    addr,
		AddrMe:     ln.Address,
		BestHeight: ln.BlockHeight,
	})
	if err != nil {
		utils.Debug.Printf("%v received no response from VersionRPC to %v",
			utils.FmtAddr(ln.Address), utils.FmtAddr(addr))
	}
}

func (ln *LightningNode) StartServer(address string) {
	lis, err := net.Listen("tcp4", address)
	if err != nil {
		panic(err)
	}
	// Open node to connections
	ln.Server = grpc.NewServer()
	pro.RegisterLightningServer(ln.Server, ln)
	go func() {
		err = ln.Server.Serve(lis)
		if err != nil {
			fmt.Printf("ERROR {Node.StartServer}: error" +
				"when trying to serve server")
		}
	}()
}

// Kill kills any threads currently managed by the Node or that
// it previously started. It also does any necessary clean up.
func (ln *LightningNode) Kill() {
	ln.Server.GracefulStop()
}

// generateFundingTransaction creates the funding transaction for a channel.
// This transaction MUST be broadcast
func (ln *LightningNode) generateFundingTransaction(request WalletRequest) *block.Transaction {
	tx, err := ln.getTransactionFromWallet(request)
	if err != nil {
		return nil
	}
	return tx
}

func (ln *LightningNode) getTransactionFromWallet(request WalletRequest) (*block.Transaction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// ask the wallet to make the transaction for us
	ln.GetTransactionFromWallet <- request
	for {
		select {
		case <-ctx.Done():
			// Oops! We ran out of time
			return nil, fmt.Errorf("[lightningnode.getTransactionFromWallet] Error: timed out")
		case tx := <-ln.ReceiveTransactionFromWallet:
			// Yay! We got a response from our node.
			return tx, nil
		}
	}
}

// generateRefundTransaction generates a refund transaction given a funding transaction
func (ln *LightningNode) generateRefundTransaction(theirPubKey []byte, fundingTx *block.Transaction, fee uint32, revKey []byte) *block.Transaction {
	// ------------------------ Handling Inputs ------------------------//
	// Assumption: 1st output is ours. Since it's a P2PK, all we need to do is provide our pubKey in the unlockingScript
	var inputs []*block.TransactionInput
	input1 := &block.TransactionInput{
		ReferenceTransactionHash: fundingTx.Hash(),
		OutputIndex:              0,
		UnlockingScript:          ln.Id.GetPublicKeyBytes(),
	}
	inputs = append(inputs, input1)
	// If a 3rd output exists, it is change and also ours.
	if len(fundingTx.Outputs) > 2 {
		input2 := &block.TransactionInput{
			ReferenceTransactionHash: fundingTx.Hash(),
			OutputIndex:              2,
			UnlockingScript:          ln.Id.GetPublicKeyBytes(),
		}
		inputs = append(inputs, input2)
	}
	// ------------------------ Handling Outputs ------------------------//
	// check to see if there was any leftover change that we still need to get.
	// We will use
	change := uint32(0)
	if len(fundingTx.Outputs) > 0 && fundingTx.Outputs[2].Amount > fee {
		change = fundingTx.Outputs[2].Amount - fee
	}
	// We're making a multi-party locking script to start off the channel.
	multi := &pro.MultiParty{
		ScriptType:       pro.ScriptType_MULTI,
		MyPublicKey:      ln.Id.GetPublicKeyBytes(),
		TheirPublicKey:   theirPubKey,
		RevocationKey:    revKey,
		AdditionalBlocks: ln.Config.AdditionalBlocks,
	}
	scriptB, err := proto.Marshal(multi)
	if err != nil {
		fmt.Printf("[lightningnode.generateRefundTransaction] Failed to marshal multi-party script ")
	}
	out := &block.TransactionOutput{
		Amount:        fundingTx.Outputs[0].Amount + change,
		LockingScript: scriptB,
	}
	// Now that we've made the output with the correct multi party output, it's time to put everything
	// together!
	unsignedRefundTx := &block.Transaction{
		Segwit:   true,
		Version:  fundingTx.Version,
		Inputs:   inputs,
		Outputs:  []*block.TransactionOutput{out},
		LockTime: ln.BlockHeight + ln.Config.LockTime,
	}
	// sign the refund transaction ourselves and add it to the witnesses
	sig, err := unsignedRefundTx.Sign(ln.Id)
	if err != nil {
		utils.Debug.Printf("[requestRefundTransaction] Error: failed to create signature\n")
	}
	unsignedRefundTx.Witnesses = [][]byte{sig}
	return unsignedRefundTx
}

func (ln *LightningNode) IncrementBlockHeight() {
	ln.BlockHeight++
}

func (ln *LightningNode) SetAddress(address string) {
	ln.Address = address
}

// generateTransactionWithCorrectScripts creates the correct locking scripts for our side of the transaction.
func (ln *LightningNode) generateTransactionWithCorrectScripts(peer *peer.Peer, theirTx *block.Transaction, pubRevKey []byte) *block.Transaction {
	channel := ln.Channels[peer]
	// my script needs to be a multisig, so that they can revoke it
	multi := &pro.MultiParty{
		ScriptType:       pro.ScriptType_MULTI,
		MyPublicKey:      ln.Id.GetPublicKeyBytes(),
		TheirPublicKey:   channel.CounterPartyPubKey,
		RevocationKey:    pubRevKey,
		AdditionalBlocks: ln.Config.AdditionalBlocks,
	}
	myScript, err := proto.Marshal(multi)
	if err != nil {
		fmt.Printf("[generateTransactionWithCorrectScripts] failed")
	}
	// their script needs to be a pay to public key, because it can't be revoked
	p2pk := &pro.PayToPublicKey{
		ScriptType: pro.ScriptType_P2PK,
		PublicKey:  channel.CounterPartyPubKey,
	}
	theirScript, err2 := proto.Marshal(p2pk)
	if err2 != nil {
		fmt.Printf("[generateTransactionWithCorrectScripts] failed")
	}
	var outputs []*block.TransactionOutput
	// Assume we are the channels funder
	myTxo := &block.TransactionOutput{
		Amount:        theirTx.Outputs[0].Amount,
		LockingScript: myScript,
	}
	theirTxo := &block.TransactionOutput{
		Amount:        theirTx.Outputs[1].Amount,
		LockingScript: theirScript,
	}
	outputs = []*block.TransactionOutput{myTxo, theirTxo}
	// but in case we aren't
	if !channel.Funder {
		myTxo = &block.TransactionOutput{
			Amount:        theirTx.Outputs[1].Amount,
			LockingScript: myScript,
		}
		theirTxo = &block.TransactionOutput{
			Amount:        theirTx.Outputs[0].Amount,
			LockingScript: theirScript,
		}
		outputs = []*block.TransactionOutput{theirTxo, myTxo}
	}
	// add in the change if it exists
	if len(theirTx.Outputs) == 3 {
		outputs = append(outputs, theirTx.Outputs[2])
	}
	// return our version of the transaction
	myTx := &block.Transaction{
		Segwit:   theirTx.Segwit,
		Version:  theirTx.Version,
		Inputs:   theirTx.Inputs,
		Outputs:  outputs,
		LockTime: theirTx.LockTime,
	}
	return myTx
}
