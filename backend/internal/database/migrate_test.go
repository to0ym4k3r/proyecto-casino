package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMigrationFilename(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		wantVersion string
		wantSlug    string
		wantOK      bool
	}{
		{name: "valid zero padded", filename: "001_init.sql", wantVersion: "001", wantSlug: "init", wantOK: true},
		{name: "valid multiple slug parts", filename: "002_kyc_tables.sql", wantVersion: "002", wantSlug: "kyc_tables", wantOK: true},
		{name: "missing extension", filename: "003_migration", wantVersion: "", wantSlug: "", wantOK: false},
		{name: "no version prefix", filename: "init.sql", wantVersion: "", wantSlug: "", wantOK: false},
		{name: "non numeric version", filename: "abc_init.sql", wantVersion: "", wantSlug: "", wantOK: false},
		{name: "no slug", filename: "004.sql", wantVersion: "", wantSlug: "", wantOK: false},
		{name: "uppercase extension", filename: "005_init.SQL", wantVersion: "", wantSlug: "", wantOK: false},
		{name: "empty filename", filename: "", wantVersion: "", wantSlug: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, slug, ok := ParseMigrationFilename(tt.filename)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if version != tt.wantVersion || slug != tt.wantSlug {
				t.Errorf("got (%q, %q), want (%q, %q)", version, slug, tt.wantVersion, tt.wantSlug)
			}
		})
	}
}

func TestLoadMigrationsOrdersByVersionAndKeepsContent(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"003_third.sql":  "SELECT 3;",
		"001_first.sql":  "SELECT 1;",
		"002_second.sql": "SELECT 2;",
		"notes.txt":      "not a migration",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	got, err := LoadMigrations(dir)
	if err != nil {
		t.Fatalf("LoadMigrations: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}

	wantVersions := []string{"001", "002", "003"}
	wantPaths := []string{"001_first.sql", "002_second.sql", "003_third.sql"}
	for i, migration := range got {
		if migration.Version != wantVersions[i] {
			t.Errorf("migrations[%d].Version = %q, want %q", i, migration.Version, wantVersions[i])
		}
		if migration.Path != wantPaths[i] {
			t.Errorf("migrations[%d].Path = %q, want %q", i, migration.Path, wantPaths[i])
		}
	}

	if got[0].SQL != "SELECT 1;" {
		t.Errorf("migrations[0].SQL = %q, want %q", got[0].SQL, "SELECT 1;")
	}
	if got[2].SQL != "SELECT 3;" {
		t.Errorf("migrations[2].SQL = %q, want %q", got[2].SQL, "SELECT 3;")
	}
}

func TestLoadMigrationsRejectsMalformedSQLFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "init.sql"), []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := LoadMigrations(dir); err == nil {
		t.Fatal("LoadMigrations: expected error for malformed filename, got nil")
	}
}

func TestLoadMigrationsMissingDir(t *testing.T) {
	if _, err := LoadMigrations(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("LoadMigrations: expected error for missing dir, got nil")
	}
}
