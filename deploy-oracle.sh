#! /bin/bash

set -euo pipefail

## environments
MEDIAN_DIR=${1:-$HOME/median}
FROM=${2:-JPY}
TO=${3:-JPX}
TOKENPAIR=$TO$FROM
OSM_DIR=${4:-$HOME/osm}

EXTRACT_ABI=true

RPC=localhost
PORT=8545
BASEDIR=$HOME/.dapp

export ETH_RPC_URL=$RPC:$PORT
export ETH_FROM=$(cat $BASEDIR/testnet/$PORT/config/account)
export ETH_KEYSTORE=$BASEDIR/testnet/$PORT/keystore
export ETH_PASSWORD=/dev/null
export ETH_GAS=7000000

EXPORT_DIR=$(cd $(dirname ${BASH_SOURCE:-$0}) && pwd)

## deploy oracle
### build medianizer
cd $MEDIAN_DIR
dapp update
dapp --use solc:0.5.12 build
[[ ! -f out/dapp.sol.json ]] && exit 1

### build osm
cd $OSM_DIR
dapp update
dapp --use solc:0.5.12 build
[[ ! -f out/dapp.sol.json ]] && exit 1

### deploy medianizer
cd $MEDIAN_DIR &&
MEDIAN=$(dapp create Median$TOKENPAIR | tail -n 1) &&
cd $OSM_DIR &&
OSM=$(dapp create OSM $MEDIAN | tail -n 1)

[[ $? != 0 ]] && exit 1

### feed settings
seth send $MEDIAN 'lift(address[])' "[$ETH_FROM]" &&
seth send $MEDIAN "setBar(uint256)" $(seth --to-uint256 1) &&
### give osm median access
seth send $MEDIAN "kiss(address)" "$OSM"
### for debug
seth send $OSM "kiss(address)" "$ETH_FROM"
seth send $MEDIAN "kiss(address)" "$ETH_FROM"

if $EXTRACT_ABI; then
    cd $MEDIAN_DIR && dapp --use solc:0.5.12 build --extract && [[ ! -f out/Median$TOKENPAIR.abi ]] && echo "[INFO] failed to extract Median abi"
    cd $OSM_DIR && dapp --use solc:0.5.12 build --extract && [[ ! -f out/OSM.abi ]] && echo "[INFO] failed to extract OSM abi"
fi

echo ""
echo "MEDIAN=$MEDIAN"
echo "OSM=$OSM"

echo $EXPORT_DIR/addresses.json
cat << EOF > $EXPORT_DIR/addresses.json
{
    "PIP_$TO": "$OSM"
}
EOF
