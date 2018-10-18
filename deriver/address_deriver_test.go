package deriver

import (
	"testing"

	. "github.com/square/beancounter/utils"
	"github.com/stretchr/testify/assert"
)

func TestAddress(t *testing.T) {
	deriver := NewAddressDeriver(Mainnet, []string{"xpub6CjzRxucHWJbmtuNTg6EjPax3V75AhsBRnFKn8MEkc8UFFEhrCoWcQN6oUBhfZWoFKqTyQ21iNVK8KMbC44ifW25uyXaMPWkRtpwcbAWXJx"}, 1, "")
	addr := deriver.Derive(0, 5)
	assert.Equal(t, addr.Path(), "m/.../0/5")
	assert.Equal(t, addr.String(), "1N4VBTZqwLkHEKX79kjJ1WaYvX4c3txioz")
	assert.Equal(t, addr.Change(), uint32(0))
	assert.Equal(t, addr.Index(), uint32(5))
	assert.Equal(t, addr.Network(), Mainnet)
	assert.Equal(t, addr.Script(), "76a914e70369bfda4ba9bdcbb96cfd269a768573d0624c88ac")
}

func TestDeriveMultiSigSegwit(t *testing.T) {
	xpubs := []string{
		"tpubDAiPiLZeUdwo9oJiE9GZnteXj2E2MEMUb4knc4yCD87bL9siDgYcvrZSHZQZcYTyraL3fxVBRCcMiyfr3oQfH1wNo8J5i8aRAN56dDXaZxC",
		"tpubDBYBpkSfvt9iVSfdX2ArZq1Q8bVSro3sotbJhdZCG9rgfjdr4aZp7g7AF1P9w95X5fzuJzdZAqYWWU7nb37c594wR22hPY5VpYziXUN2yez",
		"tpubDAaTEMnf9SPKJweLaptFdy3Vmyhim5DKQxXRbsCxmAaUp8F84YD5GhdfmABwLddjHTftSVvUPuSru6vJ3b5N2hBveiGmZNE5N5yvB6WZ96c",
		"tpubDAXKYCetkje8HRRhAvUbAyuC5iF3SgfFWCVXfmrGCw3H9ExCYZVTEoeg7TjtDhgkS7TNHDRZUQNzGACWVzZCAYXy79vqku5z1geYmnsNLaa",
	}
	deriver := NewAddressDeriver(Testnet, xpubs, 2, "")
	assert.Equal(t, "2N4TmnHspa8wqFEUfxfjzHoSUAgwoUwNWhr", deriver.Derive(0, 0).String())
}

func TestDeriveGateway(t *testing.T) {
	xpubs := []string{
		"tpubDBrCAXucLxvjC9n9nZGGcYS8pk4X1N97YJmUgdDSwG2p36gbSqeRuytHYCHe2dHxLsV2EchX9ePaFdRwp7cNLrSpnr3PsoPLUQqbvLBDWvh",
	}
	deriver := NewAddressDeriver(Testnet, xpubs, 1, "")
	assert.Equal(t, "mzoeuyGqMudyvKbkNx5dtNBNN59oKEAsPn", deriver.Derive(0, 0).String())
	assert.Equal(t, "moHN13u4RoMxujdaPxvuaTaawgWZ3LaGyo", deriver.Derive(1, 0).String())
}
