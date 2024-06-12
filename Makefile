BUILDFLAGS := -a -ldflags="-s -w" -trimpath

ifdef ComSpec
        EXE := .exe
endif

build:
	go build -o bin/mirror$(EXE) mirror/cmd/mirror

format:
	goimports -w cmd internal

release-windows:
	GOOS=windows GOARCH=amd64 go build $(BUILDFLAGS) -o bin/windows_amd64/mirror.exe mirror/cmd/mirror
	upx -9 bin/windows_amd64/mirror.exe

release-linux:
	GOOS=linux GOARCH=amd64 go build $(BUILDFLAGS) -o bin/linux_amd64/mirror mirror/cmd/mirror
	upx -9 bin/linux_amd64/mirror

release-darwin:
	GOOS=darwin GOARCH=amd64 go build $(BUILDFLAGS) -o bin/darwin_amd64/mirror mirror/cmd/mirror
#	upx -9 bin/darwin_amd64/mirror

release-darwin-arm:
	GOOS=darwin GOARCH=arm64 go build $(BUILDFLAGS) -o bin/darwin_arm64/mirror mirror/cmd/mirror
