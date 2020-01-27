# Memberlist

A basic http key/value example of how to use [hashicorp/memberlist](https://github.com/hashicorp/memberlist)

## Install

```shell
go get github.com/bilalislam/redis-failover
```

## Usage

```shell
redis-failover
-members="": comma seperated list of members
```

### Create Cluster

Start first node
```shell
redis-failover
```

Make a note of the local member address
```
Local member 192.168.1.64:60496
Listening on : 4001
Redis master : 127.0.0.1:6379
Redis slave  : 127.0.0.1:6380
```

Start second node with first node as part of the member list
```shell
redis-failover --members=192.168.1.64:60496
```

You should see the output
```
2015/10/17 22:13:49 [DEBUG] memberlist: Initiating push/pull sync with: 192.168.1.64:60496
Local member 192.168.1.64:60499
Listening on :4002
Redis master : 127.0.0.1:6379
Redis slave  : 127.0.0.1:6380
```

First node output will log the new connection
```shell
2015/10/17 22:13:49 [DEBUG] memberlist: TCP connection from: 192.168.1.64:60500
2015/10/17 22:13:52 [DEBUG] memberlist: Initiating push/pull sync with: 192.168.1.64:60499
```

##TO DO
1. choose redis slave and promote when master has down
2. lock to nodes that access the same resource