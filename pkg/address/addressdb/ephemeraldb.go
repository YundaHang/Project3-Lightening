package addressdb

import (
	"Coin/pkg/address"
	"Coin/pkg/pro"
	"errors"
	"sync"
)

type EphemeralAddressDb struct {
	addresses map[string]*address.Address
	limit     int
	sync.Mutex
}

// Add Returns true if address was added (or modified)
func (adb *EphemeralAddressDb) Add(a *address.Address) error {
	oldA := adb.addresses[a.Addr]
	if oldA != nil {
		return errors.New("address already exists")
	}
	if len(adb.addresses) >= adb.limit {
		return errors.New("address list full")
	}
	adb.addresses[a.Addr] = a
	return nil
}

func (adb *EphemeralAddressDb) Get(addr string) *address.Address {
	return adb.addresses[addr]
}

func (adb *EphemeralAddressDb) UpdateLastSeen(addr string, lastSeen uint32) error {
	a := adb.addresses[addr]
	if a == nil {
		return errors.New("address not found")
	}
	a.LastSeen = lastSeen
	return nil
}

func (adb *EphemeralAddressDb) List() []*address.Address {
	addresses := make([]*address.Address, 0, len(adb.addresses))
	for _, addr := range adb.addresses {
		addresses = append(addresses, addr)
	}
	return addresses
}

func (adb *EphemeralAddressDb) Serialize() []*pro.Address {
	addresses := make([]*pro.Address, 0, len(adb.addresses))
	for _, addr := range adb.addresses {
		addresses = append(addresses, addr.Serialize())
	}
	return addresses
}
