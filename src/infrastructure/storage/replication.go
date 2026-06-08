package storage

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/distributed-file-storage/service/src/domain"
)

type Replicator struct {
	replicationFactor int
	stores            []*DiskStore
	mu                sync.RWMutex
}

func NewReplicator(replicationFactor int, stores []*DiskStore) *Replicator {
	return &Replicator{
		replicationFactor: replicationFactor,
		stores:            stores,
	}
}

func (r *Replicator) Replicate(fileID string, chunks []*domain.FileChunk, data [][]byte) error {
	if r.replicationFactor <= 1 || len(r.stores) <= 1 {
		for i, chunk := range chunks {
			if i < len(data) {
				store := r.stores[0]
				path := fmt.Sprintf("replicas/%s/%d", fileID, chunk.ChunkIndex)
				if err := store.EnsureDir(path); err != nil {
					return fmt.Errorf("failed to ensure directory: %w", err)
				}
			}
		}
		return nil
	}

	replicas := r.replicationFactor
	if replicas > len(r.stores) {
		replicas = len(r.stores)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, replicas*len(chunks))

	for replicaIdx := 0; replicaIdx < replicas; replicaIdx++ {
		store := r.stores[replicaIdx%len(r.stores)]
		for i, chunk := range chunks {
			if i >= len(data) {
				continue
			}
			wg.Add(1)
			go func(s *DiskStore, idx int, c *domain.FileChunk, d []byte) {
				defer wg.Done()
				path := fmt.Sprintf("replicas/%s/%d", fileID, c.ChunkIndex)
				if _, _, err := s.Write(path, asReader(d)); err != nil {
					errCh <- fmt.Errorf("replica %d failed: %w", idx, err)
				}
			}(store, replicaIdx, chunk, data[i])
		}
	}

	wg.Wait()
	close(errCh)

	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		slog.Warn("replication completed with errors", "total_errors", len(errors))
		return errors[0]
	}

	slog.Debug("replication complete", "file_id", fileID, "factor", replicas,
		"chunks", len(chunks), "stores", len(r.stores))
	return nil
}

func asReader(data []byte) *byteReader {
	return &byteReader{data: data}
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, nil
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
