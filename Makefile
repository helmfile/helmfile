ORG     ?= $(shell basename $(realpath ..))
PKGS    := $(shell go list ./... | grep -v /vendor/)

TAG  ?= $(shell git describe --tags --abbrev=0 HEAD)
LAST = $(shell git describe --tags --abbrev=0 HEAD^)
BODY = "`git log ${LAST}..HEAD --oneline --decorate` `printf '\n\#\#\# [Build Info](${BUILD_URL})'`"
DATE_FMT = +"%Y-%m-%dT%H:%M:%S%z"
ifdef SOURCE_DATE_EPOCH
    BUILD_DATE ?= $(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
else
    BUILD_DATE ?= $(shell date "$(DATE_FMT)")
endif


# The ldflags for the Go build process to set the version related data
GO_BUILD_VERSION_LDFLAGS=\
  -X go.szostok.io/version.version=$(TAG) \
  -X go.szostok.io/version.buildDate=$(BUILD_DATE) \
  -X go.szostok.io/version.commit=$(shell git rev-parse --short HEAD) \
  -X go.szostok.io/version.commitDate=$(shell git log -1 --date=format:"%Y-%m-%dT%H:%M:%S%z" --format=%cd) \
  -X go.szostok.io/version.dirtyBuild=false


build:
	go build -ldflags="$(GO_BUILD_VERSION_LDFLAGS)" ${TARGETS}
.PHONY: build

generate:
	go generate ${PKGS}
.PHONY: generate

fmt:
	go fmt ${PKGS}
	gci write --skip-generated -s standard -s default -s 'prefix(github.com/helmfile/helmfile)' .
.PHONY: fmt

check:
	go vet ${PKGS}
.PHONY: check

build-test-tools:
	go build test/diff-yamls/diff-yamls.go
	curl --progress-bar --location https://github.com/homeport/dyff/releases/download/v1.5.6/dyff_1.5.6_linux_amd64.tar.gz  | tar -xzf - -C `pwd` dyff
.PHONY: build-test-tools

test:
	@which helm &> /dev/null || (echo "helm binary not found. Please see: https://helm.sh/docs/intro/install/" && exit 1)
	go build -o helmfile .
	go test -v ${PKGS} -coverprofile cover.out -race -p=1
	go tool cover -func cover.out
.PHONY: test

integration:
	bash test/integration/run.sh
.PHONY: integration

integration/vagrant:
	$(MAKE) build GOOS=linux GOARCH=amd64
	$(MAKE) build-test-tools GOOS=linux GOARCH=amd64
	vagrant up
	vagrant ssh -c 'HELMFILE_HELM3=1 make -C /vagrant integration'
.PHONY: integration/vagrant

cross:
	env CGO_ENABLED=0 gox -parallel 4 -os 'windows darwin linux' -arch '386 amd64 arm64' -osarch '!darwin/386' -output "dist/{{.Dir}}_{{.OS}}_{{.Arch}}" -ldflags="$(GO_BUILD_VERSION_LDFLAGS)" ${TARGETS}
.PHONY: cross

static-linux:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOFLAGS=-mod=readonly go build -o "dist/helmfile_linux_amd64" -ldflags="$(GO_BUILD_VERSION_LDFLAGS)" ${TARGETS}
.PHONY: static-linux

static-linux-amd64:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOFLAGS=-mod=readonly go build -o "dist/helmfile_linux_amd64" -ldflags="$(GO_BUILD_VERSION_LDFLAGS)" ${TARGETS}
.PHONY: static-linux-amd64

static-linux-arm64:
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GOFLAGS=-mod=readonly go build -o "dist/helmfile_linux_arm64" -ldflags="$(GO_BUILD_VERSION_LDFLAGS)" ${TARGETS}
.PHONY: static-linux-arm64

install:
	env CGO_ENABLED=0 go install -ldflags="$(GO_BUILD_VERSION_LDFLAGS)" ${TARGETS}
.PHONY: install

clean:
	rm dist/helmfile_*
.PHONY: clean

pristine: generate fmt
	git diff | cat
	git ls-files --exclude-standard --modified --deleted --others -x vendor  | grep -v '^go.' | diff /dev/null -
.PHONY: pristine

release: pristine cross
	@ghr -b ${BODY} -t ${GITHUB_TOKEN} -u ${ORG} ${TAG} dist
.PHONY: release

image:
	docker build -t quay.io/${ORG}/helmfile:${TAG} .

run: image
	docker run --rm -it -t quay.io/${ORG}/helmfile:${TAG} sh

push: image
	docker push quay.io/${ORG}/helmfile:${TAG}

image/debian:
	docker build -f Dockerfile.debian -t quay.io/${ORG}/helmfile:${TAG}-stable-slim .

push/debian: image/debian
	docker push quay.io/${ORG}/helmfile:${TAG}-stable-slim

tools:
	go get -u github.com/tcnksm/ghr github.com/mitchellh/gox
.PHONY: tools

release/minor:
	git checkout master
	git pull --rebase origin master
	bash -c 'if git branch | grep autorelease; then git branch -D autorelease; else echo no branch to be cleaned; fi'
	git checkout -b autorelease origin/master
	bash -c 'SEMTAG_REMOTE=origin hack/semtag final -s minor'
	git checkout master

release/patch:
	git checkout master
	git pull --rebase origin master
	bash -c 'if git branch | grep autorelease; then git branch -D autorelease; else echo no branch to be cleaned; fi'
	git checkout -b autorelease origin/master
	bash -c 'SEMTAG_REMOTE=origin hack/semtag final -s patch'
	git checkout master
