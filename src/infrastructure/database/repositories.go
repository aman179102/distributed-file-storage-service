package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/distributed-file-storage/service/src/domain"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *domain.User) error {
	_, err := r.db.Exec(
		`INSERT INTO users (id, username, email, password_hash, role, access_key_id, secret_key, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		user.ID, user.Username, user.Email, user.PasswordHash, user.Role,
		user.AccessKeyID, user.SecretKey, user.Enabled, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(id string) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.QueryRow(
		`SELECT id, username, email, password_hash, role, access_key_id, secret_key, enabled, created_at, updated_at
		FROM users WHERE id = $1 AND enabled = true`, id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role,
		&user.AccessKeyID, &user.SecretKey, &user.Enabled, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.NewNotFound("user")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetByUsername(username string) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.QueryRow(
		`SELECT id, username, email, password_hash, role, access_key_id, secret_key, enabled, created_at, updated_at
		FROM users WHERE username = $1 AND enabled = true`, username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role,
		&user.AccessKeyID, &user.SecretKey, &user.Enabled, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.NewNotFound("user")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetByAccessKey(accessKey string) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.QueryRow(
		`SELECT id, username, email, password_hash, role, access_key_id, secret_key, enabled, created_at, updated_at
		FROM users WHERE access_key_id = $1 AND enabled = true`, accessKey,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role,
		&user.AccessKeyID, &user.SecretKey, &user.Enabled, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.NewNotFound("user")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by access key: %w", err)
	}
	return user, nil
}

func (r *UserRepository) List(offset, limit int) ([]*domain.User, error) {
	rows, err := r.db.Query(
		`SELECT id, username, email, role, enabled, created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		user := &domain.User{}
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Role,
			&user.Enabled, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}
	if users == nil {
		users = []*domain.User{}
	}
	return users, nil
}

type BucketRepository struct {
	db *sql.DB
}

func NewBucketRepository(db *sql.DB) *BucketRepository {
	return &BucketRepository{db: db}
}

