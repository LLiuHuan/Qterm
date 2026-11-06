package vault

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"changeme/internal/sqlitedsn"

	"modernc.org/sqlite"
)

var (
	ErrNotFound                 = errors.New("vault secret not found")
	ErrKindMismatch             = errors.New("vault secret kind does not match")
	ErrAlreadyExists            = errors.New("vault secret already exists")
	ErrStoreNotReady            = errors.New("vault store is not ready")
	ErrInvalidVaultDatabasePath = errors.New("invalid vault database path")
)

const maxSecretNameSize = 256

type Secret struct {
	ID        string
	Name      string
	Kind      Kind
	Username  string
	PublicKey string
	Value     []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Store) Create(
	ctx context.Context,
	id string,
	name string,
	kind Kind,
	value []byte,
) (Metadata, error) {
	return s.CreateWithMetadata(ctx, id, name, "", "", kind, value)
}

func (s *Store) CreateWithMetadata(
	ctx context.Context,
	id string,
	name string,
	username string,
	publicKey string,
	kind Kind,
	value []byte,
) (Metadata, error) {
	if err := contextError(ctx); err != nil {
		return Metadata{}, err
	}
	if err := s.ready(); err != nil {
		return Metadata{}, err
	}
	if err := validateMetadata(id, kind); err != nil {
		return Metadata{}, err
	}
	name, err := normalizeSecretName(name)
	if err != nil {
		return Metadata{}, err
	}
	envelope, err := s.cipher.Seal(id, kind, value)
	if err != nil {
		return Metadata{}, err
	}
	defer clear(envelope)

	now := s.now().UTC()
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO secrets (id, name, kind, envelope, created_at, updated_at, username, public_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		name,
		kind,
		envelope,
		formatTime(now),
		formatTime(now),
		username,
		publicKey,
	)
	if err != nil {
		exists, inspectErr := s.secretExists(ctx, id)
		if inspectErr != nil {
			return Metadata{}, fmt.Errorf("inspect vault secret after create failure: %w", inspectErr)
		}
		if exists {
			return Metadata{}, ErrAlreadyExists
		}
		return Metadata{}, fmt.Errorf("create vault secret: %w", err)
	}
	if err := s.protectSidecarsAfterWrite(ctx); err != nil {
		return Metadata{}, fmt.Errorf("protect vault database sidecars: %w", err)
	}
	return Metadata{
		ID:        id,
		Name:      name,
		Kind:      kind,
		Username:  username,
		PublicKey: publicKey,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *Store) Replace(
	ctx context.Context,
	id string,
	kind Kind,
	value []byte,
) (Metadata, error) {
	return s.ReplaceWithMetadata(ctx, id, "", "", kind, value)
}

func (s *Store) ReplaceWithMetadata(
	ctx context.Context,
	id string,
	username string,
	publicKey string,
	kind Kind,
	value []byte,
) (Metadata, error) {
	if err := contextError(ctx); err != nil {
		return Metadata{}, err
	}
	if err := s.ready(); err != nil {
		return Metadata{}, err
	}
	if err := validateMetadata(id, kind); err != nil {
		return Metadata{}, err
	}

	var (
		metadata  Metadata
		createdAt string
	)
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, kind, username, public_key, created_at FROM secrets WHERE id = ?`,
		id,
	).Scan(&metadata.ID, &metadata.Name, &metadata.Kind, &metadata.Username, &metadata.PublicKey, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Metadata{}, ErrNotFound
	}
	if err != nil {
		return Metadata{}, fmt.Errorf("read vault secret before replacement: %w", err)
	}
	if metadata.Kind != kind {
		return Metadata{}, ErrKindMismatch
	}
	metadata.Username = username
	metadata.PublicKey = publicKey
	metadata.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Metadata{}, fmt.Errorf("parse replaced vault secret creation time: %w", err)
	}

	envelope, err := s.cipher.Seal(id, kind, value)
	if err != nil {
		return Metadata{}, err
	}
	defer clear(envelope)
	metadata.UpdatedAt = s.now().UTC()
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE secrets SET envelope = ?, username = ?, public_key = ?, updated_at = ? WHERE id = ? AND kind = ?`,
		envelope,
		username,
		publicKey,
		formatTime(metadata.UpdatedAt),
		id,
		kind,
	)
	if err != nil {
		return Metadata{}, fmt.Errorf("replace vault secret: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Metadata{}, fmt.Errorf("read replaced vault secret count: %w", err)
	}
	if count == 0 {
		return Metadata{}, ErrNotFound
	}
	if err := s.protectSidecarsAfterWrite(ctx); err != nil {
		return Metadata{}, fmt.Errorf("protect vault database sidecars: %w", err)
	}
	return metadata, nil
}

type Metadata struct {
	ID        string
	Name      string
	Kind      Kind
	Username  string
	PublicKey string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Store struct {
	db     *sql.DB
	cipher *Cipher
	now    func() time.Time
	path   string
}

func Open(ctx context.Context, path string, cipher *Cipher) (*Store, error) {
	return open(ctx, path, cipher, true)
}

func open(ctx context.Context, path string, cipher *Cipher, allowCreate bool) (*Store, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	cleanPath, err := validateVaultDatabasePath(path)
	if err != nil {
		return nil, err
	}
	path = cleanPath
	if cipher == nil || cipher.aead == nil {
		return nil, ErrNotReady
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, ErrInvalidVaultDatabase
		}
	} else {
		if errors.Is(err, fs.ErrNotExist) {
			if err := validateVaultDatabaseSidecars(path); err != nil {
				return nil, err
			}
			if !allowCreate {
				return nil, ErrRecoveryRequired
			}
			artifactsExist, inspectErr := vaultArtifactsExist(path)
			if inspectErr != nil {
				return nil, fmt.Errorf("inspect vault artifacts before create: %w", inspectErr)
			}
			if artifactsExist {
				return nil, ErrRecoveryRequired
			}
		} else {
			return nil, fmt.Errorf("inspect vault database: %w", err)
		}
	}
	if err := validateVaultDatabaseSidecars(path); err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create vault directory: %w", err)
	}
	if err := protectLocalPath(dir, true); err != nil {
		return nil, fmt.Errorf("protect vault directory: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			return nil, fmt.Errorf("restrict vault directory permissions: %w", err)
		}
	}

	mode := ""
	if !allowCreate {
		mode = "rw"
	}
	dsn := sqlitedsn.Build(path, sqlitedsn.Options{Mode: mode})
	connector, err := sqlite.NewConnector(dsn)
	if err != nil {
		return nil, fmt.Errorf("create vault SQLite connector: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, cipher: cipher, now: time.Now, path: filepath.Clean(path)}
	opened := false
	defer func() {
		if !opened {
			_ = db.Close()
		}
	}()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connect vault database: %w", err)
	}
	if err := protectOpenedVaultFile(path); err != nil {
		return nil, fmt.Errorf("protect vault database: %w", err)
	}
	if err := protectVaultDatabaseSidecars(path); err != nil {
		return nil, fmt.Errorf("protect vault database sidecars: %w", err)
	}
	if err := runMigrations(ctx, store); err != nil {
		return nil, err
	}
	if err := store.protectSidecarsAfterWrite(ctx); err != nil {
		return nil, fmt.Errorf("protect vault database sidecars after migration: %w", err)
	}

	opened = true
	return store, nil
}

func protectOpenedVaultFile(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrInvalidVaultDatabase
	}
	if err != nil {
		return fmt.Errorf("inspect opened vault file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return ErrInvalidVaultDatabase
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open vault file for protection: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect opened vault file: %w", err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return ErrInvalidVaultDatabase
	}
	if runtime.GOOS != "windows" {
		if err := file.Chmod(0o600); err != nil {
			return fmt.Errorf("restrict vault file permissions: %w", err)
		}
		return nil
	}
	if err := protectLocalPath(path, false); err != nil {
		return err
	}
	finalInfo, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect protected vault file: %w", err)
	}
	if !finalInfo.Mode().IsRegular() || !os.SameFile(info, finalInfo) {
		return ErrInvalidVaultDatabase
	}
	return nil
}

