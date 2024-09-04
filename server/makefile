.PHONY: build main

main: clean build

clean:
	rm -rf build/*

build:
	mkdir -p build/bin
	env GOOS=windows GOARCH=amd64 go build -o build/bin/pinzoomd.exe ./cmd/main