func (r *BucketRepository) Create(bucket *domain.Bucket) error {
	tagsJSON, err := json.Marshal(bucket.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	_, err = r.db.Exec(
		`INSERT INTO buckets (id, name, owner, region, versioning, object_lock_enabled, storage_class,
		max_size_bytes, current_size_bytes, object_count, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		bucket.ID, bucket.Name, bucket.Owner, bucket.Region, bucket.Versioning,
		bucket.ObjectLockEnabled, bucket.StorageClass, bucket.MaxSizeBytes,
		bucket.CurrentSizeBytes, bucket.ObjectCount, tagsJSON, bucket.CreatedAt, bucket.UpdatedAt,
	)
	if err != nil {
		if isDuplicateError(err) {
			return domain.NewAlreadyExists(fmt.Sprintf("bucket '%s'", bucket.Name))
		}
		return fmt.Errorf("failed to create bucket: %w", err)
	}
	return nil
}

func (r *BucketRepository) GetByID(id string) (*domain.Bucket, error) {
	bucket := &domain.Bucket{}
	var tagsJSON []byte
	err := r.db.QueryRow(
		`SELECT id, name, owner, region, versioning, object_lock_enabled, storage_class,
		max_size_bytes, current_size_bytes, object_count, tags, created_at, updated_at, deleted_at
		FROM buckets WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&bucket.ID, &bucket.Name, &bucket.Owner, &bucket.Region, &bucket.Versioning,
		&bucket.ObjectLockEnabled, &bucket.StorageClass, &bucket.MaxSizeBytes,
		&bucket.CurrentSizeBytes, &bucket.ObjectCount, &tagsJSON, &bucket.CreatedAt,
		&bucket.UpdatedAt, &bucket.DeletedAt)
	if err == sql.ErrNoRows {
		return nil, domain.NewNotFound("bucket")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket: %w", err)
	}
	if err := json.Unmarshal(tagsJSON, &bucket.Tags); err != nil {
		bucket.Tags = map[string]string{}
	}
	return bucket, nil
}

func (r *BucketRepository) GetByName(name string) (*domain.Bucket, error) {
	bucket := &domain.Bucket{}
	var tagsJSON []byte
	err := r.db.QueryRow(
		`SELECT id, name, owner, region, versioning, object_lock_enabled, storage_class,
		max_size_bytes, current_size_bytes, object_count, tags, created_at, updated_at, deleted_at
		FROM buckets WHERE name = $1 AND deleted_at IS NULL`, name,
	).Scan(&bucket.ID, &bucket.Name, &bucket.Owner, &bucket.Region, &bucket.Versioning,
		&bucket.ObjectLockEnabled, &bucket.StorageClass, &bucket.MaxSizeBytes,
		&bucket.CurrentSizeBytes, &bucket.ObjectCount, &tagsJSON, &bucket.CreatedAt,
		&bucket.UpdatedAt, &bucket.DeletedAt)
	if err == sql.ErrNoRows {
		return nil, domain.NewNotFound("bucket")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket by name: %w", err)
	}
	if err := json.Unmarshal(tagsJSON, &bucket.Tags); err != nil {
		bucket.Tags = map[string]string{}
	}
	return bucket, nil
}

func (r *BucketRepository) List(owner string, offset, limit int) ([]*domain.Bucket, error) {
	var rows *sql.Rows
	var err error
	if owner != "" {
		rows, err = r.db.Query(
			`SELECT id, name, owner, region, versioning, object_lock_enabled, storage_class,
			max_size_bytes, current_size_bytes, object_count, tags, created_at, updated_at
			FROM buckets WHERE owner = $1 AND deleted_at IS NULL
			ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			owner, limit, offset,
		)
	} else {
		rows, err = r.db.Query(
			`SELECT id, name, owner, region, versioning, object_lock_enabled, storage_class,
			max_size_bytes, current_size_bytes, object_count, tags, created_at, updated_at
			FROM buckets WHERE deleted_at IS NULL
			ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			limit, offset,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}
	defer rows.Close()

	var buckets []*domain.Bucket
	for rows.Next() {
		bucket := &domain.Bucket{}
		var tagsJSON []byte
		if err := rows.Scan(&bucket.ID, &bucket.Name, &bucket.Owner, &bucket.Region,
			&bucket.Versioning, &bucket.ObjectLockEnabled, &bucket.StorageClass,
			&bucket.MaxSizeBytes, &bucket.CurrentSizeBytes, &bucket.ObjectCount,
			&tagsJSON, &bucket.CreatedAt, &bucket.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan bucket: %w", err)
		}
		if err := json.Unmarshal(tagsJSON, &bucket.Tags); err != nil {
			bucket.Tags = map[string]string{}
		}
		buckets = append(buckets, bucket)
	}
	if buckets == nil {
		buckets = []*domain.Bucket{}
	}
	return buckets, nil
}

func (r *BucketRepository) Update(bucket *domain.Bucket) error {
	tagsJSON, err := json.Marshal(bucket.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	result, err := r.db.Exec(
		`UPDATE buckets SET versioning=$1, storage_class=$2, max_size_bytes=$3,
		current_size_bytes=$4, object_count=$5, tags=$6, updated_at=$7
		WHERE id=$8 AND deleted_at IS NULL`,
		bucket.Versioning, bucket.StorageClass, bucket.MaxSizeBytes,
		bucket.CurrentSizeBytes, bucket.ObjectCount, tagsJSON, time.Now().UTC(),
		bucket.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update bucket: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.NewNotFound("bucket")
	}
	return nil
}

func (r *BucketRepository) SoftDelete(id string) error {
	result, err := r.db.Exec(
		`UPDATE buckets SET deleted_at=$1, updated_at=$1 WHERE id=$2 AND deleted_at IS NULL`,
		time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("failed to soft delete bucket: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.NewNotFound("bucket")
	}
	return nil
}

type FileRepository struct {
	db *sql.DB
}

func NewFileRepository(db *sql.DB) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) Create(file *domain.FileInfo) error {
	metadataJSON, err := json.Marshal(file.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var ttl *int64
	if file.TTL != nil {
		t := int64(*file.TTL / time.Second)
		ttl = &t
	}

	_, err = r.db.Exec(
		`INSERT INTO files (id, bucket_id, key, size, etag, content_type, metadata, checksum_sha256,
		storage_class, version_id, status, ttl, expires_at, owner, chunk_count, storage_size,
		created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		file.ID, file.BucketID, file.Key, file.Size, file.ETag, file.ContentType, metadataJSON,
		file.ChecksumSHA256, file.StorageClass, file.VersionID, file.Status, ttl, file.ExpiresAt,
		file.Owner, file.ChunkCount, file.StorageSize, file.CreatedAt, file.UpdatedAt,
	)
	if err != nil {
		if isDuplicateError(err) {
			return domain.NewAlreadyExists(fmt.Sprintf("file '%s'", file.Key))
		}
		return fmt.Errorf("failed to create file: %w", err)
	}
	return nil
}

func (r *FileRepository) GetByID(id string) (*domain.FileInfo, error) {
	file := &domain.FileInfo{}
	var metadataJSON []byte
	var ttl *int64
	err := r.db.QueryRow(
		`SELECT id, bucket_id, key, size, etag, content_type, metadata, checksum_sha256,
		storage_class, version_id, status, ttl, expires_at, owner, chunk_count, storage_size,
		created_at, updated_at, deleted_at
		FROM files WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&file.ID, &file.BucketID, &file.Key, &file.Size, &file.ETag, &file.ContentType,
		&metadataJSON, &file.ChecksumSHA256, &file.StorageClass, &file.VersionID, &file.Status,
		&ttl, &file.ExpiresAt, &file.Owner, &file.ChunkCount, &file.StorageSize,
		&file.CreatedAt, &file.UpdatedAt, &file.DeletedAt)
	if err == sql.ErrNoRows {
		return nil, domain.NewNotFound("file")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	if ttl != nil {
		d := time.Duration(*ttl) * time.Second
		file.TTL = &d
	}
	if err := json.Unmarshal(metadataJSON, &file.Metadata); err != nil {
		file.Metadata = map[string]string{}
	}
	return file, nil
}

func (r *FileRepository) GetByBucketAndKey(bucketID, key, versionID string) (*domain.FileInfo, error) {
	file := &domain.FileInfo{}
	var metadataJSON []byte
	var ttl *int64

	var query string
	var args []interface{}

	if versionID != "" {
		query = `SELECT id, bucket_id, key, size, etag, content_type, metadata, checksum_sha256,
			storage_class, version_id, status, ttl, expires_at, owner, chunk_count, storage_size,
			created_at, updated_at, deleted_at
			FROM files WHERE bucket_id=$1 AND key=$2 AND version_id=$3 AND deleted_at IS NULL`
		args = []interface{}{bucketID, key, versionID}
	} else {
		query = `SELECT id, bucket_id, key, size, etag, content_type, metadata, checksum_sha256,
			storage_class, version_id, status, ttl, expires_at, owner, chunk_count, storage_size,
			created_at, updated_at, deleted_at
			FROM files WHERE bucket_id=$1 AND key=$2 AND deleted_at IS NULL
			ORDER BY created_at DESC LIMIT 1`
		args = []interface{}{bucketID, key}
	}

	err := r.db.QueryRow(query, args...).Scan(&file.ID, &file.BucketID, &file.Key, &file.Size,
		&file.ETag, &file.ContentType, &metadataJSON, &file.ChecksumSHA256, &file.StorageClass,
		&file.VersionID, &file.Status, &ttl, &file.ExpiresAt, &file.Owner, &file.ChunkCount,
		&file.StorageSize, &file.CreatedAt, &file.UpdatedAt, &file.DeletedAt)
	if err == sql.ErrNoRows {
		return nil, domain.NewNotFound("file")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	if ttl != nil {
		d := time.Duration(*ttl) * time.Second
		file.TTL = &d
	}
	if err := json.Unmarshal(metadataJSON, &file.Metadata); err != nil {
		file.Metadata = map[string]string{}
	}
	return file, nil
}

func (r *FileRepository) List(bucketID string, prefix string, offset, limit int) ([]*domain.FileInfo, error) {
	var rows *sql.Rows
	var err error

	if prefix != "" {
		rows, err = r.db.Query(
			`SELECT id, bucket_id, key, size, etag, content_type, storage_class, version_id,
			status, owner, chunk_count, storage_size, created_at, updated_at
			FROM files WHERE bucket_id=$1 AND key LIKE $2 AND deleted_at IS NULL AND status='active'
			ORDER BY key ASC LIMIT $3 OFFSET $4`,
			bucketID, prefix+"%", limit, offset,
		)
	} else {
		rows, err = r.db.Query(
			`SELECT id, bucket_id, key, size, etag, content_type, storage_class, version_id,
			status, owner, chunk_count, storage_size, created_at, updated_at
			FROM files WHERE bucket_id=$1 AND deleted_at IS NULL AND status='active'
			ORDER BY key ASC LIMIT $2 OFFSET $3`,
			bucketID, limit, offset,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	defer rows.Close()

	var files []*domain.FileInfo
	for rows.Next() {
		f := &domain.FileInfo{}
		if err := rows.Scan(&f.ID, &f.BucketID, &f.Key, &f.Size, &f.ETag, &f.ContentType,
			&f.StorageClass, &f.VersionID, &f.Status, &f.Owner, &f.ChunkCount,
			&f.StorageSize, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}
		f.Metadata = map[string]string{}
		files = append(files, f)
	}
	if files == nil {
		files = []*domain.FileInfo{}
	}
	return files, nil
}

func (r *FileRepository) SoftDelete(id string) error {
	result, err := r.db.Exec(
		`UPDATE files SET deleted_at=$1, status='deleted', updated_at=$1 WHERE id=$2 AND deleted_at IS NULL`,
		time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("failed to soft delete file: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.NewNotFound("file")
	}
	return nil
}

func (r *FileRepository) UpdateStatus(id string, status domain.FileStatus) error {
	_, err := r.db.Exec(
		`UPDATE files SET status=$1, updated_at=$2 WHERE id=$3 AND deleted_at IS NULL`,
		status, time.Now().UTC(), id,
	)
	return err
}

func (r *FileRepository) CountByBucket(bucketID string) (int64, int64, error) {
	var count, size int64
	err := r.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(size), 0) FROM files WHERE bucket_id=$1 AND deleted_at IS NULL AND status='active'`,
		bucketID,
	).Scan(&count, &size)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count files: %w", err)
	}
	return count, size, nil
}

func (r *FileRepository) ListExpired(limit int) ([]*domain.FileInfo, error) {
	rows, err := r.db.Query(
		`SELECT id, bucket_id, key, size, etag, content_type, version_id, status, owner,
		chunk_count, storage_size, created_at, updated_at
		FROM files WHERE expires_at IS NOT NULL AND expires_at < NOW() AND status='active' AND deleted_at IS NULL
		LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list expired files: %w", err)
	}
	defer rows.Close()

	var files []*domain.FileInfo
	for rows.Next() {
		f := &domain.FileInfo{}
		if err := rows.Scan(&f.ID, &f.BucketID, &f.Key, &f.Size, &f.ETag, &f.ContentType,
			&f.VersionID, &f.Status, &f.Owner, &f.ChunkCount, &f.StorageSize,
			&f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan expired file: %w", err)
		}
		f.Metadata = map[string]string{}
		files = append(files, f)
	}
	if files == nil {
		files = []*domain.FileInfo{}
	}
	return files, nil
}

type ChunkRepository struct {
	db *sql.DB
}

func NewChunkRepository(db *sql.DB) *ChunkRepository {
	return &ChunkRepository{db: db}
}

func (r *ChunkRepository) Create(chunk *domain.FileChunk) error {
	_, err := r.db.Exec(
		`INSERT INTO file_chunks (id, file_id, chunk_index, size, checksum, storage_node, storage_path, is_parity)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		chunk.ID, chunk.FileID, chunk.ChunkIndex, chunk.Size, chunk.Checksum,
		chunk.StorageNode, chunk.StoragePath, chunk.IsParity,
	)
	if err != nil {
		return fmt.Errorf("failed to create chunk: %w", err)
	}
	return nil
}

func (r *ChunkRepository) ListByFileID(fileID string) ([]*domain.FileChunk, error) {
	rows, err := r.db.Query(
		`SELECT id, file_id, chunk_index, size, checksum, storage_node, storage_path, is_parity
		FROM file_chunks WHERE file_id=$1 ORDER BY chunk_index ASC`, fileID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list chunks: %w", err)
	}
	defer rows.Close()

	var chunks []*domain.FileChunk
	for rows.Next() {
		chunk := &domain.FileChunk{}
		if err := rows.Scan(&chunk.ID, &chunk.FileID, &chunk.ChunkIndex, &chunk.Size,
			&chunk.Checksum, &chunk.StorageNode, &chunk.StoragePath, &chunk.IsParity); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}
	if chunks == nil {
		chunks = []*domain.FileChunk{}
	}
	return chunks, nil
}

func (r *ChunkRepository) DeleteByFileID(fileID string) error {
	_, err := r.db.Exec(`DELETE FROM file_chunks WHERE file_id=$1`, fileID)
	return err
}

type PolicyRepository struct {
	db *sql.DB
}

func NewPolicyRepository(db *sql.DB) *PolicyRepository {
	return &PolicyRepository{db: db}
}

func (r *PolicyRepository) Create(policy *domain.IAMPolicy) error {
	statementsJSON, err := json.Marshal(policy.Statements)
	if err != nil {
		return fmt.Errorf("failed to marshal statements: %w", err)
	}
	_, err = r.db.Exec(
		`INSERT INTO policies (id, name, statements, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)`,
		policy.ID, policy.Name, statementsJSON, policy.CreatedAt, policy.UpdatedAt,
	)
	if err != nil {
		if isDuplicateError(err) {
			return domain.NewAlreadyExists(fmt.Sprintf("policy '%s'", policy.Name))
		}
		return fmt.Errorf("failed to create policy: %w", err)
	}
	return nil
}

func (r *PolicyRepository) GetByID(id string) (*domain.IAMPolicy, error) {
	policy := &domain.IAMPolicy{}
	var statementsJSON []byte
	err := r.db.QueryRow(
		`SELECT id, name, statements, created_at, updated_at FROM policies WHERE id=$1`, id,
	).Scan(&policy.ID, &policy.Name, &statementsJSON, &policy.CreatedAt, &policy.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.NewNotFound("policy")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}
	if err := json.Unmarshal(statementsJSON, &policy.Statements); err != nil {
		return nil, fmt.Errorf("failed to unmarshal statements: %w", err)
	}
	return policy, nil
}

func (r *PolicyRepository) AddUserPolicy(userID, policyID string) error {
	_, err := r.db.Exec(
		`INSERT INTO user_policies (user_id, policy_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, policyID,
	)
	return err
}

func (r *PolicyRepository) GetUserPolicies(userID string) ([]*domain.IAMPolicy, error) {
	rows, err := r.db.Query(
		`SELECT p.id, p.name, p.statements, p.created_at, p.updated_at
		FROM policies p
		JOIN user_policies up ON up.policy_id = p.id
		WHERE up.user_id = $1`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user policies: %w", err)
	}
	defer rows.Close()

	var policies []*domain.IAMPolicy
	for rows.Next() {
		policy := &domain.IAMPolicy{}
		var statementsJSON []byte
		if err := rows.Scan(&policy.ID, &policy.Name, &statementsJSON,
			&policy.CreatedAt, &policy.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan policy: %w", err)
		}
		if err := json.Unmarshal(statementsJSON, &policy.Statements); err != nil {
			return nil, fmt.Errorf("failed to unmarshal statements: %w", err)
		}
		policies = append(policies, policy)
	}
	if policies == nil {
		policies = []*domain.IAMPolicy{}
	}
	return policies, nil
}

type BucketPolicyRepository struct {
	db *sql.DB
}

func NewBucketPolicyRepository(db *sql.DB) *BucketPolicyRepository {
	return &BucketPolicyRepository{db: db}
}

func (r *BucketPolicyRepository) SetPolicy(bucketID string, policyJSON string) error {
	_, err := r.db.Exec(
		`INSERT INTO bucket_policies (bucket_id, policy, created_at, updated_at)
		VALUES ($1, $2::jsonb, NOW(), NOW())
		ON CONFLICT (bucket_id) DO UPDATE SET policy=$2::jsonb, updated_at=NOW()`,
		bucketID, policyJSON,
	)
	return err
}

func (r *BucketPolicyRepository) GetPolicy(bucketID string) (*domain.BucketPolicy, error) {
	bp := &domain.BucketPolicy{}
	err := r.db.QueryRow(
		`SELECT id, bucket_id, policy::text, created_at, updated_at
		FROM bucket_policies WHERE bucket_id=$1`, bucketID,
	).Scan(&bp.ID, &bp.BucketID, &bp.Policy, &bp.CreatedAt, &bp.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.NewNotFound("bucket policy")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket policy: %w", err)
	}
	return bp, nil
}

type AuditLogEntry struct {
	ID           string
	UserID       string
	Action       string
	ResourceType string
	ResourceID   string
	Details      map[string]interface{}
	IPAddress    string
	UserAgent    string
	CreatedAt    time.Time
}

type AuditLogRepository struct {
	db *sql.DB
}

func NewAuditLogRepository(db *sql.DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

func (r *AuditLogRepository) Log(entry *AuditLogEntry) error {
	detailsJSON, err := json.Marshal(entry.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}
	_, err = r.db.Exec(
		`INSERT INTO audit_log (id, user_id, action, resource_type, resource_id, details, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		entry.ID, entry.UserID, entry.Action, entry.ResourceType, entry.ResourceID,
		detailsJSON, entry.IPAddress, entry.UserAgent, entry.CreatedAt,
	)
	return err
}

func (r *AuditLogRepository) List(limit, offset int) ([]*AuditLogEntry, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, action, resource_type, resource_id, details::text, ip_address, user_agent, created_at
		FROM audit_log ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	var entries []*AuditLogEntry
	for rows.Next() {
		entry := &AuditLogEntry{}
		var detailsStr string
		if err := rows.Scan(&entry.ID, &entry.UserID, &entry.Action, &entry.ResourceType,
			&entry.ResourceID, &detailsStr, &entry.IPAddress, &entry.UserAgent, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		_ = json.Unmarshal([]byte(detailsStr), &entry.Details)
		entries = append(entries, entry)
	}
	if entries == nil {
		entries = []*AuditLogEntry{}
	}
	return entries, nil
}

func (r *FileRepository) ListChunks(fileID string) ([]*domain.FileChunk, error) {
	rows, err := r.db.Query(
		`SELECT id, file_id, chunk_index, size, checksum, storage_node, storage_path, is_parity
		FROM file_chunks WHERE file_id=$1 ORDER BY chunk_index ASC`, fileID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list chunks: %w", err)
	}
	defer rows.Close()

	var chunks []*domain.FileChunk
	for rows.Next() {
		c := &domain.FileChunk{}
		if err := rows.Scan(&c.ID, &c.FileID, &c.ChunkIndex, &c.Size,
			&c.Checksum, &c.StorageNode, &c.StoragePath, &c.IsParity); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}
		chunks = append(chunks, c)
	}
	if chunks == nil {
		chunks = []*domain.FileChunk{}
	}
	return chunks, nil
}

func (r *FileRepository) CreateChunk(chunk *domain.FileChunk) error {
	_, err := r.db.Exec(
		`INSERT INTO file_chunks (id, file_id, chunk_index, size, checksum, storage_node, storage_path, is_parity)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		chunk.ID, chunk.FileID, chunk.ChunkIndex, chunk.Size, chunk.Checksum,
		chunk.StorageNode, chunk.StoragePath, chunk.IsParity,
	)
	return err
}

func isDuplicateError(err error) bool {
	return err != nil && (contains(err.Error(), "duplicate key") || contains(err.Error(), "UNIQUE constraint"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
