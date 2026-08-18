package security

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot walks up from the test working directory to the backend module root
// (the directory containing go.mod).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the package directory")
		}
		dir = parent
	}
}

// allGoFiles walks the module root and returns every *.go file, skipping .git.
func allGoFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// findPattern returns every file:line where pat occurs, relative to root.
func findPattern(t *testing.T, root string, files []string, pat string) []string {
	t.Helper()

	// allowlist: known-safe hits. Key is "relative/path:line".
	allowed := map[string]bool{
		"internal/handler/router.go:131":                        true,
		"internal/handler/wallet_handler.go:268":                true,
		"internal/handler/wallet_handler_crypto_test.go:228":    true,
	}

	var hits []string
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		lines := strings.Split(string(src), "\n")
		for i, line := range lines {
			if strings.Contains(line, pat) {
				rel, err := filepath.Rel(root, f)
				if err != nil {
					rel = f
				}
				key := fmt.Sprintf("%s:%d", rel, i+1)
				// Normalize Windows backslashes to forward slashes for allowlist matching
				normalizedKey := strings.ReplaceAll(key, "\\", "/")
				if allowed[normalizedKey] {
					continue
				}
				hits = append(hits, normalizedKey)
			}
		}
	}
	return hits
}

// TestBackdoorScan is the permanent "no backdoors" constancy test. It reads
// every .go file in the module and fails if it finds:
//
//   - process-launching imports and calls (the Go standard process/exec package,
//     and its Command entry point)
//   - arbitrary outbound connection calls (the net package Dial entry point)
//   - HTTP client call sites (the net/http package Get and Post entry points)
//   - admin, debug and pprof route registrations (paths that expose internal
//     tooling are flagged regardless of prefix)
//   - embedded RSA private key / Stripe live-key literals
//   - long pure-base64 string literals (possible hidden blobs)
//   - the production-default JWT secret string (finding #1)
//
// Pattern strings are assembled at runtime and the map keys avoid the literals,
// so this file never matches itself.
func TestBackdoorScan(t *testing.T) {
	root := repoRoot(t)
	files := allGoFiles(t, root)

	patterns := map[string]string{
		"process/exec import":     "os" + "/" + "exec",
		"command execution call":  "exec" + "." + "Command",
		"outbound dial call":      "net" + "." + "Dial",
		"http client GET call":    "http" + "." + "Get",
		"http client POST call":   "http" + "." + "Post",
		"debug route":             "/de" + "bug",
		"pprof route":             "/pp" + "rof",
		"admin route":             "/ad" + "min",
		"RSA private key literal": "BEGIN RS" + "A PRIVATE KEY",
		"stripe live key literal": "sk_li" + "ve_",
		"default JWT secret":      "change-me-in-prod" + "uction",
	}
	for name, pat := range patterns {
		t.Run(name, func(t *testing.T) {
			if hits := findPattern(t, root, files, pat); len(hits) > 0 {
				t.Errorf("pattern %q found in production source: %v", name, hits)
			}
		})
	}

	t.Run("long base64 string literals", func(t *testing.T) {
		litRe := regexp.MustCompile(`"([^"\\]|\\.)*"`)
		base64Re := regexp.MustCompile(`^[A-Za-z0-9+/]{50,}={0,2}$`)
		var hits []string
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			for _, m := range litRe.FindAllString(string(src), -1) {
				s := strings.Trim(m, `"`)
				if base64Re.MatchString(s) {
					rel, err := filepath.Rel(root, f)
					if err != nil {
						rel = f
					}
					hits = append(hits, fmt.Sprintf("%s: %d-char blob", rel, len(s)))
				}
			}
		}
		if len(hits) > 0 {
			t.Errorf("suspicious base64 blobs: %v", hits)
		}
	})
}
