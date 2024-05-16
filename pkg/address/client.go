package address

import (
	"Coin/pkg/pro"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"time"
)

// RPCTimeout is default timeout for rpc client calls
const RPCTimeout = 2 * time.Second

// clientUnaryInterceptor is a client unary interceptor that injects a default timeout
func clientUnaryInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	ctx, cancel := context.WithTimeout(ctx, RPCTimeout)
	defer cancel()
	return invoker(ctx, method, req, reply, cc, opts...)
}

func connectToServer(addr string) (*grpc.ClientConn, error) {
	return grpc.Dial(addr, []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithUnaryInterceptor(clientUnaryInterceptor),
	}...)
}

// GetConnection Returns callback to close connection
func (a *Address) GetConnection() (pro.CoinClient, *grpc.ClientConn, error) {
	cc, err := connectToServer(a.Addr)
	if err != nil {
		return nil, nil, err
	}
	return pro.NewCoinClient(cc), cc, err
}

func (a *Address) VersionRPC(request *pro.VersionRequest) (*pro.Empty, error) {
	c, cc, err := a.GetConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.VersionRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err2 := c.Version(context.Background(), request)
	a.SentVer = time.Now()
	return reply, err2
}

func (a *Address) GetBlocksRPC(request *pro.GetBlocksRequest) (*pro.GetBlocksResponse, error) {
	c, cc, err := a.GetConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.GetBlocksRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err2 := c.GetBlocks(context.Background(), request)
	return reply, err2
}

func (a *Address) GetDataRPC(request *pro.GetDataRequest) (*pro.GetDataResponse, error) {
	c, cc, err := a.GetConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.GetDataRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err2 := c.GetData(context.Background(), request)
	return reply, err2
}

func (a *Address) GetAddressesRPC(request *pro.Empty) (*pro.Addresses, error) {
	c, cc, err := a.GetConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.GetAddressesRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err2 := c.GetAddresses(context.Background(), request)
	return reply, err2
}

func (a *Address) SendAddressesRPC(request *pro.Addresses) (*pro.Empty, error) {
	c, cc, err := a.GetConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.SendAddressesRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err := c.SendAddresses(context.Background(), request)
	return reply, err
}

func (a *Address) ForwardTransactionRPC(request *pro.TransactionWithAddress) (*pro.Empty, error) {
	c, cc, err := a.GetConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.ForwardTransactionRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err2 := c.ForwardTransaction(context.Background(), request)
	return reply, err2
}

func (a *Address) ForwardBlockRPC(request *pro.Block) (*pro.Empty, error) {
	c, cc, err := a.GetConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.ForwardBlockRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err2 := c.ForwardBlock(context.Background(), request)
	return reply, err2
}

func (a *Address) GetWitnessesRPC(request *pro.Transaction) (*pro.Witnesses, error) {
	c, cc, err := a.GetConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.GetWitnessesRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err2 := c.GetWitnesses(context.Background(), request)
	return reply, err2
}

//-------------------------- Lightning --------------------------//

// GetLightningConnection Returns callback to close connection
func (a *Address) GetLightningConnection() (pro.LightningClient, *grpc.ClientConn, error) {
	cc, err := connectToServer(a.Addr)
	if err != nil {
		return nil, nil, err
	}
	return pro.NewLightningClient(cc), cc, err
}

func (a *Address) LightningVersionRPC(request *pro.VersionRequest) (*pro.Empty, error) {
	c, cc, err := a.GetLightningConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.LightningVersionRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err2 := c.Version(context.Background(), request)
	a.SentVer = time.Now()
	return reply, err2
}

func (a *Address) OpenChannelRPC(request *pro.OpenChannelRequest) (*pro.OpenChannelResponse, error) {
	c, cc, err := a.GetLightningConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.LightningVersionRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err2 := c.OpenChannel(context.Background(), request)
	return reply, err2
}

func (a *Address) GetUpdatedTransactionsRPC(request *pro.TransactionWithAddress) (*pro.UpdatedTransactions, error) {
	c, cc, err := a.GetLightningConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.LightningVersionRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err2 := c.GetUpdatedTransactions(context.Background(), request)
	return reply, err2
}

func (a *Address) GetRevocationKeyRPC(request *pro.SignedTransactionWithKey) (*pro.RevocationKey, error) {
	c, cc, err := a.GetLightningConnection()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = cc.Close()
		if err != nil {
			fmt.Printf("ERROR {Address.LightningVersionRPC}: " +
				"error when closing connection")
		}
	}()
	reply, err2 := c.GetRevocationKey(context.Background(), request)
	return reply, err2
}
