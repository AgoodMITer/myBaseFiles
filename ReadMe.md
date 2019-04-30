## Janus

Janus is a kind of proxy with sentinel function. It only supports two replicas now. The two endpoints will sync with each other and select a master which will be in charge of election. It can proxy a stateful sets which only one endpoint could be on online at the same time.

It could be worked as a proxy for a DB cluster, while there should be a agent which in charge of controlling status and to_master/to_slave action in side of DB instance.

It is implemented by a varietal raft protocol, but give up the most peers agree feature when doing election.

## Features

- **Integrate sentinel and proxy**: this will help to reduce the services when deploy, and also reduce the network heavy.
- **Consistency guaranteed**:  it keeps the sentinel and endpoints eventual consistency by kinds of mechanisms.

## Design

## Usage



