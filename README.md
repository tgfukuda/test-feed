# TEST Feed For The Protocol

Running Median test feed without P2P networks 

## requirements
- [golang](https://go.dev/doc/install)
- [Median](https://github.com/makerdao/median)
- [OSM](https://github.com/makerdao/osm)

for more details, https://collateral.makerdao.com/oracles-domain/untitled

## installation
```
go build .
```

## running on the local testnet
1. running testnet with https://github.com/dapphub/dapptools
2. setting up shell environments (ETH_FROM, ETH_RPC_URL, ...etc)
3. `bash deploy-oracle.sh`
4. run the go binary
