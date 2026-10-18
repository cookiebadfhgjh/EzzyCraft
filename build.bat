@echo off
rem Builds mcserver.exe (needs Go 1.21+ and internet the first time, for gorilla/websocket)
go mod tidy || exit /b 1
go build -ldflags "-s -w" -o mcserver.exe . || exit /b 1
echo Built mcserver.exe  -  run it, then open http://localhost:8080
