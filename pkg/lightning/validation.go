package lightning

import (
	"Coin/pkg/block"
	"Coin/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ValidateTransaction automatically validates a transaction
func (ln *LightningNode) ValidateTransaction(tx *block.Transaction) bool {
	// Normally, this would actually contain some validation. We've omitted that for now.
	return true
}

// ValidateAndSign is used by the server to validate incoming funding and refund transaction
func (ln *LightningNode) ValidateAndSign(tx *block.Transaction) error {
	if !ln.ValidateTransaction(tx) {
		return status.Errorf(codes.Internal, "Transaction was not valid")
	}
	// Now we can sign the valid transaction
	signature, err := utils.Sign(ln.Id.GetPrivateKey(), []byte(tx.Hash()))
	if err != nil {
		return err
	}
	tx.Witnesses = append(tx.Witnesses, signature)
	return nil
}

func (ln *LightningNode) SignTransaction(tx *block.Transaction) {
	signature, err := utils.Sign(ln.Id.GetPrivateKey(), []byte(tx.Hash()))
	if err != nil {
		return
	}
	tx.Witnesses = append(tx.Witnesses, signature)
}
