package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// Pencarian pegawai HRIS2 untuk formulir SAPA (anggota tim): nama dan jabatan terisi otomatis. Memakai sesi SSO milik
// pengguna yang sedang login, sama seperti fitur HRIS2 di halaman admin, tetapi bisa dipakai pengguna SAPA mana pun dan
// HANYA mengembalikan bidang yang dibutuhkan formulir: NIP, nama, jabatan, dan satuan kerja. Tanggal lahir, kontak, dan
// data pribadi lain tidak pernah diteruskan.

const (
	pegawaiMinQuery = 3
	pegawaiMaxHasil = 15
)

// Titik sambung yang diganti tes dan server uji (lihat GunakanHRIS2).
var (
	hris2BaseURLFn  = config.GetHRIS2BaseURL
	hris2TokenFn    = getValidAccessToken
	hris2ProviderFn = func(userID string) (string, error) {
		var p string
		err := database.DB.QueryRow(`SELECT auth_provider FROM users WHERE id = @p1`, userID).Scan(&p)
		return p, err
	}
)

// GunakanHRIS2 mengganti alamat dasar HRIS2, pengambil token SSO, dan pencari jenis akun (untuk tes). Mengembalikan
// fungsi pemulih.
func GunakanHRIS2(base func() string, token func(string) (string, error), provider func(string) (string, error)) (pulihkan func()) {
	b, t, p := hris2BaseURLFn, hris2TokenFn, hris2ProviderFn
	hris2BaseURLFn, hris2TokenFn, hris2ProviderFn = base, token, provider
	return func() { hris2BaseURLFn, hris2TokenFn, hris2ProviderFn = b, t, p }
}

// PegawaiSapa: bidang yang diteruskan ke formulir.
type PegawaiSapa struct {
	NIP         string `json:"nip"`
	Nama        string `json:"nama"`
	NamaLengkap string `json:"nama_lengkap"` // dengan gelar depan/belakang, untuk dokumen resmi
	Jabatan     string `json:"jabatan"`
	Satker      string `json:"satker"`
}

var (
	reQueryPegawai = regexp.MustCompile(`[^\p{L}\p{N} .'\-]+`)
	reNIPPegawai   = regexp.MustCompile(`^[0-9]{8,18}$`)
)

// bersihQueryPegawai membuang karakter yang bermakna khusus pada filter HRIS2 (koma, "|", kurung, garis miring terbalik,
// dan sebagainya) supaya masukan pengguna tidak bisa mengubah isi filter. Sisanya: huruf, angka, spasi, titik, minus.
func bersihQueryPegawai(q string) string {
	q = reQueryPegawai.ReplaceAllString(q, " ")
	return strings.Join(strings.Fields(q), " ")
}

func strField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				return s
			}
		}
	}
	return ""
}

func jabatanDari(m map[string]interface{}) string {
	if s := strField(m, "namaJabatan", "NamaJabatan"); s != "" {
		return s
	}
	if arr, ok := m["jabatan"].([]interface{}); ok && len(arr) > 0 {
		if first, ok := arr[0].(map[string]interface{}); ok {
			return strField(first, "namaJabatan")
		}
	}
	return ""
}

// namaDenganGelar: "Dr. Budi Santoso, S.Kom., M.M."; gelar yang kosong dilewati.
func namaDenganGelar(nama, depan, belakang string) string {
	d := strings.TrimSpace(strings.Join([]string{depan, nama}, " "))
	if belakang != "" && d != "" {
		return d + ", " + belakang
	}
	return d
}

func pegawaiDari(m map[string]interface{}) (PegawaiSapa, bool) {
	nama := strField(m, "nama", "Nama")
	if nama == "" {
		return PegawaiSapa{}, false
	}
	return PegawaiSapa{
		NIP:         strField(m, "nip18", "Nip18", "nip", "NIP"),
		Nama:        nama,
		NamaLengkap: namaDenganGelar(nama, strField(m, "gelarDepan", "GelarDepan"), strField(m, "gelarBelakang", "GelarBelakang")),
		Jabatan:     jabatanDari(m),
		Satker:      strField(m, "namaSatker", "NamaSatker"),
	}, true
}

// barisPegawai membaca daftar pegawai dari isi respons HRIS2. Isi `data` bisa berupa larik, objek berhalaman
// ({ items: [...] }), atau satu objek pegawai; bentuknya belum terdokumentasi, jadi dibaca longgar (cermin extractRows di
// frontend/components/hris2/pegawai.ts).
func barisPegawai(raw interface{}) []map[string]interface{} {
	var inner interface{} = raw
	if m, ok := raw.(map[string]interface{}); ok {
		if d, ok := m["data"]; ok {
			inner = d
		}
	}
	toRows := func(arr []interface{}) []map[string]interface{} {
		var out []map[string]interface{}
		for _, x := range arr {
			if r, ok := x.(map[string]interface{}); ok {
				out = append(out, r)
			}
		}
		return out
	}
	switch v := inner.(type) {
	case []interface{}:
		return toRows(v)
	case map[string]interface{}:
		for _, k := range []string{"items", "result", "results", "data", "list", "rows"} {
			if arr, ok := v[k].([]interface{}); ok {
				return toRows(arr)
			}
		}
		if _, ok := pegawaiDari(v); ok {
			return []map[string]interface{}{v}
		}
	}
	return nil
}

