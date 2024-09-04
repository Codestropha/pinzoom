.PHONY: build

main: clean install build

clean:
	rm -rf build/*

install:
	echo n | npm ci

run:
	npm start

build:
	@ echo "Building web..."
	mkdir -p build
	env NODE_OPTIONS=--max_old_space_size=4096 npm run build-prod
