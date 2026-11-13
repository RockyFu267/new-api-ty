// Command migrate-sqlite-postgres copies an immutable SQLite snapshot into an
// empty PostgreSQL database. It never truncates a destination or edits source.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type column struct {
	Name, SQLiteType, PGType string
	Primary                  int
}
type tableReport struct {
	Name   string `json:"name"`
	Rows   int64  `json:"rows"`
	SHA256 string `json:"sha256"`
}
type migrationReport struct {
	Database        string        `json:"database"`
	SourceSHA256    string        `json:"source_sha256"`
	SourceUnchanged bool          `json:"source_unchanged"`
	Tables          []tableReport `json:"tables"`
	CompletedAt     string        `json:"completed_at"`
}

// Retain archived tables even though the current gateway no longer uses them.
// GORM owns primary-key generation, just as it does for active application tables.
type archivedKnowledgeBase struct {
	Id            int    `gorm:"primaryKey"`
	Slug          string `gorm:"type:varchar(100);not null"`
	Name          string `gorm:"type:varchar(200);not null"`
	Description   string `gorm:"type:text;not null"`
	SourceName    string `gorm:"type:varchar(255);not null"`
	Content       string `gorm:"type:text;not null"`
	Enabled       bool   `gorm:"not null"`
	AllowDownload bool   `gorm:"not null"`
	CreatedAt     *time.Time
	UpdatedAt     *time.Time
}
type archivedKnowledgeBaseBinding struct {
	Id              int    `gorm:"primaryKey"`
	KnowledgeBaseId int    `gorm:"not null"`
	ChannelId       int    `gorm:"not null"`
	Model           string `gorm:"type:varchar(191);not null"`
	CreatedAt       *time.Time
}

