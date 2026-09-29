package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"bmj-backend/migrations"

	"github.com/jackc/pgx/v5"
)

var fileNamePattern = regexp.MustCompile(`^([0-9]{6})_([a-z0-9_]+)\.(up|down)\.sql$`)

const schemaMigrationsSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

// Migration is one ordered SQL change with an up and down script.
type Migration struct {
	Version int64
	Name    string
	UpSQL   string
	DownSQL string
}

type appliedMigration struct {
	Version int64
	Name    string
}

// Load reads migration pairs from fsys.
// Versions must be unique, ordered, and include both up and down scripts.
func Load(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	type pair struct {
		version int64
		name    string
		up      string
		down    string
		hasUp   bool
		hasDown bool
	}

	byVersion := make(map[int64]*pair)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		match := fileNamePattern.FindStringSubmatch(name)
		if match == nil {
			return nil, fmt.Errorf("invalid migration filename %s", name)
		}

		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid migration version in %s", name)
		}
		if version <= 0 {
			return nil, fmt.Errorf("migration version must be positive in %s", name)
		}

		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		sql := strings.TrimSpace(string(body))
		if sql == "" {
			return nil, fmt.Errorf("migration %s is empty", name)
		}

		item := byVersion[version]
		if item == nil {
			item = &pair{version: version, name: match[2]}
			byVersion[version] = item
		}
		if item.name != match[2] {
			return nil, fmt.Errorf("migration %06d has conflicting names", version)
		}

		switch match[3] {
		case "up":
			if item.hasUp {
				return nil, fmt.Errorf("duplicate up migration for version %06d", version)
			}
			item.up = sql
			item.hasUp = true
		case "down":
			if item.hasDown {
				return nil, fmt.Errorf("duplicate down migration for version %06d", version)
			}
			item.down = sql
			item.hasDown = true
		}
	}

	if len(byVersion) == 0 {
		return nil, fmt.Errorf("no migrations found")
	}

	versions := make([]int64, 0, len(byVersion))
	for version := range byVersion {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i] < versions[j]
	})

	loaded := make([]Migration, 0, len(versions))
	for _, version := range versions {
		item := byVersion[version]
		if !item.hasUp || !item.hasDown {
			return nil, fmt.Errorf("migration %06d_%s requires both up and down files", item.version, item.name)
		}
		loaded = append(loaded, Migration{
			Version: item.version,
			Name:    item.name,
			UpSQL:   item.up,
			DownSQL: item.down,
		})
	}

	return loaded, nil
}

// Up applies pending migrations in order.
func Up(ctx context.Context, databaseURL string) ([]Migration, error) {
	loaded, err := Load(migrations.FS)
	if err != nil {
		return nil, err
	}

	conn, err := connect(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	if err := execSQL(ctx, conn, schemaMigrationsSQL); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", safeError(err))
	}

	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return nil, err
	}

	pending, err := pendingMigrations(loaded, applied)
	if err != nil {
		return nil, err
	}

	for _, migration := range pending {
		script := migration.UpSQL + "\n" + recordAppliedSQL(migration)
		if err := execSQL(ctx, conn, script); err != nil {
			return nil, wrapMigrationError("apply", migration, err)
		}
	}

	return pending, nil
}

// Down rolls back the most recently applied migration.
func Down(ctx context.Context, databaseURL string) (Migration, error) {
	loaded, err := Load(migrations.FS)
	if err != nil {
		return Migration{}, err
	}

	conn, err := connect(ctx, databaseURL)
	if err != nil {
		return Migration{}, err
	}
	defer conn.Close(ctx)

	if err := execSQL(ctx, conn, schemaMigrationsSQL); err != nil {
		return Migration{}, fmt.Errorf("create schema_migrations: %w", safeError(err))
	}

	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return Migration{}, err
	}
	if len(applied) == 0 {
		return Migration{}, fmt.Errorf("no applied migrations to roll back")
	}

	if _, err := pendingMigrations(loaded, applied); err != nil {
		return Migration{}, err
	}

	current := loaded[len(applied)-1]
	script := current.DownSQL + "\n" + fmt.Sprintf("DELETE FROM schema_migrations WHERE version = %d;", current.Version)
	if err := execSQL(ctx, conn, script); err != nil {
		return Migration{}, wrapMigrationError("roll back", current, err)
	}

	return current, nil
}

