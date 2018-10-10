package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMax(t *testing.T) {
	v1 := uint32(0)
	v2 := uint32(3418911847)
	v3 := uint32(356309450)

	assert.Equal(t, Max(v1), v1)
	assert.Equal(t, Max(v2), v2)
	assert.Equal(t, Max(v3), v3)

	assert.Equal(t, Max(v1, v1), v1)
	assert.Equal(t, Max(v1, v2), v2)
	assert.Equal(t, Max(v1, v3), v3)
	assert.Equal(t, Max(v2, v1), v2)
	assert.Equal(t, Max(v2, v2), v2)
	assert.Equal(t, Max(v2, v3), v2)
	assert.Equal(t, Max(v3, v1), v3)
	assert.Equal(t, Max(v3, v2), v2)
	assert.Equal(t, Max(v3, v3), v3)

	assert.Equal(t, Max(v1, v2, v3), v2)
	assert.Equal(t, Max(v1, v2, v3, v1), v2)
}

func TestXpubToNetwork(t *testing.T) {
	assert.Equal(t, XpubToNetwork("xpub6C774QqLVXvX3WBMACHRVdWTyPphFh45cXFvawg9eFuNAK2DNPsWDf1zJcSyZWY59FNspYUCAUJJXhmVzCPcWzLWDm6yEQSN9982pBAsj1k"), Mainnet)

	assert.Equal(t, XpubToNetwork("tpubDC5s7LsM3QFZz8CKNz8ePa2wpvQiq5LsGXrkoaaGsLhNx44wTr13XqoKEMCFPWMK4yen2DsLN7ArrZuqRqQE24Y9kNN51bpcjNdbWpJngdG"), Testnet)
}

func TestAddressToNetwork(t *testing.T) {
	assert.Equal(t, AddressToNetwork("19YomTTzGd55JM18pmj6Vv2F7ZqkaQDnRF"), Mainnet)
	assert.Equal(t, AddressToNetwork("3DmcpZprPpPLFsBsuMeGTik11DyQVsadQK"), Mainnet)

	assert.Equal(t, AddressToNetwork("mm8xEm6YS8B7ErLYYqcdF6URWkS1BWnqtY"), Testnet)
	assert.Equal(t, AddressToNetwork("2MvmkK3F4vT2h3gLjxz66SwQ5zW5XbsdZLu"), Testnet)
	assert.Equal(t, AddressToNetwork("n3s7pVRvCEuXfF5fyh74JXmYg45q4Wev86"), Testnet)
}
