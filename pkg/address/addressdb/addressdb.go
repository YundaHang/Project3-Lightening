package addressdb

import (
	"Coin/pkg/address"
	"Coin/pkg/pro"
)

type AddressDb interface {
	Add(*address.Address) error
	Get(string) *address.Address
	UpdateLastSeen(string, uint32) error
	List() []*address.Address
	Serialize() []*pro.Address
}

func New(eph bool, limit int) AddressDb {
	return &EphemeralAddressDb{addresses: make(map[string]*address.Address), limit: limit}
}
