package test

import (
	"Coin/pkg"
	"Coin/pkg/block"
	"Coin/pkg/blockchain"
	"Coin/pkg/id"
	"Coin/pkg/lightning"
	"Coin/pkg/pro"
	"Coin/pkg/script"
	"Coin/pkg/utils"
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"
)

//---------------------------------- SegWit Tests ----------------------------------//

func TestGetWitnessesRPC(t *testing.T) {
	cluster := NewCluster(3)
	chains := []*blockchain.BlockChain{cluster[0].BlockChain, cluster[1].BlockChain, cluster[2].BlockChain}
	defer CleanUp(chains)
	StartCluster(cluster)
	ConnectCluster(cluster)
	peer := cluster[0].PeerDb.Get(cluster[1].Address)
	tx := MockedTransaction()
	ptx := block.EncodeTransaction(tx)
	witnesses1, err1 := peer.Addr.GetWitnessesRPC(ptx)
	if err1 == nil {
		t.Errorf("Peer should not have witnesses for transaction")
	}
	if witnesses1 != nil {
		t.Errorf("witnesses should be nil due to errors")
	}
	//// Sign and add signature to node 1's mapping
	sig, _ := utils.Sign(cluster[1].Id.GetPrivateKey(), []byte(tx.Hash()))
	tx.Witnesses = [][]byte{sig}
	cluster[1].SeenTransactions[tx.Hash()] = &pkg.TransactionWithCount{
		Transaction: tx,
		Count:       1,
	}
	// This call should succeed
	witnesses2, err2 := peer.Addr.GetWitnessesRPC(ptx)
	if err2 != nil {
		t.Errorf("RPC call should have succeeded")
	}
	if witnesses2 == nil {
		t.Errorf("Witnesses should not be nil")
	}
	if !bytes.Equal(witnesses2.GetWitnesses()[0], sig) {
		t.Errorf("witness should be the signature provided.")
	}
}

func TestForwardTransaction(t *testing.T) {
	cluster := NewCluster(3)
	chains := []*blockchain.BlockChain{cluster[0].BlockChain, cluster[1].BlockChain, cluster[2].BlockChain}
	defer CleanUp(chains)
	StartCluster(cluster)
	ConnectCluster(cluster)
	peer := cluster[0].PeerDb.Get(cluster[1].Address)
	tx := MockedTransaction()
	ptx := block.EncodeTransaction(tx)
	sig, _ := utils.Sign(cluster[1].Id.GetPrivateKey(), []byte(tx.Hash()))
	tx.Witnesses = [][]byte{sig}
	// first node has seen it
	cluster[0].SeenTransactions[tx.Hash()] = &pkg.TransactionWithCount{
		Transaction: tx,
		Count:       1,
	}
	// First time sending the transaction
	txWithAddress := &pro.TransactionWithAddress{
		Transaction: ptx,
		Address:     cluster[0].Address,
	}
	resp, err := peer.Addr.ForwardTransactionRPC(txWithAddress)
	if resp == nil {
		t.Errorf("resp should not be nil")
	}
	if err != nil {
		t.Errorf("err should not be nil")
	}
	// Checking that the witnesses made it over
	if !bytes.Equal(cluster[1].SeenTransactions[tx.Hash()].Transaction.Witnesses[0], sig) {
		t.Errorf("witness should be on the transaction")
	}
	// Second time sending transaction: should not succeed
	resp, err = peer.Addr.ForwardTransactionRPC(txWithAddress)
	if resp == nil {
		t.Errorf("resp should not be nil")
	}
	if err != nil {
		t.Errorf("Should not have received an error")
	}
}

//---------------------------------- Wallet Tests ----------------------------------//

