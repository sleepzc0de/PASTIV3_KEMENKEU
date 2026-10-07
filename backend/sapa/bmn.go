package sapa

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Daftar jenis BMN dan satuan jumlahnya, diatur admin di Pengaturan SAPA. Setiap jenis BMN hanya boleh memakai satuan yang
// dipetakan padanya, supaya pilihan yang tidak masuk akal (mis. Peralatan dan Mesin dalam "meter", atau Tanah dalam "unit")
// tidak mungkin dibuat, baik di formulir maupun lewat API.

const (
	MaksJenisBMN  = 100 // panjang nama jenis
	MaksSatuanBMN = 30  // panjang nama satuan
	maksDaftarBMN = 200 // batas banyak jenis/satuan
)

type SatuanBMN struct {
	Nama   string `json:"nama"`
	Aktif  bool   `json:"aktif"`
	Urutan int    `json:"urutan"`
}

type JenisBMN struct {
	Nama         string   `json:"nama"`
	Aktif        bool     `json:"aktif"`
	Urutan       int      `json:"urutan"`
	Satuan       []string `json:"satuan"`        // satuan yang diizinkan untuk jenis ini, urut tampil
	SatuanBawaan string   `json:"satuan_bawaan"` // dipilih otomatis saat jenis dipilih
}

type RefBMN struct {
	Jenis  []JenisBMN  `json:"jenis"`
	Satuan []SatuanBMN `json:"satuan"`
}

// DefaultRefBMN: daftar awal yang disemai migrasi 022 dan 057 (dan dipakai penyimpanan memori untuk tes). Mengikuti penggolongan BMN
// (Tanah, Peralatan dan Mesin, Gedung dan Bangunan, Jalan Irigasi dan Jaringan, Aset Tetap Lainnya); admin dapat mengubahnya. NUP berlaku untuk semua jenis
// (setiap barang punya nomor urut pendaftaran) dan m2 untuk jenis yang diukur luasnya (Tanah, Gedung dan Bangunan, Tanah dan Bangunan).
func DefaultRefBMN() RefBMN {
	sat := []string{"bidang", "unit", "buah", "set", "paket", "eksemplar", "NUP", "m2"}
	r := RefBMN{}
	for i, s := range sat {
		r.Satuan = append(r.Satuan, SatuanBMN{Nama: s, Aktif: true, Urutan: i + 1})
	}
	jenis := []JenisBMN{
		{Nama: "Tanah", Satuan: []string{"bidang", "m2", "NUP"}, SatuanBawaan: "bidang"},
		{Nama: "Gedung dan Bangunan", Satuan: []string{"unit", "buah", "m2", "NUP"}, SatuanBawaan: "unit"},
		{Nama: "Tanah dan Bangunan", Satuan: []string{"bidang", "unit", "paket", "m2", "NUP"}, SatuanBawaan: "unit"},
		{Nama: "Peralatan dan Mesin", Satuan: []string{"unit", "buah", "set", "paket", "NUP"}, SatuanBawaan: "unit"},
		{Nama: "Kendaraan Bermotor", Satuan: []string{"unit", "NUP"}, SatuanBawaan: "unit"},
		{Nama: "Jalan, Irigasi, dan Jaringan", Satuan: []string{"unit", "paket", "NUP"}, SatuanBawaan: "unit"},
		{Nama: "Aset Tetap Lainnya", Satuan: []string{"unit", "buah", "set", "eksemplar", "NUP"}, SatuanBawaan: "unit"},
	}
	for i, j := range jenis {
		j.Aktif, j.Urutan = true, i+1
		r.Jenis = append(r.Jenis, j)
	}
	return r
}

func samaNama(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) }

// CariJenis mencari jenis menurut nama tanpa membedakan huruf besar/kecil.
func (r RefBMN) CariJenis(nama string) (JenisBMN, bool) {
	for _, j := range r.Jenis {
		if samaNama(j.Nama, nama) {
			return j, true
		}
	}
	return JenisBMN{}, false
}

func (r RefBMN) CariSatuan(nama string) (SatuanBMN, bool) {
	for _, s := range r.Satuan {
		if samaNama(s.Nama, nama) {
			return s, true
		}
	}
	return SatuanBMN{}, false
}

