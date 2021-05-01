package backend

import (
	"github.com/square/beancounter/backend/electrum"
	. "github.com/square/beancounter/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTransactionCache(t *testing.T) {
	eb := NewElectrumBackend("foobar", "1234", Testnet)

	tx1 := electrum.Transaction{Hash: "aaaaaa", Height: 100}
	tx2 := electrum.Transaction{Hash: "bbbbbb", Height: 100}
	tx3 := electrum.Transaction{Hash: "cccccc", Height: 101}
	badTx := electrum.Transaction{Hash: "aaaaaa", Height: 102}

	eb.cacheTxs([]*electrum.Transaction{&tx1, &tx2})

	assert.Equal(t, int64(tx1.Height), eb.getTxHeight(tx1.Hash))
	assert.Equal(t, int64(tx2.Height), eb.getTxHeight(tx2.Hash))
	assert.Panics(t, func() { eb.getTxHeight(tx3.Hash) })

	eb.cacheTxs([]*electrum.Transaction{&tx2, &tx3})

	assert.Equal(t, int64(tx1.Height), eb.getTxHeight(tx1.Hash))
	assert.Equal(t, int64(tx2.Height), eb.getTxHeight(tx2.Hash))
	assert.Equal(t, int64(tx3.Height), eb.getTxHeight(tx3.Hash))

	assert.Panics(t, func() { eb.cacheTxs([]*electrum.Transaction{&badTx}) })
}