func (s *Store) protectSidecarsAfterWrite(ctx context.Context) error {
	if s == nil || s.path == "" {
		return ErrStoreNotReady
	}
	protectErr := protectVaultDatabaseSidecars(s.path)
	if contextErr := contextError(ctx); contextErr != nil {
		return errors.Join(protectErr, contextErr)
	}
	return protectErr
}

func protectVaultDatabaseSidecars(path string) error {
	if path == "" {
		return nil
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		candidate := path + suffix
		info, err := os.Lstat(candidate)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect vault database sidecar: %w", err)
		}
		if !info.Mode().IsRegular() {
			return ErrInvalidVaultDatabase
		}
		if err := protectOpenedVaultFile(candidate); err != nil {
			if _, statErr := os.Lstat(candidate); errors.Is(statErr, fs.ErrNotExist) {
				continue
			}
			return err
		}
	}
	return nil
}

func validateVaultDatabaseSidecars(path string) error {
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		info, err := os.Lstat(path + suffix)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect vault database sidecar: %w", err)
		}
		if !info.Mode().IsRegular() {
			return ErrInvalidVaultDatabase
		}
	}
	return nil
}

func validateVaultDatabasePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("vault database path is empty")
	}
	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("vault database path must be absolute: %w", ErrInvalidVaultDatabasePath)
	}
	dir := filepath.Dir(cleanPath)
	volumeRoot := filepath.Clean(filepath.VolumeName(dir) + string(os.PathSeparator))
	if cleanPath == dir || filepath.Clean(dir) == volumeRoot {
		return "", fmt.Errorf("vault database path cannot be in a filesystem root: %w", ErrInvalidVaultDatabasePath)
	}
	return cleanPath, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	protectErr := protectVaultDatabaseSidecars(s.path)
	err := s.db.Close()
	s.db = nil
	if err != nil {
		err = fmt.Errorf("close vault database: %w", err)
	}
	if protectErr != nil {
		return errors.Join(err, fmt.Errorf("protect vault database sidecars: %w", protectErr))
	}
	return err
}

