#!/bin/bash

set -euo pipefail

ADDRESSES=${1:-$HOME/dss-deploy-scripts/out/addresses.json}
TOKEN=${2:-JPX}
INTERVAL=${3:-3600} # seconds

[[ (-z $ADDRESSES) -o (! -f $ADDRESSES) -o (-z $ETH_FROM) ]] && exit 1

OSM=$(cat $ADDRESSES | tr -d '\n' | jq -r .PIP_$TOKEN)
MEDIAN=$(seth call $OSM 'src()(address)')

echo "Start $(seth call $MEDIAN 'wat()(bytes32)' | seth --to-ascii) test feeder"

echo "send authentication"
seth send $MEDIAN 'lift(address[])' "[$ETH_FROM]"

[[ $? -ne 0 ]] && exit 1

while true;
do
    seth send $MEDIAN 'poke(uint256[] val_, uint256[] age_, uint8[] v, bytes32[] r, bytes32[] s)' ;
    sleep $INTERVAL;
done
