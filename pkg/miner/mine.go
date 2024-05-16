package miner

import (
	"Coin/pkg/block"
	"Coin/pkg/utils"
	"bytes"
	"context"
	"fmt"
	"math"
	"time"
)

// Mine waits to be told to mine a block
// or to kill its thread. If it is asked
// to mine, it selects the transactions
// with the highest priority to add to the
// mining pool. The nonce is then attempted
// to be found unless the miner is stopped.
//func (m *Miner) Mine() {
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	for {
//		<-m.PoolUpdated
//		cancel()
//		// should only be mining if we're active
//		if !m.Active.Load() {
//			continue
//		}
//		// get a new context for the goroutine that we're about to spawn
//		ctx, cancel = context.WithCancel(context.Background())
//		go func(ctx context.Context) {
//			// should only mind if our transaction pool has enough priority
//			if !m.TxPool.PriorityMet() {
//				return
//			}
//			// We are currently mining
//			m.Mining.Store(true)
//			// create a new mining pool (get the highest priority transactions)
//			m.MiningPool = m.NewMiningPool()
//			// have to insert the coinbase transaction at the top of the transactions list
//			txs := append([]*block.Transaction{m.GenerateCoinbaseTransaction(m.MiningPool)}, m.MiningPool...)
//			// this is the block that we're going to mine!
//			b := block.New(m.PreviousHash, txs, string(m.DifficultyTarget))
//			// if results is true, we found a winning nonce! Otherwise, we failed (which won't ever actually happen
//			// for us)
//			result := m.CalculateNonce(ctx, b)
//			// done mining
//			m.Mining.Store(false)
//			// send the block to the node to handle
//			if result {
//				utils.Debug.Printf("%v mined %v %v", utils.FmtAddr(m.Address), b.NameTag(), b.Summarize())
//				m.SendBlock <- b
//				//need to update our own transaction pool (remove the transactions that we just mined)
//				m.HandleBlock(b)
//			}
//		}(ctx)
//	}
//}

// Mine When asked to mine, the miner selects the transactions
// with the highest priority to add to the mining pool.
func (m *Miner) Mine() *block.Block {
	// get a new context for the goroutine that we're about to spawn
	// should only mind if our transaction pool has enough priority
	if !m.TxPool.PriorityMet() {
		return nil
	}
	// We are currently mining
	m.Mining.Store(true)
	// create a new mining pool (get the highest priority transactions)
	m.MiningPool = m.NewMiningPool()
	// have to insert the coinbase transaction at the top of the transactions list
	txs := append([]*block.Transaction{m.GenerateCoinbaseTransaction(m.MiningPool)}, m.MiningPool...)
	// this is the block that we're going to mine!
	b := block.New(m.PreviousHash, txs, string(m.DifficultyTarget))
	// if results is true, we found a winning nonce! Otherwise, we failed (which won't ever actually happen
	// for us)
	// Change this to something else
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := m.CalculateNonce(ctx, b)
	// done mining
	m.Mining.Store(false)
	// send the block to the node to handle
	if result {
		utils.Debug.Printf("%v mined %v %v", utils.FmtAddr(m.Address), b.NameTag(), b.Summarize())
		m.SendBlock <- b
		//need to update our own transaction pool (remove the transactions that we just mined)
		m.HandleBlock(b)
		return b
	}
	return nil
}

// CalculateNonce finds a winning nonce for a block. It uses context to
// know whether it should quit before it finds a nonce (if another block
// was found). ASICSs are optimized for this task.
func (m *Miner) CalculateNonce(ctx context.Context, b *block.Block) bool {
	for i := uint32(0); i < m.Config.NonceLimit; i++ {
		select {
		case <-ctx.Done():
			return false
		default:
			b.Header.Nonce = i
			if bytes.Compare([]byte(b.Hash()), m.DifficultyTarget) == -1 {
				return true
			}
		}
	}
	return false
}

// GenerateCoinbaseTransaction generates a coinbase
// transaction based off the transactions in the mining pool.
// It does this by combining the fee reward to the minting reward,
// and sending that sum to itself.
func (m *Miner) GenerateCoinbaseTransaction(txs []*block.Transaction) *block.Transaction {
	// first collect the fees for all the transactions
	feeRwd := m.CalculateFees(txs)
	// find out what the minting reward is
	mntRwd := m.CalculateMintingReward()
	// get our public key, so that we can send the txo to ourselves
	pubK := m.Id.GetPublicKeyBytes()
	// Output with fee reward and minting reward to ourselves
	txo := &block.TransactionOutput{
		Amount:        feeRwd + mntRwd,
		LockingScript: pubK,
	}
	// the actual transaction. Note: no inputs since Coinbase!
	tx := &block.Transaction{
		Version:  0,
		Inputs:   []*block.TransactionInput{},
		Outputs:  []*block.TransactionOutput{txo},
		LockTime: 0,
	}
	return tx
}

// CalculateFees gets the total fees from a slice of transactions
func (m *Miner) CalculateFees(txs []*block.Transaction) uint32 {
	sums, err := m.getInputSums(txs)
	inSum := uint32(0)
	if err != nil {
		utils.Debug.Printf("[mine.CalculateFees] Error: %v", err)
	}
	for _, s := range sums {
		inSum += s
	}
	outSum := uint32(0)
	for _, t := range txs {
		outSum += t.SumOutputs()
	}
	if inSum > outSum {
		return inSum - outSum
	} else {
		fmt.Printf("[mine.CalculateFees] Error: inputs {%v} less than outputs {%v}\n", inSum, outSum)
		return 0
	}
}

// sumInputs returns the sum of the inputs of a slice of transactions,
// as well as an error if the function fails. This function sends a request to
// its GetInputsSum channel, which the node picks up. The node then handles
// the request, returning the sum of the inputs in the InputsSum channel.
// This function times out after 1 second.
func (m *Miner) getInputSums(txs []*block.Transaction) ([]uint32, error) {
	// time out after 1 second
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	// ask the node to sum the inputs for our transactions
	m.GetInputSums <- txs
	// wait until we get a response from the node in our SumInputs channel
	for {
		select {
		case <-ctx.Done():
			// Oops! We ran out of time
			return []uint32{0}, fmt.Errorf("[miner.sumInputs] Error: timed out")
		case sums := <-m.InputSums:
			// Yay! We got a response from our node.
			return sums, nil
		}
	}
}

// CalculateMintingReward calculates
// the minting reward the miner should receive based
// on the current chain length.
// Inputs:
// c	*Config the config for the miner
// chnLen	uint32 the length of the blockchain
// Returns:
// uint32	the amount of money the miner
// has minted
func (m *Miner) CalculateMintingReward() uint32 {
	c := m.Config
	chainLength := m.ChainLength.Load()
	if chainLength >= c.SubsidyHalvingRate*c.MaxHalvings {
		return 0
	}
	halvings := chainLength / c.SubsidyHalvingRate
	rwd := c.InitialSubsidy
	rwd /= uint32(math.Pow(2, float64(halvings)))
	return rwd
}
