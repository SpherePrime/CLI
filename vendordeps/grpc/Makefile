all: vet test testrace

build:
	go build github.com/SpherePrime/CLI/vendordeps/grpc/...

clean:
	go clean -i github.com/SpherePrime/CLI/vendordeps/grpc/...

deps:
	GO111MODULE=on go get -d -v github.com/SpherePrime/CLI/vendordeps/grpc/...

proto:
	@ if ! which protoc > /dev/null; then \
		echo "error: protoc not installed" >&2; \
		exit 1; \
	fi
	go generate github.com/SpherePrime/CLI/vendordeps/grpc/...

test:
	go test -cpu 1,4 -timeout 7m github.com/SpherePrime/CLI/vendordeps/grpc/...

testsubmodule:
	cd security/advancedtls && go test -cpu 1,4 -timeout 7m github.com/SpherePrime/CLI/vendordeps/grpc/security/advancedtls/...
	cd security/authorization && go test -cpu 1,4 -timeout 7m github.com/SpherePrime/CLI/vendordeps/grpc/security/authorization/...

testrace:
	go test -race -cpu 1,4 -timeout 7m github.com/SpherePrime/CLI/vendordeps/grpc/...

testdeps:
	GO111MODULE=on go get -d -v -t github.com/SpherePrime/CLI/vendordeps/grpc/...

vet: vetdeps
	./scripts/vet.sh

vetdeps:
	./scripts/vet.sh -install

.PHONY: \
	all \
	build \
	clean \
	deps \
	proto \
	test \
	testsubmodule \
	testrace \
	testdeps \
	vet \
	vetdeps
