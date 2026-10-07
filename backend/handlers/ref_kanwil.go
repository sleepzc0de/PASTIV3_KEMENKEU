package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// Referensi Kantor Wilayah (Kanwil, migrasi 056): kode 9 digit (9 karakter pertama kode satker = KL 3 + UE1 2 + wilayah 4, mis. 015040199) -> uraian dan singkatan.
// Seperti referensi UE1: dibaca semua pengguna yang punya peran, diubah superadmin. Isinya berasal dari tiga jalur (kolom sumber):
//   - "satker": nama satker pada data Digitalisasi Aset yang kode satkernya berawalan kode Kanwil itu (lihat sqlKanwilBelumTerdaftar);
//   - "sldk"  : ditarik dari SLDK, tabel DJKN.SIMAN2_R_KORWIL (kd_wileselon 9 karakter -> ur_korwil), hanya untuk KL 015;
//   - "manual": diisi atau diubah superadmin. Penarikan dari SLDK tidak pernah menimpa baris manual.

var reKodeKanwil = regexp.MustCompile(`^[0-9]{9}$`)

const (
	refKanwilNamaMaks      = 200
	refKanwilSingkatanMaks = 30
	refKanwilUrutanMaks    = 9999
	refKanwilStatusMaks    = 50
	refKanwilUrutanBawaan  = 100

	// KL Kementerian Keuangan: sama dengan filter query Digitalisasi Aset (kd_satker LIKE '015%').
	kodeKLReferensi = "015"
	// Batas pengaman jumlah baris yang dibaca dari SLDK (SIMAN2_R_KORWIL seluruh KL sekitar 17 ribu baris; satu KL jauh lebih sedikit).
	refKanwilBarisSLDKMaks = 20000

	sumberKanwilManual = "manual"
	sumberKanwilSatker = "satker"
	sumberKanwilSLDK   = "sldk"
)

type RefKanwilBaris struct {
	Kode       string     `json:"kode"`
	KodeUE1    string     `json:"kode_ue1"`
	Nama       string     `json:"nama"`
	Singkatan  string     `json:"singkatan"`
	Urutan     int        `json:"urutan"`
	Aktif      bool       `json:"aktif"`
	Sumber     string     `json:"sumber"`
	StatusSLDK string     `json:"status_sldk,omitempty"`
	DiubahOleh string     `json:"diubah_oleh,omitempty"`
	DiubahPada *time.Time `json:"diubah_pada,omitempty"`
}

const kolomRefKanwil = `kode, nama, ISNULL(singkatan, N''), urutan, aktif, sumber, ISNULL(status_sldk, N''), ISNULL(diubah_oleh, N''), diubah_pada`

func pindaiRefKanwil(r interface{ Scan(...interface{}) error }) (RefKanwilBaris, error) {
	var b RefKanwilBaris
	var pada time.Time
	if err := r.Scan(&b.Kode, &b.Nama, &b.Singkatan, &b.Urutan, &b.Aktif, &b.Sumber, &b.StatusSLDK, &b.DiubahOleh, &pada); err != nil {
		return b, err
	}
	b.KodeUE1, b.DiubahPada = b.Kode[:5], &pada
	return b, nil
}

