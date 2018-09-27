package utils

// PanicOnError panics if err is not nil
func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

type Network string

const (
	Mainnet Network = "mainnet"
	Testnet Network = "testnet"
)
