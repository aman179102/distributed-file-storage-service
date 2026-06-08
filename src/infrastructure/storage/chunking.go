package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"

	"github.com/distributed-file-storage/service/src/domain"
)

type Chunker struct {
	chunkSize int64
}

func NewChunker(chunkSize int64) *Chunker {
	return &Chunker{chunkSize: chunkSize}
}

type ChunkResult struct {
	Chunks  []*domain.FileChunk
	Data    [][]byte
	RawData []byte
}

func (c *Chunker) Split(data io.Reader) (*ChunkResult, error) {
	var rawData []byte
	var err error
	rawData, err = io.ReadAll(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	result := &ChunkResult{
		Chunks:  make([]*domain.FileChunk, 0),
		Data:    make([][]byte, 0),
		RawData: rawData,
	}

	totalSize := int64(len(rawData))
	var offset int64
	index := 0

	for offset < totalSize {
		end := offset + c.chunkSize
		if end > totalSize {
			end = totalSize
		}

		chunkData := rawData[offset:end]
		checksum := sha256.Sum256(chunkData)

		chunk := &domain.FileChunk{
			ChunkIndex:  index,
			Size:        int64(len(chunkData)),
			Checksum:    hex.EncodeToString(checksum[:]),
			StorageNode: "local",
			StoragePath: fmt.Sprintf("chunks/%d/%d", index, len(chunkData)),
			IsParity:    false,
		}

		result.Chunks = append(result.Chunks, chunk)
		result.Data = append(result.Data, chunkData)
		offset += int64(len(chunkData))
		index++
	}

	slog.Debug("file split into chunks", "total_size", totalSize, "chunk_count", len(result.Chunks), "chunk_size", c.chunkSize)
	return result, nil
}

func (c *Chunker) Merge(chunks [][]byte) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks to merge")
	}

	totalSize := 0
	for _, chunk := range chunks {
		totalSize += len(chunk)
	}

	result := make([]byte, 0, totalSize)
	for _, chunk := range chunks {
		result = append(result, chunk...)
	}

	return result, nil
}

func (c *Chunker) VerifyChecksum(data []byte, expectedChecksum string) bool {
	checksum := sha256.Sum256(data)
	return hex.EncodeToString(checksum[:]) == expectedChecksum
}

func (c *Chunker) Dedup(chunks [][]byte, existingChecksums map[string]bool) ([]*domain.FileChunk, [][]byte, map[string]bool) {
	var dedupedChunks []*domain.FileChunk
	var dedupedData [][]byte
	added := make(map[string]bool)
	skipped := 0

	for i, chunkData := range chunks {
		checksum := sha256.Sum256(chunkData)
		checksumStr := hex.EncodeToString(checksum[:])

		if existingChecksums[checksumStr] || added[checksumStr] {
			skipped++
			continue
		}

		added[checksumStr] = true
		if existingChecksums == nil {
			existingChecksums = make(map[string]bool)
		}
		existingChecksums[checksumStr] = true

		chunk := &domain.FileChunk{
			ChunkIndex:  len(dedupedChunks),
			Size:        int64(len(chunkData)),
			Checksum:    checksumStr,
			StorageNode: "local",
			StoragePath: fmt.Sprintf("chunks/%s", checksumStr),
			IsParity:    false,
		}

		dedupedChunks = append(dedupedChunks, chunk)
		dedupedData = append(dedupedData, chunkData)
	}

	slog.Debug("dedup complete", "original", len(chunks), "deduped", len(dedupedChunks), "skipped", skipped)
	return dedupedChunks, dedupedData, existingChecksums
}

func SplitIntoParts(data []byte, partSize int64) ([]domain.FilePart, [][]byte) {
	totalSize := int64(len(data))
	var parts []domain.FilePart
	var partsData [][]byte
	partNumber := 1
	var offset int64

	for offset < totalSize {
		end := offset + partSize
		if end > totalSize {
			end = totalSize
		}

		partData := data[offset:end]
		checksum := sha256.Sum256(partData)
		etag := hex.EncodeToString(checksum[:])

		parts = append(parts, domain.FilePart{
			PartNumber: partNumber,
			ETag:       fmt.Sprintf("%s-%d", etag, partNumber),
			Size:       int64(len(partData)),
			Checksum:   etag,
		})
		partsData = append(partsData, partData)
		partNumber++
		offset += int64(len(partData))
	}

	return parts, partsData
}

func ComputeETag(data []byte) string {
	checksum := sha256.Sum256(data)
	return hex.EncodeToString(checksum[:])
}

func JoinParts(partsData [][]byte) []byte {
	return bytes.Join(partsData, nil)
}

# refactor: simplify data models logic and remove duplication
