// Paket serverlog: log server yang hanya menyimpan galat (error) dan peringatan (warning).
//
// Prinsipnya supaya log tidak memberatkan server: tidak ada log akses per permintaan (hanya respons 5xx, panik, dan permintaan yang lambat yang dicatat);
// catatan info/debug tidak pernah disimpan dan, di production, juga tidak dicetak; kejadian yang sama yang berulang digabung menjadi satu baris
// dengan hitungan ("sidik jari" pesan); penulisan ke database dilakukan di latar belakang secara berkelompok dengan antrean berbatas; dan baris lama
// dihapus otomatis menurut retensi serta batas jumlah baris.
package serverlog

import (
	"strings"
	"time"
)

// Level tingkat keparahan sebuah catatan.
type Level string

const (
	Error Level = "error"
	Warn  Level = "warn"
	Info  Level = "info"
	Debug Level = "debug"
)

// peringkat: urutan keparahan untuk membandingkan dengan ambang cetak.
func (l Level) peringkat() int {
	switch l {
	case Error:
		return 3
	case Warn:
		return 2
	case Info:
		return 1
	}
	return 0
}

// Label: huruf besar untuk dicetak di konsol, mis. "ERROR".
func (l Level) Label() string { return strings.ToUpper(string(l)) }

// BacaLevel membaca nama level (error, warn/warning, info, debug); selain itu bawaan.
func BacaLevel(s string, bawaan Level) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "error", "err":
		return Error
	case "warn", "warning":
		return Warn
	case "info":
		return Info
	case "debug":
		return Debug
	}
	return bawaan
}

// ModeSimpan: catatan apa yang disimpan ke database.
type ModeSimpan string

const (
	SimpanGalatDanPeringatan ModeSimpan = "warn"  // error + warning (bawaan)
	SimpanGalat              ModeSimpan = "error" // hanya error
	SimpanMati               ModeSimpan = "off"   // tidak menyimpan apa pun
)

// BacaModeSimpan membaca pengaturan LOG_SIMPAN; selain nilai yang dikenal memakai bawaan (error + warning).
func BacaModeSimpan(s string) ModeSimpan {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "error", "galat":
		return SimpanGalat
	case "off", "mati", "false", "0", "none", "tidak":
		return SimpanMati
	}
	return SimpanGalatDanPeringatan
}

// boleh: apakah catatan berlevel l disimpan pada mode ini.
func (m ModeSimpan) boleh(l Level) bool {
	switch m {
	case SimpanMati:
		return false
	case SimpanGalat:
		return l == Error
	}
	return l == Error || l == Warn
}

// Entri: satu kejadian yang dicatat. Hanya level Error dan Warn yang pernah sampai ke penyimpanan.
type Entri struct {
	Waktu     time.Time `json:"waktu"`
	Level     Level     `json:"level"`
	Sumber    string    `json:"sumber"` // bagian aplikasi, mis. "INAPROC SYNC", "HTTP", "PANIK"
	Pesan     string    `json:"pesan"`
	Detail    string    `json:"detail,omitempty"` // mis. jejak tumpukan panik
	Rute      string    `json:"rute,omitempty"`   // pola rute (tanpa nilai parameter), hanya untuk entri dari permintaan HTTP
	Metode    string    `json:"metode,omitempty"`
	Status    int       `json:"status,omitempty"`
	DurasiMS  int       `json:"durasi_ms,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`

	sidik string // sidik jari yang sudah dihitung (diisi pencatat); kosong = dihitung perekam
}

// Gabungan: kejadian-kejadian yang sama (sidik jari yang sama) dalam satu jendela digabung; Entri adalah contoh pertamanya.
type Gabungan struct {
	Entri
	Sidik  string
	Jumlah int
	Akhir  time.Time // waktu kejadian terakhir yang digabung
}

// Baris: satu baris tersimpan (hasil gabungan) sebagaimana dibaca dari database.
type Baris struct {
	ID         int64     `json:"id"`
	Waktu      time.Time `json:"waktu"`       // kejadian pertama pada baris ini
	WaktuAkhir time.Time `json:"waktu_akhir"` // kejadian terakhir
	Level      Level     `json:"level"`
	Sumber     string    `json:"sumber"`
	Pesan      string    `json:"pesan"`
	Detail     string    `json:"detail,omitempty"`
	Jumlah     int       `json:"jumlah"`
	Rute       string    `json:"rute,omitempty"`
	Metode     string    `json:"metode,omitempty"`
	Status     int       `json:"status,omitempty"`
	DurasiMS   int       `json:"durasi_ms,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
	UserID     string    `json:"user_id,omitempty"`
	Sidik      string    `json:"sidik"`
}

// Statistik: keadaan perekam (untuk ditampilkan agar terlihat bila ada catatan yang tidak sempat disimpan).
type Statistik struct {
	Diterima   int64 `json:"diterima"`   // kejadian yang masuk antrean
	Ditulis    int64 `json:"ditulis"`    // kejadian yang sudah tersimpan di database (termasuk yang digabung)
	Dijatuhkan int64 `json:"dijatuhkan"` // kejadian yang dibuang karena antrean/penampung penuh (badai galat)
	Gagal      int64 `json:"gagal"`      // percobaan menulis ke database yang gagal (akan dicoba lagi)
	Antrean    int   `json:"antrean"`
	Kapasitas  int   `json:"kapasitas"`
	Tertunda   int   `json:"tertunda"` // kelompok yang menunggu ditulis (atau dicoba ulang)
	// DibatalkanKlien: galat "context canceled" yang sengaja tidak dicatat karena hanya berarti pengguna menutup tab atau membatalkan permintaannya, bukan masalah server.
	DibatalkanKlien int64 `json:"dibatalkan_klien"`
}