func main() {
	source := flag.String("source", "", "immutable SQLite snapshot file")
	confirm := flag.String("confirm-database", "", "exact destination database name; destination must be empty")
	reportPath := flag.String("report", "", "verification report output path")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	report, err := migrate(ctx, *source, os.Getenv("MIGRATION_POSTGRES_DSN"), *confirm)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Migration aborted:", err)
		os.Exit(1)
	}
	data, err := common.Marshal(report)
	if err != nil {
		panic(err)
	}
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, data, 0o600); err != nil {
			panic(err)
		}
	}
	fmt.Println(string(data))
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func migrate(ctx context.Context, sourcePath, dsn, confirm string) (*migrationReport, error) {
	endpoint, err := url.Parse(dsn)
	if err != nil || (endpoint.Scheme != "postgres" && endpoint.Scheme != "postgresql") || confirm == "" || strings.TrimPrefix(endpoint.Path, "/") != confirm || sourcePath == "" {
		return nil, fmt.Errorf("provide source, MIGRATION_POSTGRES_DSN and matching --confirm-database")
	}
	sourcePath, err = filepath.Abs(sourcePath)
	if err != nil {
		return nil, err
	}
	if wal, err := os.Stat(sourcePath + "-wal"); err == nil && wal.Size() > 0 {
		return nil, fmt.Errorf("source has a nonempty WAL; supply a closed online-backup snapshot")
	}
	before, err := fileHash(sourcePath)
	if err != nil {
		return nil, err
	}
	source, err := sql.Open("sqlite", "file:"+filepath.ToSlash(sourcePath)+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return nil, err
	}
	defer source.Close()
	source.SetMaxOpenConns(1)
	var integrity string
	if err = source.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return nil, fmt.Errorf("source integrity check failed: %v", err)
	}
	target, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}
	defer target.Close(ctx)
	var schema string
	if err = target.QueryRow(ctx, "SELECT current_schema()").Scan(&schema); err != nil {
		return nil, err
	}
	if schema != "public" {
		return nil, fmt.Errorf("destination must use public schema")
	}
	var otherTables int
	if err = target.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname NOT IN ('public','pg_catalog','information_schema') AND schemaname NOT LIKE 'pg_toast%'").Scan(&otherTables); err != nil {
		return nil, err
	}
	if otherTables > 0 {
		return nil, fmt.Errorf("destination contains application tables outside public; use a dedicated empty database")
	}
	var locked bool
	if err = target.QueryRow(ctx, "SELECT pg_try_advisory_lock(179445,53000)").Scan(&locked); err != nil {
		return nil, err
	}
	if !locked {
		return nil, fmt.Errorf("another migration owns this destination")
	}
	if _, err = target.Exec(ctx, "SET lock_timeout='5s'"); err != nil {
		return nil, err
	}
	rows, err := target.Query(ctx, "SELECT tablename FROM pg_tables WHERE schemaname='public'")
	if err != nil {
		return nil, err
	}
	var existing []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		existing = append(existing, name)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	for _, name := range existing {
		var count int64
		if err = target.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{name}.Sanitize()).Scan(&count); err != nil {
			return nil, err
		}
		if count != 0 {
			return nil, fmt.Errorf("destination table %s is nonempty; no data changed", name)
		}
	}
	// Use the application's own current schema, without starting relay services,
	// loading upstream credentials or running background jobs.
	if err = os.Setenv("SQL_DSN", dsn); err != nil {
		return nil, err
	}
	os.Unsetenv("LOG_SQL_DSN")
	common.IsMasterNode = true
	if err = model.InitDB(); err != nil {
		return nil, fmt.Errorf("prepare application schema: %w", err)
	}
	pool, err := model.DB.DB()
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	if err = model.DB.AutoMigrate(&model.UsageAnalysisDaily{}, &model.UsageAnalysisEvent{}, &model.UsageAnalysisState{}, &model.UsageAnalysisPrice{}); err != nil {
		return nil, err
	}
	for table, schema := range map[string]any{"knowledge_bases": &archivedKnowledgeBase{}, "knowledge_base_bindings": &archivedKnowledgeBaseBinding{}} {
		var exists int
		if err := source.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists); err != nil {
			return nil, err
		}
		if exists > 0 {
			if err := model.DB.Table(table).AutoMigrate(schema); err != nil {
				return nil, err
			}
			if table == "knowledge_bases" {
				if err := model.DB.Exec(`ALTER TABLE "knowledge_bases" ALTER COLUMN "enabled" SET DEFAULT true, ALTER COLUMN "allow_download" SET DEFAULT false`).Error; err != nil {
					return nil, err
				}
			}
		}
	}
	namesRows, err := source.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	var names []string
	for namesRows.Next() {
		var name string
		if err = namesRows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	namesRows.Close()
	tx, err := target.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// The destination must stay offline. Locks also prevent an accidental writer
	// from changing any already-created application table during this copy.
	allRows, err := tx.Query(ctx, "SELECT tablename FROM pg_tables WHERE schemaname='public'")
	if err != nil {
		return nil, err
	}
	var allTables []string
	for allRows.Next() {
		var name string
		if err = allRows.Scan(&name); err != nil {
			return nil, err
		}
		allTables = append(allTables, pgx.Identifier{name}.Sanitize())
	}
	allRows.Close()
	if err = allRows.Err(); err != nil {
		return nil, err
	}
	if len(allTables) > 0 {
		if _, err = tx.Exec(ctx, "LOCK TABLE "+strings.Join(allTables, ",")+" IN ACCESS EXCLUSIVE MODE"); err != nil {
			return nil, err
		}
		for _, table := range allTables {
			var count int64
			if err = tx.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
				return nil, err
			}
			if count > 0 {
				return nil, fmt.Errorf("destination changed during schema preparation; refusing import")
			}
		}
	}
	report := &migrationReport{Database: confirm, SourceSHA256: before, Tables: []tableReport{}}
	for _, name := range names {
		// Silent loss of SQLite-only constraints or triggers is unacceptable.
		foreign, err := source.QueryContext(ctx, "PRAGMA foreign_key_list("+pgx.Identifier{name}.Sanitize()+")")
		if err != nil {
			return nil, err
		}
		hasForeign := foreign.Next()
		foreignErr := foreign.Err()
		foreign.Close()
		if foreignErr != nil {
			return nil, foreignErr
		}
		if hasForeign {
			return nil, fmt.Errorf("table %s has foreign keys requiring an explicit migration", name)
		}
		var triggerCount int
		if err = source.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='trigger' AND tbl_name=?", name).Scan(&triggerCount); err != nil {
			return nil, err
		}
		if triggerCount > 0 {
			return nil, fmt.Errorf("table %s has triggers requiring an explicit migration", name)
		}
		columns, err := sqliteColumns(ctx, source, name)
		if err != nil {
			return nil, err
		}
		if err = ensureColumns(ctx, tx, name, columns); err != nil {
			return nil, fmt.Errorf("schema %s: %w", name, err)
		}
		if err = preserveIndexes(ctx, source, tx, name); err != nil {
			return nil, fmt.Errorf("indexes %s: %w", name, err)
		}
		fields := make([]string, len(columns))
		quoted := make([]string, len(columns))
		for i, c := range columns {
			fields[i] = c.Name
			quoted[i] = pgx.Identifier{c.Name}.Sanitize()
		}
		from, err := source.QueryContext(ctx, "SELECT "+strings.Join(quoted, ",")+" FROM "+pgx.Identifier{name}.Sanitize())
		if err != nil {
			return nil, err
		}
		copySource := &copyRows{rows: from, columns: columns}
		count, err := tx.CopyFrom(ctx, pgx.Identifier{name}, fields, copySource)
		from.Close()
		if err != nil {
			return nil, fmt.Errorf("copy %s: %w", name, err)
		}
		if copySource.err != nil {
			return nil, copySource.err
		}
		verificationFields := append([]string(nil), quoted...)
		for i, c := range columns {
			if c.PGType == "json" || c.PGType == "jsonb" {
				verificationFields[i] += "::text"
			}
		}
		to, err := tx.Query(ctx, "SELECT "+strings.Join(verificationFields, ",")+" FROM "+pgx.Identifier{name}.Sanitize())
		if err != nil {
			return nil, err
		}
		var hashes []string
		for to.Next() {
			values, e := to.Values()
			if e != nil {
				return nil, e
			}
			h, e := rowHash(values, columns)
			if e != nil {
				return nil, e
			}
			hashes = append(hashes, h)
		}
		to.Close()
		if to.Err() != nil {
			return nil, to.Err()
		}
		sourceHash := setHash(copySource.hashes)
		if int64(len(hashes)) != count || sourceHash != setHash(hashes) {
			return nil, fmt.Errorf("data verification failed for %s; transaction rolled back", name)
		}
		for _, c := range columns {
			var sequence *string
			if err = tx.QueryRow(ctx, "SELECT pg_get_serial_sequence($1,$2)", pgx.Identifier{name}.Sanitize(), c.Name).Scan(&sequence); err != nil {
				return nil, err
			}
			if sequence == nil {
				continue
			}
			var high int64
			if err = tx.QueryRow(ctx, "SELECT COALESCE(MAX("+pgx.Identifier{c.Name}.Sanitize()+"),0) FROM "+pgx.Identifier{name}.Sanitize()).Scan(&high); err != nil {
				return nil, err
			}
			var oldHigh int64
			// sqlite_sequence can be absent for a snapshot without AUTOINCREMENT.
			if e := source.QueryRowContext(ctx, "SELECT seq FROM sqlite_sequence WHERE name=?", name).Scan(&oldHigh); e == nil && oldHigh > high {
				high = oldHigh
			}
			if _, err = tx.Exec(ctx, "SELECT setval($1::regclass,$2,$3)", *sequence, max(high, 1), high > 0); err != nil {
				return nil, err
			}
		}
		report.Tables = append(report.Tables, tableReport{Name: name, Rows: count, SHA256: sourceHash})
		fmt.Fprintf(os.Stderr, "Verified %s: %d rows\n", name, count)
	}
	after, err := fileHash(sourcePath)
	if err != nil {
		return nil, err
	}
	if before != after {
		return nil, fmt.Errorf("source snapshot changed; destination transaction rolled back")
	}
	if wal, err := os.Stat(sourcePath + "-wal"); err == nil && wal.Size() > 0 {
		return nil, fmt.Errorf("source acquired WAL writes; destination transaction rolled back")
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	report.SourceUnchanged = true
	report.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	return report, nil
}

