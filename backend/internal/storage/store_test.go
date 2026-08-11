package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	jpegContent = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	pngContent  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R'}
	pdfContent  = []byte{'%', 'P', 'D', 'F', '-', '1', '.', '4', '\n', '%', 0xE2, 0xE3, 0xCF, 0xD3}
)

// ---------------------------------------------------------------------------
// ValidateKycDocument (pure function, design D2 whitelist)
// ---------------------------------------------------------------------------

func TestValidateKycDocumentValidTypes(t *testing.T) {
	cases := []struct {
		name        string
		filename    string
		data        []byte
		wantExt     string
		wantContent string
	}{
		{"jpg", "id_card.jpg", jpegContent, "jpg", "image/jpeg"},
		{"jpeg alias normalizes to jpg", "photo.jpeg", jpegContent, "jpg", "image/jpeg"},
		{"uppercase extension", "SCAN.JPG", jpegContent, "jpg", "image/jpeg"},
		{"png", "proof.png", pngContent, "png", "image/png"},
		{"pdf", "statement.pdf", pdfContent, "pdf", "application/pdf"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ext, ct, err := ValidateKycDocument(tt.filename, tt.data)
			if err != nil {
				t.Fatalf("ValidateKycDocument(%q) = error %v, want nil", tt.filename, err)
			}
			if ext != tt.wantExt || ct != tt.wantContent {
				t.Errorf("got (ext=%q, ct=%q), want (ext=%q, ct=%q)", ext, ct, tt.wantExt, tt.wantContent)
			}
		})
	}
}

func TestValidateKycDocumentRejectsBadTypes(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		data     []byte
	}{
		{"disallowed extension", "note.txt", jpegContent},
		{"no extension", "idcard", jpegContent},
		{"png bytes in jpg file", "fake.jpg", pngContent},
		{"pdf bytes in png file", "fake.png", pdfContent},
		{"jpeg bytes in pdf file", "fake.pdf", jpegContent},
		{"empty data", "empty.jpg", nil},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ext, ct, err := ValidateKycDocument(tt.filename, tt.data)
			if !errors.Is(err, ErrInvalidKycType) {
				t.Errorf("ValidateKycDocument(%q) = (ext=%q, ct=%q, err=%v), want ErrInvalidKycType",
					tt.filename, ext, ct, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LocalStore (filesystem, sandboxed under t.TempDir)
// ---------------------------------------------------------------------------

func TestLocalStoreSavePersistsBytes(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStore(dir)

	n, err := store.Save(context.Background(), "abc123.jpg", bytes.NewReader(jpegContent))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if n != int64(len(jpegContent)) {
		t.Errorf("saved bytes = %d, want %d", n, len(jpegContent))
	}

	got, err := os.ReadFile(filepath.Join(dir, "abc123.jpg"))
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if !bytes.Equal(got, jpegContent) {
		t.Errorf("stored content mismatch: got %d bytes, want %d", len(got), len(jpegContent))
	}
}

func TestLocalStoreSaveCreatesMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "kyc")
	store := NewLocalStore(dir)

	if _, err := store.Save(context.Background(), "key.jpg", bytes.NewReader(jpegContent)); err != nil {
		t.Fatalf("Save with missing dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "key.jpg")); err != nil {
		t.Errorf("stored file missing after MkdirAll: %v", err)
	}
}

func TestLocalStoreSaveRejectsTraversalKeys(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStore(dir)

	keys := []string{"../evil.jpg", "a/../b.jpg", "/abs/path.jpg", "..", "sub\\dir.jpg", "c:\\windows\\x"}
	for _, key := range keys {
		t.Run(strings.ReplaceAll(key, "\\", "_"), func(t *testing.T) {
			if _, err := store.Save(context.Background(), key, bytes.NewReader(jpegContent)); !errors.Is(err, ErrInvalidKey) {
				t.Errorf("Save(%q) = err %v, want ErrInvalidKey", key, err)
			}
			// Nothing may have been written outside the sandbox.
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Errorf("Save(%q) wrote files into sandbox: %d entries", key, len(entries))
			}
		})
	}
}

func TestLocalStoreSaveRejectsExistingKey(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStore(dir)

	if _, err := store.Save(context.Background(), "once.jpg", bytes.NewReader(jpegContent)); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	// O_EXCL semantics: a second write to the same key must fail rather than
	// silently overwrite (keys are fresh UUIDs, so this only guards misuse).
	if _, err := store.Save(context.Background(), "once.jpg", bytes.NewReader(pdfContent)); err == nil {
		t.Error("second Save with same key succeeded, want error (O_EXCL)")
	}
	got, err := os.ReadFile(filepath.Join(dir, "once.jpg"))
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if !bytes.Equal(got, jpegContent) {
		t.Error("existing file was overwritten")
	}
}

func TestLocalStoreDeleteRemovesFile(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStore(dir)

	if _, err := store.Save(context.Background(), "doc.jpg", bytes.NewReader(jpegContent)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(context.Background(), "doc.jpg"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "doc.jpg")); !os.IsNotExist(err) {
		t.Errorf("file still exists after Delete (err=%v)", err)
	}
}

func TestLocalStoreDeleteMissingKeyIsNoOp(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStore(dir)

	// Compensation paths call Delete without knowing whether Save landed;
	// removing a key that does not exist must succeed (idempotent).
	if err := store.Delete(context.Background(), "never-saved.jpg"); err != nil {
		t.Errorf("Delete of missing key = %v, want nil (idempotent)", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("Delete of missing key created entries: %d", len(entries))
	}
}

func TestLocalStoreDeleteRejectsTraversalKeys(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStore(dir)

	if err := store.Delete(context.Background(), "../escape.jpg"); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("Delete(../escape.jpg) = %v, want ErrInvalidKey", err)
	}
}
