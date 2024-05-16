package pkg

import (
	"Coin/pkg/address"
	"Coin/pkg/address/addressdb"
	"Coin/pkg/block"
	"Coin/pkg/blockchain"
	"Coin/pkg/id"
	"Coin/pkg/lightning"
	"Coin/pkg/miner"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
	"Coin/pkg/utils"
	"Coin/pkg/wallet"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
)

type TransactionWithCount struct {
	Transaction *block.Transaction
	Count       uint32
}

/*
Node is the interface for interacting with the cryptocurrency.
The node handles all top level logic and communication between different
pieces of functionality. For example, it handles the logic of maintaining
a gRPC server and passing transactions and blocks to the miner, wallet,
chain, and other nodes on the network. It is also the interface between
the person using the computer which means all transaction requests and
directives to stop or resume the node is done on the node object.

*pro.UnimplementedCoinServer
Server *grpc.Server

Config *Config						the settings for the node
Address string 						the address that the node is listening to traffic on
Id   								id.ID the id of the node
Chain  *blockchain.Blockchain 		the blockchain
Wallet *wallet.Wallet 				the wallet
Mnr    *miner.Miner 				the miner
fGetAddr bool
AddressDb 	addressdb.AddressDb		a database of addresses of nodes that it knows about in the network
PeerDb   peer.PeerDb 				a database of peers the node is currently connected to

SeenTransactions   map[string]bool 	a map used to keep track of whether a transaction has been seen on the network before or not
SeenBlocks 							a map used to keep track of whether a block has been seen on the network before or not

Paused bool
*/
type Node struct {
	*pro.UnimplementedCoinServer
	Server *grpc.Server

	Config  *Config
	Address string
	Id      id.ID

	BlockChain *blockchain.BlockChain
	Wallet     *wallet.Wallet
	Miner      *miner.Miner

	LightningNode *lightning.LightningNode
	WatchTower    *lightning.WatchTower

	SeenTransactions map[string]*TransactionWithCount
	SeenBlocks       map[string]uint32

	fGetAddr bool // starts false, set to true when we request addresses from a node, cleared when we receive less than 1000 addresses from a node

	AddressDB addressdb.AddressDb
	PeerDb    peer.PeerDb

	Paused bool

	mutex sync.RWMutex
}

// New returns a new Node object based on
// a configuration
// Inputs:
// conf *Config the desired configuration
// of the Node
// Returns:
// *Node a pointer to the new node object
func New(conf *Config) *Node {
	i, _ := id.New(conf.IdConfig)
	return &Node{
		Config:           conf,
		Address:          "",
		Id:               i,
		BlockChain:       blockchain.New(conf.ChainConfig),
		Wallet:           wallet.New(conf.WalletConfig, i),
		Miner:            miner.New(conf.MinerConfig, i),
		LightningNode:    lightning.New(conf.LightningConfig),
		WatchTower:       &lightning.WatchTower{Id: i},
		SeenTransactions: make(map[string]*TransactionWithCount),
		SeenBlocks:       make(map[string]uint32),
		fGetAddr:         false,
		AddressDB:        addressdb.New(true, 1000),
		PeerDb:           peer.NewDb(true, 200, ""),
		Paused:           false,
		mutex:            sync.RWMutex{},
	}
}

// BroadcastTransaction broadcasts transactions created by the wallet
// to other peers in the network.
func (n *Node) BroadcastTransaction(tx *block.Transaction) {
	n.SeenTransactions[tx.Hash()] = &TransactionWithCount{
		Transaction: tx,
		Count:       1,
	}

	if n.Config.MinerConfig.HasMiner {
		go n.Miner.HandleTransaction(tx)
	}
	for _, p := range n.PeerDb.List() {
		d := block.EncodeTransaction(tx)
		//TODO: remove the proto transaction's witnesses before you send it off to your peers
		tx.Witnesses = nil
		go func(addr *address.Address) {
			txWithAddr := &pro.TransactionWithAddress{
				Transaction: d,
				Address:     n.Address,
			}
			_, err := addr.ForwardTransactionRPC(txWithAddr)
			if err != nil {
				utils.Debug.Printf("%v received no response from ForwardTransactionRPC to %v",
					utils.FmtAddr(n.Address), utils.FmtAddr(p.Addr.Addr))
			}
		}(p.Addr)
	}
}

