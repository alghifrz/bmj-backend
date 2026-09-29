package migrate

import (
	"strings"
	"testing"
	"testing/fstest"

	"bmj-backend/migrations"
)

func TestLoadSortsAndPairsMigrations(t *testing.T) {
	fsys := fstest.MapFS{
		"000002_second.up.sql":   {Data: []byte("SELECT 2;")},
		"000002_second.down.sql": {Data: []byte("SELECT 0;")},
		"000001_first.up.sql":    {Data: []byte("SELECT 1;")},
		"000001_first.down.sql":  {Data: []byte("SELECT 0;")},
		"notes.txt":              {Data: []byte("ignore")},
	}

	got, err := Load(fsys)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Version != 1 || got[0].Name != "first" || got[0].UpSQL != "SELECT 1;" {
		t.Fatalf("first migration = %+v", got[0])
	}
	if got[1].Version != 2 || got[1].Name != "second" {
		t.Fatalf("second migration = %+v", got[1])
	}
}

func TestLoadRejectsIncompletePair(t *testing.T) {
	fsys := fstest.MapFS{
		"000001_only.up.sql": {Data: []byte("SELECT 1;")},
	}

	if _, err := Load(fsys); err == nil {
		t.Fatal("expected missing down file to fail")
	}
}

func TestLoadRejectsDuplicateVersion(t *testing.T) {
	fsys := fstest.MapFS{
		"000001_one.up.sql":   {Data: []byte("SELECT 1;")},
		"000001_one.down.sql": {Data: []byte("SELECT 0;")},
		"000001_two.up.sql":   {Data: []byte("SELECT 2;")},
		"000001_two.down.sql": {Data: []byte("SELECT 0;")},
	}

	if _, err := Load(fsys); err == nil {
		t.Fatal("expected duplicate version to fail")
	}
}

func TestPendingRequiresMatchingPrefix(t *testing.T) {
	all := []Migration{
		{Version: 1, Name: "first"},
		{Version: 2, Name: "second"},
	}

	pending, err := pendingMigrations(all, []appliedMigration{{Version: 1, Name: "first"}})
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 1 || pending[0].Version != 2 {
		t.Fatalf("pending = %+v", pending)
	}

	if _, err := pendingMigrations(all, []appliedMigration{{Version: 2, Name: "second"}}); err == nil {
		t.Fatal("expected a gap in applied migrations to fail")
	}
}

func TestEmbeddedInitialMigration(t *testing.T) {
	got, err := Load(migrations.FS)
	if err != nil {
		t.Fatalf("load embedded migrations: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	if got[0].Version != 1 || got[0].Name != "initial_schema" {
		t.Fatalf("migration = %06d_%s", got[0].Version, got[0].Name)
	}
	if got[1].Version != 2 || got[1].Name != "store_whatsapp_message" {
		t.Fatalf("migration = %06d_%s", got[1].Version, got[1].Name)
	}
	if !strings.Contains(got[1].UpSQL, "whatsapp_message_template") {
		t.Fatal("message migration missing template column")
	}
	if !strings.Contains(got[1].DownSQL, "DROP COLUMN IF EXISTS whatsapp_message_template") {
		t.Fatal("message migration down script missing column drop")
	}
	if got[2].Version != 3 || got[2].Name != "store_about" {
		t.Fatalf("migration = %06d_%s", got[2].Version, got[2].Name)
	}
	if !strings.Contains(got[2].UpSQL, "about_body") {
		t.Fatal("about migration missing body column")
	}
	if got[3].Version != 4 || got[3].Name != "analytics_events" {
		t.Fatalf("migration = %06d_%s", got[3].Version, got[3].Name)
	}
	if !strings.Contains(got[3].UpSQL, "CREATE TABLE analytics_events") {
		t.Fatal("analytics migration missing events table")
	}
	if !strings.Contains(got[3].DownSQL, "DROP TABLE IF EXISTS analytics_events") {
		t.Fatal("analytics migration down script missing table drop")
	}

	for _, table := range []string{
		"admins",
		"categories",
		"products",
		"product_images",
		"reviews",
		"store_settings",
		"store_locations",
	} {
		if !strings.Contains(got[0].UpSQL, "CREATE TABLE "+table) {
			t.Fatalf("up sql missing table %s", table)
		}
		if !strings.Contains(got[0].DownSQL, "DROP TABLE IF EXISTS "+table) {
			t.Fatalf("down sql missing table %s", table)
		}
	}

	if strings.Contains(strings.ToLower(got[0].UpSQL), "insert into reviews") {
		t.Fatal("initial migration seeds reviews")
	}
	if strings.Contains(strings.ToLower(got[0].UpSQL), "insert into products") {
		t.Fatal("initial migration seeds products")
	}
}

func TestConnectRequiresURL(t *testing.T) {
	if _, err := connect(t.Context(), "  "); err == nil {
		t.Fatal("expected empty database url to fail")
	}
}
