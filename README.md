Beancounter
==========

[![license](http://img.shields.io/badge/license-apache_2.0-blue.svg?style=flat)](https://raw.githubusercontent.com/square/beancounter/master/LICENSE) [![travis](https://img.shields.io/travis/com/square/beancounter.svg?maxAge=3600&logo=travis&label=travis)](https://travis-ci.com/square/beancounter)
[![coverage](https://coveralls.io/repos/github/square/beancounter/badge.svg?branch=master)](https://coveralls.io/r/square/beancounter) [![report](https://goreportcard.com/badge/github.com/square/beancounter)](https://goreportcard.com/report/github.com/square/beancounter)

Beancounter is a command line utility to audit the balance of Hierarchical Deterministic (HD) wallets. The tool is
designed to scale and work for wallets with a large number of addresses or a large number of transactions.
The tool supports various types of wallets, including multisig + segwit.

We picked Go as the programming language, because Go makes building the tool for different platforms trivial. We also
appreciate the ability to distribute static binaries which don't have external dependencies.

We built Beancounter because we wanted a tool which can compute the balance of a wallet at any
given block height.

The process to determine a deterministic wallet's balance involves the following:
1. Use the blockchain to build an address index. Bitcoin Core doesn't offer this, but Btcd,
   Electrum, and a few other tools do.
2. Derive external (receive) and internal (change) addresses.
3. Fetch the transaction history for a single address. Prune the history at the desired block height.
4. Compute the credit and debit for each transaction. Repeat until we find a large number of
   unused addresses.

This tool currently supports two backends: Electrum and Btcd. Using Electrum is easier (connects to
public nodes) but has some disadvantages (need to trust a third party, potential privacy leak, etc.).
Using Btcd requires running a private node and the initial sync can take a long time.

![logo](https://raw.githubusercontent.com/square/beancounter/master/coffee.jpg)

Getting Started (using Electrum)
================================

```
brew install dep

dep ensure
go run main.go -m 2 -n 4 --account 1234
```

Sample Output
=============

```
$ go run main.go -m 1 -n 1 --account 1234 --network testnet --lookahead 10
Enter tpub #1 out of #1:
tpubD8L6UhrL8ML9Ao47k4pmdvUoiA6QUJVzrJ9BXLgU9idRKnvdRFGgjcxmVxojWGvCcjMi6QWCp8uMpCwWdSFRDNJ7utizxLy27sVWXQT4Jz7
Checking balance for m/1'/1234/0/0 mzoeuyGqMudyvKbkNx5dtNBNN59oKEAsPn ... 100000000 100000000
Checking balance for m/1'/1234/0/1 mz37noAanMGMW1BMGvyhNnY1fqfTg726Ka ... 15674439 115674439
Checking balance for m/1'/1234/0/2 mzPCbXLqiHLNhVj6reah8VWXVT8X69Ssuf ... 30000000 145674439
Checking balance for m/1'/1234/0/3 my1FMCXyo84tC1LkFXX8LctbtpycUmUnPx ... 0 145674439
Checking balance for m/1'/1234/0/4 n1GohMiYdx8Q8PSBynH34vdgZXH1tid7cW ... 0 145674439
Checking balance for m/1'/1234/0/5 mzMF12VsXCL23ov7RwSbZdVnLzJ1MnxEzT ... ∅
Checking balance for m/1'/1234/0/6 n1EstV7h4Jyx1XKLY7fdx6ufFRMxBVdieN ... 100000000 245674439
Checking balance for m/1'/1234/0/7 mi2udMvJHeeJJNp5wWKToa86L2cJUKzrby ... 3000000 248674439
Checking balance for m/1'/1234/0/8 mwiGFquFDNhCE7z66xU1GPZxr3Vr838ayJ ... ∅
Checking balance for m/1'/1234/0/9 mzQinQemMnnfKtJmqvnwC4PcvmJrKxnSMi ... ∅
Checking balance for m/1'/1234/0/10 mofjpvyi5Hn5veLnU6xvWiFvuuN9ABMiez ... ∅
Checking balance for m/1'/1234/0/11 mosHkEQ17EzDbBotFeZdUdw9JymCB6ygdD ... ∅
Checking balance for m/1'/1234/0/12 n2fo4VZADUoboCbGhyw3Rrp5s9HtdeiS1L ... ∅
Checking balance for m/1'/1234/0/13 mybf8GEdfxqBRhzwPMfxu1LNnCWoR2oSro ... ∅
Checking balance for m/1'/1234/0/14 mmfFY4UJHJBSjz3ve7tvaSrxNwCReVtifC ... 0 248674439
Checking balance for m/1'/1234/0/15 myEyz9WPaZmXqnhtN86xM8S66wVysBRNtN ... 0 248674439
Checking balance for m/1'/1234/0/16 mp1eTBsFMfE1akuhEad4z2GaYLXRxnQ5wR ... ∅
Checking balance for m/1'/1234/0/17 mqyEEeArE2VdWcvkRCE14kKPxngJn5vrw3 ... ∅
Checking balance for m/1'/1234/0/18 mqRHRhMwGv1eARSicG4Jr2PGjnNomTJvU1 ... ∅
Checking balance for m/1'/1234/0/19 mskQduVL7QBZCEWJUit8u9N8Cj4CFouPWX ... 1000000 249674439
Checking balance for m/1'/1234/0/20 mpMrwsAcYpNV5DsDq3HBv4F45TzZwvFCNo ... ∅
Checking balance for m/1'/1234/0/21 n3dXhVju9R2kJPjhnZME2E1yiQm1Scq6yn ... ∅
Checking balance for m/1'/1234/0/22 ms2f9UsMBE1n71kDZrH2yiUTSCoaS3C1sX ... ∅
Checking balance for m/1'/1234/0/23 moo5ASJxjmYomHXFRsqV9DFdbd9j5Knk2w ... ∅
Checking balance for m/1'/1234/0/24 n2AYb9qVFmBtyBQSUonFMcaqo1pXDwhHKC ... ∅
Checking balance for m/1'/1234/0/25 n4FWvGyuimxGjQJd7ZWht56vF3nudxYVeo ... ∅
Checking balance for m/1'/1234/0/26 mmNXWy7aXQoiKfqyYpX2yfDvqUKjMEp9SL ... ∅
Checking balance for m/1'/1234/0/27 n4rT8fSspf7WRSE46drbdEPjyYvncbrdgz ... ∅
Checking balance for m/1'/1234/0/28 mh972ndpUc46sa81vqDNz31U9QvdRpYgUB ... ∅
Checking balance for m/1'/1234/1/0 moHN13u4RoMxujdaPxvuaTaawgWZ3LaGyo ... 246600 249921039
Checking balance for m/1'/1234/1/1 n2aNi43rgX8YD5NMJK55dgHp7n7rdGzbYj ... 6504400 256425439
Checking balance for m/1'/1234/1/2 mv5dfNyTKMwED5g26LMZJYwN5QckEXrqe2 ... 300000 256725439
Checking balance for m/1'/1234/1/3 mth6eDXa7Yx6Bccci6j2PfgWgGVPZYw8qo ... ∅
Checking balance for m/1'/1234/1/4 mhx1M6yV58z9LAyEKemU6dcn5vNSdHU6rN ... ∅
Checking balance for m/1'/1234/1/5 mp9MMATLEzuJgmCVodLeN385zcB7J4crRR ... ∅
Checking balance for m/1'/1234/1/6 n2eB9gA1ywoUh1tszRSwBHTW4kunQqCPJm ... ∅
Checking balance for m/1'/1234/1/7 n4Yt3Wng8waeykRC8Tgr1JQ6Gz5SQFZaYD ... ∅
Checking balance for m/1'/1234/1/8 mnJBZg297ETSQZrJKGHDLLfuQwTMmK5U9h ... ∅
Checking balance for m/1'/1234/1/9 mvaqsto6jVBUTX7Fxgzx6GW3un94xuGW3D ... ∅
Checking balance for m/1'/1234/1/10 mrqAS4K81RftZ1n6TJPDMMJUzThNZh1SVV ... ∅
Checking balance for m/1'/1234/1/11 muxFaecKfqKt98G4VNUyqEVwVvqoPAtspj ... ∅

+----------------+------------------------------------+------------------------------------------------------------------+
|      PATH      |              ADDRESS               |                         TRANSACTION HASH                         |
+----------------+------------------------------------+------------------------------------------------------------------+
| m/1'/1234/0/0  | mzoeuyGqMudyvKbkNx5dtNBNN59oKEAsPn | ac3b83a9f90f73c7cac1e07b017d5c78ce6c79e74a0d72a6c80e84fb0adeb6ba |
| m/1'/1234/0/1  | mz37noAanMGMW1BMGvyhNnY1fqfTg726Ka | 820198df9af70251fdfcc3cae3f2995f09093a1677e6ac7800f6417d6679e929 |
| m/1'/1234/0/2  | mzPCbXLqiHLNhVj6reah8VWXVT8X69Ssuf | 60014a64f5c808ce7af726ae28e60ad986dff519a5d8014528df39976b10d705 |
| m/1'/1234/0/3  | my1FMCXyo84tC1LkFXX8LctbtpycUmUnPx | 7d9abe7323077358acfb80ef2ec0374a37c07bc097609c48da486d6a9266581e |
| m/1'/1234/0/3  | my1FMCXyo84tC1LkFXX8LctbtpycUmUnPx | f2e6bc46d8bf08d19854bc8dfde1a3d4968fa8d09f2185ea6e2711ec0ce449ec |
| m/1'/1234/0/4  | n1GohMiYdx8Q8PSBynH34vdgZXH1tid7cW | 6b344d0823c3f3ff1e084232674320e1409a12d2af50a6320063cd8cf5797e9a |
| m/1'/1234/0/4  | n1GohMiYdx8Q8PSBynH34vdgZXH1tid7cW | 75b465e8785b87fe11b625f5fd030e4f314c028c25b3ea84ae96bb1a3d369796 |
| m/1'/1234/0/6  | n1EstV7h4Jyx1XKLY7fdx6ufFRMxBVdieN | f49001a572942a506bd414c51f5a0e9cd349899d05c868b1de2fa6742a66d5aa |
| m/1'/1234/0/7  | mi2udMvJHeeJJNp5wWKToa86L2cJUKzrby | 5554c15d13002786a70a7151aad4eddce76633c60bc7f90e3dc70eb4f9c4b2b0 |
| m/1'/1234/0/7  | mi2udMvJHeeJJNp5wWKToa86L2cJUKzrby | bd09a74381ffad78c162976ec27fc9c1dceda3c2bfe367541a7140b8dd6e1f4c |
| m/1'/1234/0/14 | mmfFY4UJHJBSjz3ve7tvaSrxNwCReVtifC | c499c223f401175b14c8cb28893219dcd8b49563e59b9b4865dd4407c2de27c8 |
| m/1'/1234/0/14 | mmfFY4UJHJBSjz3ve7tvaSrxNwCReVtifC | df47e4bf599ecf27d8f3b34e85fcd7384d753823208d06171ff16899bc1709ce |
| m/1'/1234/0/15 | myEyz9WPaZmXqnhtN86xM8S66wVysBRNtN | acffdca982d1ce6fcd1f51dbb9dda686441058f93686008a2c446c42a94a66a7 |
| m/1'/1234/0/15 | myEyz9WPaZmXqnhtN86xM8S66wVysBRNtN | df47e4bf599ecf27d8f3b34e85fcd7384d753823208d06171ff16899bc1709ce |
| m/1'/1234/0/19 | mskQduVL7QBZCEWJUit8u9N8Cj4CFouPWX | 77bddf5ea1feef8eaf52375fdda8e08a5df6a581d0f34f5b3a849448f54c8d83 |
| m/1'/1234/1/0  | moHN13u4RoMxujdaPxvuaTaawgWZ3LaGyo | f2e6bc46d8bf08d19854bc8dfde1a3d4968fa8d09f2185ea6e2711ec0ce449ec |
| m/1'/1234/1/1  | n2aNi43rgX8YD5NMJK55dgHp7n7rdGzbYj | bd09a74381ffad78c162976ec27fc9c1dceda3c2bfe367541a7140b8dd6e1f4c |
| m/1'/1234/1/2  | mv5dfNyTKMwED5g26LMZJYwN5QckEXrqe2 | df47e4bf599ecf27d8f3b34e85fcd7384d753823208d06171ff16899bc1709ce |
+----------------+------------------------------------+------------------------------------------------------------------+

+----------------+------------------------------------+-----------+
|      PATH      |              ADDRESS               |  BALANCE  |
+----------------+------------------------------------+-----------+
| m/1'/1234/0/0  | mzoeuyGqMudyvKbkNx5dtNBNN59oKEAsPn | 100000000 |
| m/1'/1234/0/1  | mz37noAanMGMW1BMGvyhNnY1fqfTg726Ka |  15674439 |
| m/1'/1234/0/2  | mzPCbXLqiHLNhVj6reah8VWXVT8X69Ssuf |  30000000 |
| m/1'/1234/0/3  | my1FMCXyo84tC1LkFXX8LctbtpycUmUnPx |         0 |
| m/1'/1234/0/4  | n1GohMiYdx8Q8PSBynH34vdgZXH1tid7cW |         0 |
| m/1'/1234/0/6  | n1EstV7h4Jyx1XKLY7fdx6ufFRMxBVdieN | 100000000 |
| m/1'/1234/0/7  | mi2udMvJHeeJJNp5wWKToa86L2cJUKzrby |   3000000 |
| m/1'/1234/0/14 | mmfFY4UJHJBSjz3ve7tvaSrxNwCReVtifC |         0 |
| m/1'/1234/0/15 | myEyz9WPaZmXqnhtN86xM8S66wVysBRNtN |         0 |
| m/1'/1234/0/19 | mskQduVL7QBZCEWJUit8u9N8Cj4CFouPWX |   1000000 |
| m/1'/1234/1/0  | moHN13u4RoMxujdaPxvuaTaawgWZ3LaGyo |    246600 |
| m/1'/1234/1/1  | n2aNi43rgX8YD5NMJK55dgHp7n7rdGzbYj |   6504400 |
| m/1'/1234/1/2  | mv5dfNyTKMwED5g26LMZJYwN5QckEXrqe2 |    300000 |
+----------------+------------------------------------+-----------+

+---------------+--------------------+-------------------+---------------------+
| TOTAL BALANCE | LAST RECEIVE INDEX | LAST CHANGE INDEX |     REPORT TIME     |
+---------------+--------------------+-------------------+---------------------+
|     256725439 |                 28 |                11 | 26 Sep 18 13:38 PDT |
+---------------+--------------------+-------------------+---------------------+
```
