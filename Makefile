.PHONY: build
build:
	@mkdir -p bin
	@go build -i -v -o ./bin/godl

.PHONY: test
test:
	@go test --race -v -i
	@go test --race -v
	@golint .
	@go vet .

.PHONY: verify-vendor
verify-vendor: build
	@./bin/godl verify
