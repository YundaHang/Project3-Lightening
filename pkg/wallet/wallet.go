package wallet

import (
	"Coin/pkg/block"
	"Coin/pkg/id"
	"Coin/pkg/pro"
	"Coin/pkg/script"
	"Coin/pkg/utils"
	"bytes"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// CoinInfo holds the information about a TransactionOutput
// necessary for making a TransactionInput.
// ReferenceTransactionHash is the hash of the transaction that the
// output is from.
// OutputIndex is the index into the Outputs array of the
// Transaction that the TransactionOutput is from.
// TransactionOutput is the actual TransactionOutput
type CoinInfo struct {
	ReferenceTransactionHash string
	OutputIndex              uint32
	TransactionOutput        *block.TransactionOutput
}

// Wallet handles keeping track of the owner's coins
//
// # CoinCollection is the owner of this wallet's set of coins
//
// UnseenSpentCoins is a mapping of transaction hashes (which are strings)
// to a slice of coinInfos. It's used for keeping track of coins that we've
// used in a transaction but haven't yet seen in a block.
//
// UnconfirmedSpentCoins is a mapping of Coins to number of confirmations
// (which are integers). We can't confirm that a Coin has been spent until
// we've seen enough POW on top the block containing our sent transaction.
//
// UnconfirmedReceivedCoins is a mapping of CoinInfos to number of confirmations
// (which are integers). We can't confirm we've received a Coin until
// we've seen enough POW on top the block containing our received transaction.
type Wallet struct {
	Config              *Config
	Id                  id.ID
	TransactionRequests chan *block.Transaction
	Address             string
	Balance             uint32

	// All coins
	CoinCollection map[CoinInfo]bool

	// Not yet seen
	UnseenSpentCoins map[string][]CoinInfo

	// Seen but not confirmed
	UnconfirmedSpentCoins    map[CoinInfo]uint32
	UnconfirmedReceivedCoins map[CoinInfo]uint32
}

// SetAddress sets the address
// of the node in the wallet.
func (w *Wallet) SetAddress(a string) {
	w.Address = a
}

// New creates a wallet object
func New(config *Config, id id.ID) *Wallet {
	if !config.HasWallet {
		return nil
	}
	return &Wallet{
		Config:                   config,
		Id:                       id,
		TransactionRequests:      make(chan *block.Transaction),
		Balance:                  0,
		CoinCollection:           make(map[CoinInfo]bool),
		UnseenSpentCoins:         make(map[string][]CoinInfo),
		UnconfirmedSpentCoins:    make(map[CoinInfo]uint32),
		UnconfirmedReceivedCoins: make(map[CoinInfo]uint32),
	}
}

// generateTransactionInputs creates the transaction inputs required to make a transaction.
// In addition to the inputs, it returns the amount of change the wallet holder should
// return to themselves, and the coinInfos used
func (w *Wallet) generateTransactionInputs(amount uint32, fee uint32) (uint32, []*block.TransactionInput, []CoinInfo) {
	// the inputs that we will eventually be returning
	var inputs []*block.TransactionInput
	// the coinInfos that we're using
	var coinInfos []CoinInfo
	// the total amount of the coins that we've used so far for our inputs
	total := uint32(0)
	// Now that we know our balance is enough, we can loop through our coins until we've reached
	// a large enough total to meet our amount and fee
	for coinInfo, _ := range w.CoinCollection {
		if total >= amount+fee {
			break
		}
		// have to generate the unlockingScripts so that we can prove we have the ability to spend
		// this coin
		unlockingScript, err := coinInfo.TransactionOutput.MakeSignature(w.Id)
		if err != nil {
			utils.Debug.Printf("[generateTransactionInputs] Error: failed to create unlockingScript\n")
		}
		// actually create the transaction input
		txi := &block.TransactionInput{
			ReferenceTransactionHash: coinInfo.ReferenceTransactionHash,
			OutputIndex:              coinInfo.OutputIndex,
			UnlockingScript:          unlockingScript,
		}
		coinInfos = append(coinInfos, coinInfo)
		inputs = append(inputs, txi)
		total += coinInfo.TransactionOutput.Amount
	}
	change := total - (amount + fee)
	return change, inputs, coinInfos
}

// generateTransactionOutputs generates the transaction outputs required to create a transaction.
func (w *Wallet) generateTransactionOutputs(
	amount uint32,
	receiverPK []byte,
	change uint32,
) []*block.TransactionOutput {
	// make sure that the public key we're sending our amount to is valid
	if receiverPK == nil || len(receiverPK) == 0 {
		utils.Debug.Printf("[generateTransactionOutputs] Error: receiver's public key is invalid")
		return nil
	}
	// the outputs that we will eventually return
	var outputs []*block.TransactionOutput
	// the output for the person we're sending this transaction output to
	myScript := &pro.PayToPublicKey{PublicKey: w.Id.GetPublicKeyBytes()}
	myScriptB, err := proto.Marshal(myScript)
	if err != nil {
		myScriptB = []byte{}
		fmt.Printf("[wallet.generateTransactionOutputs] Failed to marshal script")
	}
	theirScript := &pro.PayToPublicKey{PublicKey: receiverPK}
	theirScriptB, err2 := proto.Marshal(theirScript)
	if err2 != nil {
		theirScriptB = []byte{}
		fmt.Printf("[wallet.generateTransactionOutputs] Failed to marshal script")
	}
	txoSending := &block.TransactionOutput{Amount: amount, LockingScript: theirScriptB}
	outputs = append(outputs, txoSending)
	// if there's change, we should send that back to ourselves.
	if change != 0 {
		txoChange := &block.TransactionOutput{Amount: change, LockingScript: myScriptB}
		outputs = append(outputs, txoChange)
	}
	return outputs
}

// RequestTransaction allows the wallet to send a transaction to the node,
// which will propagate the transaction along the P2P network.
func (w *Wallet) RequestTransaction(amount uint32, fee uint32, recipientPK []byte) *block.Transaction {
	// have to ensure that we have enough money to actually make this transaction
	if w.Balance < amount+fee {
		utils.Debug.Printf("%v did not have a large enough balance to make the requested transaction\n"+
			"Balance: %v\nTransaction cost: %v", utils.FmtAddr(w.Address), w.Balance, amount+fee)
		return nil
	}
	change, inputs, coinInfos := w.generateTransactionInputs(amount, fee)
	if coinInfos == nil {
		utils.Debug.Printf("[wallet.RequestTransaction] coinInfos were nil")
		return nil
	}
	outputs := w.generateTransactionOutputs(amount, recipientPK, change)
	tx := &block.Transaction{
		Version:  0,
		Inputs:   inputs,
		Outputs:  outputs,
		LockTime: 0,
	}
	// now that we have the transaction, we can add the coinInfos to our UnseenSpentCoins
	// and temporarily remove from the CoinCollection
	w.UnseenSpentCoins[tx.Hash()] = coinInfos
	for _, ci := range coinInfos {
		delete(w.CoinCollection, ci)
	}
	// if we want to broadcast, send to the channel that the node monitors
	go func() {
		w.TransactionRequests <- tx
	}()
	// we do this here in case generateTransactionInputs doesn't work
	// have to make sure that the balance is decremented so that the wallet owner can't keep spamming their coin
	coinTotals := amount + fee + change
	w.Balance -= coinTotals
	return tx
}

// HandleBlock handles the transactions of a new block. It:
// (1) sees if any of the inputs are ones that we've spent
// (2) sees if any of the incoming outputs on the block are ours
// (3) updates our unconfirmed coins, since we've just gotten
// another confirmation!
func (w *Wallet) HandleBlock(txs []*block.Transaction) {
	// most of the time, we will just be handling the transactions
	for _, tx := range txs {
		// see if this is a transaction we've spent a coin on
		if _, ok := w.UnseenSpentCoins[tx.Hash()]; ok {
			w.handleSeenCoins(tx.Hash())
		}
		// check outputs to see if they contain any coins for us
		for i, txo := range tx.Outputs {
			pK := &pro.PayToPublicKey{}
			err := proto.Unmarshal(txo.LockingScript, pK)
			if err != nil {
				fmt.Printf("[wallet.HandleBlock] Failed to unmarshal")
				continue
			}
			if bytes.Equal(pK.GetPublicKey(), w.Id.GetPublicKeyBytes()) {
				w.addCoin(tx.Hash(), uint32(i), txo)
			}
		}
	}
	w.updateConfirmations()
}

// addCoin adds a received coin to our UnconfirmedReceivedCoins
func (w *Wallet) addCoin(hash string, index uint32, output *block.TransactionOutput) {
	coinInfo := CoinInfo{
		ReferenceTransactionHash: hash,
		OutputIndex:              index,
		TransactionOutput:        output,
	}
	w.UnconfirmedReceivedCoins[coinInfo] = 0
}

func (w *Wallet) updateConfirmations() {
	// update unconfirmed spent coins
	for coinInfo, numConfirmations := range w.UnconfirmedSpentCoins {
		if numConfirmations == w.Config.SafeBlockAmount {
			// if we've seen enough blocks, we can safely remove this
			// coin from our coin collection. It's been spent!
			delete(w.CoinCollection, coinInfo)
			delete(w.UnconfirmedSpentCoins, coinInfo)
		} else {
			// otherwise, we still have to wait :(
			w.UnconfirmedSpentCoins[coinInfo] = numConfirmations + 1
		}
	}
	// update unconfirmed received coins
	for coinInfo, numConfirmations := range w.UnconfirmedReceivedCoins {
		if numConfirmations == w.Config.SafeBlockAmount {
			// if we've seen enough blocks, we can safely add this
			// coin to our coin collection. It's spendable!
			w.CoinCollection[coinInfo] = true
			// Also need to update our balance
			w.Balance += coinInfo.TransactionOutput.Amount
			delete(w.UnconfirmedReceivedCoins, coinInfo)
		} else {
			// otherwise, we still have to wait :(
			w.UnconfirmedReceivedCoins[coinInfo] = numConfirmations + 1
		}
	}
}

// handleSeenCoins moves coins from UnseenSpentCoins to
// UnconfirmedSpentCoins
func (w *Wallet) handleSeenCoins(hash string) {
	seenCoins, _ := w.UnseenSpentCoins[hash]
	// remove from unseen, since we've now seen our
	// transaction in a block
	delete(w.UnseenSpentCoins, hash)
	// move the seen coins over to unconfirmed
	for _, coinInfo := range seenCoins {
		w.UnconfirmedSpentCoins[coinInfo] = 0
	}
}

// partialInput is used for EC HandleFork
type partialInput struct {
	ReferenceTransactionHash string
	OutputIndex              uint32
}

// HandleFork handles a fork, updating the wallet's relevant fields.
func (w *Wallet) HandleFork(blocks []*block.Block) {
	// get the coins that we need to check
	txis := map[partialInput]CoinInfo{}
	// fill txis with partial inputs
	for ci, _ := range w.UnconfirmedSpentCoins {
		pi := partialInput{
			ReferenceTransactionHash: ci.ReferenceTransactionHash,
			OutputIndex:              ci.OutputIndex,
		}
		txis[pi] = ci
	}

	for _, b := range blocks {
		for _, tx := range b.Transactions {
			unseen := make(map[string][]CoinInfo)
			for _, txi := range tx.Inputs {
				pi := partialInput{
					ReferenceTransactionHash: txi.ReferenceTransactionHash,
					OutputIndex:              txi.OutputIndex,
				}
				if ci, ok := txis[pi]; ok {
					// add that coin back to our unseen local map
					if w.UnconfirmedSpentCoins[ci] < w.Config.SafeBlockAmount {
						delete(w.UnconfirmedSpentCoins, ci)
						if cis, ok2 := unseen[tx.Hash()]; ok2 {
							unseen[tx.Hash()] = append(cis, ci)
						} else {
							unseen[tx.Hash()] = []CoinInfo{ci}
						}

					}
				}
			}
			// actually add them back to the wallet's map
			for key, val := range unseen {
				w.UnseenSpentCoins[key] = val
			}
			for _, txo := range tx.Outputs {
				pK := &pro.PayToPublicKey{}
				err := proto.Unmarshal(txo.LockingScript, pK)
				if err != nil {
					fmt.Printf("[wallet.HandleFork] Failed to unmarshal")
				}
				if bytes.Equal(pK.GetPublicKey(), w.Id.GetPublicKeyBytes()) {
					w.RemoveFromUnconfirmed(txo)

				}
			}
		}
	}
}

func (w *Wallet) RemoveFromUnconfirmed(txo *block.TransactionOutput) {
	for ci, pri := range w.UnconfirmedReceivedCoins {
		if txo == ci.TransactionOutput && pri < w.Config.SafeBlockAmount {
			delete(w.UnconfirmedReceivedCoins, ci)
		}
	}
}

// HandleRevokedOutput returns true if it successfully handles revoking
// the transaction
func (w *Wallet) HandleRevokedOutput(hash string, txo *block.TransactionOutput,
	outIndex uint32, secRevKey []byte, scriptType int) *block.Transaction {
	// TODO
	if !RevKeySuccessful(txo.LockingScript, secRevKey, scriptType) {
		utils.Debug.Printf("The revocation key is not valid")
		return nil
	}

	// Preparing Input
	unlockingScript, err := utils.Sign(w.Id.GetPrivateKey(), []byte(hash))
	if err != nil {
		utils.Debug.Print("Error singing transaction")
		return nil
	}
	input := &block.TransactionInput{
		ReferenceTransactionHash: hash,
		OutputIndex:              outIndex,
		UnlockingScript:          unlockingScript,
	}

	// Preparing Output
	lockingScript := &pro.PayToPublicKey{PublicKey: w.Id.GetPublicKeyBytes()}
	lockingScriptByte, err := proto.Marshal(lockingScript)
	if err != nil {
		utils.Debug.Printf("Can not marshal locking script")
		return nil
	}

	output := &block.TransactionOutput{
		Amount:        txo.Amount - w.Config.DefaultFee,
		LockingScript: lockingScriptByte,
	}

	// Preparing transaction
	tx := &block.Transaction{
		Segwit:    true,
		Version:   0,
		Inputs:    []*block.TransactionInput{input},
		Outputs:   []*block.TransactionOutput{output},
		Witnesses: [][]byte{unlockingScript},
		LockTime:  0,
	}

	go func() {
		w.TransactionRequests <- tx
	}()

	return tx
}

// GenerateFundingTransaction is very similar to RequestTransaction, except it does NOT broadcast to the node.
// Also, the outputs are slightly different.
func (w *Wallet) GenerateFundingTransaction(amount uint32, fee uint32, counterparty []byte) *block.Transaction {
	// TODO
	if w.Balance < amount+2*fee {
		utils.Debug.Printf("%v did not have a large enough balance to make the requested transaction\n"+
			"Balance: %v\nTransaction cost: %v", utils.FmtAddr(w.Address), w.Balance, amount+2*fee)
		return nil
	}

	// Preparing Input
	change, inputs, coinInfos := w.generateTransactionInputs(amount+fee, fee)

	if coinInfos == nil {
		utils.Debug.Printf("[wallet.RequestTransaction] coinInfos were nil")
		return nil
	}

	// Preparing Output
	var outputs []*block.TransactionOutput
	multiPartyLockingScript := &pro.MultiParty{
		MyPublicKey:    w.Id.GetPublicKeyBytes(),
		TheirPublicKey: counterparty,
	}
	multiPartyLockingScriptByte, err := proto.Marshal(multiPartyLockingScript)
	if err != nil {
		utils.Debug.Printf("Can not marshal locking script")
		return nil
	}

	// outputs contains [coin to funder, coin to counterparty, change to funder]
	outputs = append(outputs, &block.TransactionOutput{
		Amount:        amount,
		LockingScript: multiPartyLockingScriptByte,
	})
	outputs = append(outputs, &block.TransactionOutput{
		Amount:        0,
		LockingScript: multiPartyLockingScriptByte,
	})
	if change > 0 {
		outputs = append(outputs, &block.TransactionOutput{
			Amount:        change + fee,
			LockingScript: multiPartyLockingScriptByte,
		})
	}

	// Preparing transaction
	tx := &block.Transaction{
		Segwit:    true,
		Version:   0,
		Inputs:    inputs,
		Outputs:   outputs,
		Witnesses: [][]byte{},
		LockTime:  0,
	}

	coinTotals := amount + 2*fee + change
	w.Balance -= coinTotals

	return tx
}

// RevKeySuccessful checks whether a secret revocation key is valid for a txo's lockingScript.
func RevKeySuccessful(lockingScript []byte, secRevKey []byte, scriptType int) bool {
	pubRevKey := utils.PkFromSk(secRevKey)
	switch scriptType {
	case script.MULTI:
		s := &pro.MultiParty{}
		err := proto.Unmarshal(lockingScript, s)
		if err != nil || !bytes.Equal(s.GetRevocationKey(), pubRevKey) {
			return false
		}
		return true
	case script.HTLC:
		s := &pro.HashedTimeLock{}
		err := proto.Unmarshal(lockingScript, s)
		if err != nil || !bytes.Equal(s.GetRevocationKey(), pubRevKey) {
			return false
		}
		return true
	default:
		return false
	}
}
