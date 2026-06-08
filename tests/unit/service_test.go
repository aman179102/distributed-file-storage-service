package unit

import (
	"bytes"
	"testing"

	"github.com/distributed-file-storage/service/src/infrastructure/auth"
	"github.com/distributed-file-storage/service/src/infrastructure/storage"
)

func TestRBACEngine_Authorize(t *testing.T) {
	rbac := auth.NewRBACEngine()

	admin := &domain.UserStub{Role: "admin"}
	reader := &domain.UserStub{Role: "readonly"}

	result := rbac.Authorize(admin, "s3:ListBuckets", "*")
	if !result.Allowed {
		t.Error("admin should be authorized for all actions")
	}

	result = rbac.Authorize(reader, "s3:DeleteBucket", "*")
	if result.Allowed {
		t.Error("readonly should not be authorized for DeleteBucket")
	}
}

func TestChunker_SplitAndMerge(t *testing.T) {
	chunker := storage.NewChunker(1024)

	testData := make([]byte, 5000)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	result, err := chunker.Split(bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(result.Chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}

	merged, err := chunker.Merge(result.Data)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if !bytes.Equal(merged, testData) {
		t.Error("merged data does not match original")
	}
}

func TestChunker_VerifyChecksum(t *testing.T) {
	chunker := storage.NewChunker(1024)
	data := []byte("test data for checksum verification")

	if !chunker.VerifyChecksum(data, storage.ComputeETag(data)) {
		t.Error("checksum verification should pass")
	}

	otherETag := storage.ComputeETag([]byte("different data"))
	if chunker.VerifyChecksum(data, otherETag) {
		t.Error("checksum verification should fail for different data")
	}
}

func TestChunker_Dedup(t *testing.T) {
	chunker := storage.NewChunker(1024)
	existing := map[string]bool{}

	chunks := [][]byte{
		[]byte("unique data 1"),
		[]byte("duplicate data"),
		[]byte("unique data 2"),
		[]byte("duplicate data"),
	}

	dedupedChunks, dedupedData, updatedExisting := chunker.Dedup(chunks, existing)

	if len(dedupedChunks) != 3 {
		t.Errorf("expected 3 chunks after dedup, got %d", len(dedupedChunks))
	}

	if len(updatedExisting) != 3 {
		t.Errorf("expected 3 entries in existing map, got %d", len(updatedExisting))
	}

	if len(dedupedData) != 3 {
		t.Errorf("expected 3 data entries after dedup, got %d", len(dedupedData))
	}
}

func TestComputeETag(t *testing.T) {
	data := []byte("hello")
	etag := storage.ComputeETag(data)
	if etag == "" {
		t.Error("etag should not be empty")
	}

	if etag == storage.ComputeETag([]byte("world")) {
		t.Error("different data should produce different etags")
	}

	if etag != storage.ComputeETag([]byte("hello")) {
		t.Error("same data should produce same etag")
	}
}

func TestSplitIntoParts(t *testing.T) {
	data := make([]byte, 5000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	parts, partsData := storage.SplitIntoParts(data, 1024)

	if len(parts) == 0 {
		t.Fatal("expected at least 1 part")
	}

	lastPartSize := int64(len(data)) % 1024
	if lastPartSize == 0 {
		lastPartSize = 1024
	}

	if parts[len(parts)-1].Size != lastPartSize {
		t.Errorf("last part size mismatch: got %d, want %d", parts[len(parts)-1].Size, lastPartSize)
	}

	merged := storage.JoinParts(partsData)
	if !bytes.Equal(merged, data) {
		t.Error("merged parts do not match original data")
	}
}

func TestDiskStore(t *testing.T) {
	store, err := storage.NewDiskStore("/tmp/test-filestore-data", "/tmp/test-filestore-tmp")
	if err != nil {
		t.Fatalf("NewDiskStore failed: %v", err)
	}

	testKey := "test/hello.txt"
	testData := []byte("Hello, Distributed Storage!")

	checksum, written, err := store.Write(testKey, bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if written != int64(len(testData)) {
		t.Errorf("written size mismatch: got %d, want %d", written, len(testData))
	}

	if checksum == "" {
		t.Error("checksum should not be empty")
	}

	if !store.Exists(testKey) {
		t.Error("file should exist after write")
	}

	var buf bytes.Buffer
	n, err := store.Read(testKey, &buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if n != int64(len(testData)) {
		t.Errorf("read size mismatch: got %d, want %d", n, len(testData))
	}

	if !bytes.Equal(buf.Bytes(), testData) {
		t.Error("read data does not match written data")
	}

	size, err := store.Size(testKey)
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != int64(len(testData)) {
		t.Errorf("size mismatch: got %d, want %d", size, len(testData))
	}

	if err := store.Delete(testKey); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if store.Exists(testKey) {
		t.Error("file should not exist after delete")
	}

	if err := store.Delete("/nonexistent/path"); err != nil {
		t.Error("deleting nonexistent file should not error")
	}
}

func TestDiskStoreReadRange(t *testing.T) {
	store, err := storage.NewDiskStore("/tmp/test-filestore-range", "/tmp/test-filestore-tmp2")
	if err != nil {
		t.Fatalf("NewDiskStore failed: %v", err)
	}

	testData := []byte("Hello World!")
	store.Write("range_test.txt", bytes.NewReader(testData))

	var buf bytes.Buffer
	n, err := store.ReadRange("range_test.txt", &buf, 6, 5)
	if err != nil {
		t.Fatalf("ReadRange failed: %v", err)
	}

	if n != 5 {
		t.Errorf("expected 5 bytes, got %d", n)
	}

	expected := "World"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}

	store.Delete("range_test.txt")
}

func TestReedSolomonCodec_EncodeDecode(t *testing.T) {
	codec := storage.NewReedSolomonCodec(4, 2)

	testData := [][]byte{
		[]byte("data chunk 1"),
		[]byte("data chunk 2"),
		[]byte("data chunk 3"),
		[]byte("data chunk 4"),
	}

	chunks, encoded, err := codec.Encode(testData)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if len(chunks) != 6 {
		t.Errorf("expected 6 total chunks (4 data + 2 parity), got %d", len(chunks))
	}

	if len(encoded) != 6 {
		t.Errorf("expected 6 encoded data parts, got %d", len(encoded))
	}

	parityCount := 0
	for _, c := range chunks {
		if c.IsParity {
			parityCount++
		}
	}
	if parityCount != 2 {
		t.Errorf("expected 2 parity chunks, got %d", parityCount)
	}

	if !codec.CanRecover(1) {
		t.Error("should be able to recover 1 missing shard")
	}
	if !codec.CanRecover(2) {
		t.Error("should be able to recover 2 missing shards")
	}
	if codec.CanRecover(3) {
		t.Error("should NOT be able to recover 3 missing shards with 2 parity shards")
	}
}

func TestValidateBucketName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid-bucket-123", false},
		{"ab", true},
		{"a", true},
		{string(make([]byte, 64)), true},
		{"UPPERCASE", true},
		{"valid.bucket.name", false},
		{"valid-bucket.name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := auth.ValidateBucketName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBucketName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type domainUserStub struct {
	Role string
}

type UserStub struct {
	Role string
}

func (u *UserStub) HasRole(role string) bool {
	return u.Role == role
}

func (u *UserStub) Can(requiredRole string) bool {
	roles := map[string]int{"admin": 100, "operator": 50, "user": 10, "readonly": 1}
	userLevel := roles[u.Role]
	requiredLevel := roles[requiredRole]
	return userLevel >= requiredLevel
}
