<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/license-Proprietary-red" alt="License">
  <img src="https://img.shields.io/badge/API-S3--Compatible-orange" alt="API">
  <img src="https://img.shields.io/badge/storage-Erasure%20Coding-brightgreen" alt="Erasure Coding">
  <img src="https://img.shields.io/badge/auth-JWT%20%2B%20RBAC-blueviolet" alt="Auth">
  <img src="https://img.shields.io/badge/k8s-ready-blue" alt="Kubernetes">
</p>

<h1 align="center">🗄️ Distributed File Storage Service</h1>
<p align="center"><b>Run your own S3 — one command, zero cloud bill.</b></p>

<p align="center">
  <i>S3-compatible object storage with erasure coding, JWT auth, Prometheus metrics, and Kubernetes support.</i>
  <br>
  <b>Open source. Self-hosted. Production-grade.</b>
</p>

<br>

<p align="center">
  <img src="https://via.placeholder.com/800x400/1a1a2e/e94560?text=📸+Hero+Image+-+Screenshot+Coming+Soon" alt="Hero Screenshot" width="800">
  <br>
  <i>(Add your hero image here — see prompt below)</i>
</p>

---

## 🚀 Why?

> AWS S3 is great. Until the bill arrives.

This project gives you **everything S3 offers** — buckets, objects, multipart uploads, range requests, access control — but **runs on your own hardware**.

| AWS S3 (1 TB/month) | This Project |
|---------------------|--------------|
| ~$23/month + egress | Free (your server) |
| Vendor lock-in | Full control |
| No erasure coding | Reed-Solomon built-in |
| Opaque pricing | 100% transparent |

---

## ✨ Features

| Category | Capabilities |
|----------|-------------|
| **📦 API** | S3-compatible REST — buckets, objects, multipart upload, range requests |
| **🔐 Auth** | JWT login with refresh tokens, RBAC, IAM-style policy engine |
| **💾 Storage** | Erasure coding (Reed-Solomon), file chunking, SHA-256 dedup, replication |
| **📊 Observability** | Prometheus metrics, JSON structured logs, correlation IDs, audit logging |
| **⚡ Performance** | Configurable shards, parallel uploads, graceful shutdown, connection draining |
| **🐳 Deployment** | Docker Compose, Kubernetes manifests, Makefile, `.env.example` |
| **🧪 Testing** | Unit tests, integration tests, coverage reports, load-testing script |

### Deep Dive: Erasure Coding

Your file is split into **4 data shards + 2 parity shards** using Reed-Solomon. Even if 2 disks fail, your data is fully recoverable. No expensive RAID controllers needed.

```
File ──→ [D1][D2][D3][D4][P1][P2]
         └── survive any 2 failures ──→ ✓
```

---

## 🧪 Quick Start

```bash
# 1. Clone
git clone https://github.com/aman179102/distributed-file-storage-service.git
cd distributed-file-storage-service

# 2. Start everything (PostgreSQL + Redis + app)
docker compose up -d

# 3. Verify
curl http://localhost:8080/health
# {"status":"ok","version":"1.0.0","uptime":5}

# 4. Login
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"admin123"}' | jq .
# → Get your JWT token

# 5. Create a bucket
TOKEN="<your-jwt-token>"
curl -s -X POST http://localhost:8080/api/v1/buckets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-first-bucket"}' | jq .

# 6. Upload a file
echo "Hello S3!" > test.txt
curl -s -X PUT "http://localhost:8080/api/v1/buckets/1/objects/test.txt" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @test.txt | jq .

# 7. Download it
curl -s "http://localhost:8080/api/v1/buckets/1/objects/test.txt" \
  -H "Authorization: Bearer $TOKEN"
# → "Hello S3!"

# 8. Check Prometheus metrics
curl http://localhost:8080/metrics | head -20
```

---

## 🔧 One-Click Deploy

### Docker (recommended)
```bash
make docker-up
```

### Kubernetes
```bash
kubectl apply -f deploy/kubernetes/
```

### Local Dev
```bash
make dev
```

---

## 📡 API at a Glance

| Method | Endpoint | What it does |
|--------|----------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/api/v1/auth/login` | Get JWT token |
| `GET` | `/api/v1/buckets` | List all buckets |
| `POST` | `/api/v1/buckets` | Create bucket |
| `PUT` | `/api/v1/buckets/{id}/objects/{key}` | Upload file |
| `GET` | `/api/v1/buckets/{id}/objects/{key}` | Download file |
| `HEAD` | `/api/v1/buckets/{id}/objects/{key}` | File metadata |
| `DELETE` | `/api/v1/buckets/{id}/objects/{key}` | Delete file |
| `POST` | `/api/v1/buckets/{id}/uploads` | Start multipart upload |
| `PUT` | `/api/v1/buckets/{id}/uploads/{uploadId}` | Upload a part |
| `GET` | `/metrics` | Prometheus metrics |

Full API docs → see `docs/api.md`

---

## 🏗️ Architecture

```
┌──────────────────────────────────────────┐
│            HTTP/REST API                 │
│       (S3-Compatible Endpoints)          │
├──────────────────────────────────────────┤
│      Auth & Authorization Layer          │
│       JWT + RBAC + IAM Policies          │
├──────────────────────────────────────────┤
│        Business Logic Layer               │
│  File Service · Bucket Service · Policy  │
├──────────────────────────────────────────┤
│           Storage Layer                   │
│  Chunking · Dedup · Erasure · Replication│
├──────────────────────────────────────────┤
│     Infrastructure                       │
│  PostgreSQL │ Redis │ Local Disk Store   │
├──────────────────────────────────────────┤
│     Observability                        │
│  Metrics │ JSON Logs │ Audit Trails      │
└──────────────────────────────────────────┘
```

---

## ⚙️ Configuration

```env
SERVER_PORT=8080
DB_HOST=localhost
STORAGE_DATA_DIR=/data/files
STORAGE_DATA_SHARDS=4
STORAGE_PARITY_SHARDS=2
AUTH_JWT_SECRET=<change-this-in-production>
```

Copy `.env.example` → `.env` and you're set.

---

## 🧪 Testing

```bash
make test        # All tests
make test-unit   # Unit tests only
make test-cover  # With coverage report
```

---

## 📋 Project Checklist

- [x] Dockerfile + docker-compose.yml
- [x] Kubernetes manifests
- [x] Makefile
- [x] README, CONTRIBUTING, CHANGELOG
- [x] `.gitignore`, `.env.example`
- [x] Load-testing script
- [x] Prometheus + Grafana integration
- [x] JWT auth + RBAC
- [x] Erasure coding (Reed-Solomon)

---

## 🙌 Show Your Support

If this project helps you, please ⭐ **star the repo** — it motivates me to keep building!

[![GitHub stars](https://img.shields.io/github/stars/aman179102/distributed-file-storage-service?style=social)](https://github.com/aman179102/distributed-file-storage-service)

---

## 📄 License

Proprietary — see [LICENSE](./LICENSE). For commercial licensing: amankk179102@gmail.com

---

<p align="center">
  <b>Built with ❤️ in India</b>
  <br>
  <i>Questions? Open an issue or DM me</i>
</p>
