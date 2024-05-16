package test

import (
	"Coin/pkg"
	"Coin/pkg/block"
	"Coin/pkg/blockchain"
	"Coin/pkg/id"
	"Coin/pkg/pro"
	"Coin/pkg/utils"
	"Coin/pkg/wallet"
	"fmt"
	"github.com/phayes/freeport"
	"log"
	"os"
	"strconv"
	"testing"
	"time"
)

func setNodeConfig(conf *pkg.Config, i int) *pkg.Config {
	conf.ChainConfig.BlockInfoDBPath = "blockinfodata" + strconv.Itoa(i)
	conf.ChainConfig.CoinDBPath = "coindata" + strconv.Itoa(i)
	conf.ChainConfig.ChainWriterDBPath = "data" + strconv.Itoa(i)
	return conf
}

// CleanUp is used to clean up testing side effects, where num is
// the number of blockchains (which create directories)
func CleanUp(chains []*blockchain.BlockChain) {
	paths := []string{"coindata", "blockinfodata", "data"}
	for i, chain := range chains {
		// manually close the levelDBs
		chain.BlockInfoDB.Close()
		chain.CoinDB.Close()
		// erase the paths
		for _, path := range paths {
			path += strconv.Itoa(i)
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				if err2 := os.RemoveAll(path); err2 != nil {
					fmt.Errorf("coudld not remove %v", path)
				}
			}
		}
	}
}

func GetFreePort() int {
	port, err := freeport.GetFreePort()
	if err != nil {
		log.Fatal(err)
	}
	return port
}

func GenesisConfig(port int) *pkg.Config {
	c := pkg.DefaultConfig(port)
	c.HasCustomId = true
	c.CustomID, _ = id.LoadInSmplID(blockchain.GENPK, blockchain.GENPVK)
	return c
}

// NewCluster
// First node is always the genesis node
func NewCluster(n int) []*pkg.Node {
	cluster := []*pkg.Node{NewGenesisNode()}
	for i := 1; i < n; i++ {
		conf := setNodeConfig(pkg.DefaultConfig(GetFreePort()), i)
		cluster = append(cluster, pkg.New(conf))
	}
	return cluster
}

func StartCluster(c []*pkg.Node) {
	for _, node := range c {
		node.Start()
	}
}

func StartMiners(c []*pkg.Node) {
	for _, node := range c {
		node.Miner.StartMiner()
	}
}

func ConnectCluster(c []*pkg.Node) {
	for i := 0; i < len(c); i++ {
		for j := 0; j < len(c); j++ {
			if i == j {
				continue
			}
			c[i].LightningNode.ConnectToPeer(c[j].LightningNode.Address)
			c[i].ConnectToPeer(c[j].Address)
		}
	}
}

func NewGenesisNode() *pkg.Node {
	return pkg.New(setNodeConfig(GenesisConfig(GetFreePort()), 0))
}

func CheckMainChains(t *testing.T, nodes []*pkg.Node) {
	t.Helper()
	n1Chn := nodes[0].BlockChain
	for i := 1; i < len(nodes); i++ {
		CheckEqualBlocks(t, n1Chn.List(), nodes[i].BlockChain.List())
	}
}

func CheckTransactionSeen(t *testing.T, nodes []*pkg.Node, tx *block.Transaction) {
	t.Helper()
	for _, n := range nodes {
		if _, ok := n.SeenTransactions[tx.Hash()]; !ok {
			t.Errorf("Error: node {%v} should have seen transaction {%v}", utils.FmtAddr(n.Address), tx.Hash())
		}
	}
}

func CheckEqualBlocks(t *testing.T, c1 []*block.Block, c2 []*block.Block) {
	t.Helper()
	if len(c1) != len(c2) {
		t.Errorf("Failed: One node had a main chain length of %v, while another node had %v\n", len(c1), len(c2))
	}
	for i := 0; i < len(c1); i++ {
		if c1[i].Hash() != c2[i].Hash() {
			t.Errorf("Failed: At idx %v, one node's block hash was %v, while another node's block hash was %v", i, c1[i].Hash(), c2[i].Hash())
		}
	}
}

