package sapa

import (
	"context"
	"sort"
	"strings"
)

// Aturan bisnis daftar jenis BMN dan satuan jumlahnya (lihat bmn.go).

// RefBMNAktif: daftar yang ditawarkan pada formulir (hanya yang aktif), untuk semua pengguna SAPA.
func (l *Layanan) RefBMNAktif(ctx context.Context, id Identitas) (RefBMN, error) {
	if err := Akses(id); err != nil {
		return RefBMN{}, err
	}
	ref, err := l.Repo.AmbilRefBMN(ctx)
	if err != nil {
		return RefBMN{}, err
	}
	return ref.Aktif(), nil
}

// RefBMNSemua: seluruh daftar termasuk yang nonaktif (admin).
func (l *Layanan) RefBMNSemua(ctx context.Context, id Identitas) (RefBMN, error) {
	if err := l.khususAdmin(id); err != nil {
		return RefBMN{}, err
	}
	return l.Repo.AmbilRefBMN(ctx)
}

func rentangUrutan(u int) bool { return u >= 0 && u <= 9999 }

func (l *Layanan) SimpanSatuanBMN(ctx context.Context, id Identitas, s SatuanBMN) error {
	if err := l.khususAdmin(id); err != nil {
		return err
	}
	s.Nama = strings.TrimSpace(s.Nama)
	if !namaDaftarSah(s.Nama, MaksSatuanBMN) {
		return validasi("Nama satuan wajib diisi (maksimal %d karakter, tanpa garis miring)", MaksSatuanBMN)
	}
	if !rentangUrutan(s.Urutan) {
		return validasi("Urutan harus antara 0 dan 9999")
	}
	ref, err := l.Repo.AmbilRefBMN(ctx)
	if err != nil {
		return err
	}
	if ada, ok := ref.CariSatuan(s.Nama); ok {
		s.Nama = ada.Nama // pertahankan penulisan yang sudah ada
	} else if len(ref.Satuan) >= maksDaftarBMN {
		return validasi("Daftar satuan sudah penuh (maksimal %d)", maksDaftarBMN)
	}
	return l.Repo.SimpanSatuanBMN(ctx, s, id.Nama)
}

// HapusSatuanBMN menghapus satuan. Ditolak bila satuan itu satu-satunya satuan pada suatu jenis, karena jenis tanpa satuan
// tidak dapat dipakai; jenis lain hanya kehilangan satuan itu (satuan bawaannya dialihkan bila perlu).
func (l *Layanan) HapusSatuanBMN(ctx context.Context, id Identitas, nama string) error {
	if err := l.khususAdmin(id); err != nil {
		return err
	}
	ref, err := l.Repo.AmbilRefBMN(ctx)
	if err != nil {
		return err
	}
	ada, ok := ref.CariSatuan(nama)
	if !ok {
		return ErrTidakDitemukan
	}
	var tunggal []string
	var ubah []JenisBMN
	for _, j := range ref.Jenis {
		if !j.Mengizinkan(ada.Nama) {
			continue
		}
		if len(j.Satuan) == 1 {
			tunggal = append(tunggal, j.Nama)
			continue
		}
		var sisa []string
		for _, s := range j.Satuan {
			if !samaNama(s, ada.Nama) {
				sisa = append(sisa, s)
			}
		}
		j.Satuan = sisa
		if samaNama(j.SatuanBawaan, ada.Nama) {
			j.SatuanBawaan = sisa[0]
		}
		ubah = append(ubah, j)
	}
	if len(tunggal) > 0 {
		sort.Strings(tunggal)
		return konflik("Satuan \"%s\" tidak bisa dihapus karena menjadi satu-satunya satuan pada jenis BMN: %s. Tambahkan satuan lain pada jenis itu dulu, atau nonaktifkan satuan ini.", ada.Nama, strings.Join(tunggal, ", "))
	}
	for _, j := range ubah {
		if err := l.Repo.SimpanJenisBMN(ctx, j, id.Nama); err != nil {
			return err
		}
	}
	return l.Repo.HapusSatuanBMN(ctx, ada.Nama)
}

