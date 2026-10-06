package handlers

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// Referensi Unit Eselon I (migrasi 052): kode 5 digit -> uraian dan singkatan. Satu daftar untuk seluruh aplikasi (Digitalisasi Aset, Dashboard,
// ekspor, SAPA). Dibaca semua pengguna login; diubah admin/superadmin.

var reKodeUE1 = regexp.MustCompile(`^[0-9]{5}$`)

const (
	refUE1NamaMaks      = 200
	refUE1SingkatanMaks = 30
	refUE1UrutanMaks    = 9999
)

type RefUE1Baris struct {
	Kode       string     `json:"kode"`
	Nama       string     `json:"nama"`
	Singkatan  string     `json:"singkatan"`
	Urutan     int        `json:"urutan"`
	Aktif      bool       `json:"aktif"`
	DiubahOleh string     `json:"diubah_oleh,omitempty"`
	DiubahPada *time.Time `json:"diubah_pada,omitempty"`
}

// Label singkat untuk grafik dan pilihan: "01504 · DJP". Tanpa singkatan dipakai uraiannya; kode yang belum ada di referensi menjadi "UE1 01504".
func (r RefUE1Baris) Label() string {
	switch {
	case r.Singkatan != "":
		return r.Kode + " · " + r.Singkatan
	case r.Nama != "":
		return r.Kode + " · " + r.Nama
	}
	return "UE1 " + r.Kode
}

// muatRefUE1 membaca seluruh referensi (kecil, belasan baris) menjadi peta menurut kode. Dibaca ulang tiap permintaan supaya perubahan admin
// langsung terlihat tanpa cache yang perlu dikosongkan.
func muatRefUE1(ctx context.Context) (map[string]RefUE1Baris, error) {
	daftar, err := daftarRefUE1(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]RefUE1Baris, len(daftar))
	for _, r := range daftar {
		out[r.Kode] = r
	}
	return out, nil
}

