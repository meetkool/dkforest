PKGS       = $(shell go list ./... | grep -v /vendor/ | grep -v /bindata)
VERSION    = $(shell git describe)
VERSION64  = $(shell git describe | base64)
SHA        = $(shell git rev-parse HEAD)
PWD        = $(shell pwd)

SELF_HOST_FLAGS = -ldflags "-s -w -X main.version=$(VERSION64) -X main.versionVoid=${VERSION} -X main.sha=$(SHA) -X main.development=0"

WINDOWS_ENVS   = GOOS="windows" GOARCH="amd64" CGO_ENABLED="1" CC="/usr/local/opt/mingw-w64/bin/x86_64-w64-mingw32-gcc"
WINDOWS_FILENAME   = snb.$(VERSION).windows.amd64.exe

LINUX_ENVS   = GOOS="linux"   GOARCH="amd64" CGO_ENABLED="1"
LINUX_FILENAME   = dkf.$(VERSION).linux.amd64

build-docker-bin: bindata-prod
	docker run --rm -it -v $(PWD):/root/dkf -w /root/dkf $(DOCKER_IMG) sh -c \
		'$(ENVS) go build --tags "fts5" -gcflags=all=-trimpath=$(GOPATH) $(SELF_HOST_FLAGS) -o dist/$(FILENAME) cmd/dkf/main.go'

build-linux: ENVS=$(LINUX_ENVS)
build-linux: DOCKER_IMG=golang:1.18-stretch
build-linux: FILENAME=$(LINUX_FILENAME)
build-linux: build-docker-bin tar-file clean-file build-checksum

tar-file:
	tar -czvf dist/$(FILENAME).tar.gz dist/$(FILENAME)

zip-file:
	zip dist/$(FILENAME).zip dist/$(FILENAME)

build-checksum:
	openssl dgst -sha256 dist/$(FILENAME).tar.gz | cut -d ' ' -f 2 > dist/$(FILENAME).tar.gz.checksum

build-checksum-zip:
	openssl dgst -sha256 dist/$(FILENAME).zip | cut -d ' ' -f 2 > dist/$(FILENAME).zip.checksum

clean-file:
	rm dist/$(FILENAME)

clean-dist:
	rm -fr ./dist

test:
	@go test $(PKGS)

bindata:
	go-bindata $(DEBUG) -pkg bindata -o bindata/bindata.go -prefix "pkg/web/public/" pkg/web/public/...

bindata-dev: DEBUG=-debug
bindata-dev: bindata

bindata-prod: DEBUG=
bindata-prod: bindata

scp-master: FILENAME=$(LINUX_FILENAME)
scp-master:
	scp -pr dist/$(FILENAME).tar.gz dkf:/root

extract-master: FILENAME=$(LINUX_FILENAME)
extract-master:
	ssh dkf 'cd /root && tar -xvz -f$(FILENAME).tar.gz && mv ./dist/$(FILENAME) ./dist/darkforest && rm $(FILENAME).tar.gz'

restart-master:
	ssh dkf 'service darkforest restart'

deploy-master: clean-dist build-linux scp-master extract-master restart-master

serve:
	air

count:
	@find \
		./pkg \
		./scripts \
		-name '*.go' \
		| xargs wc -l \
		| sort

size:
	@find \
		./pkg \
        ./scripts \
        -name '*.go' \
        | xargs du -sch

.PHONY: serve bindata bindata-dev
