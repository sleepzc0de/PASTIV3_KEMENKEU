package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	mssql "github.com/microsoft/go-mssqldb"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/dto"
	"pasti-v3-backend/utils"
)

const ssoStateTTL = 10 * time.Minute

func SSOLogin(c *gin.Context) {
	endpoints := config.GetSSOEndpoints()
	cfg := config.Cfg

	state, err := utils.GenerateRandomURLSafeString(32)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membuat state SSO")
		return
	}
	codeVerifier, err := utils.GenerateRandomURLSafeString(64)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membuat code verifier")
		return
	}
	codeChallenge := utils.GenerateCodeChallengeS256(codeVerifier)

	expiresAt := time.Now().Add(ssoStateTTL)
	_, err = database.DB.Exec(
		`INSERT INTO sso_states (state, code_verifier, expires_at) VALUES (@p1, @p2, @p3)`,
		state, codeVerifier, expiresAt,
	)
	if err != nil {
		log.Println("[SSO ERROR] gagal insert sso_states:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyimpan state SSO")
		return
	}

	params := url.Values{}
	params.Set("client_id", cfg.SSOClientID)
	params.Set("redirect_uri", cfg.SSORedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", cfg.SSOScope)
	params.Set("state", state)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	authorizeURL := endpoints.AuthorizeEndpoint + "?" + params.Encode()
	c.Redirect(http.StatusFound, authorizeURL)
}

