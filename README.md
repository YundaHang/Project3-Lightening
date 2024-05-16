## Introduction
Now that we're familiar with a blockchain's Layer 1 from Coin, it's time that we move on to a Layer 2 scaling solution! In class, you've learned about off-chain payment channels. Lightning is a scaling protocol for Bitcoin, and we've done our best to imitate that here.

## Components
1. **Node**
    - We already learned about nodes in Coin, but we'll be making a few updates with this assignment. First, we'll be implementing the **[SegWit](https://en.wikipedia.org/wiki/SegWit)** protocol, which allows us to fit more transactions in each block. Second,  we'll be accomodating our new Lightning node!
2. **LightningNode**
    -  Like our normal nodes, lightning nodes run their own server, deal with transactions, and have a collection of other (lightning) peers. Unlike our normal nodes, lightning nodes keep track of channels, which  keep track of transactions without having to broadcast everything to the main network.
4. **Channel**
    - Once a channel is opened between two parties and backed by a transaction on the main network, channel members can send updated transactions back and forth without having to broadcast all minor updates to the main chain. Think of all the pesky fees and time saved! When one party decides to "settle," they simply broadcast the most recent transaction to the main network.
6. **Watchtower**
    - Since every transaction in a channel is technically valid, we have to employ the services of a watchtower to ensure that our counterparty doesn't cheat and broadcast an outdated transaction! The watchtower will keep a collection of all revocation keys for past transactions, ensuring that if the counterparty does decide to cheat, they'll be caught (and lose all of their coins in the process)!
8. **Server**
    - You'll have to deal with two servers for this assignment! First, you'll implement the **SegWit** protocol on the node's server. Then you'll implement the lightning protocol on the lightning node's server. 