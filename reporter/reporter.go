package reporter

import (
	"fmt"
	"sync"
)

// Reporter tracks our progress while we are fetching data. It then spits out the balance and
// various pieces of information.

type Reporter struct {
	AddressesScheduled uint32
	AddressesFetched   uint32
	TxScheduled        uint32
	TxFetched          uint32
	TxAfterFilter      int
	Peers              int
}

var instance *Reporter
var once sync.Once

func GetInstance() *Reporter {
	once.Do(func() {
		instance = &Reporter{}
	})
	return instance
}

func (r *Reporter) Log(msg string) {
	fmt.Printf("%d/%d %d/%d/%d %d: %s\n", r.AddressesScheduled, r.AddressesFetched,
		r.TxScheduled, r.TxFetched, r.TxAfterFilter, r.Peers, msg)
}
