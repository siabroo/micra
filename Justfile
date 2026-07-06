# Run tests across every micra module.
test:
    cd core && go test -short ./...
    cd adapters/loggerslog && go test -short ./...
    cd adapters/otelinit && go test -short ./...
    cd adapters/otelpgx && go test -short ./...
    cd components/pgxpool && go test -short ./...
    cd components/httpserver && go test -short ./...
    cd components/grpcserver && go test -short ./...
    cd components/grpcclient && go test -short ./...

# Run vet across every micra module.
vet:
    cd core && go vet ./...
    cd adapters/loggerslog && go vet ./...
    cd adapters/otelinit && go vet ./...
    cd adapters/otelpgx && go vet ./...
    cd components/pgxpool && go vet ./...
    cd components/httpserver && go vet ./...
    cd components/grpcserver && go vet ./...
    cd components/grpcclient && go vet ./...

# Tidy every module's go.mod.
tidy:
    cd core && go mod tidy
    cd adapters/loggerslog && go mod tidy
    cd adapters/otelinit && go mod tidy
    cd adapters/otelpgx && go mod tidy
    cd components/pgxpool && go mod tidy
    cd components/httpserver && go mod tidy
    cd components/grpcserver && go mod tidy
    cd components/grpcclient && go mod tidy

# Run golangci-lint across every micra module.
lint:
    cd core && golangci-lint run
    cd adapters/loggerslog && golangci-lint run
    cd adapters/otelinit && golangci-lint run
    cd adapters/otelpgx && golangci-lint run
    cd components/pgxpool && golangci-lint run
    cd components/httpserver && golangci-lint run
    cd components/grpcserver && golangci-lint run
    cd components/grpcclient && golangci-lint run

# Run Docker-backed integration tests (requires a running Docker daemon).
test-integration:
    cd adapters/otelpgx && go test -tags=integration ./...
    cd components/pgxpool && go test -tags=integration ./...

# Tag and push all module versions in dependency order. Usage: just release VERSION=v0.1.0
# Assumes go.mod files already require core@VERSION with no replace directives.
release VERSION:
    git tag core/{{VERSION}}
    git push origin core/{{VERSION}}
    git tag adapters/loggerslog/{{VERSION}}
    git tag adapters/otelinit/{{VERSION}}
    git tag adapters/otelpgx/{{VERSION}}
    git tag components/grpcclient/{{VERSION}}
    git tag components/grpcserver/{{VERSION}}
    git tag components/httpserver/{{VERSION}}
    git tag components/pgxpool/{{VERSION}}
    git push origin \
      adapters/loggerslog/{{VERSION}} adapters/otelinit/{{VERSION}} adapters/otelpgx/{{VERSION}} \
      components/grpcclient/{{VERSION}} components/grpcserver/{{VERSION}} components/httpserver/{{VERSION}} \
      components/pgxpool/{{VERSION}}
