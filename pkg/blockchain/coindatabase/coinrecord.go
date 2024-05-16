package coindatabase

import "Coin/pkg/pro"

// CoinRecord is a record of which coins created by a Transaction
// have been spent. It is stored in the CoinDatabase's db.
type CoinRecord struct {
	Version        uint32
	OutputIndexes  []uint32
	Amounts        []uint32
	LockingScripts [][]byte
}

// EncodeCoinRecord returns a pro.CoinRecord given a CoinRecord.
func EncodeCoinRecord(cr *CoinRecord) *pro.CoinRecord {
	var outputIndexes []uint32
	var amounts []uint32
	var lockingScripts [][]byte
	for i := 0; i < len(cr.OutputIndexes); i++ {
		outputIndexes = append(outputIndexes, cr.OutputIndexes[i])
		amounts = append(amounts, cr.Amounts[i])
		lockingScripts = append(lockingScripts, cr.LockingScripts[i])
	}
	return &pro.CoinRecord{
		Version:        cr.Version,
		OutputIndexes:  outputIndexes,
		Amounts:        amounts,
		LockingScripts: lockingScripts,
	}
}

// DecodeCoinRecord returns a CoinRecord given a pro.CoinRecord.
func DecodeCoinRecord(pcr *pro.CoinRecord) *CoinRecord {
	var outputIndexes []uint32
	var amounts []uint32
	var lockingScripts [][]byte
	for i := 0; i < len(pcr.GetOutputIndexes()); i++ {
		outputIndexes = append(outputIndexes, pcr.GetOutputIndexes()[i])
		amounts = append(amounts, pcr.GetAmounts()[i])
		lockingScripts = append(lockingScripts, pcr.GetLockingScripts()[i])
	}
	return &CoinRecord{
		Version:        pcr.GetVersion(),
		OutputIndexes:  outputIndexes,
		Amounts:        amounts,
		LockingScripts: lockingScripts,
	}
}