func TestGenerateFundingTransaction(t *testing.T) {
	cluster := NewCluster(2)
	chains := []*blockchain.BlockChain{cluster[0].BlockChain, cluster[1].BlockChain}
	defer CleanUp(chains)
	StartCluster(cluster)
	ConnectCluster(cluster)
	// store the coins in the first blockchain
	FillWalletWithCoins(cluster[0].Wallet, 100, 100)
	counterParty := cluster[1].LightningNode.Id.GetPublicKeyBytes()
	tx := cluster[0].Wallet.GenerateFundingTransaction(80, 20, counterParty)
	if tx == nil {
		t.Errorf("Uh oh. Transaction should not be nil")
	}
	// -------- Checking transaction fields --------
	AssertSize(t, len(tx.Inputs), 2)
	AssertSize(t, len(tx.Outputs), 3)
	if tx.Witnesses == nil {
		t.Errorf("Witnesses should be empty, not nil")
	}
	if !tx.Segwit {
		t.Errorf("This should be a SegWit transaction")
	}
	pubKey := cluster[0].Wallet.Id.GetPublicKeyBytes()
	// Check that locking script contains both our key and their key
	for _, txo := range tx.Outputs {
		if !bytes.Contains(txo.LockingScript, pubKey) {
			t.Errorf("Lockingscript should contain our key!")
		}
		if !bytes.Contains(txo.LockingScript, counterParty) {
			t.Errorf("Lockingscript should contain their key!")
		}
	}
	// Check output amounts
	if tx.Outputs[0].Amount != 80 {
		t.Errorf("First output is to yourself, and should be total amount of channel")
	}
	if tx.Outputs[1].Amount != 0 {
		t.Errorf("Second output is to counter party, and should be 0")
	}
	if tx.Outputs[2].Amount != 100 {
		t.Errorf("Third output is change, and you should have to use anther coin in order to " +
			"have enough for refund transaction")
	}
}

// Essentially just have to make sure that they claim the
// output and broadcast it to the chain. LockingScript
// should contain revocationKey
func TestHandleRevokedTransaction(t *testing.T) {
	w := CreateMockedWallet()
	tx := MockedTransaction()
	pubRevKey, secRevKey := lightning.GenerateRevocationKey()
	if w.HandleRevokedOutput(tx.Hash(), tx.Outputs[0], 0, secRevKey, 1) != nil {
		t.Errorf("Should not revoke mocked transaction")
	}
	tx = MakeRevocableTransaction(pubRevKey, w.Id.GetPrivateKey())
	newTx := w.HandleRevokedOutput(tx.Hash(), tx.Outputs[2], 2, secRevKey, script.MULTI)
	if newTx == nil {
		t.Errorf("Should have revoked this transaction")
	}
	AssertSize(t, len(newTx.Inputs), 1)
	AssertSize(t, len(newTx.Outputs), 1)
	AssertSize(t, len(newTx.Witnesses), 1)
	if newTx.Outputs[0].Amount != 95 {
		t.Errorf("txo amount should be 95")
	}
	p2pk := &pro.PayToPublicKey{PublicKey: w.Id.GetPublicKeyBytes()}
	script, _ := proto.Marshal(p2pk)
	if !bytes.Equal(newTx.Outputs[0].LockingScript, script) {
		t.Errorf("Locking script should be wallet's public key")
	}
}

//---------------------------------- Server Tests ----------------------------------//

func TestOpenChannel(t *testing.T) {
	cluster := NewCluster(2)
	chains := []*blockchain.BlockChain{cluster[0].BlockChain, cluster[1].BlockChain}
	defer CleanUp(chains)
	StartCluster(cluster)
	ConnectCluster(cluster)
	lightning0 := cluster[0].LightningNode
	lightning1 := cluster[1].LightningNode
	peer := lightning0.PeerDb.Get(lightning1.Address)

	// Making fake transactions
	fundingTx := MockedTransaction()
	refundTx := MockedTransaction()
	refundTx.Version = 1

	// request that we'll send over
	openChannelRequest := &pro.OpenChannelRequest{
		Address:            lightning0.Address,
		PublicKey:          lightning0.Id.GetPublicKeyBytes(),
		FundingTransaction: block.EncodeTransaction(fundingTx),
		RefundTransaction:  block.EncodeTransaction(refundTx),
	}

	resp, err := peer.Addr.OpenChannelRPC(openChannelRequest)
	if err != nil {
		t.Errorf("Should not have thrown an error")
	}

	fundingTx = block.DecodeTransaction(resp.GetSignedFundingTransaction())
	refundTx = block.DecodeTransaction(resp.GetSignedRefundTransaction())

	// Peer should have signed both transactions
	AssertSize(t, len(fundingTx.Witnesses), 1)
	AssertSize(t, len(refundTx.Witnesses), 1)
}

