package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/dto"
	"pasti-v3-backend/utils"
)

// getValidAccessToken mengambil access_token SSO yang tersimpan untuk user,
// otomatis refresh kalau sudah/hampir expired.
func getValidAccessToken(userID string) (string, error) {
	var encAccess, encRefresh string
	var refreshValid bool
	var expiresAt time.Time

	row := database.DB.QueryRow(
		`SELECT access_token_enc, ISNULL(refresh_token_enc, ''), expires_at FROM sso_tokens WHERE user_id = @p1`,
		userID,
	)
	if err := row.Scan(&encAccess, &encRefresh, &expiresAt); err != nil {
		return "", fmt.Errorf("token SSO tidak ditemukan, silakan login ulang via SSO: %w", err)
	}
	refreshValid = encRefresh != ""

	// Beri buffer 30 detik sebelum expired untuk hindari race condition
	if time.Now().Add(30 * time.Second).Before(expiresAt) {
		return utils.DecryptString(encAccess)
	}

	if !refreshValid {
		return "", fmt.Errorf("token SSO sudah kedaluwarsa dan tidak ada refresh token, silakan login ulang via SSO")
	}

	refreshToken, err := utils.DecryptString(encRefresh)
	if err != nil {
		return "", fmt.Errorf("gagal membaca refresh token: %w", err)
	}

	return refreshSSOToken(userID, refreshToken)
}

func refreshSSOToken(userID, refreshToken string) (string, error) {
	cfg := config.Cfg
	endpoints := config.GetSSOEndpoints()

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", cfg.SSOClientID)
	form.Set("client_secret", cfg.SSOClientSecret)

	httpClient := utils.NewSSOHTTPClient()
	req, _ := http.NewRequest(http.MethodPost, endpoints.TokenEndpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal refresh token SSO: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("refresh token ditolak SSO (status %d), silakan login ulang", resp.StatusCode)
	}

	var tokenData dto.SSOTokenResponse
	if err := json.Unmarshal(body, &tokenData); err != nil || tokenData.AccessToken == "" {
		return "", fmt.Errorf("format respons refresh token tidak valid")
	}

	if err := saveSSOToken(userID, tokenData); err != nil {
		log.Println("[HRIS2 WARN] gagal update token setelah refresh:", err)
	}

	return tokenData.AccessToken, nil
}

// SearchPegawai mencari pegawai lewat API HRIS2 Kemenkeu, memakai access_token
// SSO milik user yang sedang login (fitur ini hanya tersedia untuk user SSO).
func SearchPegawai(c *gin.Context) {
	userID := c.GetString("user_id")

	var authProvider string
	if err := database.DB.QueryRow(`SELECT auth_provider FROM users WHERE id = @p1`, userID).Scan(&authProvider); err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "User tidak ditemukan")
		return
	}
	if authProvider != "sso" {
		utils.ErrorResponse(c, http.StatusForbidden, "Fitur pencarian pegawai HRIS2 hanya tersedia untuk pengguna yang login via SSO Kemenkeu")
		return
	}

	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Parameter pencarian 'q' wajib diisi")
		return
	}

	accessToken, err := getValidAccessToken(userID)
	if err != nil {
		log.Println("[HRIS2 ERROR] gagal ambil access token:", err)
		utils.ErrorResponse(c, http.StatusUnauthorized, "Sesi SSO Anda telah berakhir, silakan logout lalu login ulang via SSO Kemenkeu")
		return
	}

	baseURL := config.GetHRIS2BaseURL()

	// Catatan: syntax filter ini memakai konvensi library Sieve (umum dipakai
	// .NET dengan parameter Filters/Sorts/Page/PageSize seperti spec ini),
	// mencari kata kunci di kolom Nama ATAU Nip18 (case-insensitive contains).
	// Kalau ternyata API HRIS2 pakai syntax berbeda, sesuaikan baris ini
	// setelah lihat hasil respons/error dari API sungguhan.
	filter := fmt.Sprintf("(Nama|Nip18)@=*%s", query)

	reqURL := fmt.Sprintf("%s/api/Profile/GetAllPegawai?Filters=%s&PageSize=25",
		baseURL, url.QueryEscape(filter))

	httpClient := utils.NewSSOHTTPClient()
	req, _ := http.NewRequest(http.MethodGet, reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := utils.DoWithRetry(httpClient, req, 2)
	if err != nil {
		log.Println("[HRIS2 ERROR] gagal request ke HRIS2:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API HRIS2 Kemenkeu")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Token SSO ditolak oleh HRIS2, silakan login ulang")
		return
	}
	if resp.StatusCode != http.StatusOK {
		log.Println("[HRIS2 ERROR] status non-200:", resp.StatusCode, "| body:", string(body))
		utils.ErrorResponse(c, http.StatusBadGateway, fmt.Sprintf("HRIS2 mengembalikan status %d", resp.StatusCode))
		return
	}

	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons HRIS2")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Pencarian berhasil", raw)
}

