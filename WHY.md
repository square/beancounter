# Why we wrote Beancounter
### _and why is wasn't trivial_
Intuitively, computing the balance for a Bitcoin wallet should be easy. This document explains the issues we ran into with both, our hot and cold wallets. We solved these issues by implementing Beancounter, a tool we then open-sourced.

# How is the blockchain structured?
A simplified explanation: Bitcoin stores transactions on a blockchain. A blockchain is a list of blocks. Each block contains transactions.

A transaction consumes previous transaction:index pairs (debits) and creates new outputs (credits). The outputs are address:amount pairs and are called unspent transactions (UTXO) until they get consumed in a subsequent transaction.

```
            [...] -> [block dc52a] -> [...]
                           |
                           +-- transaction d1e6d
                           | +-- debit: prevtx 068aa:1
                           | +-- debit: prevtx bfa74:4
                           | +-- credit: some address:amount
                           | +-- credit: another address:amount
                           | ...
                           |
                           +- transaction 1c867
                           | ...
                           |
                           ...
```

The important point for our discussion is that the funds are the UTXOs and there’s no per-address state. Each Bitcoin client needs to compute the per-address balance for the wallets they care about.

_Note: not all cryptocurrencies are implemented in this way. For instance, Ethereum maintains contract state._

https://github.com/bitcoinbook/bitcoinbook (or the paper version of the book) is a good place to learn more about the underlying structures.

# What is a Bitcoin wallet?
A wallet is a collection of addresses. For [Hierarchical Deterministic wallets][bip32] (which is what we use), each wallet has a huge number of addresses (typically 2^31 * 2). To figure out the balance, we need to find each addresses’ balance and sum things up. Doing the following would however be extremely slow:

```
balance = 0;
for (int i=0; i<2^31; i++) {
  address = wallet.get(i)
  balance += lookup(address)
}

```

[bip32]: https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki

# How does [insert favorite wallet software] do it?
Most Bitcoin software defines a gap window (called lookahead in [BitcoinJ][btcj]). The software initially has a small set of addresses (the size of the gap, say 20) and watches for transactions which include addresses from this set. As the software sees transactions, it computes the 20 following addresses and adds them to the set.

Note: the gap heuristic does not always work well in practice (there have been cases where the default was too small).

[btcj]: https://bitcoinj.github.io/

# What happened when we tried using Electrum
- Our hot wallets have a large number of transactions at a high velocity (multiple transactions per block).
  - We could potentially fix Electrum. Would require learning the code base and contributing patches (or filing bugs and hoping maintainers will be interested to address them).
- Our cold wallet were initially using a custom derivation scheme, not compatible with Electrum.
  - This is no longer true, we eventually switched to using the same scheme as our hot wallets.


# Why we can’t simply use our core Bitcoin nodes
- Square runs a few Bitcoin nodes. We explored using them (although we would need to figure out a way to safely expose the RPC protocol only to specific people).
- The standard Bitcoin node however does not maintain an index of address => balance. It also does not maintain an index of address => transaction. The only index the core nodes maintain is transaction => block (for all transaction or only for unspent transaction depending whether prunning is enabled or not).
  - ([there][pr1] [were][pr2] [some][pr3] PRs to address this, but none of them were ready).
  - We could query the entire history, pick out the addresses we care about and compute the balance. But it requires writing a fair amount of code.

[pr1]: https://github.com/bitcoin/bitcoin/pull/5048
[pr2]: https://github.com/bitcoin/bitcoin/pull/8660
[pr3]: https://github.com/bitcoin/bitcoin/pull/9806


# What about alternative Bitcoin nodes?
- There are multiple alternative implementations which maintain an address => transactions index (and also an address => balance):
  - Electrum servers ([original unmaintained implementation][elect] and [ElectrumX][electx])
  - [btcd][btcd] (Alternative Bitcoin implementation in Go)
  - [Bitcoin fork][btcfork] with an index patch (the author tried to upstream the changes).
  - [indexd-server][btcidx] which sits on top of a core node.
- The simplest API (address => balance) however does not take a max block height as a parameter (at least neither Electrum nor btcd’s APIs). A new block is mined every ~10 min, but it takes much longer to iterate over all the addresses in our large hot wallets. We are thus unable to determine the wallet balance.
- We had to use two indexes, the address => transactions index combined with the transactions => block index to compute the balance. This turns out to be quite a bit of engineering effort!

[btcd]:    https://github.com/btcsuite/btcd
[btcfork]: https://github.com/btcdrak/bitcoin
[btcidx]:  https://github.com/CounterpartyXCP/indexd-server
[elect]:   https://github.com/spesmilo/electrum-server
[electx]:  https://github.com/kyuupichan/electrumx

# What about trust. Can we trust Electrum nodes?
- Beancounter allows us to connect to our own Electrum node, our own btcd node, or public Electrum nodes. This allows us to pick the trust model we care about.
- Engineers can connect to their own nodes (e.g. when debugging).
non-technical people can still use Beancounter without having to worry about getting access to a full node.

# Converting human dates to block height.
- The tool we built works with blocks. Auditors want to know the wallet’s balance on a specific date (e.g. 3/31/18 at 00:00:00 GMT)
- Each block on the blockchain carries a timestamp, but the timestamps are not monotonic!
- However, the median of any [11][blk_height] consecutive blocks is guaranteed to be monotonic.
- We can therefore map a block to a monotonic time (by defining the block’s time as being the median of its timestamp, its previous 5 blocks and its next 5 blocks) and perform a binary search to map a human date (e.g. 3/31/18 at 00:00:00 GMT) to the (closest) specific block.

[blk_height]: https://github.com/bitcoin/bitcoin/blob/e8ad580f51532f6bfa6cb55688bffcc12196c0ac/src/chain.h#L324


# Random other issues/thoughts
- Electrum API is a little painful to use. E.g. some methods return a stream of results, but we only want one result. No way to unsubscribe from the stream.
- Electrum’s API versioning is a little messy—not 100% backwards compatible and the initial client library we picked wasn’t very well implemented.
- Running a node (for development purpose) takes a long time to sync and requires a large SSD (at least 250GB ~ Oct ‘18). For some reason, the btcd node takes ~7 days for its initial sync (what is it doing?).
- We cannot easily give random people (e.g. external auditors) access to our private Bitcoin or btcd node. The RPC mechanism does not support fine grained permissions / read-only access.
- We had to deal with potential race conditions when generating additional addresses to watch while synchronously scanning the blockchain. Our hands were tied when it came to parallelism. Talking to Electrum servers can be slow and needs to be multiplexed.
- Many other random issues (see github commit history for a full list).
Perhaps we could have leveraged some other tool? Some kind of Hadoop map-reduce over the blockchain.
