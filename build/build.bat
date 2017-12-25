cd %~dp0

cd ../spider/main
go build -o ../../build/bin/spider
cd %~dp0

cd ../archer/main
go build -o ../../build/bin/archer
cd %~dp0

cd ../stg/main
go build -o ../../build/bin/stg
cd %~dp0

cd ../krang/main
go build -o ../../build/bin/krang
cd %~dp0

cd ../replay/main
go build -o ../../build/bin/replay
cd %~dp0

pause