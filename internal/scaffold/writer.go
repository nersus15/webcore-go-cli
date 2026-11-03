package scaffold

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

// WriteFile membuat direktori induk bila perlu, lalu menulis content ke
// path. Menolak menimpa file yang sudah ada kecuali force=true — supaya
// generator tidak diam-diam menghapus kode yang sudah ditulis tangan.
//
// Untuk file .go, content dilewatkan dulu ke go/format (library yang
// dipakai gofmt sendiri) — supaya hasil generator selalu rapi tanpa harus
// menata spasi/alignment manual di tiap template, dan supaya kesalahan
// sintaks pada template ketahuan sebagai error saat generate, bukan
// baru saat `go build`.
func WriteFile(path string, content []byte, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("file sudah ada, lewati (pakai --force untuk timpa): %s", path)
	}

	if strings.HasSuffix(path, ".go") {
		formatted, err := format.Source(content)
		if err != nil {
			return fmt.Errorf("hasil generate %s bukan Go yang valid: %w", path, err)
		}
		content = formatted
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("buat direktori %s: %w", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("tulis file %s: %w", path, err)
	}

	fmt.Printf("  created: %s\n", path)
	return nil
}
