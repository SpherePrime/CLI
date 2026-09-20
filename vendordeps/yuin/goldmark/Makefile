.PHONY: test fuzz lint gen

lint:
	golangci-lint run -c .golangci.yml ./...

test:
	go test -coverprofile=profile.out -coverpkg=github.com/dwertyfa288/CLI/vendordeps/yuin/goldmark,github.com/dwertyfa288/CLI/vendordeps/yuin/goldmark/ast,github.com/dwertyfa288/CLI/vendordeps/yuin/goldmark/extension,github.com/dwertyfa288/CLI/vendordeps/yuin/goldmark/extension/ast,github.com/dwertyfa288/CLI/vendordeps/yuin/goldmark/parser,github.com/dwertyfa288/CLI/vendordeps/yuin/goldmark/renderer,github.com/dwertyfa288/CLI/vendordeps/yuin/goldmark/renderer/html,github.com/dwertyfa288/CLI/vendordeps/yuin/goldmark/text,github.com/dwertyfa288/CLI/vendordeps/yuin/goldmark/util ./...

cov: test
	go tool cover -html=profile.out

fuzz:
	cd ./fuzz && go test -fuzz=FuzzDefault

gen:
	go generate ./...