// tokenHRIS2 memeriksa bahwa pengguna login via SSO dan mengambil token SSO-nya. Mengembalikan false setelah menjawab galat.
func tokenHRIS2(c *gin.Context) (string, bool) {
	userID := c.GetString("user_id")
	provider, err := hris2ProviderFn(userID)
	if err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "User tidak ditemukan")
		return "", false
	}
	if provider != "sso" {
		utils.ErrorResponse(c, http.StatusForbidden, "Pencarian pegawai HRIS2 hanya tersedia untuk pengguna yang login via SSO Kemenkeu.")
		return "", false
	}
	token, err := hris2TokenFn(userID)
	if err != nil {
		log.Println("[SAPA HRIS2] gagal mengambil token SSO:", err)
		utils.ErrorResponseWithCode(c, http.StatusUnauthorized, "Sesi SSO Anda telah berakhir, silakan logout lalu login ulang via SSO Kemenkeu", utils.CodeSSOSessionExpired)
		return "", false
	}
	return token, true
}

// panggilHRIS2 melakukan GET ke HRIS2 dan mengembalikan isi respons JSON. Galat dijawab langsung ke klien (false).
func panggilHRIS2(c *gin.Context, ctx context.Context, token, path string) (interface{}, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hris2BaseURLFn()+path, nil)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
		return nil, false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := utils.DoWithRetry(utils.NewSSOHTTPClient(), req, 2)
	if err != nil {
		log.Println("[SAPA HRIS2] gagal menghubungi HRIS2:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API HRIS2 Kemenkeu")
		return nil, false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		utils.ErrorResponseWithCode(c, http.StatusUnauthorized, "Token SSO ditolak oleh HRIS2, silakan login ulang", utils.CodeSSOSessionExpired)
		return nil, false
	case resp.StatusCode == http.StatusNotFound:
		return nil, true
	case resp.StatusCode != http.StatusOK:
		log.Println("[SAPA HRIS2] status tak terduga:", resp.StatusCode)
		utils.ErrorResponse(c, http.StatusBadGateway, fmt.Sprintf("HRIS2 mengembalikan status %d", resp.StatusCode))
		return nil, false
	}
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal membaca respons HRIS2")
		return nil, false
	}
	// Galat di dalam amplop (isError) tetap dibalas HTTP 200.
	if m, ok := raw.(map[string]interface{}); ok && m["isError"] == true {
		log.Println("[SAPA HRIS2] isError dari HRIS2")
		utils.ErrorResponse(c, http.StatusBadGateway, "HRIS2 mengembalikan kesalahan")
		return nil, false
	}
	return raw, true
}

// GET /sapa/pegawai?q=nama-atau-nip
func SearchSapaPegawai(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	if _, ok := sapaMasuk(c, ctx); !ok {
		return
	}
	q := bersihQueryPegawai(c.Query("q"))
	if len([]rune(strings.ReplaceAll(q, " ", ""))) < pegawaiMinQuery { // spasi tidak dihitung: "a b" terlalu luas
		utils.ErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Ketik minimal %d huruf atau angka", pegawaiMinQuery))
		return
	}
	if len([]rune(q)) > 60 {
		q = string([]rune(q)[:60])
	}
	token, ok := tokenHRIS2(c)
	if !ok {
		return
	}
	// Filter memakai konvensi Sieve: cari di kolom Nama atau Nip18 (mengandung, tanpa membedakan huruf besar/kecil).
	filter := fmt.Sprintf("(Nama|Nip18)@=*%s", q)
	path := fmt.Sprintf("/api/Profile/GetAllPegawai?Filters=%s&PageSize=%d", url.QueryEscape(filter), pegawaiMaxHasil)
	raw, ok := panggilHRIS2(c, ctx, token, path)
	if !ok {
		return
	}
	hasil := []PegawaiSapa{}
	for _, row := range barisPegawai(raw) {
		if p, ok := pegawaiDari(row); ok {
			hasil = append(hasil, p)
			if len(hasil) >= pegawaiMaxHasil {
				break
			}
		}
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"pegawai": hasil, "batas": pegawaiMaxHasil})
}

// GET /sapa/pegawai/:nip: data lengkap satu pegawai (jabatan aktif ada di sini, tidak selalu ada pada hasil pencarian).
func GetSapaPegawai(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	if _, ok := sapaMasuk(c, ctx); !ok {
		return
	}
	nip := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, c.Param("nip"))
	if !reNIPPegawai.MatchString(nip) {
		utils.ErrorResponse(c, http.StatusBadRequest, "NIP harus berupa 8 sampai 18 digit angka")
		return
	}
	token, ok := tokenHRIS2(c)
	if !ok {
		return
	}
	raw, ok := panggilHRIS2(c, ctx, token, "/api/Profile/GetPegawai?nip="+url.QueryEscape(nip))
	if !ok {
		return
	}
	var data map[string]interface{}
	if m, ok := raw.(map[string]interface{}); ok {
		if d, ok := m["data"].(map[string]interface{}); ok {
			data = d
		} else if _, ok := pegawaiDari(m); ok {
			data = m
		}
	}
	p, ok := pegawaiDari(data)
	if raw == nil || data == nil || !ok {
		utils.ErrorResponse(c, http.StatusNotFound, "Pegawai dengan NIP tersebut tidak ditemukan di HRIS2")
		return
	}
	if p.NIP == "" {
		p.NIP = nip
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", p)
}
