// Package gomod membaca module path dari go.mod project yang sedang
// dikerjakan, supaya generator tahu import path yang benar untuk
// package yang dibuatnya.
package gomod

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// DetectModulePath membaca go.mod di direktori kerja saat ini.
func DetectModulePath() (string, error) {
	f, err := os.Open("go.mod")
	if err != nil {
		return "", fmt.Errorf("baca go.mod (jalankan perintah ini dari root project Go): %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("baris 'module' tidak ditemukan di go.mod")
}
