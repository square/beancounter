package deriver

import (
	"testing"

	. "github.com/square/beancounter/utils"
	"github.com/stretchr/testify/assert"
)

func TestDeriveMultiSigSegwit(t *testing.T) {
	xpubs := []string{
		"tpubD9jBarsLCKot45kvTTu7yWxmNnkBPbpn2CgS1F3yuxZGgohTkamYwJJKenZHrsYwPRJY66dk3ZUt3aZwZqFf7QGsgUUUcNSvvb9NXHFt5Vb",
		"tpubD8u5eHk8B6q63G4ekfBB2eVCeBYTrW2uCvi7Z3BF6bY2z33jW14Uzna8f5cFQWS3HFwdAzUWxqESq7j5x2CUQ7uBPgpXnpf8X9FPnh63XYd",
		"tpubD9dkteiWZqa3jL17meqVvy1RSUQsmJvU3hBfAosMzrSa69DacRku8yHy3E2fma1Q4Den25ukcsBL3bYTFyKjKbF8CEWHu86Xg9YXiY6CkeC",
		"tpubD8GrNWdYHDjdJryH5tng8LUzHSrEo4gToaguKnzthEqTxyfF13jTsp4sMtmso4n1VC58R5Wvt4Ua4npZTecR1xaGGYJgLLQj5sQGdD2xh2N",
	}
	deriver := NewAddressDeriver(Testnet, xpubs, 2, 0)
	assert.Equal(t, "2N4TmnHspa8wqFEUfxfjzHoSUAgwoUwNWhr", deriver.Derive(0, 0).String())
}

func TestDeriveGateway(t *testing.T) {
	xpubs := []string{
		"tpubD8L6UhrL8ML9Ao47k4pmdvUoiA6QUJVzrJ9BXLgU9idRKnvdRFGgjcxmVxojWGvCcjMi6QWCp8uMpCwWdSFRDNJ7utizxLy27sVWXQT4Jz7",
	}
	deriver := NewAddressDeriver(Testnet, xpubs, 1, 1234)
	assert.Equal(t, "mzoeuyGqMudyvKbkNx5dtNBNN59oKEAsPn", deriver.Derive(0, 0).String())
	assert.Equal(t, "moHN13u4RoMxujdaPxvuaTaawgWZ3LaGyo", deriver.Derive(1, 0).String())
}