// Start starts a node on the network. At first, the node is
// not technically connected to the network, since it has no
// one to connect to. So, this method opens up a listener and
// creates a gRPC server that it can be used to make and listen to
// requests on the network. It also starts another go routine
// for listening to messages from the wallet, miner, and blockchain
func (n *Node) Start() {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	addr := fmt.Sprintf("%v:%v", hostname, n.Config.Port)
	n.Address = addr
	n.PeerDb.SetAddr(addr)
	utils.Debug.Printf("%v started", utils.FmtAddr(n.Address))
	if n.Config.MinerConfig.HasMiner {
		n.Miner.SetAddress(addr)
	}
	if n.Config.ChainConfig.HasChain {
		n.BlockChain.SetAddress(addr)
	}
	if n.Config.WalletConfig.HasWallet {
		n.Wallet.SetAddress(addr)
	}
	// Added for Project 3: Lightning
	n.LightningNode.SetAddress(addr)
	n.LightningNode.Start()
	n.StartServer(addr)
	go func() {
		if n.Config.MinerConfig.HasMiner {
			for {
				select {
				case t := <-n.Wallet.TransactionRequests:
					n.BroadcastTransaction(t)
				case b := <-n.Miner.SendBlock:
					n.HandleMinerBlock(b)
				case b := <-n.BlockChain.ConfirmBlock:
					n.Wallet.HandleBlock(b.Transactions)
				case txs := <-n.Miner.GetInputSums:
					sums := n.BlockChain.GetInputSums(txs)
					n.Miner.InputSums <- sums
				case req := <-n.LightningNode.GetTransactionFromWallet:
					tx := n.Wallet.GenerateFundingTransaction(req.Amount, req.Fee, req.CounterPartyPubKey)
					n.LightningNode.ReceiveTransactionFromWallet <- tx
				case tx := <-n.LightningNode.BroadcastTransaction:
					n.BroadcastTransaction(tx)
				case r := <-n.LightningNode.RevocationKeys:
					n.WatchTower.RevocationKeys[r.TransactionHash] = r
				case r := <-n.WatchTower.RevokedTransactions:
					n.Wallet.HandleRevokedOutput(
						r.TransactionHash,
						r.TransactionOutput,
						r.OutputIndex,
						r.RevKey,
						r.ScriptType)
				}
			}
		} else {
			for {
				select {
				case t := <-n.Wallet.TransactionRequests:
					n.BroadcastTransaction(t)
				}
			}
		}
	}()
}

// HandleMinerBlock handles a block
// that was just made by the miner. It does this
// by sending the block to the chain so that it can be
// added, to the wallet, and to the network to be
// broadcast.
func (n *Node) HandleMinerBlock(b *block.Block) {
	n.SeenBlocks[b.Hash()] = 1
	// (1) send to chain
	n.BlockChain.HandleBlock(b)
	// (2) send a newly safe block to the wallet, appending
	// the new block to unsafe blocks
	if n.Config.WalletConfig.HasWallet {
		n.Wallet.HandleBlock(b.Transactions)
	}
	// (3) send to network to broadcast
	for _, p := range n.PeerDb.List() {
		//_, err := p.Addr.ForwardBlockRPC(block.EncodeBlock(b))
		//if err != nil {
		//	utils.Debug.Printf("%v received no response from ForwardBlockRPC to %v",
		//		utils.FmtAddr(n.Address), utils.FmtAddr(p.Addr.Addr))
		//}
		go func(addr *address.Address) {
			_, err := addr.ForwardBlockRPC(block.EncodeBlock(b))
			if err != nil {
				utils.Debug.Printf("%v received no response from ForwardBlockRPC to %v",
					utils.FmtAddr(n.Address), utils.FmtAddr(p.Addr.Addr))
			}
		}(p.Addr)
	}

}

// GetBalance returns the balance (amount of money)
// that someone currently has.
// Inputs:
// pk string the public key of the person that the
// balance wants to be known for.
// Returns:
// uint32 the amount of money (the balance) that
// the person with that public key has
func (n *Node) GetBalance(pk []byte) uint32 {
	return n.BlockChain.GetBalance(pk)
}

