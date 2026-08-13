.PHONY: build run fmt vet clean deb snap release

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

# Tag and push a release: make release VERSION=v0.1.0
release:
	@test -n "$(VERSION)" || (echo "usage: make release VERSION=vX.Y.Z" >&2; exit 1)
	scripts/release.sh $(VERSION)
