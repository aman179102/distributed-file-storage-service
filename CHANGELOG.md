# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-06-08

### Added
- Initial release of the Distributed File Storage Service
- S3-compatible REST API with full CRUD operations for buckets and objects
- Erasure coding with Reed-Solomon implementation (configurable data/parity shards)
- File chunking with configurable chunk size
- Content-based deduplication
- Data replication across multiple storage nodes
- PostgreSQL metadata store with automatic schema migrations
- JWT-based authentication with access/refresh token rotation
- RBAC authorization with IAM-style policy evaluation
- Multipart upload support for large files
- Range request support (partial content retrieval)
- TTL/expiration for automatic object cleanup
- Prometheus metrics with custom instrumentation
- Structured JSON logging with correlation IDs
- Graceful shutdown handling (SIGTERM/SIGINT)
- Comprehensive audit logging for all sensitive operations
- CORS configuration with security headers
- Rate limiting per IP
- Health check endpoint
- Docker multi-stage build with distroless runtime
- Docker Compose for local development
- Kubernetes deployment manifests (Deployment, Service, Ingress, HPA, PVC)
- Horizontal Pod Autoscaler configuration
- Comprehensive test suite (unit + integration)
- Makefile with common development commands
