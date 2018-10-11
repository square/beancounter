package blockfinder

import (
	"fmt"
	"github.com/square/beancounter/backend"
	"sort"
	"time"
)

// Blockfinder uses the backend to find the last block before a given timestamp.
// For each block, the block's time is computed by taking the median of the previous  11 blocks.
type Blockfinder struct {
	blocks         map[uint32]time.Time
	backend        backend.Backend
	blockResponses <-chan *backend.BlockResponse
}

// New instantiates a new Blockfinder
func New(b backend.Backend) *Blockfinder {
	bf := &Blockfinder{
		backend: b,
	}
	bf.blocks = make(map[uint32]time.Time)
	bf.blockResponses = b.BlockResponses()
	return bf
}

// Returns block height, block median, block timestamp
func (bf *Blockfinder) Search(timestamp time.Time) (uint32, time.Time, time.Time) {
	target := timestamp.Unix()

	min := uint32(10) // any small number above 5 works
	minMedian := bf.searchSync(min)

	// Use chainheight - 6 (because of min confirmations) - 5 (because of the way we compute median)
	max := bf.backend.ChainHeight() - 11
	maxMedian := bf.searchSync(max)

	for max-min > 1 {
		avg := (max + min) / 2
		avgTimestamp := bf.searchSync(avg)
		fmt.Printf("min: %d %d, avg: %d %d, max: %d %d, target: %d\n",
			min, minMedian, avg, avgTimestamp, max, maxMedian, target)

		if avgTimestamp < minMedian || avgTimestamp > maxMedian {
			panic("non-monotonic medians")
		}

		if target == avgTimestamp {
			min = avg
			minMedian = avgTimestamp
			break
		} else if target > avgTimestamp {
			min = avg
			minMedian = avgTimestamp
		} else {
			max = avg
			maxMedian = avgTimestamp
		}
	}

	bf.backend.BlockRequest(min)
	blockHeader := <-bf.blockResponses

	// Give recorder backend a chance to write the data
	bf.backend.Finish()

	return min, time.Unix(minMedian, 0), blockHeader.Timestamp
}

// TODO: cache requests
// around 283655 is a good test case for this function...
// We define the median time as the median of time timestamps from 5 blocks before and 5 blocks
// after. We have to pick a total of 11 blocks, because that's how the validation rule is defined.
// (https://en.bitcoin.it/wiki/Block_timestamp, https://github.com/bitcoin/bitcoin/blob/0.17/src/chain.h#L307)
// but we don't have to do the previous 11. Any consecutive 11 blocks has monotonic medians. By looking
// at the previous 5 and next 5, we reduce the delta between the block time displayed on a website
// such as live.blockcypher.com and the median we compute. It makes things less confusing for people
// who might not understand why we need to look at the median.
func (bf *Blockfinder) searchSync(height uint32) int64 {
	for i := height - 5; i <= (height + 5); i++ {
		bf.backend.BlockRequest(i)
	}
	timestamps := []int64{}
	for i := 0; i < 11; i++ {
		blockHeader := <-bf.blockResponses
		timestamps = append(timestamps, blockHeader.Timestamp.Unix())
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	return timestamps[5]
}
