package blockfinder

import (
	"github.com/square/beancounter/backend"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestFindBLock(t *testing.T) {
	b, err := backend.NewFixtureBackend("../fixtures/blocks.json")
	assert.NoError(t, err)

	bf := New(b)
	height, median, timestamp := bf.Search(time.Unix(1533153600, 0))

	assert.Equal(t, height, uint32(534733))
	assert.Equal(t, median.Unix(), int64(1533152846))
	assert.Equal(t, timestamp.Unix(), int64(1533152846))
}
