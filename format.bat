for %%f in (%~dp0src\mirror\*.go) do gofmt -w=true %%f
for %%f in (%~dp0src\console\*.go) do gofmt -w=true %%f