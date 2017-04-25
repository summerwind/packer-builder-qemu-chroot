VERSION=1.0.0
COMMIT=$(shell git rev-parse --verify HEAD)

PACKAGES=$(shell go list ./... | grep -v /vendor/)
BUILD_FLAGS=-ldflags "-X main.VERSION=$(VERSION) -X main.COMMIT=$(COMMIT)"

.PHONY: all
all: build

.PHONY: build
build: vendor
	go build $(BUILD_FLAGS) .

.PHONY: test
test:
	go test -v $(PACKAGES)
	go vet $(PACKAGES)

.PHONY: clean
clean:
	rm -rf packer-builder-qemu-chroot
	rm -rf dist

dist:
	mkdir -p dist
	
	GOARCH=amd64 GOOS=linux go build $(BUILD_FLAGS) .
	tar -czf release/packer-builder-qemu-chroot_linux_amd64.tar.gz packer-builder-qemu-chroot
	rm -rf packer-builder-qemu-chroot

vendor:
	glide install -v
