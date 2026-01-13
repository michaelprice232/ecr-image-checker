.PHONY: coverage-html test run

coverage-html:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html

test:
	go test -v ./...

run:
	go run ecr-image-checker.go
