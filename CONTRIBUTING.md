# Contributing to Distributed File Storage Service

## Development Setup

### Prerequisites
- Go 1.22 or later
- Docker and Docker Compose (for local dependencies)
- PostgreSQL 16 (or use Docker Compose)
- Redis 7 (or use Docker Compose)

### Quick Start

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd distributed-file-storage-service
   ```

2. Start dependencies:
   ```bash
   make dev
   ```

3. Run tests:
   ```bash
   make test
   ```

### Project Structure
```
src/
  api/           - HTTP handlers, middleware, routing
  config/        - Configuration loading
  core/          - Business logic / use cases
  domain/        - Domain models and entities
  infrastructure/ - Database, storage, auth implementations
tests/
  unit/          - Unit tests
  integration/   - Integration tests
deploy/          - Docker, Kubernetes, Compose files
```

### Coding Standards
- Follow standard Go formatting (`gofmt`)
- Run `make lint` before committing
- Write tests for all new functionality
- Maintain minimum 80% test coverage
- Document all public APIs and types
- Use structured logging via `log/slog`
- Handle errors properly; never use `_` for error returns

### Pull Request Process
1. Create a feature branch from `main`
2. Write tests for your changes
3. Ensure all tests pass: `make test`
4. Update documentation if needed
5. Submit a pull request with a clear description

### Commit Messages
Follow conventional commits:
- `feat:` new feature
- `fix:` bug fix
- `test:` test changes
- `docs:` documentation
- `refactor:` code refactoring
- `chore:` maintenance tasks
