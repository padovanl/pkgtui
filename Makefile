.PHONY: build run fmt vet clean deb snap

build:
	go build -o pkgtui .

run: build
	./pkgtui

fmt:
	gofmt -l .

vet:
	go vet ./...

clean:
	rm -rf pkgtui dist

# Local .deb/.tar.gz via goreleaser, no publishing.
deb:
	goreleaser release --snapshot --clean --skip=publish

# Local .snap via snapcraft (reads snap/snapcraft.yaml).
snap:
	snapcraft pack