func CreateMockedTransaction(inputAmounts []uint32, outputAmounts []uint32) *block.Transaction {
	// fill array of proto inputs
	var inputArray []*pro.TransactionInput
	for i := 0; i < len(inputAmounts); i++ {
		input := &pro.TransactionInput{
			OutputIndex: uint32(i),
		}
		inputArray = append(inputArray, input)
	}
	// fill array of proto outputs
	var outputArray []*pro.TransactionOutput
	for i := 0; i < len(outputAmounts); i++ {
		output := &pro.TransactionOutput{
			Amount: outputAmounts[i],
		}
		outputArray = append(outputArray, output)
	}
	//create transaction
	protoTransaction := &pro.Transaction{
		Inputs:  inputArray,
		Outputs: outputArray,
	}
	transaction := block.DecodeTransaction(protoTransaction)
	return transaction
}

func AssertBalance(t *testing.T, w *wallet.Wallet, amount uint32) {
	if w.Balance != amount {
		t.Errorf("Expected wallet balance: %v\n Actual wallet balance: %v", amount, w.Balance)
	}
}

func CreateMockedWallet() *wallet.Wallet {
	var i, _ = id.CreateSimpleID()
	w := wallet.New(wallet.DefaultConfig(), i)
	return w
}

func CreateMockedGenesisWallet() *wallet.Wallet {
	w := CreateMockedWallet()
	genBlock := GenesisBlock()
	genBlock.Transactions[0].Outputs[0].LockingScript = w.Id.GetPublicKeyBytes()
	w.HandleBlock(genBlock.Transactions)
	return w
}

//FillWalletWithCoins will fill a wallet with n coins of amount amt
func FillWalletWithCoins(w *wallet.Wallet, n uint32, amt uint32) {
	b := MockedBlockWithNCoins(w, n, amt)
	w.HandleBlock(b.Transactions)
	for i := 0; i < 6; i++ {
		w.HandleBlock(MockedBlock().Transactions)
	}
}

func CreateDifficultyTarget(numZeros int) []byte {
	return utils.CalcPOWD(numZeros)
}

func CreateHardestDifficultyTarget() []byte {
	ret := ""
	for i := 0; i < 64; i++ {
		ret += "0"
	}
	return []byte(ret)
}

func AssertSize(t *testing.T, actualSize int, expectedSize int) {
	if actualSize != expectedSize {
		t.Errorf("Expected size: %v\n Actual size: %v", expectedSize, actualSize)
	}
}

func CheckTransactionInTXPool(t *testing.T, n *pkg.Node, tx *block.Transaction) {
	t.Helper()
	if !n.Miner.TxPool.TxQ.Has(tx) {
		t.Errorf("Miner {%v} did not have transaction %v in its TXPool", utils.FmtAddr(n.Miner.Address), tx.Hash())
	}
}

// GenerateTransactions creates transactions. If transactions is nil, it
// will create a new list. Otherwise, it will create them based on inputted
// transactions
func GenerateTransactions(transactions []*block.Transaction) []*block.Transaction {
	var txs []*block.Transaction
	if transactions == nil {
		for i := 0; i < 10; i++ {
			txo := &block.TransactionOutput{
				Amount:        100 + uint32(i),
				LockingScript: []byte{},
			}
			tx := &block.Transaction{
				Version:  0,
				Inputs:   []*block.TransactionInput{},
				Outputs:  []*block.TransactionOutput{txo},
				LockTime: uint32(time.Now().UnixNano()),
			}
			txs = append(txs, tx)
		}
	} else {
		for _, tx := range transactions {
			txi := &block.TransactionInput{
				ReferenceTransactionHash: tx.Hash(),
				OutputIndex:              0,
				UnlockingScript:          []byte{},
			}
			txo := &block.TransactionOutput{
				Amount:        tx.Outputs[0].Amount - 10,
				LockingScript: []byte{},
			}
			newTx := &block.Transaction{
				Version:  0,
				Inputs:   []*block.TransactionInput{txi},
				Outputs:  []*block.TransactionOutput{txo},
				LockTime: 0,
			}
			txs = append(txs, newTx)
		}
	}
	return txs
}
