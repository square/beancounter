package reporter

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Reporter tracks our progress while we are fetching data. It then spits out the balance and
// various pieces of information.

type Reporter struct {
	addressesScheduled uint32
	addressesFetched   uint32
	txScheduled        uint32
	txFetched          uint32
	peers              int32
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
	fmt.Printf("%d/%d %d/%d %d: %s\n", r.GetAddressesScheduled(), r.GetAddressesFetched(),
		r.GetTxScheduled(), r.GetTxFetched(), r.GetPeers(), msg)
}

func (r *Reporter) Logf(format string, args ...interface{}) {
	r.Log(fmt.Sprintf(format, args...))
}

func (r *Reporter) IncAddressesFetched() {
	atomic.AddUint32(&r.addressesFetched, 1)
}

func (r *Reporter) GetAddressesFetched() uint32 {
	return atomic.LoadUint32(&r.addressesFetched)
}

func (r *Reporter) IncAddressesScheduled() {
	atomic.AddUint32(&r.addressesScheduled, 1)
}

func (r *Reporter) GetAddressesScheduled() uint32 {
	return atomic.LoadUint32(&r.addressesScheduled)
}

func (r *Reporter) IncTxFetched() {
	atomic.AddUint32(&r.txFetched, 1)
}

func (r *Reporter) GetTxFetched() uint32 {
	return atomic.LoadUint32(&r.txFetched)
}

func (r *Reporter) IncTxScheduled() {
	atomic.AddUint32(&r.txScheduled, 1)
}

func (r *Reporter) GetTxScheduled() uint32 {
	return atomic.LoadUint32(&r.txScheduled)
}

func (r *Reporter) GetPeers() int32 {
	return atomic.LoadInt32(&r.peers)
}

func (r *Reporter) SetPeers(n int32) {
	atomic.StoreInt32(&r.peers, n)
}
