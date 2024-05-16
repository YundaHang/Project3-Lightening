package pkg

import (
	"Coin/pkg/address"
	"Coin/pkg/block"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
	"Coin/pkg/utils"
	"errors"
	"fmt"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Checks to see that requesting node is a peer and updates last seen for the peer
func (n *Node) peerCheck(addr string) error {
	if n.PeerDb.Get(addr) == nil {
		return errors.New("request from non-peered node")
	}
	err := n.PeerDb.UpdateLastSeen(addr, uint32(time.Now().UnixNano()))
	if err != nil {
		fmt.Printf("ERROR {Node.peerCheck}: error" +
			"when calling updatelastseen.\n")
	}
	return nil
}

// Version Handles version request (a request to become a peer)
func (n *Node) Version(ctx context.Context, in *pro.VersionRequest) (*pro.Empty, error) {
	// Reject all outdated versions (this is not true to Satoshi Client)
	if int(in.Version) != n.Config.Version {
		return &pro.Empty{}, nil
	}
	// If addr map is full or does not contain addr of ver, reject
	newAddr := address.New(in.AddrMe, uint32(time.Now().UnixNano()))
	if n.AddressDB.Get(newAddr.Addr) != nil {
		err := n.AddressDB.UpdateLastSeen(newAddr.Addr, newAddr.LastSeen)
		if err != nil {
			return &pro.Empty{}, nil
		}
	} else if err := n.AddressDB.Add(newAddr); err != nil {
		return &pro.Empty{}, nil
	}
	newPeer := peer.New(n.AddressDB.Get(newAddr.Addr), in.Version, in.BestHeight)
	// Check if we are waiting for a ver in response to a ver, do not respond if this is a confirmation of peering
	pendingVer := newPeer.Addr.SentVer != time.Time{} && newPeer.Addr.SentVer.Add(n.Config.VersionTimeout).After(time.Now())
	if n.PeerDb.Add(newPeer) && !pendingVer {
		newPeer.Addr.SentVer = time.Now()
		_, err := newAddr.VersionRPC(&pro.VersionRequest{
			Version:    uint32(n.Config.Version),
			AddrYou:    in.AddrYou,
			AddrMe:     n.Address,
			BestHeight: n.BlockChain.Length,
		})
		if err != nil {
			return &pro.Empty{}, err
		}
	}
	return &pro.Empty{}, nil
}

// GetBlocks Handles get blocks request (request for blocks past a certain block)
func (n *Node) GetBlocks(ctx context.Context, in *pro.GetBlocksRequest) (*pro.GetBlocksResponse, error) {
	blockHashes := make([]string, 0)
	br := n.BlockChain.BlockInfoDB.GetBlockRecord(in.TopBlockHash)
	if br == nil {
		return &pro.GetBlocksResponse{}, fmt.Errorf("[GetBlocks] did not have block")
	}
	if ind := br.Height; ind < n.BlockChain.Length {
		upperIndex := n.BlockChain.Length
		// Can send a maximum of 50 0 headers
		if ind+500 < upperIndex {
			upperIndex = ind + 500
		}
		for _, bn := range n.BlockChain.GetBlocks(ind+1, upperIndex) {
			blockHashes = append(blockHashes, bn.Hash())
		}
	}
	return &pro.GetBlocksResponse{BlockHashes: blockHashes}, nil
}

// GetData Handles get data request (request for a specific block identified by its hash)
func (n *Node) GetData(ctx context.Context, in *pro.GetDataRequest) (*pro.GetDataResponse, error) {
	blk := n.BlockChain.GetBlock(in.BlockHash)
	if blk == nil {
		utils.Debug.Printf("Node {%v} received a data req from the network for a block {%v} that could not be found locally.\n",
			n.Address, in.BlockHash)
		return &pro.GetDataResponse{}, nil
	}
	return &pro.GetDataResponse{Block: block.EncodeBlock(blk)}, nil
}

// SendAddresses Handles send addresses request (request for nodes to peer with the requesting node)
func (n *Node) SendAddresses(ctx context.Context, in *pro.Addresses) (*pro.Empty, error) {
	// Forward nodes to all neighbors if new nodes were found (without redundancy)
	foundNew := false
	for _, addr := range in.Addrs {
		if addr.Addr == n.Address {
			continue
		}
		newAddr := address.New(addr.Addr, addr.LastSeen)
		if p := n.PeerDb.Get(addr.Addr); p != nil {
			if p.Addr.LastSeen < addr.LastSeen {
				err := n.PeerDb.UpdateLastSeen(addr.Addr, addr.LastSeen)
				if err != nil {
					fmt.Printf("ERROR {Node.SendAddresses}: error" +
						"when calling updatelastseen.\n")
				}
				foundNew = true
			}
		} else if a := n.AddressDB.Get(addr.Addr); a != nil {
			if a.LastSeen < addr.LastSeen {
				err := n.AddressDB.UpdateLastSeen(addr.Addr, addr.LastSeen)
				if err != nil {
					fmt.Printf("ERROR {Node.SendAddresses}: error" +
						"when calling updatelastseen.\n")
				}
			}
		} else {
			err := n.AddressDB.Add(newAddr)
			if err == nil {
				foundNew = true
			}
		}
		// Try to connect to each new address as true peers (it is okay if this is repeated, this may be a reboot)
		go func() {
			_, err := newAddr.VersionRPC(&pro.VersionRequest{
				Version:    uint32(n.Config.Version),
				AddrYou:    newAddr.Addr,
				AddrMe:     n.Address,
				BestHeight: n.BlockChain.Length,
			})
			if err != nil {
				utils.Debug.Printf("%v recieved no response from VersionRPC to %v",
					utils.FmtAddr(n.Address), utils.FmtAddr(addr.Addr))
			}
		}()
	}
	if foundNew {
		bcPeers := n.PeerDb.GetRandom(2, []string{n.Address})
		for _, p := range bcPeers {
			_, err := p.Addr.SendAddressesRPC(in)
			if err != nil {
				utils.Debug.Printf("%v recieved no response from SendAddressesRPC to %v",
					utils.FmtAddr(n.Address), utils.FmtAddr(p.Addr.Addr))
			}
		}
	}
	return &pro.Empty{}, nil
}

// Handles get addresses request (request for all known addresses from a specific node)
func (n *Node) GetAddresses(ctx context.Context, in *pro.Empty) (*pro.Addresses, error) {
	utils.Debug.Printf("Node {%v} received a GetAddresses req from the network.\n",
		n.Address)
	return &pro.Addresses{Addrs: n.AddressDB.Serialize()}, nil
}

// ForwardTransaction Handles forward transaction request (tx propagation)
func (n *Node) ForwardTransaction(ctx context.Context, in *pro.TransactionWithAddress) (*pro.Empty, error) {
	theirTx, addr := block.DecodeTransactionWithAddress(in)
	myTx := block.DecodeTransaction(in.GetTransaction())
	n.mutex.Lock()
	defer n.mutex.Unlock()

	// TODO: handle using myTX
	if n.SeenTransactions[theirTx.Hash()] != nil {
		n.SeenTransactions[theirTx.Hash()].Count += 1
		return &pro.Empty{}, nil
	} else {
		if theirTx.Segwit {
			newAddr := address.New(addr, uint32(time.Now().UnixNano()))
			witness, _ := newAddr.GetWitnessesRPC(block.EncodeTransaction(theirTx))
			myTx.Witnesses = witness.GetWitnesses()
		}

		n.SeenTransactions[myTx.Hash()] = &TransactionWithCount{Transaction: myTx, Count: 1}
	}

	//------------------------ Do NOT edit below this line ----------------------------------//

	if !n.CheckTransaction(theirTx) {
		utils.Debug.Printf("%v recieved invalid %v", utils.FmtAddr(n.Address), theirTx.NameTag())
		return &pro.Empty{}, errors.New("transaction is not valid")
	}
	utils.Debug.Printf("%v recieved valid %v", utils.FmtAddr(n.Address), theirTx.NameTag())
	if n.Config.MinerConfig.HasMiner {
		n.Miner.HandleTransaction(theirTx)
	}

	for _, p := range n.PeerDb.List() {
		go func(addr *address.Address) {
			txWithAddr := &pro.TransactionWithAddress{
				Transaction: block.EncodeTransaction(theirTx),
				Address:     n.Address,
			}
			_, err := addr.ForwardTransactionRPC(txWithAddr)
			if err != nil {
				utils.Debug.Printf("%v recieved no response from ForwardTransaction to %v",
					utils.FmtAddr(n.Address), utils.FmtAddr(p.Addr.Addr))
			}
		}(p.Addr)
	}
	return &pro.Empty{}, nil
}

// ForwardBlock Handles forward block request (block propagation)
func (n *Node) ForwardBlock(ctx context.Context, in *pro.Block) (*pro.Empty, error) {
	b := block.DecodeBlock(in)

	// If we've seen this transaction more than once before, don't forward
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if numSeen, ok := n.SeenBlocks[b.Hash()]; ok {
		n.SeenBlocks[b.Hash()] = numSeen + 1
		if numSeen > 1 {
			return &pro.Empty{}, nil
		}
	} else {
		n.SeenBlocks[b.Hash()] = 1
	}

	if !n.CheckBlock(b) {
		utils.Debug.Printf("%v recieved invalid %v", utils.FmtAddr(n.Address), b.NameTag())
		return &pro.Empty{}, errors.New("block is not valid")
	}
	mnChn := n.BlockChain.LastHash == b.Header.PreviousHash && n.BlockChain.CoinDB.ValidateBlock(b.Transactions)
	n.BlockChain.HandleBlock(b)
	if n.Config.MinerConfig.HasMiner && mnChn {
		go n.Miner.HandleBlock(b)
	}
	if n.Config.WalletConfig.HasWallet && mnChn {
		go n.Wallet.HandleBlock(b.Transactions)
	}
	for _, p := range n.PeerDb.List() {
		go func(addr *address.Address) {
			_, err := addr.ForwardBlockRPC(block.EncodeBlock(b))
			if err != nil {
				utils.Debug.Printf("%v received no response from ForwardBlockRPC to %v",
					utils.FmtAddr(n.Address), utils.FmtAddr(p.Addr.Addr))
			}
		}(p.Addr)
	}
	return &pro.Empty{}, nil
}

// GetWitnesses is called by another SegWit node to get the witnesses (signatures) from you.
func (n *Node) GetWitnesses(ctx context.Context, in *pro.Transaction) (*pro.Witnesses, error) {
	// TODO
	tx := block.DecodeTransaction(in)

	if n.SeenTransactions[tx.Hash()] == nil {
		return &pro.Witnesses{}, status.Errorf(codes.Internal, "There is no witness!")
	} else {
		return &pro.Witnesses{
			Witnesses: n.SeenTransactions[tx.Hash()].Transaction.Witnesses}, nil
	}
}
