package backend

import (
	"sync"
	"testing"
	"time"

	"github.com/square/beancounter/deriver"
	"github.com/square/beancounter/utils"
	"github.com/stretchr/testify/assert"
)

func TestNonExistantFixtureFile(t *testing.T) {
	b, err := NewFixtureBackend("testdata/badpath")
	assert.Nil(t, b)
	assert.Error(t, err)
}

func TestBadFixtureFile(t *testing.T) {
	b, err := NewFixtureBackend("testdata/nonjsonfixture")
	assert.Nil(t, b)
	assert.Error(t, err)
}

func TestFinish(t *testing.T) {
	b, err := NewFixtureBackend("../accounter/testdata/tpub_data.json")
	assert.NoError(t, err)

	closed := make(chan bool)
	go func(ch chan bool) {
		b.processRequests()
		closed <- true
	}(closed)

	b.Finish()

	select {
	case <-closed:
		// processRequests() shut down as expected
		// PASS
	case <-time.Tick(100 * time.Millisecond):
		t.Errorf("expected a call to Finish() to cleanly shutdown the FixtureBackend")
	}
}

func TestNoAddress(t *testing.T) {
	b, err := NewFixtureBackend("../accounter/testdata/tpub_data.json")
	assert.NoError(t, err)

	b.AddrRequest(deriver.NewAddress("m/1'/1/0/1", "BAD_ADDRESS", utils.Testnet, 0, 1))

	var addrs []*AddrResponse
	var txs []*TxResponse

	fetchResults(b, &addrs, &txs, 100*time.Millisecond)

	// we don't know if address is legit or not, we just know if the address shows up
	// in the blockchain. The default behavior is just to return the address with no transactions
	assert.Len(t, addrs, 1)
	assert.False(t, addrs[0].HasTransactions())
	assert.Equal(t, "BAD_ADDRESS", addrs[0].Address.String())
	assert.Len(t, txs, 0)
}

func TestAddressNoTransactions(t *testing.T) {
	b, err := NewFixtureBackend("../accounter/testdata/tpub_data.json")
	assert.NoError(t, err)

	b.AddrRequest(deriver.NewAddress("m/1'/1234/0/61", "mfsNoNz57ANkYrCzHaLZDLoMGujBW8u3zv", utils.Testnet, 0, 61))

	var addrs []*AddrResponse
	var txs []*TxResponse

	fetchResults(b, &addrs, &txs, 100*time.Millisecond)

	assert.Len(t, addrs, 1)
	assert.False(t, addrs[0].HasTransactions())
	assert.Equal(t, "mfsNoNz57ANkYrCzHaLZDLoMGujBW8u3zv", addrs[0].Address.String())
	assert.Len(t, txs, 0)
}

func TestAddressWithTransactions(t *testing.T) {
	b, err := NewFixtureBackend("../accounter/testdata/tpub_data.json")
	assert.NoError(t, err)

	b.AddrRequest(deriver.NewAddress("m/1'/1234/0/7", "mi2udMvJHeeJJNp5wWKToa86L2cJUKzrby", utils.Testnet, 0, 7))

	var addrs []*AddrResponse
	var txs []*TxResponse

	fetchResults(b, &addrs, &txs, 100*time.Millisecond)

	assert.Len(t, addrs, 1)
	assert.True(t, addrs[0].HasTransactions())
	assert.Len(t, addrs[0].TxHashes, 2)
	assert.Contains(t, addrs[0].TxHashes, "5554c15d13002786a70a7151aad4eddce76633c60bc7f90e3dc70eb4f9c4b2b0")
	assert.Contains(t, addrs[0].TxHashes, "bd09a74381ffad78c162976ec27fc9c1dceda3c2bfe367541a7140b8dd6e1f4c")
	assert.Len(t, txs, 0)

	for _, tx := range addrs[0].TxHashes {
		b.TxRequest(tx)
	}

	fetchResults(b, &addrs, &txs, 100*time.Millisecond)
	// ensure that txs contain the same transaction hashes as addrs[0].TxHashes
	var txHashes []string
	txHashes = append(txHashes, txs[0].Hash)
	txHashes = append(txHashes, txs[1].Hash)
	assert.Contains(t, txHashes, "5554c15d13002786a70a7151aad4eddce76633c60bc7f90e3dc70eb4f9c4b2b0")
	assert.Contains(t, txHashes, "bd09a74381ffad78c162976ec27fc9c1dceda3c2bfe367541a7140b8dd6e1f4c")
}

func fetchResults(b Backend, addrs *[]*AddrResponse, txs *[]*TxResponse, timeout time.Duration) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		addrResponses := b.AddrResponses()
		txResponses := b.TxResponses()
		for {
			select {
			case addrResp, ok := <-addrResponses:
				if !ok {
					addrResponses = nil
					continue
				}
				*addrs = append(*addrs, addrResp)
			case txResp, ok := <-txResponses:
				if !ok {
					txResponses = nil
					continue
				}
				*txs = append(*txs, txResp)
			case <-time.Tick(timeout):

				wg.Done()
				return
			}
		}
	}()

	wg.Wait()
}
