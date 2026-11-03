package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nersus15/webcore-cli/internal/names"
)

var migrationSeqRe = regexp.MustCompile(`^(\d{5})_`)

// NewMigration membuat file migrasi goose baru (NNNNN_nama.sql dengan
// anotasi -- +goose Up / -- +goose Down), mengikuti konvensi yang
// didokumentasikan di webcore-docs § Migrasi Database. Nomor urut
// dideteksi otomatis dari file yang sudah ada di direktori tujuan.
func NewMigration(name, moduleScope string, force bool) error {
	dir := "migrations"
	if moduleScope != "" {
		dir = filepath.Join("modules", names.Package(moduleScope), "migrations")
	}

	next, err := nextMigrationSeq(dir)
	if err != nil {
		return err
	}

	slug := strings.ToLower(strings.Join(splitForSlug(name), "_"))
	filename := fmt.Sprintf("%05d_%s.sql", next, slug)

	content, err := render(migrationTmpl, map[string]string{"Name": name})
	if err != nil {
		return err
	}

	return WriteFile(filepath.Join(dir, filename), content, force)
}

func nextMigrationSeq(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("baca direktori migrasi %s: %w", dir, err)
	}

	max := 0
	for _, e := range entries {
		m := migrationSeqRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err == nil && n > max {
			max = n
		}
	}

	return max + 1, nil
}

func splitForSlug(name string) []string {
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	return strings.Fields(name)
}

const migrationTmpl = `-- +goose Up
-- TODO: tulis migrasi untuk "{{.Name}}"


-- +goose Down
-- TODO: tulis rollback untuk "{{.Name}}"
`