func TestGetUpdatedTransactions(t *testing.T) {
	cluster := NewCluster(2)
	chains := []*blockchain.BlockChain{cluster[0].BlockChain, cluster[1].BlockChain}
	defer CleanUp(chains)
	StartCluster(cluster)
	ConnectCluster(cluster)
	lightning0 := cluster[0].LightningNode
	lightning1 := cluster[1].LightningNode
	peer := lightning0.PeerDb.Get(lightning1.Address)

	// Open up the channel
	openChannelRequest := &pro.OpenChannelRequest{
		Address:            lightning0.Address,
		PublicKey:          lightning0.Id.GetPublicKeyBytes(),
		FundingTransaction: block.EncodeTransaction(MockedTransaction()),
		RefundTransaction:  block.EncodeTransaction(MockedTransaction()),
	}
	_, err := peer.Addr.OpenChannelRPC(openChannelRequest)
	if err != nil {
		t.Errorf("Should not have thrown an error")
	}

	newState := MockedLightningTransaction(lightning0)

	sig, _ := utils.Sign(cluster[1].Id.GetPrivateKey(), []byte(newState.Hash()))
	newState.Witnesses = [][]byte{sig}

	req := &pro.TransactionWithAddress{
		Transaction: block.EncodeTransaction(newState),
		Address:     lightning0.Address,
	}

	resp, err2 := peer.Addr.GetUpdatedTransactionsRPC(req)
	if err2 != nil {
		t.Errorf("Should not have thrown an error")
	}
	mySignedTx := block.DecodeTransaction(resp.GetSignedTransaction())
	if len(mySignedTx.Witnesses) != 2 {
		t.Errorf("Both parties should have signed")
	}
	theirUnsignedTx := block.DecodeTransaction(resp.GetUnsignedTransaction())
	if len(theirUnsignedTx.Witnesses) != 0 {
		t.Errorf("Should be unsigned")
	}
}

func TestGetRevocationKey(t *testing.T) {
	cluster := NewCluster(2)
	chains := []*blockchain.BlockChain{cluster[0].BlockChain, cluster[1].BlockChain}
	defer CleanUp(chains)
	StartCluster(cluster)
	ConnectCluster(cluster)
	lightning0 := cluster[0].LightningNode
	lightning1 := cluster[1].LightningNode
	peer := lightning0.PeerDb.Get(lightning1.Address)

	// Open up the channel
	openChannelRequest := &pro.OpenChannelRequest{
		Address:            lightning0.Address,
		PublicKey:          lightning0.Id.GetPublicKeyBytes(),
		FundingTransaction: block.EncodeTransaction(MockedLightningTransaction(lightning0)),
		RefundTransaction:  block.EncodeTransaction(MockedLightningTransaction(lightning0)),
	}
	_, err := peer.Addr.OpenChannelRPC(openChannelRequest)
	if err != nil {
		t.Errorf("Should not have thrown an error")
	}

	newState := MockedLightningTransaction(lightning0)

	sig, _ := utils.Sign(cluster[1].Id.GetPrivateKey(), []byte(newState.Hash()))
	newState.Witnesses = [][]byte{sig}

	req := &pro.TransactionWithAddress{
		Transaction: block.EncodeTransaction(newState),
		Address:     lightning0.Address,
	}

	resp, err2 := peer.Addr.GetUpdatedTransactionsRPC(req)
	if err2 != nil {
		t.Errorf("Should not have thrown an error")
	}

	fakeRevKey := []byte{00, 01, 02, 03}
	request := &pro.SignedTransactionWithKey{
		SignedTransaction: resp.GetSignedTransaction(),
		RevocationKey:     fakeRevKey,
		Address:           lightning0.Address,
	}

	// have to open a fake channel
	me := lightning1.PeerDb.Get(lightning0.Address)
	resp3, err3 := peer.Addr.GetRevocationKeyRPC(request)
	// Make sure we got a response
	if resp3 == nil {
		t.Errorf("response should not be nil")
	}
	if err3 != nil {
		t.Errorf("RPC call should not have errored")
	}
	// Check that the other node now has a revocation key

	AssertSize(t, len(lightning1.Channels[me].TheirRevocationKeys), 1)
}

