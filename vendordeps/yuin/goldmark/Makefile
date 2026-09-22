.PHONY: test fuzz lint gen

lint:
	golangci-lint run -c .golangci.yml ./...

test:
	go test -coverprofile=profile.out -coverpkg=github.com/SpherePrime/CLI/vendordeps/yuin/goldmark,github.com/SpherePrime/CLI/vendordeps/yuin/goldmark/ast,github.com/SpherePrime/CLI/vendordeps/yuin/goldmark/extension,github.com/SpherePrime/CLI/vendordeps/yuin/goldmark/extension/ast,github.com/SpherePrime/CLI/vendordeps/yuin/goldmark/parser,github.com/SpherePrime/CLI/vendordeps/yuin/goldmark/renderer,github.com/SpherePrime/CLI/vendordeps/yuin/goldmark/renderer/html,github.com/SpherePrime/CLI/vendordeps/yuin/goldmark/text,github.com/SpherePrime/CLI/vendordeps/yuin/goldmark/util ./...

cov: test
	go tool cover -html=profile.out

fuzz:
	cd ./fuzz && go test -fuzz=FuzzDefault

gen:
	go generate ./...
