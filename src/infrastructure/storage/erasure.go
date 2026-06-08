package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/distributed-file-storage/service/src/domain"
)

type ReedSolomonCodec struct {
	dataShards   int
	parityShards int
	totalShards  int
}

func NewReedSolomonCodec(dataShards, parityShards int) *ReedSolomonCodec {
	return &ReedSolomonCodec{
		dataShards:   dataShards,
		parityShards: parityShards,
		totalShards:  dataShards + parityShards,
	}
}

func (rs *ReedSolomonCodec) Encode(chunks [][]byte) ([]*domain.FileChunk, [][]byte, error) {
	if len(chunks) == 0 {
		return nil, nil, fmt.Errorf("no chunks to encode")
	}

	shardSize := 0
	for _, chunk := range chunks {
		if len(chunk) > shardSize {
			shardSize = len(chunk)
		}
	}

	paddedChunks := make([][]byte, rs.dataShards)
	for i := 0; i < rs.dataShards; i++ {
		if i < len(chunks) {
			paddedChunks[i] = make([]byte, shardSize)
			copy(paddedChunks[i], chunks[i])
		} else {
			paddedChunks[i] = make([]byte, shardSize)
		}
	}

	parityData := make([][]byte, rs.parityShards)
	for i := 0; i < rs.parityShards; i++ {
		parityData[i] = make([]byte, shardSize)
	}

	for byteIdx := 0; byteIdx < shardSize; byteIdx++ {
		for i := 0; i < rs.parityShards; i++ {
			var parity byte
			for j := 0; j < rs.dataShards; j++ {
				coeff := byte((i + 1) * (j + 1))
				parity ^= paddedChunks[j][byteIdx] * coeff
			}
			parityData[i][byteIdx] = parity
		}
	}

	var resultChunks []*domain.FileChunk
	var resultData [][]byte

	for i := 0; i < rs.dataShards; i++ {
		if i < len(chunks) {
			h := sha256.Sum256(chunks[i])
			chunk := &domain.FileChunk{
				ChunkIndex:  i,
				Size:        int64(len(chunks[i])),
				Checksum:    hex.EncodeToString(h[:]),
				StorageNode: "local",
				StoragePath: fmt.Sprintf("erasure/data_%d", i),
				IsParity:    false,
			}
			resultChunks = append(resultChunks, chunk)
			resultData = append(resultData, chunks[i])
		}
	}

	for i := 0; i < rs.parityShards; i++ {
		h := sha256.Sum256(parityData[i])
		chunk := &domain.FileChunk{
			ChunkIndex:  rs.dataShards + i,
			Size:        int64(len(parityData[i])),
			Checksum:    hex.EncodeToString(h[:]),
			StorageNode: "local",
			StoragePath: fmt.Sprintf("erasure/parity_%d", i),
			IsParity:    true,
		}
		resultChunks = append(resultChunks, chunk)
		resultData = append(resultData, parityData[i])
	}

	slog.Debug("erasure encoding complete",
		"data_shards", rs.dataShards,
		"parity_shards", rs.parityShards,
		"shard_size", shardSize,
	)

	return resultChunks, resultData, nil
}

func (rs *ReedSolomonCodec) Decode(chunks []*domain.FileChunk, data [][]byte) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks to decode")
	}

	available := make(map[int][]byte)
	for i, chunk := range chunks {
		if i < len(data) {
			available[chunk.ChunkIndex] = data[i]
		}
	}

	presentDataShards := 0
	for i := 0; i < rs.dataShards; i++ {
		if _, ok := available[i]; ok {
			presentDataShards++
		}
	}

	if presentDataShards < rs.dataShards {
		needed := rs.dataShards - presentDataShards
		recovered := 0
		for i := rs.dataShards; i < rs.totalShards && recovered < needed; i++ {
			if parityData, ok := available[i]; ok {
				for j := 0; j < rs.dataShards; j++ {
					if _, ok := available[j]; !ok {
						recoveredChunk := make([]byte, len(parityData))
						parityIdx := i - rs.dataShards
						if parityIdx == 0 {
							for k := 0; k < rs.dataShards; k++ {
								if k != j {
									if d, ok := available[k]; ok {
										for b := 0; b < len(parityData) && b < len(d); b++ {
											recoveredChunk[b] ^= d[b]
										}
									}
								}
							}
							for b := 0; b < len(parityData); b++ {
								recoveredChunk[b] ^= parityData[b]
							}
						}
						available[j] = recoveredChunk
						break
					}
				}
				recovered++
			}
		}
	}

	reconstructed := make([]byte, 0)
	for i := 0; i < rs.dataShards; i++ {
		if d, ok := available[i]; ok {
			reconstructed = append(reconstructed, d...)
		}
	}

	return reconstructed, nil
}

func (rs *ReedSolomonCodec) CanRecover(missingShards int) bool {
	return missingShards <= rs.parityShards
}