func sqliteColumns(ctx context.Context, db *sql.DB, table string) ([]column, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+pgx.Identifier{table}.Sanitize()+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []column
	for rows.Next() {
		var index, notnull, pk int
		var name, kind string
		var defaultValue any
		if err = rows.Scan(&index, &name, &kind, &notnull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		out = append(out, column{Name: name, SQLiteType: kind, Primary: pk})
	}
	return out, rows.Err()
}

func ensureColumns(ctx context.Context, tx pgx.Tx, table string, columns []column) error {
	rows, err := tx.Query(ctx, "SELECT column_name,udt_name FROM information_schema.columns WHERE table_schema='public' AND table_name=$1", table)
	if err != nil {
		return err
	}
	known := map[string]string{}
	for rows.Next() {
		var name, kind string
		if err = rows.Scan(&name, &kind); err != nil {
			return err
		}
		known[name] = kind
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if len(known) == 0 {
		return fmt.Errorf("unrecognized source table; add an explicit schema instead of guessing column semantics")
	}
	for i := range columns {
		kind, ok := known[columns[i].Name]
		if !ok {
			return fmt.Errorf("source column %s has no destination; refusing to omit data", columns[i].Name)
		}
		columns[i].PGType = kind
	}
	return nil
}

// Preserve source uniqueness and lookup indexes, including the archived tables.
// Refuse expressions/collations that need a deliberate cross-dialect translation.
func preserveIndexes(ctx context.Context, source *sql.DB, tx pgx.Tx, table string) error {
	type index struct {
		name, origin    string
		unique, partial int
	}
	rows, err := source.QueryContext(ctx, "PRAGMA index_list("+pgx.Identifier{table}.Sanitize()+")")
	if err != nil {
		return err
	}
	var indexes []index
	for rows.Next() {
		var sequence int
		var item index
		if err = rows.Scan(&sequence, &item.name, &item.unique, &item.origin, &item.partial); err != nil {
			rows.Close()
			return err
		}
		if item.origin != "pk" {
			indexes = append(indexes, item)
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, item := range indexes {
		info, err := source.QueryContext(ctx, "PRAGMA index_xinfo("+pgx.Identifier{item.name}.Sanitize()+")")
		if err != nil {
			return err
		}
		var fields []string
		for info.Next() {
			var seq, cid, desc, key int
			var name, collation sql.NullString
			if err = info.Scan(&seq, &cid, &name, &desc, &collation, &key); err != nil {
				info.Close()
				return err
			}
			if key == 0 {
				continue
			}
			if cid < 0 || !name.Valid || (collation.Valid && !strings.EqualFold(collation.String, "BINARY")) {
				info.Close()
				return fmt.Errorf("unsupported expression/collation in %s", item.name)
			}
			field := pgx.Identifier{name.String}.Sanitize()
			if desc != 0 {
				field += " DESC"
			}
			fields = append(fields, field)
		}
		info.Close()
		if err = info.Err(); err != nil {
			return err
		}
		predicate := ""
		if item.partial != 0 {
			var statement string
			if err = source.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='index' AND name=?", item.name).Scan(&statement); err != nil {
				return err
			}
			where := strings.LastIndex(strings.ToUpper(statement), " WHERE ")
			if where < 0 {
				return fmt.Errorf("missing partial index predicate")
			}
			predicate = strings.TrimSpace(statement[where+7:])
			if !regexp.MustCompile("(?i)^[`\"]?[a-z_][a-z0-9_]*[`\"]? IS (NOT )?NULL$").MatchString(predicate) {
				return fmt.Errorf("unsupported partial index predicate in %s", item.name)
			}
			predicate = strings.ReplaceAll(predicate, "`", `"`)
		}
		// GORM may already have created the same index under a different name.
		known, err := tx.Query(ctx, `SELECT i.indisunique, ARRAY(SELECT pg_get_indexdef(i.indexrelid,k,true) FROM generate_series(1,i.indnatts) k), COALESCE(pg_get_expr(i.indpred,i.indrelid),'') FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=$1`, table)
		if err != nil {
			return err
		}
		found := false
		for known.Next() {
			var unique bool
			var cols []string
			var filter string
			if err = known.Scan(&unique, &cols, &filter); err != nil {
				known.Close()
				return err
			}
			if unique == (item.unique != 0) && normalizedIndexSQL(strings.Join(cols, ",")) == normalizedIndexSQL(strings.Join(fields, ",")) && normalizedIndexSQL(filter) == normalizedIndexSQL(predicate) {
				found = true
			}
		}
		known.Close()
		if err = known.Err(); err != nil {
			return err
		}
		if found {
			continue
		}
		// Stable short name avoids SQLite autoindex names and PostgreSQL's 63-byte
		// identifier truncation. The source name remains part of the hash.
		name := fmt.Sprintf("sqlite_import_%x", sha256.Sum256([]byte(table+"/"+item.name)))[:54]
		unique := ""
		if item.unique != 0 {
			unique = "UNIQUE "
		}
		statement := "CREATE " + unique + "INDEX " + pgx.Identifier{name}.Sanitize() + " ON " + pgx.Identifier{table}.Sanitize() + " (" + strings.Join(fields, ",") + ")"
		if predicate != "" {
			statement += " WHERE " + predicate
		}
		if _, err = tx.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func normalizedIndexSQL(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.NewReplacer(`"`, "", "(", "", ")", "").Replace(value)), ""))
}

type copyRows struct {
	rows    *sql.Rows
	columns []column
	values  []any
	hashes  []string
	err     error
}

func (c *copyRows) Next() bool {
	if c.err != nil || !c.rows.Next() {
		return false
	}
	c.values = make([]any, len(c.columns))
	pointers := make([]any, len(c.columns))
	for i := range pointers {
		pointers[i] = &c.values[i]
	}
	if c.err = c.rows.Scan(pointers...); c.err != nil {
		return false
	}
	for i := range c.values {
		c.values[i], c.err = convertValue(c.values[i], c.columns[i].PGType)
		if c.err != nil {
			return false
		}
	}
	h, err := rowHash(c.values, c.columns)
	if err != nil {
		c.err = err
		return false
	}
	c.hashes = append(c.hashes, h)
	return true
}
func (c *copyRows) Values() ([]any, error) { return c.values, c.err }
func (c *copyRows) Err() error {
	if c.err != nil {
		return c.err
	}
	return c.rows.Err()
}

func convertValue(v any, kind string) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch kind {
	case "bool":
		switch n := v.(type) {
		case bool:
			return n, nil
		case int64:
			if n == 0 || n == 1 {
				return n == 1, nil
			}
		}
		return nil, fmt.Errorf("invalid boolean source value")
	case "int2", "int4", "int8":
		if n, ok := v.(int64); ok {
			return n, nil
		}
		n, err := strconv.ParseInt(fmt.Sprint(v), 10, 64)
		return n, err
	case "float4", "float8":
		if n, ok := v.(float64); ok {
			return n, nil
		}
		n, err := strconv.ParseFloat(fmt.Sprint(v), 64)
		return n, err
	case "numeric":
		var n pgtype.Numeric
		err := n.Scan(fmt.Sprint(v))
		return n, err
	case "timestamp", "timestamptz":
		if t, ok := v.(time.Time); ok {
			return t.UTC().Truncate(time.Microsecond), nil
		}
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05.999999999", "2006-01-02"} {
			if t, err := time.Parse(layout, fmt.Sprint(v)); err == nil {
				return t.UTC().Truncate(time.Microsecond), nil
			}
		}
		return nil, fmt.Errorf("unrecognized source timestamp")
	case "bytea":
		if b, ok := v.([]byte); ok {
			return b, nil
		}
		return []byte(fmt.Sprint(v)), nil
	default:
		if b, ok := v.([]byte); ok {
			return string(b), nil
		}
		return fmt.Sprint(v), nil
	}
}

