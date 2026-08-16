.PHONY: generate
generate:
	go generate ./...

.PHONY: fmt
fmt:
	tagalign -fix -sort ./...
	goimports -l -w .
	golines -l -w .
	gofumpt -l -w .

.PHONY: lint
lint:
	golangci-lint run ./...