// SearchPegawaiByNIP mencari 1 pegawai berdasarkan NIP lewat HRIS2,
// dipakai admin/superadmin saat membuat akun user baru.
func SearchPegawaiByNIP(c *gin.Context) {
	adminUserID := c.GetString("user_id")
	nip := strings.TrimSpace(c.Param("nip"))

	if nip == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "NIP wajib diisi")
		return
	}

	accessToken, err := getValidAccessToken(adminUserID)
	if err != nil {
		log.Println("[HRIS2 ERROR] gagal ambil access token:", err)
		utils.ErrorResponse(c, http.StatusUnauthorized, "Sesi SSO Anda telah berakhir, silakan logout lalu login ulang via SSO Kemenkeu untuk menggunakan fitur ini")
		return
	}

	profile, err := fetchPegawaiByNIP(accessToken, nip)
	if err != nil {
		log.Println("[HRIS2 ERROR] gagal cari pegawai by NIP:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal mengambil data pegawai dari HRIS2: "+err.Error())
		return
	}

	if profile == nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Pegawai dengan NIP tersebut tidak ditemukan di HRIS2")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Data pegawai ditemukan", profile)
}

// fetchPegawaiByNIP query ke endpoint GET /api/Profile/GetPegawai?nip=...
func fetchPegawaiByNIP(accessToken, nip string) (map[string]interface{}, error) {
	baseURL := config.GetHRIS2BaseURL()
	reqURL := fmt.Sprintf("%s/api/Profile/GetPegawai?nip=%s", baseURL, url.QueryEscape(nip))

	httpClient := utils.NewSSOHTTPClient()
	req, _ := http.NewRequest(http.MethodGet, reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := utils.DoWithRetry(httpClient, req, 2)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HRIS2 status %d: %s", resp.StatusCode, truncateString(string(body), 150))
	}

	// Respons HRIS2 berbentuk: { "statusCode": 200, "isError": false, "data": {...} }
	// Data pegawai sesungguhnya ada di dalam field "data", perlu di-unwrap.
	var envelope struct {
		StatusCode int                    `json:"statusCode"`
		Message    string                 `json:"message"`
		IsError    bool                   `json:"isError"`
		Data       map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("gagal parse respons HRIS2")
	}

	if envelope.IsError {
		return nil, fmt.Errorf("HRIS2 mengembalikan error: %s", envelope.Message)
	}
	if envelope.Data == nil || len(envelope.Data) == 0 {
		return nil, nil
	}

	return envelope.Data, nil
}

// getStringField mengambil field dari map hasil HRIS2, mencoba beberapa
// kemungkinan nama key (karena struktur respons belum terdokumentasi resmi).
func getStringField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// getJabatanAktif mengambil namaJabatan dari elemen pertama array "jabatan"
// (posisi Definitif/aktif saat ini), sesuai struktur respons HRIS2 GetPegawai.
func getJabatanAktif(m map[string]interface{}) string {
	jabatanRaw, ok := m["jabatan"]
	if !ok {
		return ""
	}
	jabatanArr, ok := jabatanRaw.([]interface{})
	if !ok || len(jabatanArr) == 0 {
		return ""
	}
	first, ok := jabatanArr[0].(map[string]interface{})
	if !ok {
		return ""
	}
	return getStringField(first, "namaJabatan")
}