//---------------------------------- Lightning Tests ----------------------------------//

// Setting up a channel between two nodes
func TestCreateChannel(t *testing.T) {
	cluster := NewCluster(2)
	chains := []*blockchain.BlockChain{cluster[0].BlockChain, cluster[1].BlockChain}
	defer CleanUp(chains)
	StartCluster(cluster)
	ConnectCluster(cluster)
	// store the coins in the first blockchain
	FillWalletWithCoins(cluster[0].Wallet, 100, 100)
	// naming the lightning nodes
	lightning0 := cluster[0].LightningNode
	lightning1 := cluster[1].LightningNode
	peer := lightning0.PeerDb.Get(lightning1.Address)
	lightning0.CreateChannel(peer, lightning1.Id.GetPublicKeyBytes(), 100, 10)
	//---------- Making sure all of first node's channels are correct ----------//
	AssertSize(t, 1, len(lightning0.Channels))
	channel := lightning0.Channels[peer]
	if !channel.Funder {
		t.Errorf("Should be funder")
	}
	if channel.FundingTransaction == nil {
		t.Errorf("funding transaction should not be nil")
	}
	if channel.State != 0 {
		t.Errorf("Channel should be at state 0")
	}
	if !bytes.Equal(channel.CounterPartyPubKey, lightning1.Id.GetPublicKeyBytes()) {
		t.Errorf("pubKeys should be equal")
	}
	AssertSize(t, len(channel.MyTransactions), 1)
	AssertSize(t, len(channel.TheirTransactions), 1)
	AssertSize(t, 1, len(channel.MyRevocationKeys))
	AssertSize(t, 0, len(channel.TheirRevocationKeys))
	// Get keys for each party
	myPk := lightning0.Id.GetPublicKey()
	theirPk, _ := utils.Byt2PK(channel.CounterPartyPubKey)
	// Check the funding transaction
	tx := channel.FundingTransaction
	if len(tx.Witnesses) != 1 {
		t.Errorf("funding transaction should only be signed counter party")
	}
	if !utils.Verify(theirPk, tx.Hash(), tx.Witnesses[0]) {
		t.Errorf("They need to have signed this transaction")
	}
	// Check refund transaction
	tx = channel.MyTransactions[0]
	if len(channel.MyTransactions[0].Witnesses) != 2 {
		t.Errorf("refund transaction should contain both signatures")
	}
	if !utils.Verify(myPk, tx.Hash(), tx.Witnesses[0]) {
		t.Errorf("I should have signed this transaction")
	}
	if !utils.Verify(theirPk, tx.Hash(), tx.Witnesses[1]) {
		t.Errorf("They should have signed this transaction")
	}

	//---------- Making sure all of second node's channels are correct ----------//
	AssertSize(t, 1, len(lightning1.Channels))
	peer = lightning1.PeerDb.Get(lightning0.Address)
	channel = lightning1.Channels[peer]
	tx = channel.FundingTransaction
	if channel.Funder {
		t.Errorf("Should not be funder")
	}
	if channel.FundingTransaction == nil {
		t.Errorf("funding transaction should not be nil")
	}
	if channel.State != 0 {
		t.Errorf("Channel should be at state 0")
	}
	if !bytes.Equal(channel.CounterPartyPubKey, lightning0.Id.GetPublicKeyBytes()) {
		t.Errorf("pubKeys should be equal")
	}
	AssertSize(t, 1, len(channel.MyTransactions))
	AssertSize(t, 1, len(channel.TheirTransactions))
	// Unlike the first node, we don't have a revocation key for the refund transaction.
	// We have to wait until later to get it
	AssertSize(t, len(channel.MyRevocationKeys), 1)
	AssertSize(t, len(channel.TheirRevocationKeys), 0)
	// swap keys (from other node's perspective)
	myPk, theirPk = theirPk, myPk
	if len(tx.Witnesses) != 1 {
		t.Errorf("funding transaction should only be signed by me")
	}
	if !utils.Verify(myPk, tx.Hash(), tx.Witnesses[0]) {
		t.Errorf("I need to have signed this transaction")
	}
	// Check refund transaction
	tx = channel.MyTransactions[0]
	if len(channel.MyTransactions[0].Witnesses) != 2 {
		t.Errorf("refund transaction should contain both signatures")
	}
	if !utils.Verify(myPk, tx.Hash(), tx.Witnesses[1]) {
		t.Errorf("I should have signed this transaction")
	}
	if !utils.Verify(theirPk, tx.Hash(), tx.Witnesses[0]) {
		t.Errorf("They should have signed this transaction")
	}
}

