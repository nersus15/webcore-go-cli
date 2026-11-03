// Package names mengonversi nama yang diketik user (mis. "order_item",
// "order-item", "Order Item") ke bentuk yang dibutuhkan template: PascalCase
// untuk identifier Go, dan lowercase tanpa separator untuk nama package/dir.
package names

import "strings"

func splitWords(s string) []string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")

	var words []string
	var current strings.Builder
	runes := []rune(s)

	for i, r := range runes {
		if r == ' ' {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			continue
		}
		// pisah juga di titik transisi lower->Upper, mis. "orderItem" -> "order","Item"
		if i > 0 && current.Len() > 0 {
			prev := runes[i-1]
			if prev >= 'a' && prev <= 'z' && r >= 'A' && r <= 'Z' {
				words = append(words, current.String())
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// Pascal mengonversi ke PascalCase, dipakai untuk nama struct/tipe Go.
// "order_item" -> "OrderItem"
func Pascal(s string) string {
	var b strings.Builder
	for _, w := range splitWords(s) {
		if w == "" {
			continue
		}
		r := []rune(w)
		b.WriteString(strings.ToUpper(string(r[0])))
		if len(r) > 1 {
			b.WriteString(strings.ToLower(string(r[1:])))
		}
	}
	return b.String()
}

// Camel mengonversi ke camelCase, dipakai untuk nama variabel.
// "order_item" -> "orderItem"
func Camel(s string) string {
	p := Pascal(s)
	if p == "" {
		return p
	}
	r := []rune(p)
	return strings.ToLower(string(r[0])) + string(r[1:])
}

// Package mengonversi ke nama package/direktori Go yang valid: lowercase,
// tanpa separator. "Order Item" -> "orderitem"
func Package(s string) string {
	var b strings.Builder
	for _, w := range splitWords(s) {
		b.WriteString(strings.ToLower(w))
	}
	return b.String()
}

// Table menebak nama tabel database dari nama entity: lowercase + "s"
// di akhir (pluralisasi naif, sengaja sederhana — ganti manual kalau perlu
// bentuk jamak yang tidak beraturan, mis. "category" -> "categories").
func Table(s string) string {
	p := Package(s)
	if strings.HasSuffix(p, "s") {
		return p
	}
	if strings.HasSuffix(p, "y") && len(p) > 1 {
		b := p[len(p)-2]
		if b != 'a' && b != 'e' && b != 'i' && b != 'o' && b != 'u' {
			return p[:len(p)-1] + "ies"
		}
	}
	return p + "s"
}