func (l *Layanan) SimpanJenisBMN(ctx context.Context, id Identitas, j JenisBMN) error {
	if err := l.khususAdmin(id); err != nil {
		return err
	}
	j.Nama = strings.TrimSpace(j.Nama)
	if !namaDaftarSah(j.Nama, MaksJenisBMN) {
		return validasi("Nama jenis BMN wajib diisi (maksimal %d karakter, tanpa garis miring)", MaksJenisBMN)
	}
	if !rentangUrutan(j.Urutan) {
		return validasi("Urutan harus antara 0 dan 9999")
	}
	ref, err := l.Repo.AmbilRefBMN(ctx)
	if err != nil {
		return err
	}
	if ada, ok := ref.CariJenis(j.Nama); ok {
		j.Nama = ada.Nama
	} else if len(ref.Jenis) >= maksDaftarBMN {
		return validasi("Daftar jenis BMN sudah penuh (maksimal %d)", maksDaftarBMN)
	}

	// Satuan harus ada di daftar satuan; urutan dipertahankan, duplikat dibuang, nama disamakan dengan daftar.
	var satuan []string
	dilihat := map[string]bool{}
	for _, s := range j.Satuan {
		kanon, ok := ref.CariSatuan(s)
		if !ok {
			return validasi("Satuan \"%s\" belum ada di daftar satuan; tambahkan dulu", strings.TrimSpace(s))
		}
		if k := strings.ToLower(kanon.Nama); !dilihat[k] {
			dilihat[k] = true
			satuan = append(satuan, kanon.Nama)
		}
	}
	if len(satuan) == 0 {
		return validasi("Pilih minimal satu satuan untuk jenis BMN ini")
	}
	j.Satuan = satuan
	if strings.TrimSpace(j.SatuanBawaan) == "" {
		j.SatuanBawaan = satuan[0]
	}
	if !j.Mengizinkan(j.SatuanBawaan) {
		return validasi("Satuan bawaan \"%s\" harus termasuk satuan yang dipilih untuk jenis ini", strings.TrimSpace(j.SatuanBawaan))
	}
	for _, s := range satuan {
		if samaNama(s, j.SatuanBawaan) {
			j.SatuanBawaan = s
		}
	}
	return l.Repo.SimpanJenisBMN(ctx, j, id.Nama)
}

func (l *Layanan) HapusJenisBMN(ctx context.Context, id Identitas, nama string) error {
	if err := l.khususAdmin(id); err != nil {
		return err
	}
	ref, err := l.Repo.AmbilRefBMN(ctx)
	if err != nil {
		return err
	}
	ada, ok := ref.CariJenis(nama)
	if !ok {
		return ErrTidakDitemukan
	}
	return l.Repo.HapusJenisBMN(ctx, ada.Nama)
}

// cekBMN memeriksa jenis dan satuan pada Nota Dinas Satker terhadap daftar admin, lalu menyamakan penulisannya dengan daftar.
// Mengembalikan seluruh galat sekaligus (digabung dengan galat isian lain) supaya pengguna memperbaikinya dalam satu putaran.
func (l *Layanan) cekBMN(ctx context.Context, d *DataNDSatker) error {
	d.Rapikan()
	ref, err := l.Repo.AmbilRefBMN(ctx)
	if err != nil {
		return err
	}
	var galat []string
	if len(ref.Aktif().Jenis) == 0 {
		galat = append(galat, "Daftar jenis BMN belum diatur; hubungi admin SAPA")
	} else {
		var g []string
		d.JenisBMN, d.Satuan, g = ValidasiBMN(ref, d.JenisBMN, d.Satuan)
		galat = append(galat, g...)
	}
	galat = append(galat, d.Validasi()...)
	if len(galat) > 0 {
		return &ErrValidasi{Rincian: unik(galat)}
	}
	return nil
}

func unik(s []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(s))
	for _, x := range s {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
