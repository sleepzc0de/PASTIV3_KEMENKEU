package apiakses

import (
	"errors"
	"net/netip"
	"strings"
)

const maksEntriIP = 20

// DaftarIP memeriksa dan merapikan allowlist IP: alamat tunggal atau CIDR (IPv4/IPv6) dipisah koma, spasi, atau baris baru. Hasilnya bentuk baku ("10.1.2.3/32" untuk
// alamat tunggal, alamat jaringan untuk CIDR) tanpa duplikat. Rentang yang membuka semuanya (/0) ditolak: bila tidak ingin membatasi, kosongkan daftarnya.
func DaftarIP(s string) ([]string, error) {
	f := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\r' || r == '\t' })
	if len(f) > maksEntriIP {
		return nil, errors.New("Daftar IP terlalu panjang (maksimal 20 entri)")
	}
	var out []string
	lihat := map[string]bool{}
	for _, e := range f {
		p, err := parsePrefix(e)
		if err != nil {
			return nil, errors.New("Ada alamat IP atau CIDR yang tidak valid")
		}
		if p.Bits() == 0 {
			return nil, errors.New("Rentang IP yang mencakup semua alamat (/0) tidak diizinkan; kosongkan daftar bila tidak ingin membatasi")
		}
		k := p.String()
		if !lihat[k] {
			lihat[k] = true
			out = append(out, k)
		}
	}
	return out, nil
}

func parsePrefix(e string) (netip.Prefix, error) {
	if strings.Contains(e, "/") {
		p, err := netip.ParsePrefix(e)
		if err != nil {
			return netip.Prefix{}, err
		}
		a := p.Addr().Unmap()
		// IPv4 yang ditulis sebagai IPv6 terpetakan (::ffff:a.b.c.d/120) disetarakan dengan IPv4-nya.
		if p.Addr().Is4In6() {
			if p.Bits() < 96 {
				return netip.Prefix{}, errors.New("rentang terlalu lebar")
			}
			p = netip.PrefixFrom(a, p.Bits()-96)
		}
		return p.Masked(), nil
	}
	a, err := netip.ParseAddr(e)
	if err != nil {
		return netip.Prefix{}, err
	}
	a = a.Unmap()
	return netip.PrefixFrom(a, a.BitLen()), nil
}

// IPDiizinkan: apakah ip (teks, mis. dari c.ClientIP()) termasuk daftar. Daftar kosong = semua alamat boleh. IP yang tidak terbaca ditolak bila ada daftar.
func IPDiizinkan(daftar []string, ip string) bool {
	if len(daftar) == 0 {
		return true
	}
	a, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	a = a.Unmap()
	for _, e := range daftar {
		p, err := netip.ParsePrefix(e)
		if err != nil {
			continue
		}
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// DaftarUE1 memeriksa pembatasan UE1: kode 5 digit dipisah koma/spasi, tanpa duplikat. Kosong = tidak membatasi.
func DaftarUE1(s string) ([]string, error) {
	f := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\r' || r == '\t' })
	if len(f) > 30 {
		return nil, errors.New("Terlalu banyak kode UE1 (maksimal 30)")
	}
	var out []string
	lihat := map[string]bool{}
	for _, k := range f {
		if len(k) != 5 {
			return nil, errors.New("Kode UE1 harus 5 digit angka")
		}
		for _, r := range k {
			if r < '0' || r > '9' {
				return nil, errors.New("Kode UE1 harus 5 digit angka")
			}
		}
		if !lihat[k] {
			lihat[k] = true
			out = append(out, k)
		}
	}
	return out, nil
}
