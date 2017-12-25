#!/bin/sh


Dir=$(cd `dirname $0`; pwd)
cd ${Dir}


cd ../spider/main
go build -o ../../build/bin/spider
cd -

cd ../archer/main
go build -o ../../build/bin/archer
cd -

cd ../stg/main
go build -o ../../build/bin/stg
cd -

cd ../krang/main
go build -o ../../build/bin/krang
cd -

cd ../replay/main
go build -o ../../build/bin/replay
cd -