func TestUpdateState(t *testing.T) {
	//--------------------- Copied from TestCreateChannel ---------------------//
	cluster := NewCluster(2)
	chains := []*blockchain.BlockChain{cluster[0].BlockChain, cluster[1].BlockChain}
	defer CleanUp(chains)
	StartCluster(cluster)
	ConnectCluster(cluster)
	// store the coins in the first blockchain
	FillWalletWithCoins(cluster[0].Wallet, 100, 100)
	// naming the lightning nodes
	lightning0 := cluster[0].LightningNode
	lightning1 := cluster[1].LightningNode
	peer1 := lightning0.PeerDb.Get(lightning1.Address)
	peer0 := lightning1.PeerDb.Get(lightning0.Address)
	lightning0.CreateChannel(peer1, lightning1.Id.GetPublicKeyBytes(), 100, 10)

	//--------------------- Actual test ---------------------//
	// Alice updates state
	updatedTx := MakeUpdatedTransaction(t, lightning0, peer1, 20, true)
	lightning0.UpdateState(peer1, updatedTx)
	// Now Bob updates state
	updatedTx = MakeUpdatedTransaction(t, lightning1, peer0, 10, false)
	lightning1.UpdateState(peer0, updatedTx)
	// Now Alice updates for a last time
	updatedTx = MakeUpdatedTransaction(t, lightning0, peer1, 15, false)
	lightning0.UpdateState(peer1, updatedTx)
	//--------------------- Alice's view ---------------------//
	channel := lightning0.Channels[peer1]
	AssertSize(t, len(channel.MyTransactions), 4)
	AssertSize(t, len(channel.TheirTransactions), 4)
	AssertSize(t, len(channel.TheirRevocationKeys), 3)
	AssertSize(t, len(channel.MyRevocationKeys), 4)
	// Check proper storage of transaction versions. First one is the same
	for i := 1; i < 4; i++ {
		myTx := channel.MyTransactions[i]
		theirTx := channel.TheirTransactions[i]
		if myTx.Hash() == theirTx.Hash() {
			t.Errorf("Hashes should not be the same. Make sure that you are storing each" +
				"party's version of the transaction properly")
		}
	}
	//--------------------- Bob's view ---------------------//
	channel = lightning1.Channels[peer0]
	AssertSize(t, len(channel.MyTransactions), 4)
	AssertSize(t, len(channel.TheirTransactions), 4)
	AssertSize(t, len(channel.TheirRevocationKeys), 3)
	AssertSize(t, len(channel.MyRevocationKeys), 4)

	// Check proper storage of transaction versions. First one is the same
	for i := 1; i < 4; i++ {
		myTx := channel.MyTransactions[i]
		theirTx := channel.TheirTransactions[i]
		if myTx.Hash() == theirTx.Hash() {
			t.Errorf("Hashes should not be the same. Make sure that you are storing each" +
				"party's version of the transaction properly")
		}
	}
}

func TestWatchTowerHandleBlock(t *testing.T) {
	i, _ := id.New(id.DefaultConfig())
	wt := &lightning.WatchTower{
		Id:                  i,
		RevocationKeys:      make(map[string]*lightning.RevocationInfo),
		RevokedTransactions: make(chan *lightning.RevocationInfo),
	}
	tx := MockedTransaction()
	tx.Outputs = append(tx.Outputs, &block.TransactionOutput{10, []byte{00, 11}})
	b := MockedBlock()
	b.Transactions = []*block.Transaction{tx}
	revocationInfo := &lightning.RevocationInfo{}
	wt.RevocationKeys[tx.Hash()] = revocationInfo
	revoked := wt.HandleBlock(b)
	if revoked == nil {
		t.Errorf("Block should have caught this transaction")
	}
	b2 := MockedBlock()
	if wt.HandleBlock(b2) != nil {
		t.Errorf("Block should NOT have caught this transaction")
	}
}
