format: FORCE
	goimports -w src\mirror src\console

bin\mirror.exe: $(DIR)src/mirror/*.go $(DIR)src/console/*.go
	go install mirror

compress: bin\mirror.exe
	upx -t bin\mirror.exe || upx -q bin\mirror.exe

FORCE: