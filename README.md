# Distributed File Storage Service

A production-grade, S3-compatible distributed file storage service built in Go with erasure coding, replication, chunking/deduplication, and comprehensive observability.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        HTTP/REST API                         │
│                   (S3-Compatible Endpoints)                  │
├─────────────────────────────────────────────────────────────┤
│                     Auth & Authorization                     │
│               (JWT + RBAC + IAM Policies)                    │
├─────────────────────────────────────────────────────────────┤
│                     Business Logic Layer                      │
│          (File Service, Bucket Service, Policy Engine)       │
├─────────────────────────────────────────────────────────────┤
│                     Storage Layer                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Chunking │  │ Dedup    │  │ Erasure  │  │ Replic.  │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
├─────────────────────────────────────────────────────────────┤
│     Infrastructure Layer                                      │
│  ┌──────────────┐  ┌──────────┐  ┌──────────────────────┐  │
│  │  PostgreSQL  │  │  Redis   │  │  Disk Store (local)  │  │
│  └──────────────┘  └──────────┘  └──────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                  Observability                                │
│     Metrics (Prometheus) | Logging (JSON) | Tracing (OTel)  │
└─────────────────────────────────────────────────────────────┘
```

## Features

- **S3-Compatible REST API**: Full CRUD for buckets and objects including multipart uploads and range requests
- **Erasure Coding**: Reed-Solomon implementation with configurable data/parity shards for data durability
- **File Chunking**: Automatic splitting of large files into configurable chunks
- **Content Deduplication**: SHA-256 based dedup to eliminate redundant storage
- **Replication**: Configurable replication factor across storage nodes
- **Metadata Store**: PostgreSQL-backed with automatic schema migrations
- **Authentication**: JWT with access/refresh token rotation and short expiry
- **Authorization**: RBAC with IAM-style policy evaluation engine
- **Multipart Upload**: Support for large file uploads with parallel parts
- **TTL/Expiration**: Automatic cleanup of expired objects
- **Range Requests**: Partial content retrieval with HTTP Range headers
- **Prometheus Metrics**: Request counts, latency histograms, error rates, storage stats
- **Structured Logging**: JSON-formatted logs with correlation IDs
- **Graceful Shutdown**: Handles SIGTERM/SIGINT with connection draining
- **Audit Logging**: All sensitive operations logged for compliance

## Quick Start

### Prerequisites
- Go 1.22+
- Docker & Docker Compose

### Clone & Run
```bash
git clone <repository-url>
cd distributed-file-storage-service

# Start dependencies and application
make dev

# Or with Docker
make docker-up
```

### Verify
```bash
curl http://localhost:8080/health
# {"status":"ok","version":"1.0.0","uptime":123}
```

## API Overview

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /health | Health check |
| POST | /api/v1/auth/login | User login |
| GET | /api/v1/buckets | List buckets |
| POST | /api/v1/buckets | Create bucket |
| GET | /api/v1/buckets/{id} | Get bucket details |
| PUT | /api/v1/buckets/{id}/objects/{key} | Upload object |
| GET | /api/v1/buckets/{id}/objects/{key} | Download object |
| HEAD | /api/v1/buckets/{id}/objects/{key} | Get object metadata |
| DELETE | /api/v1/buckets/{id}/objects/{key} | Delete object |
| POST | /api/v1/buckets/{id}/uploads | Start multipart upload |
| PUT | /api/v1/buckets/{id}/uploads/{uploadId} | Upload part |
| GET | /metrics | Prometheus metrics |

## Configuration

Configuration is via environment variables. Copy `.env.example` to `.env` and adjust.

Key settings:
- `SERVER_PORT` - HTTP server port (default: 8080)
- `DB_HOST` - PostgreSQL host
- `STORAGE_DATA_DIR` - Data storage directory
- `STORAGE_DATA_SHARDS` / `STORAGE_PARITY_SHARDS` - Erasure coding config
- `AUTH_JWT_SECRET` - JWT signing secret (change in production!)

## Deployment

### Docker
```bash
make docker-build
make docker-up
```

### Kubernetes
```bash
kubectl apply -f deploy/kubernetes/
```

## Testing
```bash
make test          # All tests
make test-unit     # Unit tests
make test-cover    # With coverage report
```

## License
MIT - see LICENSE file
