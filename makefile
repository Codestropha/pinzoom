.PHONY: dist server web widget

main: build dist

clean:
	rm -rf dist/*

build:
	make clean
	make build-server
	make build-web

dist:
	./dist.sh

build-server:
	(cd server; make)

build-web:
	(cd web; make)
