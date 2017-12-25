#!/bin/sh

Dir=$(cd `dirname $0`; pwd)
cd ${Dir}

nohup ./bin/stg &
nohup ./bin/krang &
nohup ./bin/archer &
nohup ./bin/spider &

