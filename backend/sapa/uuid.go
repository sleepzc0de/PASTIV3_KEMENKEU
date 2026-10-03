package sapa

import (
	"regexp"
	"strings"
)

var reUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// BakukanUUID menerima UUID bentuk baku (8-4-4-4-12 heksadesimal, dengan tanda hubung) dan mengembalikannya dalam huruf kecil.
// Bentuk lain (tanpa tanda hubung, berkurung, awalan urn:) ditolak supaya satu usulan hanya punya satu alamat.
func BakukanUUID(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if !reUUID.MatchString(s) {
		return "", false
	}
	return strings.ToLower(s), true
}
