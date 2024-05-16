package address

import (
	"Coin/pkg/pro"
	"time"
)

type Address struct {
	Addr     string
	LastSeen uint32
	SentVer  time.Time
}

func New(addr string, lastSeen uint32) *Address {
	return &Address{Addr: addr, LastSeen: lastSeen, SentVer: time.Time{}}
}

func (a *Address) Serialize() *pro.Address {
	return &pro.Address{Addr: a.Addr, LastSeen: a.LastSeen}
}
