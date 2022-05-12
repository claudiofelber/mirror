ifdef ComSpec
        EXE := .exe
endif

build:
	go build -o bin/mirror$(EXE) mirror/cmd/mirror

format:
	goimports -w cmd internal

release-windows:
	GOARCH=amd64 GOOS=windows go build -o bin/windows_amd64/mirror.exe -a -ldflags="-w -s" mirror/cmd/mirror
	upx -9 bin/windows_amd64/mirror.exe

release-darwin:
	GOARCH=amd64 GOOS=darwin go build -o bin/darwin_amd64/mirror -a -ldflags="-w -s" mirror/cmd/mirror
#	upx -9 bin/darwin_amd64/mirror
