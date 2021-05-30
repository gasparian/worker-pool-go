.SILENT:

.PHONY: test
test:
	go test -v -cover -race -count=1 -timeout=5m ./wp/...