func rowHash(values []any, columns []column) (string, error) {
	normalized := make([]string, len(values))
	for i, v := range values {
		if v == nil {
			normalized[i] = "null"
			continue
		}
		if columns[i].PGType == "int2" || columns[i].PGType == "int4" || columns[i].PGType == "int8" {
			normalized[i] = "integer:" + fmt.Sprint(v)
			continue
		}
		if columns[i].PGType == "numeric" {
			var s string
			if n, ok := v.(pgtype.Numeric); ok {
				value, err := n.Value()
				if err != nil {
					return "", err
				}
				s = fmt.Sprint(value)
			} else {
				s = fmt.Sprint(v)
			}
			r, ok := new(big.Rat).SetString(s)
			if !ok {
				return "", fmt.Errorf("invalid numeric value")
			}
			normalized[i] = "number:" + r.RatString()
		} else if t, ok := v.(time.Time); ok {
			normalized[i] = "time:" + t.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
		} else if b, ok := v.([]byte); ok {
			normalized[i] = fmt.Sprintf("bytes:%x", b)
		} else {
			normalized[i] = fmt.Sprintf("%T:%v", v, v)
		}
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
func setHash(hashes []string) string {
	sort.Strings(hashes)
	h := sha256.New()
	for _, s := range hashes {
		io.WriteString(h, s)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
