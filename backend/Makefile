.PHONY: generate
generate:
	go generate ./...

# tagalign exits 3 when it rewrote a file, so its status is ignored with `-`:
# -fix already applied the change and a non-zero exit would abort the target.
#
# golines runs with --no-reformat-tags because it aligns struct tags by key name
# while tagalign aligns them by position: with both on they rewrite each other
# forever, and `make fmt` never reaches a fixed point.
.PHONY: fmt
fmt:
	-tagalign -fix -sort ./...
	goimports -l -w .
	golines --no-reformat-tags -l -w .
	gofumpt -l -w .

.PHONY: lint
lint:
	golangci-lint run ./...