func (s *Store) Save(
	ctx context.Context,
	id string,
	name string,
	kind Kind,
	value []byte,
) (Metadata, error) {
	if err := contextError(ctx); err != nil {
		return Metadata{}, err
	}
	if err := s.ready(); err != nil {
		return Metadata{}, err
	}
	if err := validateMetadata(id, kind); err != nil {
		return Metadata{}, err
	}
	name, err := normalizeSecretName(name)
	if err != nil {
		return Metadata{}, err
	}
	envelope, err := s.cipher.Seal(id, kind, value)
	if err != nil {
		return Metadata{}, err
	}
	defer clear(envelope)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Metadata{}, fmt.Errorf("begin save vault secret: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	now := s.now().UTC()
	metadata := Metadata{
		ID:        id,
		Name:      name,
		Kind:      kind,
		CreatedAt: now,
		UpdatedAt: now,
	}
	var (
		existingKind      Kind
		existingCreatedAt string
	)
	err = tx.QueryRowContext(ctx, `SELECT kind, created_at FROM secrets WHERE id = ?`, id).Scan(
		&existingKind,
		&existingCreatedAt,
	)
	switch {
	case err == nil:
		if existingKind != kind {
			return Metadata{}, ErrKindMismatch
		}
		metadata.CreatedAt, err = parseTime(existingCreatedAt)
		if err != nil {
			return Metadata{}, fmt.Errorf("parse vault secret creation time: %w", err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE secrets SET name = ?, envelope = ?, updated_at = ? WHERE id = ?`,
			name,
			envelope,
			formatTime(now),
			id,
		); err != nil {
			return Metadata{}, fmt.Errorf("update vault secret: %w", err)
		}
	case errors.Is(err, sql.ErrNoRows):
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO secrets (id, name, kind, envelope, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			id,
			name,
			kind,
			envelope,
			formatTime(now),
			formatTime(now),
		); err != nil {
			return Metadata{}, fmt.Errorf("insert vault secret: %w", err)
		}
	default:
		return Metadata{}, fmt.Errorf("read vault secret before save: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Metadata{}, fmt.Errorf("commit vault secret: %w", err)
	}
	if err := s.protectSidecarsAfterWrite(ctx); err != nil {
		return Metadata{}, fmt.Errorf("protect vault database sidecars: %w", err)
	}
	return metadata, nil
}

func (s *Store) Rename(ctx context.Context, id, name string) (Metadata, error) {
	if err := contextError(ctx); err != nil {
		return Metadata{}, err
	}
	if err := s.ready(); err != nil {
		return Metadata{}, err
	}
	if id == "" || len(id) > maxRecordIDSize {
		return Metadata{}, ErrInvalidRecord
	}
	name, err := normalizeSecretName(name)
	if err != nil {
		return Metadata{}, err
	}

	now := s.now().UTC()
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE secrets SET name = ?, updated_at = ? WHERE id = ?`,
		name,
		formatTime(now),
		id,
	)
	if err != nil {
		return Metadata{}, fmt.Errorf("rename vault secret: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Metadata{}, fmt.Errorf("read renamed vault secret count: %w", err)
	}
	if count == 0 {
		return Metadata{}, ErrNotFound
	}
	if err := s.protectSidecarsAfterWrite(ctx); err != nil {
		return Metadata{}, fmt.Errorf("protect vault database sidecars: %w", err)
	}

	var (
		metadata  Metadata
		createdAt string
		updatedAt string
	)
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, kind, username, public_key, created_at, updated_at FROM secrets WHERE id = ?`,
		id,
	).Scan(&metadata.ID, &metadata.Name, &metadata.Kind, &metadata.Username, &metadata.PublicKey, &createdAt, &updatedAt); err != nil {
		return Metadata{}, fmt.Errorf("read renamed vault secret: %w", err)
	}
	metadata.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Metadata{}, fmt.Errorf("parse renamed vault secret creation time: %w", err)
	}
	metadata.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Metadata{}, fmt.Errorf("parse renamed vault secret update time: %w", err)
	}
	return metadata, nil
}

func (s *Store) Read(ctx context.Context, id string) (Secret, error) {
	if err := contextError(ctx); err != nil {
		return Secret{}, err
	}
	if err := s.ready(); err != nil {
		return Secret{}, err
	}
	if id == "" || len(id) > maxRecordIDSize {
		return Secret{}, ErrInvalidRecord
	}

	var (
		secret               Secret
		envelope             []byte
		createdAt, updatedAt string
	)
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, kind, username, public_key, envelope, created_at, updated_at FROM secrets WHERE id = ?`,
		id,
	).Scan(&secret.ID, &secret.Name, &secret.Kind, &secret.Username, &secret.PublicKey, &envelope, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Secret{}, ErrNotFound
	}
	if err != nil {
		return Secret{}, fmt.Errorf("read vault secret: %w", err)
	}
	defer clear(envelope)
	secret.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Secret{}, fmt.Errorf("parse vault secret creation time: %w", err)
	}
	secret.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Secret{}, fmt.Errorf("parse vault secret update time: %w", err)
	}
	secret.Value, err = s.cipher.Open(secret.ID, secret.Kind, envelope)
	if err != nil {
		return Secret{}, err
	}
	if err := contextError(ctx); err != nil {
		clear(secret.Value)
		return Secret{}, err
	}
	return secret, nil
}

func (s *Store) List(ctx context.Context) ([]Metadata, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, name, kind, username, public_key, created_at, updated_at FROM secrets ORDER BY created_at, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("query vault secret metadata: %w", err)
	}
	defer rows.Close()

	metadata := make([]Metadata, 0)
	for rows.Next() {
		var (
			item                 Metadata
			createdAt, updatedAt string
		)
		if err := rows.Scan(&item.ID, &item.Name, &item.Kind, &item.Username, &item.PublicKey, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan vault secret metadata: %w", err)
		}
		item.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse vault metadata creation time: %w", err)
		}
		item.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse vault metadata update time: %w", err)
		}
		metadata = append(metadata, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate vault secret metadata: %w", err)
	}
	return metadata, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := s.ready(); err != nil {
		return err
	}
	if id == "" || len(id) > maxRecordIDSize {
		return ErrInvalidRecord
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM secrets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete vault secret: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted vault secret count: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	if err := s.protectSidecarsAfterWrite(ctx); err != nil {
		return fmt.Errorf("protect vault database sidecars: %w", err)
	}
	return nil
}

func (s *Store) ready() error {
	if s == nil || s.db == nil || s.cipher == nil || s.cipher.aead == nil || s.now == nil {
		return ErrStoreNotReady
	}
	return nil
}

func (s *Store) secretExists(ctx context.Context, id string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(
		ctx,
		`SELECT 1 FROM secrets WHERE id = ?`,
		id,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func normalizeSecretName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxSecretNameSize || !utf8.ValidString(name) || strings.IndexByte(name, 0) >= 0 {
		return "", ErrInvalidRecord
	}
	return name, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}