// Version returns the latest applied migration, or "none".
func Version(ctx context.Context, databaseURL string) (string, error) {
	conn, err := connect(ctx, databaseURL)
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)

	if err := execSQL(ctx, conn, schemaMigrationsSQL); err != nil {
		return "", fmt.Errorf("create schema_migrations: %w", safeError(err))
	}

	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return "", err
	}
	if len(applied) == 0 {
		return "none", nil
	}

	current := applied[len(applied)-1]
	return fmt.Sprintf("%06d_%s", current.Version, current.Name), nil
}

func pendingMigrations(all []Migration, applied []appliedMigration) ([]Migration, error) {
	if len(applied) > len(all) {
		return nil, fmt.Errorf("database has more applied migrations than local files")
	}

	for i, row := range applied {
		current := all[i]
		if current.Version != row.Version || current.Name != row.Name {
			return nil, fmt.Errorf("applied migration %06d_%s does not match local migration %06d_%s", row.Version, row.Name, current.Version, current.Name)
		}
	}

	return all[len(applied):], nil
}

func appliedVersions(ctx context.Context, conn *pgx.Conn) ([]appliedMigration, error) {
	rows, err := conn.Query(ctx, `SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", safeError(err))
	}
	defer rows.Close()

	var applied []appliedMigration
	for rows.Next() {
		var row appliedMigration
		if err := rows.Scan(&row.Version, &row.Name); err != nil {
			return nil, fmt.Errorf("read schema_migrations: %w", safeError(err))
		}
		applied = append(applied, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", safeError(err))
	}

	return applied, nil
}

func recordAppliedSQL(migration Migration) string {
	return fmt.Sprintf(
		"INSERT INTO schema_migrations (version, name) VALUES (%d, '%s');",
		migration.Version,
		migration.Name,
	)
}

func connect(ctx context.Context, databaseURL string) (*pgx.Conn, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, fmt.Errorf("database url is empty")
	}

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return nil, connectFailure(err)
	}

	return conn, nil
}

func execSQL(ctx context.Context, conn *pgx.Conn, sql string) error {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return fmt.Errorf("sql is empty")
	}

	if _, err := conn.PgConn().Exec(ctx, sql).ReadAll(); err != nil {
		return err
	}

	return nil
}

func wrapMigrationError(action string, migration Migration, err error) error {
	label := fmt.Sprintf("%06d_%s", migration.Version, migration.Name)
	if containsCredential(err) {
		return fmt.Errorf("%s migration %s failed", action, label)
	}
	return fmt.Errorf("%s migration %s: %w", action, label, err)
}

func safeError(err error) error {
	if err == nil || !containsCredential(err) {
		return err
	}
	return fmt.Errorf("database operation failed")
}

func connectFailure(err error) error {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "password authentication failed"), strings.Contains(message, "authentication failed"):
		return fmt.Errorf("database authentication failed")
	case strings.Contains(message, "cannot parse"), strings.Contains(message, "invalid dsn"), strings.Contains(message, "invalid url"):
		return fmt.Errorf("invalid database url")
	case strings.Contains(message, "no such host"):
		return fmt.Errorf("database host could not be resolved")
	case strings.Contains(message, "timeout"), strings.Contains(message, "connection refused"), strings.Contains(message, "network is unreachable"):
		return fmt.Errorf("database is unreachable")
	case strings.Contains(message, "ssl"), strings.Contains(message, "tls"):
		return fmt.Errorf("database tls handshake failed")
	default:
		return fmt.Errorf("connect to database")
	}
}

func containsCredential(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "postgres://") ||
		strings.Contains(message, "postgresql://") ||
		strings.Contains(message, "password=")
}
