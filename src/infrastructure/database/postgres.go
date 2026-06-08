package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"

	"github.com/distributed-file-storage/service/src/config"
)

func NewPostgresDB(cfg config.DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	slog.Info("connected to PostgreSQL", "host", cfg.Host, "port", cfg.Port, "db", cfg.DBName)
	return db, nil
}

func RunMigrations(db *sql.DB, migrationsDir string) error {
	slog.Info("running database migrations", "dir", migrationsDir)

	migrations := []struct {
		name string
		sql  string
	}{
		{
			name: "001_create_users",
			sql: `CREATE TABLE IF NOT EXISTS users (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				username VARCHAR(255) UNIQUE NOT NULL,
				email VARCHAR(255) UNIQUE NOT NULL,
				password_hash VARCHAR(255) NOT NULL,
				role VARCHAR(50) NOT NULL DEFAULT 'user',
				access_key_id VARCHAR(255) UNIQUE,
				secret_key VARCHAR(255),
				enabled BOOLEAN NOT NULL DEFAULT true,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
			CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
			CREATE INDEX IF NOT EXISTS idx_users_access_key ON users(access_key_id);`,
		},
		{
			name: "002_create_buckets",
			sql: `CREATE TABLE IF NOT EXISTS buckets (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name VARCHAR(255) NOT NULL,
				owner VARCHAR(255) NOT NULL,
				region VARCHAR(100) NOT NULL DEFAULT 'us-east-1',
				versioning VARCHAR(20) NOT NULL DEFAULT 'disabled',
				object_lock_enabled BOOLEAN NOT NULL DEFAULT false,
				storage_class VARCHAR(50) NOT NULL DEFAULT 'STANDARD',
				max_size_bytes BIGINT NOT NULL DEFAULT 0,
				current_size_bytes BIGINT NOT NULL DEFAULT 0,
				object_count BIGINT NOT NULL DEFAULT 0,
				tags JSONB DEFAULT '{}',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ,
				UNIQUE(name, deleted_at)
			);
			CREATE INDEX IF NOT EXISTS idx_buckets_name ON buckets(name);
			CREATE INDEX IF NOT EXISTS idx_buckets_owner ON buckets(owner);
			CREATE INDEX IF NOT EXISTS idx_buckets_deleted ON buckets(deleted_at);`,
		},
		{
			name: "003_create_files",
			sql: `CREATE TABLE IF NOT EXISTS files (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				bucket_id UUID NOT NULL REFERENCES buckets(id),
				key TEXT NOT NULL,
				size BIGINT NOT NULL DEFAULT 0,
				etag VARCHAR(255) NOT NULL,
				content_type VARCHAR(255) NOT NULL DEFAULT 'application/octet-stream',
				metadata JSONB DEFAULT '{}',
				checksum_sha256 VARCHAR(64) NOT NULL DEFAULT '',
				storage_class VARCHAR(50) NOT NULL DEFAULT 'STANDARD',
				version_id VARCHAR(255) NOT NULL DEFAULT '',
				status VARCHAR(20) NOT NULL DEFAULT 'active',
				ttl BIGINT,
				expires_at TIMESTAMPTZ,
				owner VARCHAR(255) NOT NULL DEFAULT '',
				chunk_count INT NOT NULL DEFAULT 0,
				storage_size BIGINT NOT NULL DEFAULT 0,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ,
				UNIQUE(bucket_id, key, version_id, deleted_at)
			);
			CREATE INDEX IF NOT EXISTS idx_files_bucket ON files(bucket_id);
			CREATE INDEX IF NOT EXISTS idx_files_key ON files(key);
			CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);
			CREATE INDEX IF NOT EXISTS idx_files_expires ON files(expires_at);
			CREATE INDEX IF NOT EXISTS idx_files_owner ON files(owner);
			CREATE INDEX IF NOT EXISTS idx_files_deleted ON files(deleted_at);`,
		},
		{
			name: "004_create_file_chunks",
			sql: `CREATE TABLE IF NOT EXISTS file_chunks (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				file_id UUID NOT NULL REFERENCES files(id),
				chunk_index INT NOT NULL,
				size BIGINT NOT NULL DEFAULT 0,
				checksum VARCHAR(64) NOT NULL DEFAULT '',
				storage_node VARCHAR(255) NOT NULL DEFAULT 'local',
				storage_path TEXT NOT NULL,
				is_parity BOOLEAN NOT NULL DEFAULT false
			);
			CREATE INDEX IF NOT EXISTS idx_chunks_file ON file_chunks(file_id);
			CREATE INDEX IF NOT EXISTS idx_chunks_index ON file_chunks(file_id, chunk_index);`,
		},
		{
			name: "005_create_multipart_uploads",
			sql: `CREATE TABLE IF NOT EXISTS multipart_uploads (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				bucket_id UUID NOT NULL REFERENCES buckets(id),
				key TEXT NOT NULL,
				initiator VARCHAR(255) NOT NULL DEFAULT '',
				content_type VARCHAR(255) NOT NULL DEFAULT 'application/octet-stream',
				metadata JSONB DEFAULT '{}',
				storage_class VARCHAR(50) NOT NULL DEFAULT 'STANDARD',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			CREATE INDEX IF NOT EXISTS idx_uploads_bucket ON multipart_uploads(bucket_id);
			CREATE INDEX IF NOT EXISTS idx_uploads_key ON multipart_uploads(key);`,
		},
		{
			name: "006_create_upload_parts",
			sql: `CREATE TABLE IF NOT EXISTS upload_parts (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				upload_id UUID NOT NULL REFERENCES multipart_uploads(id),
				part_number INT NOT NULL,
				etag VARCHAR(255) NOT NULL DEFAULT '',
				size BIGINT NOT NULL DEFAULT 0,
				checksum VARCHAR(64) NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(upload_id, part_number)
			);
			CREATE INDEX IF NOT EXISTS idx_parts_upload ON upload_parts(upload_id);`,
		},
		{
			name: "007_create_policies",
			sql: `CREATE TABLE IF NOT EXISTS policies (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name VARCHAR(255) NOT NULL UNIQUE,
				statements JSONB NOT NULL DEFAULT '[]',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			CREATE INDEX IF NOT EXISTS idx_policies_name ON policies(name);`,
		},
		{
			name: "008_create_bucket_policies",
			sql: `CREATE TABLE IF NOT EXISTS bucket_policies (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				bucket_id UUID NOT NULL REFERENCES buckets(id),
				policy JSONB NOT NULL DEFAULT '{}',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(bucket_id)
			);
			CREATE INDEX IF NOT EXISTS idx_bucket_policies_bucket ON bucket_policies(bucket_id);`,
		},
		{
			name: "009_create_user_policies",
			sql: `CREATE TABLE IF NOT EXISTS user_policies (
				user_id UUID NOT NULL REFERENCES users(id),
				policy_id UUID NOT NULL REFERENCES policies(id),
				PRIMARY KEY (user_id, policy_id)
			);`,
		},
		{
			name: "010_create_audit_log",
			sql: `CREATE TABLE IF NOT EXISTS audit_log (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				user_id UUID REFERENCES users(id),
				action VARCHAR(255) NOT NULL,
				resource_type VARCHAR(255) NOT NULL,
				resource_id VARCHAR(255),
				details JSONB DEFAULT '{}',
				ip_address VARCHAR(45),
				user_agent TEXT,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(user_id);
			CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action);
			CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_log(resource_type, resource_id);
			CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);`,
		},
	}

	for _, m := range migrations {
		slog.Debug("running migration", "name", m.name)
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.name, err)
		}
		slog.Info("migration completed", "name", m.name)
	}

	if err := seedDefaultData(db); err != nil {
		return fmt.Errorf("seeding default data failed: %w", err)
	}

	slog.Info("all migrations completed successfully")
	return nil
}

func seedDefaultData(db *sql.DB) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	_, err = db.Exec(`INSERT INTO users (username, email, password_hash, role, access_key_id, secret_key)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		"admin", "admin@filestore.local",
		"$2a$12$LJ3m4ys3Lk0TSwHCpNqr8Oq5I1QxKz5Yy1Z5z5z5z5z5z5z5z5z5z5u",
		"admin",
		"AKIAIOSFODNN7EXAMPLE",
		"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	)
	if err != nil {
		return err
	}

	_, err = db.Exec(`INSERT INTO policies (name, statements) VALUES ($1, $2)`,
		"AdminFullAccess",
		`[{"effect":"Allow","actions":["s3:*"],"resources":["*"]}]`,
	)
	return err
}

type txKey struct{}

func WithTransaction(db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func NowPtr() *time.Time {
	t := time.Now().UTC()
	return &t
}
