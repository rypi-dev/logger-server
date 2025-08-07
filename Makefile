PKGS := ./...

.PHONY: all test coverage report clean lint fmt vet ci

all: fmt vet lint test coverage report

fmt:
	go fmt $(PKGS)

vet:
	go vet $(PKGS)

lint:
	golangci-lint run $(PKGS)
	@echo "Linting terminé sans erreur."

test:
	go test -race $(PKGS)
	@echo "Tests OK."

ci: fmt vet lint test

coverage: coverage.out

coverage.out:
	go test $(PKGS) -coverprofile=coverage.out

report: coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Rapport couverture généré : coverage.html"

clean:
	rm -f coverage.out coverage.html