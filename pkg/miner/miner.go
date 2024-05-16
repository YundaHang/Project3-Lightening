package miner

import (
	"Coin/pkg/block"
	"Coin/pkg/blockchain"
	"Coin/pkg/id"
	"Coin/pkg/utils"
	"fmt"
	"go.uber.org/atomic"
	"sync"
)

// Miner supports the functionality of mining new transactions broadcast from the network to a new block.
// Config represents the configuration (settings) for the miner.
// Id represents the identity of the miner, so that the miner can properly make the coinbase transaction.
// TxPool contains all transactions that the miner is either waiting to mine, or is mining.
// MiningPool contains all transactions that the miner is currently mining.
// ChainLength is the length of the main chain.
// Active is a channel used to entirely shut down the miner's ability to mine.
// Mining tells whether the miner is currently mining.
// SendBlock is used to send newly mined blocks to the node in order to be broadcast on the network.
// PoolUpdated is used to send alerts of pool updates to the miner
// GetInputCoins is used by the miner to ask the node for the coins used for the inputs on
// a block
// InputCoins is the channel by which the node sends the requested coins back to the miner
type Miner struct {
	Config *Config
	Id     id.ID

	TxPool     *TxPool
	MiningPool MiningPool

	PreviousHash     string
	Address          string
	ChainLength      *atomic.Uint32
	DifficultyTarget []byte

	Active *atomic.Bool
	Mining *atomic.Bool

	SendBlock   chan *block.Block
	PoolUpdated chan bool

	GetInputSums chan []*block.Transaction
	InputSums    chan []uint32

	mutex sync.Mutex
}

// New constructs a new Miner according to a config and the id of a node.
func New(c *Config, id id.ID) *Miner {
	if !c.HasMiner {
		return nil
	}
	return &Miner{
		Config:           c,
		PreviousHash:     blockchain.GenesisBlock(blockchain.DefaultConfig()).Hash(),
		Id:               id,
		TxPool:           NewTxPool(c),
		MiningPool:       []*block.Transaction{},
		ChainLength:      atomic.NewUint32(1),
		SendBlock:        make(chan *block.Block),
		PoolUpdated:      make(chan bool),
		GetInputSums:     make(chan []*block.Transaction),
		InputSums:        make(chan []uint32),
		Mining:           atomic.NewBool(false),
		DifficultyTarget: c.InitialPOWDifficulty,
		Active:           atomic.NewBool(false),
	}
}

// SetAddress sets the address of the node that the miner is currently on.
func (m *Miner) SetAddress(a string) {
	m.mutex.Lock()
	m.Address = a
	m.mutex.Unlock()
}

// StartMiner is a wrapper around the mine method just in case any additional work is needed to do before or after
// mining in the future.
func (m *Miner) StartMiner() {
	m.Active.Store(true)
	//go m.Mine()
	//m.PoolUpdated <- true
}

// HandleBlock handles a validated block from the network. The transactions on the block need to be checked
// with the transaction pool, in case the transaction pool has any transactions that have already been mined.
//The miner's perspective of the hash of the last block on the main chain needs to be reset, and the chain length
//needs to be updated. Lastly, the miner needs to restart.
// Inputs:
// b - a new block that is being added to the blockchain.
func (m *Miner) HandleBlock(b *block.Block) {
	if b == nil {
		return
	}
	m.IncrementChainLength()
	m.UpdateTXPool(b.Transactions)
}

// UpdateTXPool handles updating
// the transaction pool based
// on the new transactions in the block.
func (m *Miner) UpdateTXPool(txs []*block.Transaction) {
	if txs == nil {
		fmt.Printf("ERROR {Miner.HndlChkBlk}: The " +
			"slice of inputted transactions was nil.\n")
		return
	}
	//T
	m.TxPool.CheckTransactions(txs)
	if m.Active.Load() {
		m.PoolUpdated <- true
	}
}

// HandleTransaction handles a validated transaction from the network. If the miner isn't currently mining and
// the priority threshold is met, then the miner is told to mine.
// Inputs:
// t *block.Transaction the validated transaction that was received from the network
func (m *Miner) HandleTransaction(t *block.Transaction) {
	if t == nil {
		fmt.Printf("ERROR {Miner.HndlTx}: The" +
			"inputted transaction was nil.\n")
		return
	}
	sums, err := m.getInputSums([]*block.Transaction{t})
	if err != nil {
		utils.Debug.Printf("[miner.HandleTransaction] Failed to get inputs for transaction")
	}
	m.TxPool.Add(t, sums[0])
	if m.Active.Load() {
		m.PoolUpdated <- true
	}
}

// SetChainLength sets the miner's perspective of the length of the main chain.
// Inputs:
// l - the most updated length of the blockchain so that the miner can appropriately calculate its minting reward
func (m *Miner) SetChainLength(l uint32) {
	m.ChainLength.Store(l)
}

// IncrementChainLength increments the miner's perspective of the length of the main chain.
func (m *Miner) IncrementChainLength() {
	m.ChainLength.Inc()
}

func (m *Miner) Pause() {
	m.Active.Store(false)
	m.PoolUpdated <- true
	utils.Debug.Printf("%v paused mining", utils.FmtAddr(m.Address))
}

func (m *Miner) Resume() {
	m.Active.Store(true)
	m.PoolUpdated <- true
	utils.Debug.Printf("%v resumed mining", utils.FmtAddr(m.Address))
}

// Kill closes the miner's channels and stops the current mining process.
func (m *Miner) Kill() {
	m.Active.Store(false)
	close(m.PoolUpdated)
	close(m.SendBlock)
}