// StartMiner starts the miner, which means the miner
// is now actively waiting for enough transactions
// to mine.
func (n *Node) StartMiner() {
	n.Miner.StartMiner()
}

// ConnectToPeer connects to a certain peer in the network. This just
// serves as an interface for the real functionality contained
// within the Router.
// Inputs:
// addr string the address of the node that you want
// to connect to.
func (n *Node) ConnectToPeer(addr string) {
	a := address.New(addr, 0)
	_, err := a.VersionRPC(&pro.VersionRequest{
		Version:    uint32(n.Config.Version),
		AddrYou:    addr,
		AddrMe:     n.Address,
		BestHeight: n.BlockChain.Length,
	})
	if err != nil {
		utils.Debug.Printf("%v received no response from VersionRPC to %v",
			utils.FmtAddr(n.Address), utils.FmtAddr(addr))
	}
}

// BroadcastAddress broadcasts the node's address
func (n *Node) BroadcastAddress() {
	myAddr := pro.Address{Addr: n.Address, LastSeen: uint32(time.Now().UnixNano())}
	for _, p := range n.PeerDb.List() {
		go func(addr *address.Address) {
			_, err := addr.SendAddressesRPC(&pro.Addresses{Addrs: []*pro.Address{&myAddr}})
			if err != nil {
				utils.Debug.Printf("%v received no response from SendAddressesRPC to %v",
					utils.FmtAddr(n.Address), utils.FmtAddr(p.Addr.Addr))
			}
		}(p.Addr)
	}
}

// Bootstrap attempts to build a blockchain based on the
// pre-existing one that other nodes have. This may happen
// when a node first joins the network, or if the node left
// the network for a while (paused), then rejoined.
func (n *Node) Bootstrap() error {
	utils.Debug.Printf("%v bootstrapping from %v peers with top block %v", utils.FmtAddr(n.Address), len(n.PeerDb.List()), n.BlockChain.LastBlock.NameTag())
	topBlockHash := n.BlockChain.LastHash
	var wg sync.WaitGroup
	var longestRes *pro.GetBlocksResponse
	var addr *address.Address
	if len(n.PeerDb.List()) == 0 {
		return errors.New("no peers to bootstrap from")
	}
	for _, p := range n.PeerDb.List() {
		wg.Add(1)
		go func(p *peer.Peer) {
			res, err := p.Addr.GetBlocksRPC(&pro.GetBlocksRequest{TopBlockHash: topBlockHash})
			if err != nil {
				wg.Done()
				return
			}
			if longestRes == nil || len(res.BlockHashes) > len(longestRes.BlockHashes) {
				longestRes = res
				addr = p.Addr
			}
			wg.Done()
		}(p)
	}
	wg.Wait()
	if longestRes == nil {
		return errors.New("no peers gave responses")
	}
	for _, h := range longestRes.BlockHashes {
		pb, _ := addr.GetDataRPC(&pro.GetDataRequest{BlockHash: h})
		b := block.DecodeBlock(pb.Block)
		n.SeenBlocks[b.Hash()] = 1
		n.BlockChain.HandleBlock(b)
	}
	return nil
}

func (n *Node) StartServer(addr string) {
	lis, err := net.Listen("tcp4", addr)
	if err != nil {
		panic(err)
	}
	// Open node to connections
	n.Server = grpc.NewServer()
	pro.RegisterCoinServer(n.Server, n)
	go func() {
		err = n.Server.Serve(lis)
		if err != nil {
			fmt.Printf("ERROR {Node.StartServer}: error" +
				"when trying to serve server")
		}
	}()
}

func (n *Node) PauseNetwork() {
	n.Server.Stop()
	utils.Debug.Printf("%v paused", utils.FmtAddr(n.Address))
}

func (n *Node) ResumeNetwork() {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	addr := fmt.Sprintf("%v:%v", hostname, n.Config.Port)
	n.StartServer(addr)
	utils.Debug.Printf("%v resumed", utils.FmtAddr(n.Address))
}

// Kill kills any threads currently managed by the Node or that
// it previously started. It also does any necessary clean up.
func (n *Node) Kill() {
	n.Server.GracefulStop()
}
