build:
	go build -o bin/mirror mirror/cmd/mirror

format:
	goimports -w cmd internal

release:
	GOARCH=amd64 GOOS=windows go build -o bin/windows_amd64/mirror.exe -a -ldflags="-w -s" mirror/cmd/mirror
	upx -9 bin/windows_amd64/mirror.exe