func SSOCallback(c *gin.Context) {
	cfg := config.Cfg

	redirectError := func(reason string) {
		errURL := cfg.FrontendURL + "/login?error=sso_failed&reason=" + url.QueryEscape(reason)
		c.Redirect(http.StatusFound, errURL)
	}

	code := c.Query("code")
	state := c.Query("state")
	errParam := c.Query("error")

	if errParam != "" {
		log.Println("[SSO ERROR] Query param error dari Kemenkeu:", errParam)
		redirectError("Kemenkeu menolak: " + errParam)
		return
	}
	if code == "" || state == "" {
		redirectError("Parameter tidak lengkap dari SSO")
		return
	}

	var codeVerifier string
	var expiresAt time.Time
	err := database.DB.QueryRow(
		`SELECT code_verifier, expires_at FROM sso_states WHERE state = @p1`, state,
	).Scan(&codeVerifier, &expiresAt)

	if err == sql.ErrNoRows {
		redirectError("Sesi login sudah tidak valid, silakan klik tombol SSO lagi")
		return
	} else if err == nil && time.Now().After(expiresAt) {
		redirectError("Sesi login sudah kedaluwarsa")
		return
	} else if err != nil {
		redirectError("Kesalahan server saat validasi sesi")
		return
	}
	database.DB.Exec(`DELETE FROM sso_states WHERE state = @p1`, state)

	endpoints := config.GetSSOEndpoints()
	httpClient := utils.NewSSOHTTPClient()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", cfg.SSORedirectURI)
	form.Set("client_id", cfg.SSOClientID)
	form.Set("client_secret", cfg.SSOClientSecret)
	form.Set("code_verifier", codeVerifier)

	encodedForm := form.Encode()

	tokenReq, _ := http.NewRequest(http.MethodPost, endpoints.TokenEndpoint, strings.NewReader(encodedForm))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.Header.Set("User-Agent", "PASTI-V3-Backend/1.0")
	tokenReq.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(encodedForm)), nil
	}

	tokenResp, err := utils.DoWithRetry(httpClient, tokenReq, 3)
	if err != nil {
		log.Println("[SSO ERROR] gagal request token_endpoint:", err)
		redirectError("Gagal menghubungi server SSO Kemenkeu (timeout jaringan)")
		return
	}
	defer tokenResp.Body.Close()

	tokenBody, _ := io.ReadAll(tokenResp.Body)

	if tokenResp.StatusCode != http.StatusOK {
		redirectError(fmt.Sprintf("Server SSO menolak token (status %d)", tokenResp.StatusCode))
		return
	}

	var tokenData dto.SSOTokenResponse
	if err := json.Unmarshal(tokenBody, &tokenData); err != nil || tokenData.AccessToken == "" {
		redirectError("Format respons token tidak valid")
		return
	}

	userInfoReq, _ := http.NewRequest(http.MethodGet, endpoints.UserinfoEndpoint, nil)
	userInfoReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
	userInfoReq.Header.Set("User-Agent", "PASTI-V3-Backend/1.0")

	userInfoResp, err := utils.DoWithRetry(httpClient, userInfoReq, 3)
	if err != nil {
		redirectError("Gagal mengambil data profil dari SSO Kemenkeu")
		return
	}
	defer userInfoResp.Body.Close()

	userInfoBody, _ := io.ReadAll(userInfoResp.Body)

	if userInfoResp.StatusCode != http.StatusOK {
		redirectError(fmt.Sprintf("Server SSO menolak profil (status %d)", userInfoResp.StatusCode))
		return
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(userInfoBody, &claims); err != nil {
		redirectError("Format data profil tidak valid")
		return
	}

	getClaim := func(key string) string {
		if v, ok := claims[key]; ok && v != nil {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	sub := getClaim("sub")
	if sub == "" {
		redirectError("Data identitas pegawai tidak lengkap")
		return
	}

	email := getClaim("email")
	nip := getClaim("nip")
	nip9 := getClaim("nip9")

	employeeID, err := upsertEmployee(claims, sub, nip, nip9, email)
	if err != nil {
		log.Println("[SSO ERROR] gagal upsertEmployee:", err)
		redirectError("Gagal menyimpan data pegawai: " + truncateError(err, 200))
		return
	}

	accessToken, expiresIn, userID, err := upsertUserFromSSO(employeeID, sub, email, nip, getClaim("name"))
	if err != nil {
		log.Println("[SSO ERROR] gagal upsertUserFromSSO:", err)
		redirectError("Gagal membuat sesi akun: " + truncateError(err, 200))
		return
	}

	// Simpan access_token & refresh_token SSO (terenkripsi) untuk dipakai
	// nanti memanggil API HRIS2 atas nama user ini.
	if err := saveSSOToken(userID, tokenData); err != nil {
		log.Println("[SSO WARN] gagal menyimpan token SSO untuk integrasi HRIS2:", err)
		// Tidak fatal — login tetap lanjut, hanya fitur HRIS2 yang tidak akan berfungsi
	}

	log.Println("[SSO SUCCESS] Login berhasil untuk sub:", sub, "email:", email)

	redirectURL := cfg.FrontendURL + "/sso/callback#token=" + accessToken + "&expires_in=" + urlItoa(expiresIn)
	c.Redirect(http.StatusFound, redirectURL)
}

func truncateError(err error, maxLen int) string {
	return truncateString(err.Error(), maxLen)
}

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func urlItoa(i int) string {
	return url.QueryEscape(string(rune(i)))
}

func upsertEmployee(claims map[string]interface{}, sub, nip, nip9, email string) (string, error) {
	getClaim := func(key string) string {
		if v, ok := claims[key]; ok && v != nil {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	rawJSON, _ := json.Marshal(claims)

	var existingIDRaw mssql.UniqueIdentifier
	err := database.DB.QueryRow(`SELECT id FROM employees WHERE sso_sub = @p1`, sub).Scan(&existingIDRaw)

	if err == sql.ErrNoRows {
		newID := uuid.New().String()
		_, err = database.DB.Exec(`
			INSERT INTO employees (
				id, sso_sub, nip, nip9, nik, name, email, preferred_username,
				jabatan, jenis_jabatan, satker, kode_satker, organisasi, kode_organisasi,
				kode_kl, nama_kl, phone_number, picture, raw_claims
			) VALUES (
				@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8,
				@p9, @p10, @p11, @p12, @p13, @p14,
				@p15, @p16, @p17, @p18, @p19
			)`,
			newID, sub, nip, nip9, getClaim("nik"), getClaim("name"), email, getClaim("preferred_username"),
			getClaim("jabatan"), getClaim("jenis_jabatan"), getClaim("satker"), getClaim("kode_satker"),
			getClaim("organisasi"), getClaim("kode_organisasi"), getClaim("kode_kl"), getClaim("nama_kl"),
			getClaim("phone_number"), getClaim("picture"), string(rawJSON),
		)
		if err != nil {
			return "", fmt.Errorf("insert employee gagal: %w", err)
		}
		return newID, nil
	} else if err != nil {
		return "", fmt.Errorf("query cek employee existing gagal: %w", err)
	}

	existingID := existingIDRaw.String()

	_, err = database.DB.Exec(`
		UPDATE employees SET
			nip=@p2, nip9=@p3, nik=@p4, name=@p5, email=@p6, preferred_username=@p7,
			jabatan=@p8, jenis_jabatan=@p9, satker=@p10, kode_satker=@p11,
			organisasi=@p12, kode_organisasi=@p13, kode_kl=@p14, nama_kl=@p15,
			phone_number=@p16, picture=@p17, raw_claims=@p18, updated_at=SYSUTCDATETIME()
		WHERE id=@p1`,
		existingID, nip, nip9, getClaim("nik"), getClaim("name"), email, getClaim("preferred_username"),
		getClaim("jabatan"), getClaim("jenis_jabatan"), getClaim("satker"), getClaim("kode_satker"),
		getClaim("organisasi"), getClaim("kode_organisasi"), getClaim("kode_kl"), getClaim("nama_kl"),
		getClaim("phone_number"), getClaim("picture"), string(rawJSON),
	)
	if err != nil {
		return "", fmt.Errorf("update employee gagal: %w", err)
	}
	return existingID, nil
}

// upsertUserFromSSO sekarang juga mengembalikan userID (untuk dipakai
// menyimpan token SSO ke tabel sso_tokens).
func upsertUserFromSSO(employeeID, sub, email, nip, fullName string) (accessToken string, expiresIn int, userID string, err error) {
	isProtected := utils.IsProtectedIdentity(email, nip)

	var userIDRaw mssql.UniqueIdentifier
	var role string
	errQ := database.DB.QueryRow(
		`SELECT id, role FROM users WHERE employee_id = @p1`, employeeID,
	).Scan(&userIDRaw, &role)

	if errQ == sql.ErrNoRows {
		userID = uuid.New().String()
		role = "user"
		if isProtected {
			role = "superadmin"
		}
		username := email
		if username == "" {
			username = sub
		}

		_, err = database.DB.Exec(`
			INSERT INTO users (id, username, email, full_name, role, is_active, auth_provider, is_protected, employee_id)
			VALUES (@p1, @p2, @p3, @p4, @p5, 1, 'sso', @p6, @p7)`,
			userID, username, email, fullName, role, isProtected, employeeID,
		)
		if err != nil {
			return "", 0, "", fmt.Errorf("insert user gagal: %w", err)
		}
	} else if errQ != nil {
		return "", 0, "", fmt.Errorf("query cek user existing gagal: %w", errQ)
	} else {
		userID = userIDRaw.String()

		if isProtected {
			_, err = database.DB.Exec(
				`UPDATE users SET role='superadmin', is_protected=1, is_active=1, full_name=@p1, last_login=SYSUTCDATETIME() WHERE id=@p2`,
				fullName, userID,
			)
			role = "superadmin"
		} else {
			_, err = database.DB.Exec(
				`UPDATE users SET full_name=@p1, last_login=SYSUTCDATETIME() WHERE id=@p2`,
				fullName, userID,
			)
		}
		if err != nil {
			return "", 0, "", fmt.Errorf("update user gagal: %w", err)
		}
	}

	accessToken, expiresIn, err = utils.GenerateAccessToken(userID, email, role)
	if err != nil {
		return "", 0, "", fmt.Errorf("generate JWT gagal: %w", err)
	}
	return accessToken, expiresIn, userID, nil
}

// saveSSOToken menyimpan (insert/update) access_token & refresh_token SSO
// dalam bentuk terenkripsi, dipakai belakangan untuk memanggil API HRIS2.
func saveSSOToken(userID string, tokenData dto.SSOTokenResponse) error {
	encAccess, err := utils.EncryptString(tokenData.AccessToken)
	if err != nil {
		return fmt.Errorf("enkripsi access_token gagal: %w", err)
	}

	var encRefresh sql.NullString
	if tokenData.RefreshToken != "" {
		er, err := utils.EncryptString(tokenData.RefreshToken)
		if err == nil {
			encRefresh = sql.NullString{String: er, Valid: true}
		}
	}

	expiresIn := tokenData.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 300 // fallback 5 menit kalau server tidak kirim expires_in
	}
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	var existingIDRaw mssql.UniqueIdentifier
	err = database.DB.QueryRow(`SELECT id FROM sso_tokens WHERE user_id = @p1`, userID).Scan(&existingIDRaw)

	if err == sql.ErrNoRows {
		newID := uuid.New().String()
		_, err = database.DB.Exec(`
			INSERT INTO sso_tokens (id, user_id, access_token_enc, refresh_token_enc, expires_at)
			VALUES (@p1, @p2, @p3, @p4, @p5)`,
			newID, userID, encAccess, encRefresh, expiresAt,
		)
		return err
	} else if err != nil {
		return err
	}

	_, err = database.DB.Exec(`
		UPDATE sso_tokens SET access_token_enc=@p1, refresh_token_enc=@p2, expires_at=@p3, updated_at=SYSUTCDATETIME()
		WHERE user_id=@p4`,
		encAccess, encRefresh, expiresAt, userID,
	)
	return err
}
