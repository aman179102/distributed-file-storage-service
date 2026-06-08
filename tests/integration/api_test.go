package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/distributed-file-storage/service/src/api"
	"github.com/distributed-file-storage/service/src/api/handlers"
	"github.com/distributed-file-storage/service/src/config"
	"github.com/distributed-file-storage/service/src/core"
	"github.com/distributed-file-storage/service/src/infrastructure/auth"
	"github.com/distributed-file-storage/service/src/infrastructure/database"
	"github.com/distributed-file-storage/service/src/infrastructure/metrics"
	"github.com/distributed-file-storage/service/src/infrastructure/storage"
)

type mockMetrics struct{}

func (m *mockMetrics) RecordStorageOp(operation string, duration time.Duration, err error) {}
func (m *mockMetrics) SetStorageBytes(typeName string, bytes int64)                       {}
func (m *mockMetrics) SetObjectCount(status string, count int64)                           {}
func (m *mockMetrics) SetBucketCount(count int64)                                          {}

func setupTestHandler(t *testing.T) http.Handler {
	t.Helper()
	cfg := &config.Config{}
	cfg.Server.CORSOrigins = []string{"*"}
	cfg.Server.RateLimitPerIP = 100
	cfg.Server.APIBasePath = "/api/v1"

	diskStore, err := storage.NewDiskStore("/tmp/test-int-fs", "/tmp/test-int-tmp")
	if err != nil {
		t.Fatalf("failed to create disk store: %v", err)
	}

	chunker := storage.NewChunker(64 * 1024 * 1024)
	codec := storage.NewReedSolomonCodec(4, 2)
	replicator := storage.NewReplicator(1, []*storage.DiskStore{diskStore})
	mm := metrics.NewMetricsManager()

	userRepo := database.NewUserRepository(nil)
	bucketRepo := database.NewBucketRepository(nil)
	fileRepo := database.NewFileRepository(nil)
	auditRepo := database.NewAuditLogRepository(nil)

	jwtManager := auth.NewJWTManager("test-secret", 15*60, 7*24*60*60)
	rbac := auth.NewRBACEngine()

	fileService := core.NewFileService(cfg, fileRepo, bucketRepo, diskStore, chunker, codec, replicator, mm, auditRepo)
	bucketService := core.NewBucketService(bucketRepo, fileRepo, auditRepo, rbac, mm)

	bucketHandler := handlers.NewBucketHandler(bucketService)
	fileHandler := handlers.NewFileHandler(fileService)
	authHandler := handlers.NewAuthHandler(userRepo, jwtManager, cfg)
	healthHandler := handlers.NewHealthHandler("1.0.0-test")

	return api.NewRouter(cfg, jwtManager, rbac, mm, bucketHandler, fileHandler, authHandler, healthHandler)
}

func TestHealthEndpoint(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", resp["status"])
	}
	if resp["version"] != "1.0.0-test" {
		t.Errorf("expected version '1.0.0-test', got %v", resp["version"])
	}
}

func TestCORSHeaders(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" && origin != "https://example.com" {
		t.Errorf("unexpected CORS origin: %s", origin)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options header")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options header")
	}
}

func TestRateLimitHeaders(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("missing X-RateLimit-Limit header")
	}
	if rec.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("missing X-RateLimit-Remaining header")
	}
}

func TestUnauthorizedAccess(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/buckets", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED, got %v", errObj["code"])
	}
}

func TestMetricsEndpoint(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuthLoginInvalidCredentials(t *testing.T) {
	handler := setupTestHandler(t)

	body := map[string]string{"username": "nonexistent", "password": "wrong"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestDiskStoreIntegration(t *testing.T) {
	store, err := storage.NewDiskStore("/tmp/test-int-disk", "/tmp/test-int-disk-tmp")
	if err != nil {
		t.Fatalf("NewDiskStore failed: %v", err)
	}

	testData := []byte("integration test data for distributed storage")
	checksum, _, err := store.Write("integration/test.txt", bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if checksum == "" {
		t.Fatal("checksum should not be empty")
	}

	var buf bytes.Buffer
	if _, err := store.Read("integration/test.txt", &buf); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if !bytes.Equal(buf.Bytes(), testData) {
		t.Fatal("read data mismatch")
	}

	store.Delete("integration/test.txt")
}

func TestChunkerE2E(t *testing.T) {
	chunker := storage.NewChunker(100)
	testData := make([]byte, 1000)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	result, err := chunker.Split(bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	codec := storage.NewReedSolomonCodec(4, 2)
	encodedChunks, encodedData, err := codec.Encode(result.Data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := codec.Decode(encodedChunks, encodedData)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	merged, err := chunker.Merge([][]byte{decoded})
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if !bytes.Equal(merged, testData) {
		t.Fatal("end-to-end encode/decode/merge mismatch")
	}
}