// Mengizinkan: apakah satuan ini termasuk satuan yang diizinkan untuk jenis tersebut.
func (j JenisBMN) Mengizinkan(satuan string) bool {
	for _, s := range j.Satuan {
		if samaNama(s, satuan) {
			return true
		}
	}
	return false
}

// Aktif menyisakan jenis yang aktif dengan satuannya yang aktif, serta satuan yang aktif; urut menurut urutan lalu nama.
// Jenis tanpa satuan aktif dibuang karena tidak bisa dipakai.
func (r RefBMN) Aktif() RefBMN {
	out := RefBMN{Jenis: []JenisBMN{}, Satuan: []SatuanBMN{}}
	aktif := map[string]bool{}
	for _, s := range r.Satuan {
		if s.Aktif {
			out.Satuan = append(out.Satuan, s)
			aktif[strings.ToLower(s.Nama)] = true
		}
	}
	for _, j := range r.Jenis {
		if !j.Aktif {
			continue
		}
		c := j
		c.Satuan = nil
		for _, s := range j.Satuan {
			if aktif[strings.ToLower(s)] {
				c.Satuan = append(c.Satuan, s)
			}
		}
		if len(c.Satuan) == 0 {
			continue
		}
		if !c.Mengizinkan(c.SatuanBawaan) {
			c.SatuanBawaan = c.Satuan[0]
		}
		out.Jenis = append(out.Jenis, c)
	}
	sort.SliceStable(out.Jenis, func(a, b int) bool {
		if out.Jenis[a].Urutan != out.Jenis[b].Urutan {
			return out.Jenis[a].Urutan < out.Jenis[b].Urutan
		}
		return strings.ToLower(out.Jenis[a].Nama) < strings.ToLower(out.Jenis[b].Nama)
	})
	sort.SliceStable(out.Satuan, func(a, b int) bool {
		if out.Satuan[a].Urutan != out.Satuan[b].Urutan {
			return out.Satuan[a].Urutan < out.Satuan[b].Urutan
		}
		return strings.ToLower(out.Satuan[a].Nama) < strings.ToLower(out.Satuan[b].Nama)
	})
	return out
}

// ValidasiBMN memeriksa pasangan jenis dan satuan terhadap daftar. Mengembalikan nama kanonik (huruf besar/kecil seperti di
// daftar) dan daftar galat. Jenis atau satuan kosong tidak dilaporkan di sini (kewajibannya diperiksa di DataNDSatker.Validasi).
func ValidasiBMN(ref RefBMN, jenis, satuan string) (jenisKanon, satuanKanon string, galat []string) {
	jenis, satuan = strings.TrimSpace(jenis), strings.TrimSpace(satuan)
	jenisKanon, satuanKanon = jenis, satuan
	if jenis == "" {
		return
	}
	aktif := ref.Aktif()
	j, ok := aktif.CariJenis(jenis)
	if !ok {
		if _, ada := ref.CariJenis(jenis); ada {
			galat = append(galat, fmt.Sprintf("Jenis BMN \"%s\" sudah tidak tersedia; pilih jenis lain", jenis))
		} else {
			galat = append(galat, fmt.Sprintf("Jenis BMN \"%s\" tidak ada di daftar; pilih dari daftar jenis BMN", jenis))
		}
		return
	}
	jenisKanon = j.Nama
	if satuan == "" {
		return
	}
	for _, s := range j.Satuan {
		if samaNama(s, satuan) {
			satuanKanon = s
			return
		}
	}
	galat = append(galat, fmt.Sprintf("Satuan \"%s\" tidak sesuai untuk jenis BMN \"%s\"; satuan yang boleh: %s", satuan, j.Nama, strings.Join(j.Satuan, ", ")))
	return
}

// namaDaftarSah: nama jenis/satuan tidak kosong, tidak melebihi batas, dan tanpa karakter kontrol atau pemisah jalur.
func namaDaftarSah(nama string, maks int) bool {
	if nama == "" || utf8.RuneCountInString(nama) > maks {
		return false
	}
	for _, r := range nama {
		if unicode.IsControl(r) || r == '/' || r == '\\' {
			return false
		}
	}
	return true
}
