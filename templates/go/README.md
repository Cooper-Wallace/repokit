# {{repo_name}}

{{description}}

---

## Overview
A modern Go project structured with standard `cmd/` and `internal/` package layout, automated testing, and strict quality gates.

## Getting Started

### Prerequisites
- Go 1.23+

### Build & Run
```bash
git clone https://github.com/{{repo_slug}}.git
cd {{repo_name}}

# Build binary
go build -o bin/{{repo_name}} ./cmd/{{repo_name}}

# Run binary
./bin/{{repo_name}}
```

### Running Tests & Static Analysis
```bash
# Run unit tests with race detector and coverage
go test -v -race -cover ./...

# Run static analysis
go vet ./...
```

## Development & Contribution Flow
- **Conventional Commits**: All commit messages must follow [Conventional Commits](https://www.conventionalcommits.org/).
- **Feature Branches**: Branch off `main` via `feat/*` or `fix/*`.
- **Pull Requests**: Changes are merged via squash-and-merge with CI status checks.
