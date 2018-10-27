package accounter

import (
	"testing"

	"github.com/square/beancounter/backend"
	"github.com/square/beancounter/deriver"
	. "github.com/square/beancounter/utils"
	"github.com/stretchr/testify/assert"
)

func TestProcessTransactions(t *testing.T) {
	a := Accounter{
		blockHeight:  100,
		transactions: make(map[string]transaction),
	}
	// https://api.blockcypher.com/v1/btc/main/txs/38f6366700f12dc902718ab5222c8ae67a4514ed07ee8aea364feec22bf6424f?limit=50&includeHex=true
	a.transactions["1"] = transaction{
		height: 10,
		hex:    "020000000001012afcb974e6a5404d4a93d051746a2fe419d92f5c04a2f8cd19c80fc0068defa8010000001716001423819adc0d76e951ba8b0568d8e05b141d4ba5adfeffffff025b2297080000000017a9149ae902bcdf8859abc54a1baf253ea294336a212e87283463660000000017a9140662710b351b7834914b0303cf32c08f3e937c6d87024830450221008a52e1944137e724bebb477b9bf545b62b83d14e435ce9fb25b05bb9857a038202204b79214c7f90b76372d3ce11d9d645b159abcd1038fa20201c4168b09b103192012102f3ccaceecb31987e00e0d159459dcafea4c9bcd64f36b0dee034279bab96d1985c510800",
		vin:    []vin{},
		vout:   []vout{},
	}

	// https://api.blockcypher.com/v1/btc/main/txs/8e6c5effcc2d31b55bb7964747338db3a273b62c9b2385de50117ea09baefcb4?limit=50&includeHex=true
	a.transactions["2"] = transaction{
		height: 100,
		hex:    "0100000001c75e0c4591ca1a9a2b0d899b4312407fe4b220e6ad5cdee9c3e02be4bc99f321cd0300008b483045022100d0d49ae3f43a3a7799faa2c951dec497a867f6b904b7edba9f87755be15fd67e02200a454de50d7b582d0ed033403fac99d9e41536c018bf0b68058a79f85ef060a6014104fcf07bb1222f7925f2b7cc15183a40443c578e62ea17100aa3b44ba66905c95d4980aec4cd2f6eb426d1b1ec45d76724f26901099416b9265b76ba67c8b0b73dffffffff0388080000000000001976a914fa0692278afe508514b5ffee8fe5e97732ce066988ac0000000000000000166a146f6d6e69000000000000002700000022abdcac9622020000000000001976a9146d17f31f8cef7831b87896402bf2986d4361afb988ac00000000",
		vin:    []vin{},
		vout:   []vout{},
	}

	// https://api.blockcypher.com/v1/btc/main/txs/72abc310daf35dd21d3ba92e72bc39c47f245ba99441db7d8b05642842dbd62c?limit=50&includeHex=true
	a.transactions["3"] = transaction{
		height: 120,
		hex:    "020000000126a2fd32e2a97ce4d1ef244247edb1fa986b20f5e1d4b2a9f4279ac3f8c97fee020000006a473044022079d203284b5d656d229500451c03bf9f9242e865e205a883c2a3f847a598e579022008e7912074faaa343b0fbe8107f773ec19dcd2371a592c28f38d3ce68ab5bb3201210267759b65b67634ce5fb3114b3c9547f7efcd5a095fa6fc2bb8c532ec5df9d479feffffff0294cb43000000000017a914d237f4c583a567ab7e3ba091ee14953774b1ceda87198025000000000017a914bd706cbfc32ffaff0d2604f4bfbe490e0b36e211875d510800",
		vin:    []vin{},
		vout:   []vout{},
	}

	// https://api.blockcypher.com/v1/btc/main/txs/da47ec573c7639e61ca1bc77ab866f17fe0f1c55ee4aeb6c6daa8d35e3df950c?limit=50&includeHex=true
	a.transactions["4"] = transaction{
		height: 0,
		hex:    "02000000000105201000e0ad28b2c06cc333f9325f49b6e1532dc47b071a0b0e9039a2eecc2f3f000000001716001403ff881365a8c3318c645b2db7de3d0e9bb01e32feffffff2cd6db422864058b7ddb4194a95b247fc439bc722ea93b1dd25df3da10c3ab720100000017160014d970197441c15e71fa5926dcf494ecca0540de39feffffff68e8dcb6ac8e0edba261b953059839390738eca19139b7bbe97658c5f9866cc90000000000feffffffb3fe4a69cc708263cf3281e0a876c657ea6a424065d5d92f251802c28c7b422f00000000171600148a3e01c8043955c6d2cd754fbc9e0c063a69fcf1feffffffdbce7c09a67bfa1a2737a04205e5d145b689f98c634326a8be92d3aadd2787450000000017160014e0ae2e0c87f5ec2a885457af322550652265141cfeffffff02e00f9700000000001976a914c8bfd11d19fbdab3a0a0b525c8040b96ccac183f88ac02880c000000000017a9141c0aab9855abc6d9564714dbbfc0b8da5a8f2aca870247304402202f750e6b1e5b6759a784178ab9ce4162f2812c597690202655ad569e90a7f30802204cdb33c66c31da483e4c4bfac7d85bc9f1552fc2aa7b0eb060311cfc8db0c623012102d007f6f2ce40cc13c295598bb447faa5cb0a42cdacd39fbcb15d1152d87dc898024730440220561b78a66d16ab1f741a1f4c6cc42ad7257cfac9238f14bb10f9e351f2535c9002202383c70c4daa3b69229081b00a69732023b1dd1474eb0b9e32a7c08e112f642b012102c3bc868e47418bdef127d702af9593ce42790038a27f019ec467b0cd3802fbc802473044022076c69f83accbe0a5a42bbc8b03e4c77205a2bbb64da03ed0a8ece9bd9248e5b902204694c3ac5ec87ba4d93da30e9871a1e5581555722d60a534c79e92f55012d1080121037cd588476186076662d993913a9450ee12816efa5dbfbf1a41208694966c7c6c02483045022100b182661da8afbb51b6528e63a566b06d915e8d6cb3375bf7c26e825c9c89cb54022065bb0af673614611a5c9574dd2d47cf941afc6752dba7c6456f9a3e3beab74e30121029f0e39491bcebac56a52ce46c0bf0a782563698108bba3f290fdf232cb0c0635024830450221008a7d81574993ad3944102aac990e584ec09aa1071f31b0fdefa4adde21da613c022009d7ed96ddebe52ac760e5799dab659f53a961a79dda4929d18e1eb525372bfe012102f2eafee2c4ab2197c5394fb388128cd986b6c880eb0ba27e025ed74c64e7e6a75d510800",
		vin:    []vin{},
		vout:   []vout{},
	}

	a.processTransactions()

	assert.Equal(t, len(a.transactions), 2)
	assert.Equal(t, len(a.transactions["1"].vin), 1)
	assert.Equal(t, a.transactions["1"].vin[0], vin{
		prevHash: "a8ef8d06c00fc819cdf8a2045c2fd919e42f6a7451d0934a4d40a5e674b9fc2a",
		index:    1,
	})
	assert.Equal(t, len(a.transactions["1"].vout), 2)
	assert.Equal(t, a.transactions["1"].vout[0], vout{
		value:   144122459,
		address: "a9149ae902bcdf8859abc54a1baf253ea294336a212e87",
		ours:    false,
	})
	assert.Equal(t, a.transactions["1"].vout[1], vout{
		value:   1717777448,
		address: "a9140662710b351b7834914b0303cf32c08f3e937c6d87",
		ours:    false,
	})

	assert.Equal(t, len(a.transactions["2"].vin), 1)
	assert.Equal(t, a.transactions["2"].vin[0], vin{
		prevHash: "21f399bce42be0c3e9de5cade620b2e47f4012439b890d2b9a1aca91450c5ec7",
		index:    973,
	})
	assert.Equal(t, len(a.transactions["2"].vout), 3)
	assert.Equal(t, a.transactions["2"].vout[0], vout{
		value:   2184,
		address: "76a914fa0692278afe508514b5ffee8fe5e97732ce066988ac",
		ours:    false,
	})
	assert.Equal(t, a.transactions["2"].vout[1], vout{
		value:   0,
		address: "6a146f6d6e69000000000000002700000022abdcac96",
		ours:    false,
	})
	assert.Equal(t, a.transactions["2"].vout[2], vout{
		value:   546,
		address: "76a9146d17f31f8cef7831b87896402bf2986d4361afb988ac",
		ours:    false,
	})
}

func TestComputeBalanceTestnet(t *testing.T) {
	pubs := []string{"tpubDBrCAXucLxvjC9n9nZGGcYS8pk4X1N97YJmUgdDSwG2p36gbSqeRuytHYCHe2dHxLsV2EchX9ePaFdRwp7cNLrSpnr3PsoPLUQqbvLBDWvh"}
	deriver := deriver.NewAddressDeriver(Testnet, pubs, 1, "")
	b, err := backend.NewFixtureBackend("testdata/tpub_data.json")
	assert.NoError(t, err)
	a := New(b, deriver, 100, 1435169)

	assert.Equal(t, uint64(267893477), a.ComputeBalance())
}
