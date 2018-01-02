cd %~dp0

cd ../spider/main
go build -o ../../build/bin/spider.exe
cd %~dp0

cd ../archer/main
go build -o ../../build/bin/archer.exe
cd %~dp0

cd ../stg/main
go build -o ../../build/bin/stg.exe
cd %~dp0

cd ../krang/main
go build -o ../../build/bin/krang.exe
cd %~dp0

cd ../replay/main
go build -o ../../build/bin/replay.exe
cd %~dp0

pause