func daftarRefUE1(ctx context.Context) ([]RefUE1Baris, error) {
	rows, err := database.DB.QueryContext(ctx, `SELECT kode, nama, ISNULL(singkatan, N''), urutan, aktif, ISNULL(diubah_oleh, N''), diubah_pada FROM ref_ue1 ORDER BY urutan, kode`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RefUE1Baris{}
	for rows.Next() {
		var r RefUE1Baris
		var pada time.Time
		if err := rows.Scan(&r.Kode, &r.Nama, &r.Singkatan, &r.Urutan, &r.Aktif, &r.DiubahOleh, &pada); err != nil {
			return nil, err
		}
		r.DiubahPada = &pada
		out = append(out, r)
	}
	return out, rows.Err()
}

// labelUE1: label untuk kode UE1 menurut peta referensi (peta boleh nil: referensi gagal dibaca).
func labelUE1(kode string, ref map[string]RefUE1Baris) string {
	if kode == "" || kode == "(kosong)" {
		return "(kosong)"
	}
	if r, ada := ref[kode]; ada {
		return r.Label()
	}
	return "UE1 " + kode
}

// kodeUE1BelumTerdaftar: kode UE1 yang muncul di data satker Digitalisasi Aset tetapi belum ada di referensi, beserta jumlah satkernya.
type kodeUE1BelumTerdaftar struct {
	Kode   string `json:"kode"`
	Satker int64  `json:"satker"`
}

func ue1BelumTerdaftar(ctx context.Context) []kodeUE1BelumTerdaftar {
	out := []kodeUE1BelumTerdaftar{}
	rows, err := database.DB.QueryContext(ctx, `SELECT s.Kode_UE1, COUNT_BIG(*) FROM `+dgSumberAlias(ctx, "DIGITALISASI_SATKER", "Kode_Satker", "s")+`
		WHERE s.Kode_UE1 IS NOT NULL AND NOT EXISTS (SELECT 1 FROM ref_ue1 r WHERE r.kode = s.Kode_UE1)
		GROUP BY s.Kode_UE1 ORDER BY s.Kode_UE1`)
	if err != nil {
		log.Println("[REF UE1 WARN] gagal membaca kode UE1 yang belum terdaftar:", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k kodeUE1BelumTerdaftar
		if rows.Scan(&k.Kode, &k.Satker) == nil {
			out = append(out, k)
		}
	}
	return out
}

// GetRefUE1: GET /referensi/ue1. Semua pengguna login; hasilnya juga memuat kode di data aset yang belum punya referensi.
func GetRefUE1(c *gin.Context) {
	ctx, cancel := dgKonteks(c, 10*time.Second)
	defer cancel()
	daftar, err := daftarRefUE1(ctx)
	if err != nil {
		log.Println("[REF UE1 ERROR] baca:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca referensi UE1")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil referensi UE1", gin.H{"daftar": daftar, "belum_terdaftar": ue1BelumTerdaftar(ctx)})
}

type refUE1Masukan struct {
	Nama      string `json:"nama"`
	Singkatan string `json:"singkatan"`
	Urutan    *int   `json:"urutan"`
	Aktif     *bool  `json:"aktif"`
}

// validasiRefUE1 merapikan masukan dan mengembalikan pesan galat pertama (kosong bila sah).
func validasiRefUE1(kode string, m *refUE1Masukan) string {
	if !reKodeUE1.MatchString(kode) {
		return "Kode UE1 harus 5 digit angka"
	}
	m.Nama = strings.Join(strings.Fields(m.Nama), " ")
	m.Singkatan = strings.Join(strings.Fields(m.Singkatan), " ")
	switch {
	case m.Nama == "":
		return "Uraian UE1 wajib diisi"
	case utf8.RuneCountInString(m.Nama) > refUE1NamaMaks:
		return "Uraian UE1 terlalu panjang (maksimal 200 karakter)"
	case utf8.RuneCountInString(m.Singkatan) > refUE1SingkatanMaks:
		return "Singkatan terlalu panjang (maksimal 30 karakter)"
	case m.Urutan != nil && (*m.Urutan < 0 || *m.Urutan > refUE1UrutanMaks):
		return "Urutan harus antara 0 dan 9999"
	}
	return ""
}

// PutRefUE1: PUT /referensi/ue1/:kode (admin/superadmin). Membuat kode baru atau mengubah yang ada. Urutan bawaan 100 untuk kode baru dan
// tidak berubah untuk kode lama; aktif bawaan true untuk kode baru.
func PutRefUE1(c *gin.Context) {
	kode := strings.TrimSpace(c.Param("kode"))
	var m refUE1Masukan
	if err := c.ShouldBindJSON(&m); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	if pesan := validasiRefUE1(kode, &m); pesan != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, pesan)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var singkatan sql.NullString
	if m.Singkatan != "" {
		singkatan = sql.NullString{String: m.Singkatan, Valid: true}
	}
	urutanBaru, aktifBaru := 100, true
	if m.Urutan != nil {
		urutanBaru = *m.Urutan
	}
	if m.Aktif != nil {
		aktifBaru = *m.Aktif
	}
	oleh := c.GetString("username")

	// Kode lama: urutan/aktif hanya berubah bila dikirim. Kode baru: nilai bawaan di atas. Dalam satu transaksi dengan kunci rentang, jadi dua
	// permintaan bersamaan untuk kode baru yang sama tidak bertabrakan di kunci utama.
	err := func() error {
		tx, err := database.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback() // tidak berpengaruh setelah Commit
		res, err := tx.ExecContext(ctx, `
			UPDATE ref_ue1 WITH (UPDLOCK, SERIALIZABLE)
			SET nama = @p2, singkatan = @p3, urutan = COALESCE(CAST(@p4 AS INT), urutan), aktif = COALESCE(CAST(@p5 AS BIT), aktif),
			    diubah_oleh = @p6, diubah_pada = SYSUTCDATETIME()
			WHERE kode = @p1`, kode, m.Nama, singkatan, nullInt(m.Urutan), nullBool(m.Aktif), oleh)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO ref_ue1 (kode, nama, singkatan, urutan, aktif, diubah_oleh) VALUES (@p1, @p2, @p3, @p4, @p5, @p6)`,
				kode, m.Nama, singkatan, urutanBaru, aktifBaru, oleh); err != nil {
				return err
			}
		}
		return tx.Commit()
	}()
	if err != nil {
		log.Println("[REF UE1 ERROR] simpan:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyimpan referensi UE1")
		return
	}
	var r RefUE1Baris
	var pada time.Time
	if err := database.DB.QueryRowContext(ctx, `SELECT kode, nama, ISNULL(singkatan, N''), urutan, aktif, ISNULL(diubah_oleh, N''), diubah_pada FROM ref_ue1 WHERE kode = @p1`, kode).
		Scan(&r.Kode, &r.Nama, &r.Singkatan, &r.Urutan, &r.Aktif, &r.DiubahOleh, &pada); err != nil {
		log.Println("[REF UE1 ERROR] baca ulang:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Referensi tersimpan, tetapi gagal dibaca ulang")
		return
	}
	r.DiubahPada = &pada
	utils.SuccessResponse(c, http.StatusOK, "Referensi UE1 disimpan", r)
}

// DeleteRefUE1: DELETE /referensi/ue1/:kode (admin/superadmin). Data aset tidak terpengaruh; kode itu kembali tampil sebagai "UE1 <kode>".
func DeleteRefUE1(c *gin.Context) {
	kode := strings.TrimSpace(c.Param("kode"))
	if !reKodeUE1.MatchString(kode) {
		utils.ErrorResponse(c, http.StatusBadRequest, "Kode UE1 harus 5 digit angka")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	res, err := database.DB.ExecContext(ctx, `DELETE FROM ref_ue1 WHERE kode = @p1`, kode)
	if err != nil {
		log.Println("[REF UE1 ERROR] hapus:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menghapus referensi UE1")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Kode UE1 tidak ditemukan")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Referensi UE1 dihapus", gin.H{"kode": kode})
}

func nullInt(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func nullBool(p *bool) interface{} {
	if p == nil {
		return nil
	}
	return *p
}
