# {{repo_name}}

{{description}}

---

## Overview
A Java backend service structured with layered architecture (controllers, services, models).

## Prerequisites
- Java 17+ (or 21+)
- Maven 3.8+

## Getting Started

### Build
```bash
mvn clean compile
```

### Run Tests
```bash
mvn test
```

### Package & Run
```bash
mvn clean package
java -jar target/{{repo_name}}-0.1.0-SNAPSHOT.jar
```

## Architecture
```text
src/main/java/com/cooper/{{package_name}}/
├── Application.java
├── controller/       # Web & REST API endpoints
├── service/          # Business logic
└── model/            # Domain models and DTOs
```

## Development & Contribution Flow
- **Conventional Commits**: All commit messages must follow [Conventional Commits](https://www.conventionalcommits.org/).
- **Feature Branches**: Branch off `main` via `feat/*` or `fix/*`.
- **Pull Requests**: Changes are merged via squash-and-merge with CI status checks.
