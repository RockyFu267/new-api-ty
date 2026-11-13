package main

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMigrationValueConversionDoesNotRoundBalancesOrConfuseNull(t *testing.T) {
	value, err := convertValue(int64(9007199254740991), "int8")
	require.NoError(t, err)
	assert.Equal(t, int64(9007199254740991), value)
	_, err = convertValue(int64(2), "bool")
	require.Error(t, err)
	value, err = convertValue(int64(1), "bool")
	require.NoError(t, err)
	assert.Equal(t, true, value)
	value, err = convertValue(nil, "text")
	require.NoError(t, err)
	assert.Nil(t, value)
	stamp := time.Date(2026, 11, 12, 12, 1, 2, 123456789, time.FixedZone("UTC+8", 8*3600))
	value, err = convertValue(stamp, "timestamptz")
	require.NoError(t, err)
	assert.Equal(t, stamp.UTC().Truncate(time.Microsecond), value)
	a, err := rowHash([]any{nil}, []column{{PGType: "text"}})
	require.NoError(t, err)
	b, err := rowHash([]any{""}, []column{{PGType: "text"}})
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
	assert.Equal(t, setHash([]string{"a", "b"}), setHash([]string{"b", "a"}))
	_, err = migrate(context.Background(), "source.db", "postgres://127.0.0.1/wrong", "confirmed")
	require.ErrorContains(t, err, "matching --confirm-database")
}

func TestPostgreSQLMigrationVerifiesRollbackSequencesAndNonemptyProtection(t *testing.T) {
	dsn := os.Getenv("LOCAL_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set LOCAL_MIGRATION_TEST_DSN to a new empty loopback *_migration_test database")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, u.Hostname())
	require.True(t, strings.HasSuffix(u.Path, "_migration_test"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pg, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	var tables int
	require.NoError(t, pg.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname='public'").Scan(&tables))
	require.Zero(t, tables, "integration fixture must be a fresh database")
	t.Cleanup(func() {
		defer pg.Close(context.Background())
		_, err := pg.Exec(context.Background(), "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
		require.NoError(t, err)
	})
	oldDB, oldLog := model.DB, model.LOG_DB
	oldMaster := common.IsMasterNode
	oldMain, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	t.Setenv("SQL_DSN", os.Getenv("SQL_DSN"))
	t.Setenv("LOG_SQL_DSN", os.Getenv("LOG_SQL_DSN"))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLog
		common.IsMasterNode = oldMaster
		common.SetDatabaseTypes(oldMain, oldLogType)
	})
	path := filepath.Join(t.TempDir(), "snapshot.db")
	from, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, from.AutoMigrate(&model.Token{}, &model.Channel{}, &model.Option{}))
	require.NoError(t, from.Exec(`CREATE TABLE users(id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT UNIQUE, password TEXT, quota BIGINT, used_quota BIGINT, aff_code TEXT UNIQUE)`).Error)
	require.NoError(t, from.Table("knowledge_bases").AutoMigrate(&archivedKnowledgeBase{}))
	require.NoError(t, from.Exec("CREATE UNIQUE INDEX old_knowledge_slug ON knowledge_bases(slug)").Error)
	user := model.User{Id: 1, Username: "迁移用户", Password: "!test-only", Quota: 9007199254740991, UsedQuota: 7000000000, AffCode: "unique-aff"}
	require.NoError(t, from.Exec("INSERT INTO users(id,username,password,quota,used_quota,aff_code) VALUES(?,?,?,?,?,?)", user.Id, user.Username, user.Password, user.Quota, user.UsedQuota, user.AffCode).Error)
	require.NoError(t, from.Exec("INSERT INTO users(id,username,aff_code) VALUES(5000,'removed-user','removed-aff')").Error)
	require.NoError(t, from.Exec("DELETE FROM users WHERE id=5000").Error)
	require.NoError(t, from.Create(&model.Token{Id: 3, UserId: 1, Key: "migration-only-key", UnlimitedQuota: true, RemainQuota: 9007199254740991}).Error)
	require.NoError(t, from.Create(&model.Channel{Id: 7, Key: "disabled-test-key", ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeyStatusList: map[int]int{0: 1}}}).Error)
	require.NoError(t, from.Create(&model.Option{Key: "保存选项", Value: `{"fx":7.3,"text":"中文"}`}).Error)
	stamp := time.Date(2026, 11, 12, 2, 3, 4, 123456000, time.UTC)
	require.NoError(t, from.Table("knowledge_bases").Create(&archivedKnowledgeBase{Id: 11, Slug: "test", Name: "中文", Description: "", SourceName: "", Content: "test", Enabled: true, CreatedAt: &stamp}).Error)
	require.NoError(t, from.Model(&model.Token{}).Where("id = ?", 3).Update("unlimited_quota", 2).Error)
	sqlitePool, err := from.DB()
	require.NoError(t, err)
	require.NoError(t, sqlitePool.Close())
	_, err = migrate(ctx, path, dsn, strings.TrimPrefix(u.Path, "/"))
	require.ErrorContains(t, err, "invalid boolean")
	// Earlier table copies must have rolled back with the invalid token value.
	var count int
	require.NoError(t, pg.QueryRow(ctx, "SELECT count(*) FROM channels").Scan(&count))
	assert.Zero(t, count)
	from, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, from.Model(&model.Token{}).Where("id = ?", 3).Update("unlimited_quota", true).Error)
	sqlitePool, err = from.DB()
	require.NoError(t, err)
	require.NoError(t, sqlitePool.Close())
	report, err := migrate(ctx, path, dsn, strings.TrimPrefix(u.Path, "/"))
	require.NoError(t, err)
	assert.True(t, report.SourceUnchanged)
	assert.Len(t, report.Tables, 5)
	var quota, used int64
	var username string
	require.NoError(t, pg.QueryRow(ctx, "SELECT username,quota,used_quota FROM users WHERE id=1").Scan(&username, &quota, &used))
	assert.Equal(t, user.Username, username)
	assert.Equal(t, int64(user.Quota), quota)
	assert.Equal(t, int64(user.UsedQuota), used)
	var next int64
	require.NoError(t, pg.QueryRow(ctx, "SELECT nextval(pg_get_serial_sequence('users','id'))").Scan(&next))
	assert.Equal(t, int64(5001), next)
	_, err = pg.Exec(ctx, `INSERT INTO knowledge_bases(slug,name,description,source_name,content) VALUES('test','x','','','')`)
	require.Error(t, err, "source unique index must still reject duplicate slugs")
	var id int
	var enabled, download bool
	require.NoError(t, pg.QueryRow(ctx, `INSERT INTO knowledge_bases(slug,name,description,source_name,content) VALUES('second','x','','','') RETURNING id,enabled,allow_download`).Scan(&id, &enabled, &download))
	// PostgreSQL sequences may leave gaps after a rejected unique insert.
	assert.Equal(t, 13, id)
	assert.True(t, enabled)
	assert.False(t, download)
	_, err = migrate(ctx, path, dsn, strings.TrimPrefix(u.Path, "/"))
	require.ErrorContains(t, err, "nonempty")
	require.NoError(t, pg.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count))
	assert.Equal(t, 1, count)
	t.Logf("verified %d tables; source SHA256 %s", len(report.Tables), report.SourceSHA256)
}
