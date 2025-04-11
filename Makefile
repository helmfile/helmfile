ORG     ?= $(shell basename $(realpath ..))
PKGS    := $(shell go list ./... | grep -v /vendor/)

INSTALL_DEPS ?= 0

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

# Define a function to check if a command exists
# Usage: $(call check_command,command_name,installation_instructions)
# Returns 0 if command exists, 1 if not
define check_command
	command -v $(1) >/dev/null
endef

# Install all required Go tools
install-go-deps:
	@echo "Installing Go dependencies..."
	@go install github.com/daixiang0/gci@latest
	@go install github.com/tcnksm/ghr@latest
	@go install github.com/mitchellh/gox@latest
	@echo "Go dependencies installed successfully"
.PHONY: install-go-deps

# Check for all required dependencies
check-deps:
	@echo "Checking dependencies..."
	@missing_deps="" ; \
	missing_go_tools="" ; \
	$(call check_command,go,https://golang.org/doc/install) || missing_deps="$$missing_deps\n- go: install from https://golang.org/doc/install" ; \
	$(call check_command,gci,) || missing_go_tools="$$missing_go_tools gci" ; \
	$(call check_command,ghr,) || missing_go_tools="$$missing_go_tools ghr" ; \
	$(call check_command,gox,) || missing_go_tools="$$missing_go_tools gox" ; \
	$(call check_command,curl,https://curl.se/docs/install.html) || missing_deps="$$missing_deps\n- curl: install using your system package manager" ; \
	$(call check_command,docker,https://docs.docker.com/get-docker/) || missing_deps="$$missing_deps\n- docker: install from https://docs.docker.com/get-docker/" ; \
	$(call check_command,git,https://git-scm.com/downloads) || missing_deps="$$missing_deps\n- git: install from https://git-scm.com/downloads" ; \
	$(call check_command,bash,) || missing_deps="$$missing_deps\n- bash: install using your system package manager" ; \
    if [ -n "$$missing_go_tools" ]; then \
        if [ -n "$$missing_deps" ]; then \
            printf "\nCannot install Go tools because Go itself is missing.\n" ; \
        elif [ -n "$$INSTALL_DEPS" ]; then \
            echo "Installing missing Go tools:$$missing_go_tools" ; \
            for tool in $$missing_go_tools; do \
                case $$tool in \
                    gci) go install github.com/daixiang0/gci@latest ;; \
                    ghr) go install github.com/tcnksm/ghr@latest ;; \
                    gox) go install github.com/mitchellh/gox@latest ;; \
                esac ; \
            done ; \
            echo "Go tools installed successfully" ; \
        else \
            printf "\nMissing Go tools:$$missing_go_tools\n" ; \
            echo "Run 'make install-go-deps' to install them, or run 'make check-deps INSTALL_DEPS=1' to check and install." ; \
            missing_deps="$$missing_deps\n- Go tools: run 'make install-go-deps'" ; \
        fi ; \
    fi ; \
    if [ -n "$$missing_deps" ] && [ -z "$$INSTALL_DEPS" ]; then \
        printf "\nMissing dependencies:$$missing_deps\n\n" ; \
        echo "Please install the missing dependencies and try again." ; \
        exit 1 ; \
    else \
        echo "All dependencies are available" ; \
    fi
.PHONY: check-deps

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

test-build:
	@which helm &> /dev/null || (echo "helm binary not found. Please see: https://helm.sh/docs/intro/install/" && exit 1)
	go build -o helmfile .
.PHONY: test-build

test: test-build
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
