Beancounter
==========

[![license](http://img.shields.io/badge/license-apache_2.0-blue.svg?style=flat)](https://raw.githubusercontent.com/square/beancounter/master/LICENSE) [![travis](https://img.shields.io/travis/com/square/beancounter.svg?maxAge=3600&logo=travis&label=travis)](https://travis-ci.com/square/beancounter)
[![coverage](https://coveralls.io/repos/github/square/beancounter/badge.svg?branch=master)](https://coveralls.io/r/square/beancounter) [![report](https://goreportcard.com/badge/github.com/square/beancounter)](https://goreportcard.com/report/github.com/square/beancounter)

Beancounter is a command line utility to audit the balance of [Hierarchical Deterministic (HD)][bip32] wallets at a given point in time (or block height). The tool is designed to scale and work for wallets with a large number of addresses or a large number of transactions, with support ranging from simple watch wallets to more complicated multisig + segwit. If you're curious, [here's why we decided to write Beancounter](WHY.md) in the first place.

Beancounter currently supports two types of backends to query the blockchain:
1. Electrum public servers. When using these servers, Beancounter behaves in a similar fashion to an Electrum client wallet. The servers are queried for transaction history for specific addresses. Using Electrum servers is easiest but requires trusting public servers to return accurate information. There is also potential privacy exposure.

2. Private Btcd node. Btcd is a Bitcoin full node which implements transaction indexes. Setting up a Btcd node can take some time (the initial sync takes ~7 days) and requires maintaining the node up-to-date. The benefit is however a higher level of guarantee that the transaction history is accurate.

![logo](https://raw.githubusercontent.com/square/beancounter/master/coffee.jpg)

[bip32]: https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki

Getting Started
===============

Installing
----------
Requires Go 1.11 or higher.

```
$ git clone https://github.com/square/beancounter/
$ cd beancounter
$ go build -mod=vendor
```

Deriving the child pubkey
-------------------------
Let's imagine we want to track the balance of `tpubD8L6UhrL8ML9...` and the derivation being used is `m/1'/1234/change/index`.

We need to manually perform the `1234` derivation:

```
$ ./beancounter keytree 1234
Enter pubkey #1 out of #1:
tpubD8L6UhrL8ML9Ao47k4pmdvUoiA6QUJVzrJ9BXLgU9idRKnvdRFGgjcxmVxojWGvCcjMi6QWCp8uMpCwWdSFRDNJ7utizxLy27sVWXQT4Jz7
Child pubkey #1: tpubDBrCAXucLxvjC9n9nZGGcYS8pk4X1N97YJmUgdDSwG2p36gbSqeRuytHYCHe2dHxLsV2EchX9ePaFdRwp7cNLrSpnr3PsoPLUQqbvLBDWvh
```

We can then use `tpubDBrCAXucLxvj...` to compute the balance.

Compute balance of a HD wallet (using Electrum)
-----------------------------------------------
```
$ ./beancounter compute-balance --type multisig --block-height 1438791
Enter pubkey #1 out of #1:
tpubDBrCAXucLxvjC9n9nZGGcYS8pk4X1N97YJmUgdDSwG2p36gbSqeRuytHYCHe2dHxLsV2EchX9ePaFdRwp7cNLrSpnr3PsoPLUQqbvLBDWvh
...
Balance: 267893477
```

Compute balance of a single address (using Electrum)
----------------------------------------------------
```
$ ./beancounter compute-balance --type single-address --lookahead 1 --block-height 1438791
Enter single address:
mzoeuyGqMudyvKbkNx5dtNBNN59oKEAsPn
...
Balance: 111168038
```

Compute balance of a HD wallet (using Btcd)
-------------------------------------------

[https://github.com/btcsuite/btcd](https://github.com/btcsuite/btcd) contains information on how to setup a node.

Beancounter requires `addrindex=1`, `txindex=1`, and `notls=1`. If your node is on a remote server,
we recommend tunneling the RPC traffic over ssh or some other secure tunnel.

```
rpcuser=mia
rpcpass=ilovebrownies
rpclisten=127.0.0.1:8334
notls=1

blocksonly=1
addrindex=1
txindex=1
```

Once the Btcd is up and running, you can do:
```
$ ./beancounter compute-balance --type multisig --block-height 1438791 --backend btcd --addr localhost:8334 --rpcuser mia --rpcpass ilovebrownies
Enter pubkey #1 out of #1:
tpubDBrCAXucLxvjC9n9nZGGcYS8pk4X1N97YJmUgdDSwG2p36gbSqeRuytHYCHe2dHxLsV2EchX9ePaFdRwp7cNLrSpnr3PsoPLUQqbvLBDWvh
...
Balance: 267893477
```

Details
=======

Beancounter is implemented in Go. We picked Go because we wanted an easy build and distribution process. We appreciate the ability to cross-compile and distribute static binaries which don't have external dependencies.

We use the following process to determine a deterministic wallet's balance at a given block height:

1. Derive external (receive) and internal (change) addresses.
2. For each address, query the backend for a list of transactions. We keep deriving additional addresses until we find a large number of unused addresses.
3. Prune the transaction list to remove transactions newer than the block height.
4. For each transaction, query the backend for the raw transaction.
5. For each transaction, track whether the output belongs to the wallet and whether
   it got spent.
6. Iterate over the transactions and compute the final balance.

Contributing
============

We appreciate any pull request which fixes bugs or adds features!

If you need ideas on how to contribute, we would enjoy a 3rd backend (Bitcoin-core based, processing
each block by streaming the entire blockchain) as well as additional wallet types (e.g. multisig non-segwit).

Additional unittests and improvements to comments/docs are also always welcome.

License
=======

    Copyright 2018 Square, Inc.

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