func daftarRefKanwil(ctx context.Context) ([]RefKanwilBaris, error) {
	rows, err := database.DB.QueryContext(ctx, `SELECT `+kolomRefKanwil+` FROM ref_kanwil ORDER BY urutan, kode`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RefKanwilBaris{}
	for rows.Next() {
		b, err := pindaiRefKanwil(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// kanwilBelumTerdaftar: kode Kanwil (9 karakter pertama kode satker) pada data satker Digitalisasi Aset yang belum ada di referensi, beserta jumlah satkernya dan saran
// uraian dari data satker itu sendiri (kosong bila tidak ada satker yang namanya menyiratkan kantor wilayah/pusat).
type kanwilBelumTerdaftar struct {
	Kode    string `json:"kode"`
	KodeUE1 string `json:"kode_ue1"`
	Satker  int64  `json:"satker"`
	Saran   string `json:"saran"`
}

// Saran uraian: satker (pada kode Kanwil yang sama) yang namanya menyebut kantor wilayah atau kantor pusat; induk satker lebih dulu, lalu nama terpendek, lalu kode terkecil.
// Ini tebakan dari nama; kode yang tidak punya satker semacam itu tidak diberi saran (diisi manual atau ditarik dari SLDK).
var kataKunciSaranKanwil = []string{"KANTOR WILAYAH", "KANWIL", "KANTOR PUSAT"}

// sqlKanwilBelumTerdaftar menyusun SELECT (kode, jumlah satker, saran) untuk kode Kanwil yang belum ada di ref_kanwil. Sumber data mengikuti cakupan peran pembaca
// (dgSumberAlias). Teks SQL hanya berisi nama tabel/kolom tetap dan kata kunci tetap di atas, tanpa masukan pengguna.
func sqlKanwilBelumTerdaftar(ctx context.Context) string {
	var cocok []string
	for _, k := range kataKunciSaranKanwil {
		cocok = append(cocok, "x.Nama_Satker LIKE N'%"+k+"%'")
	}
	return `SELECT k.kode, k.satker, ISNULL(sar.nama, N'') AS saran
		FROM (
			SELECT LEFT(s.Kode_Satker, 9) AS kode, COUNT_BIG(*) AS satker
			FROM ` + dgSumberAlias(ctx, "DIGITALISASI_SATKER", "Kode_Satker", "s") + `
			WHERE s.Kode_Satker IS NOT NULL AND LEN(s.Kode_Satker) >= 9 AND LEFT(s.Kode_Satker, 9) NOT LIKE N'%[^0-9]%'
			GROUP BY LEFT(s.Kode_Satker, 9)
		) k
		OUTER APPLY (
			SELECT TOP 1 LEFT(LTRIM(RTRIM(x.Nama_Satker)), ` + fmt.Sprint(refKanwilNamaMaks) + `) AS nama
			FROM ` + dgSumberAlias(ctx, "DIGITALISASI_SATKER", "Kode_Satker", "x") + `
			WHERE LEFT(x.Kode_Satker, 9) = k.kode AND x.Nama_Satker IS NOT NULL AND LEN(LTRIM(RTRIM(x.Nama_Satker))) > 0
			  AND (` + strings.Join(cocok, " OR ") + `)
			ORDER BY CASE WHEN x.Jenis_Satker = N'INDUK SATKER' THEN 0 ELSE 1 END, LEN(x.Nama_Satker), x.Kode_Satker
		) sar
		WHERE NOT EXISTS (SELECT 1 FROM ref_kanwil r WHERE r.kode = k.kode)`
}

func daftarKanwilBelumTerdaftar(ctx context.Context) []kanwilBelumTerdaftar {
	out := []kanwilBelumTerdaftar{}
	rows, err := database.DB.QueryContext(ctx, sqlKanwilBelumTerdaftar(ctx)+` ORDER BY k.kode`)
	if err != nil {
		log.Println("[REF KANWIL WARN] gagal membaca kode Kanwil yang belum terdaftar:", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k kanwilBelumTerdaftar
		if rows.Scan(&k.Kode, &k.Satker, &k.Saran) == nil {
			k.KodeUE1 = k.Kode[:5]
			out = append(out, k)
		}
	}
	return out
}

// GetRefKanwil: GET /referensi/kanwil. Semua pengguna yang punya peran; memuat juga kode Kanwil di data aset yang belum punya referensi (menurut cakupan peran pembaca)
// dan apakah koneksi SLDK tersedia untuk penarikan.
func GetRefKanwil(c *gin.Context) {
	ctx, cancel := dgKonteks(c, 15*time.Second)
	defer cancel()
	daftar, err := daftarRefKanwil(ctx)
	if err != nil {
		log.Println("[REF KANWIL ERROR] baca:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca referensi Kanwil")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil referensi Kanwil", gin.H{
		"daftar": daftar, "belum_terdaftar": daftarKanwilBelumTerdaftar(ctx), "sldk_tersedia": database.SLDKDB != nil,
	})
}

type refKanwilMasukan struct {
	Nama      string `json:"nama"`
	Singkatan string `json:"singkatan"`
	Urutan    *int   `json:"urutan"`
	Aktif     *bool  `json:"aktif"`
}

// validasiRefKanwil merapikan masukan dan mengembalikan pesan galat pertama (kosong bila sah).
func validasiRefKanwil(kode string, m *refKanwilMasukan) string {
	if !reKodeKanwil.MatchString(kode) {
		return "Kode Kanwil harus 9 digit angka"
	}
	m.Nama = strings.Join(strings.Fields(m.Nama), " ")
	m.Singkatan = strings.Join(strings.Fields(m.Singkatan), " ")
	switch {
	case m.Nama == "":
		return "Uraian Kanwil wajib diisi"
	case utf8.RuneCountInString(m.Nama) > refKanwilNamaMaks:
		return "Uraian Kanwil terlalu panjang (maksimal 200 karakter)"
	case utf8.RuneCountInString(m.Singkatan) > refKanwilSingkatanMaks:
		return "Singkatan terlalu panjang (maksimal 30 karakter)"
	case m.Urutan != nil && (*m.Urutan < 0 || *m.Urutan > refKanwilUrutanMaks):
		return "Urutan harus antara 0 dan 9999"
	}
	return ""
}

// PutRefKanwil: PUT /referensi/kanwil/:kode (superadmin). Membuat kode baru atau mengubah yang ada; barisnya selalu bertanda sumber "manual" sesudahnya, sehingga penarikan
// dari SLDK tidak menimpanya. Urutan bawaan 100 dan aktif bawaan true untuk kode baru; kode lama hanya berubah bila dikirim.
func PutRefKanwil(c *gin.Context) {
	kode := strings.TrimSpace(c.Param("kode"))
	var m refKanwilMasukan
	if err := c.ShouldBindJSON(&m); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	if pesan := validasiRefKanwil(kode, &m); pesan != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, pesan)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var singkatan sql.NullString
	if m.Singkatan != "" {
		singkatan = sql.NullString{String: m.Singkatan, Valid: true}
	}
	urutanBaru, aktifBaru := refKanwilUrutanBawaan, true
	if m.Urutan != nil {
		urutanBaru = *m.Urutan
	}
	if m.Aktif != nil {
		aktifBaru = *m.Aktif
	}
	oleh := c.GetString("username")

	err := func() error {
		tx, err := database.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback() // tidak berpengaruh setelah Commit
		res, err := tx.ExecContext(ctx, `
			UPDATE ref_kanwil WITH (UPDLOCK, SERIALIZABLE)
			SET nama = @p2, singkatan = @p3, urutan = COALESCE(CAST(@p4 AS INT), urutan), aktif = COALESCE(CAST(@p5 AS BIT), aktif),
			    sumber = N'`+sumberKanwilManual+`', diubah_oleh = @p6, diubah_pada = SYSUTCDATETIME()
			WHERE kode = @p1`, kode, m.Nama, singkatan, nullInt(m.Urutan), nullBool(m.Aktif), oleh)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO ref_kanwil (kode, nama, singkatan, urutan, aktif, sumber, diubah_oleh) VALUES (@p1, @p2, @p3, @p4, @p5, N'`+sumberKanwilManual+`', @p6)`,
				kode, m.Nama, singkatan, urutanBaru, aktifBaru, oleh); err != nil {
				return err
			}
		}
		return tx.Commit()
	}()
	if err != nil {
		log.Println("[REF KANWIL ERROR] simpan:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyimpan referensi Kanwil")
		return
	}
	r, err := pindaiRefKanwil(database.DB.QueryRowContext(ctx, `SELECT `+kolomRefKanwil+` FROM ref_kanwil WHERE kode = @p1`, kode))
	if err != nil {
		log.Println("[REF KANWIL ERROR] baca ulang:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Referensi tersimpan, tetapi gagal dibaca ulang")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Referensi Kanwil disimpan", r)
}

// DeleteRefKanwil: DELETE /referensi/kanwil/:kode (superadmin). Data aset dan peran pengguna tidak terpengaruh; kodenya kembali tampil tanpa uraian.
func DeleteRefKanwil(c *gin.Context) {
	kode := strings.TrimSpace(c.Param("kode"))
	if !reKodeKanwil.MatchString(kode) {
		utils.ErrorResponse(c, http.StatusBadRequest, "Kode Kanwil harus 9 digit angka")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	res, err := database.DB.ExecContext(ctx, `DELETE FROM ref_kanwil WHERE kode = @p1`, kode)
	if err != nil {
		log.Println("[REF KANWIL ERROR] hapus:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menghapus referensi Kanwil")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Kode Kanwil tidak ditemukan")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Referensi Kanwil dihapus", gin.H{"kode": kode})
}

// PostRefKanwilDariSatker: POST /referensi/kanwil/dari-satker (superadmin). Menambahkan SEMUA kode Kanwil di data satker yang belum punya referensi dan punya saran uraian
// (sumber "satker"). Kode tanpa saran dibiarkan; jumlahnya dikembalikan supaya bisa diisi manual atau ditarik dari SLDK. Tidak pernah mengubah baris yang sudah ada.
func PostRefKanwilDariSatker(c *gin.Context) {
	ctx, cancel := dgKonteks(c, 30*time.Second)
	defer cancel()
	res, err := database.DB.ExecContext(ctx, `INSERT INTO ref_kanwil (kode, nama, urutan, sumber, diubah_oleh)
		SELECT t.kode, t.saran, `+fmt.Sprint(refKanwilUrutanBawaan)+`, N'`+sumberKanwilSatker+`', @p1 FROM (`+sqlKanwilBelumTerdaftar(ctx)+`) t WHERE t.saran <> N''`, c.GetString("username"))
	if err != nil {
		log.Println("[REF KANWIL ERROR] tambah dari satker:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menambahkan referensi Kanwil dari data satker")
		return
	}
	n, _ := res.RowsAffected()
	log.Printf("[REF KANWIL] %d kode Kanwil ditambahkan dari data satker oleh %s", n, c.GetString("username"))
	utils.SuccessResponse(c, http.StatusOK, "Referensi Kanwil ditambahkan dari data satker", gin.H{"ditambahkan": n, "tanpa_saran": len(daftarKanwilBelumTerdaftar(ctx))})
}

// ---------------------------------------------------------------- tarik dari SLDK

// kanwilSLDK: satu baris SIMAN2_R_KORWIL yang dibutuhkan. Kolom pribadi pada tabel itu (nip, nama, jabatan, email, telepon) sengaja tidak dibaca.
type kanwilSLDK struct{ Kode, Nama, Status string }

// Dedup per kd_wileselon (tabel sumber heap tanpa kunci; kolom updated_at/id_korwil memilih baris terbaru), hanya KL 015.
const sqlKanwilSLDK = `SELECT kode, nama, status FROM (
	SELECT LTRIM(RTRIM(K.kd_wileselon)) AS kode, K.ur_korwil AS nama, K.status_korwil AS status,
	       ROW_NUMBER() OVER (PARTITION BY LTRIM(RTRIM(K.kd_wileselon)) ORDER BY K.updated_at DESC, K.id_korwil DESC) AS rn
	FROM DJKN.SIMAN2_R_KORWIL AS K
	WHERE K.kd_wileselon LIKE N'` + kodeKLReferensi + `%'
) X WHERE X.rn = 1 ORDER BY X.kode`

func bacaKanwilSLDK(ctx context.Context, sldk *sql.DB) ([]kanwilSLDK, error) {
	rows, err := sldk.QueryContext(ctx, sqlKanwilSLDK)
	if err != nil {
		return nil, fmt.Errorf("query SLDK gagal: %w", err)
	}
	defer rows.Close()
	var out []kanwilSLDK
	for rows.Next() {
		var k, n, s sql.NullString
		if err := rows.Scan(&k, &n, &s); err != nil {
			return nil, fmt.Errorf("baca baris SLDK: %w", err)
		}
		if len(out) >= refKanwilBarisSLDKMaks {
			return nil, fmt.Errorf("SLDK mengembalikan lebih dari %d baris; periksa filter query", refKanwilBarisSLDKMaks)
		}
		out = append(out, kanwilSLDK{Kode: strings.TrimSpace(k.String), Nama: n.String, Status: s.String})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query SLDK terputus: %w", err)
	}
	return out, nil
}

// HasilTarikKanwil merangkum penarikan dari SLDK.
type HasilTarikKanwil struct {
	Dibaca         int `json:"dibaca"`
	Ditambahkan    int `json:"ditambahkan"`
	Diperbarui     int `json:"diperbarui"`
	TanpaPerubahan int `json:"tanpa_perubahan"`
	DilewatiManual int `json:"dilewati_manual"` // baris yang diisi superadmin tidak pernah ditimpa
	TidakSah       int `json:"tidak_sah"`       // kode bukan 9 digit angka, uraian kosong, atau kode ganda
	Dipotong       int `json:"dipotong"`        // uraian atau status yang melebihi panjang kolom
}

func potongRune(s string, maks int) (string, bool) {
	if utf8.RuneCountInString(s) <= maks {
		return s, false
	}
	return string([]rune(s)[:maks]), true
}

// terapkanKanwilSLDK menulis hasil baca SLDK ke ref_kanwil dalam satu transaksi: kode baru ditambahkan (sumber "sldk"), baris bersumber "satker" atau "sldk" diperbarui bila
// berbeda, baris "manual" tidak disentuh.
func terapkanKanwilSLDK(ctx context.Context, db *sql.DB, baris []kanwilSLDK, oleh string) (HasilTarikKanwil, error) {
	h := HasilTarikKanwil{Dibaca: len(baris)}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return h, err
	}
	defer tx.Rollback() // tidak berpengaruh setelah Commit

	type ada struct{ sumber, nama, status string }
	existing := map[string]ada{}
	rows, err := tx.QueryContext(ctx, `SELECT kode, sumber, nama, ISNULL(status_sldk, N'') FROM ref_kanwil WITH (UPDLOCK, HOLDLOCK)`)
	if err != nil {
		return h, err
	}
	for rows.Next() {
		var k string
		var a ada
		if err := rows.Scan(&k, &a.sumber, &a.nama, &a.status); err != nil {
			rows.Close()
			return h, err
		}
		existing[k] = a
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return h, err
	}
	rows.Close()

	sudah := map[string]bool{}
	for _, b := range baris {
		kode := strings.TrimSpace(b.Kode)
		nama, p1 := potongRune(strings.Join(strings.Fields(b.Nama), " "), refKanwilNamaMaks)
		status, p2 := potongRune(strings.Join(strings.Fields(b.Status), " "), refKanwilStatusMaks)
		if !reKodeKanwil.MatchString(kode) || nama == "" || sudah[kode] {
			h.TidakSah++
			continue
		}
		sudah[kode] = true
		if p1 || p2 {
			h.Dipotong++
		}
		var st interface{}
		if status != "" {
			st = status
		}
		a, dikenal := existing[kode]
		switch {
		case !dikenal:
			if _, err := tx.ExecContext(ctx, `INSERT INTO ref_kanwil (kode, nama, urutan, sumber, status_sldk, diubah_oleh) VALUES (@p1, @p2, @p3, N'`+sumberKanwilSLDK+`', @p4, @p5)`,
				kode, nama, refKanwilUrutanBawaan, st, oleh); err != nil {
				return h, err
			}
			h.Ditambahkan++
		case a.sumber == sumberKanwilManual:
			h.DilewatiManual++
		case a.sumber == sumberKanwilSLDK && a.nama == nama && a.status == status:
			h.TanpaPerubahan++
		default:
			if _, err := tx.ExecContext(ctx, `UPDATE ref_kanwil SET nama = @p2, sumber = N'`+sumberKanwilSLDK+`', status_sldk = @p3, diubah_oleh = @p4, diubah_pada = SYSUTCDATETIME() WHERE kode = @p1`,
				kode, nama, st, oleh); err != nil {
				return h, err
			}
			h.Diperbarui++
		}
	}
	if err := tx.Commit(); err != nil {
		return h, err
	}
	return h, nil
}

var tarikKanwilBerjalan atomic.Bool

// PostRefKanwilTarikSLDK: POST /referensi/kanwil/tarik-sldk (superadmin). Membaca DJKN.SIMAN2_R_KORWIL (KL 015) dari SLDK lalu memperbarui referensi. Tabelnya kecil (belasan ribu
// baris untuk semua KL), jadi dijalankan langsung dalam satu permintaan. Satu penarikan dalam satu waktu; SLDK yang tidak mengembalikan baris tidak mengubah apa pun.
func PostRefKanwilTarikSLDK(c *gin.Context) {
	sldk := database.SLDKDB
	if sldk == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Koneksi ke SLDK belum tersedia. Periksa pengaturan SLDK_DB_* di server.")
		return
	}
	if !tarikKanwilBerjalan.CompareAndSwap(false, true) {
		utils.ErrorResponse(c, http.StatusConflict, "Penarikan referensi Kanwil lain sedang berjalan. Tunggu sampai selesai.")
		return
	}
	defer tarikKanwilBerjalan.Store(false)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	baris, err := bacaKanwilSLDK(ctx, sldk)
	if err != nil {
		log.Println("[REF KANWIL ERROR] baca SLDK:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal membaca referensi Kanwil dari SLDK. Periksa koneksi dan hak akses ke SLDK.")
		return
	}
	if len(baris) == 0 {
		utils.ErrorResponse(c, http.StatusConflict, "SLDK tidak mengembalikan satu baris pun untuk KL "+kodeKLReferensi+"; referensi tidak diubah. Periksa akses ke tabel SIMAN2_R_KORWIL.")
		return
	}
	oleh := c.GetString("username")
	hasil, err := terapkanKanwilSLDK(ctx, database.DB, baris, oleh)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Println("[REF KANWIL ERROR] simpan hasil SLDK melewati batas waktu")
		} else {
			log.Println("[REF KANWIL ERROR] simpan hasil SLDK:", err)
		}
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyimpan referensi Kanwil dari SLDK; tidak ada yang diubah.")
		return
	}
	log.Printf("[REF KANWIL] penarikan SLDK oleh %s: %+v", oleh, hasil)
	utils.SuccessResponse(c, http.StatusOK, "Referensi Kanwil ditarik dari SLDK", hasil)
}
