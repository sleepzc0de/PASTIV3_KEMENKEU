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
		errDesc := c.Query("error_description")
		log.Println("[SSO ERROR] Query param error dari Kemenkeu:", errParam, "| description:", errDesc)
		redirectError("Kemenkeu menolak: " + errParam)
		return
	}
	if code == "" || state == "" {
		log.Println("[SSO ERROR] code atau state kosong. code:", code, "state:", state)
		redirectError("Parameter tidak lengkap dari SSO")
		return
	}

	var codeVerifier string
	var expiresAt time.Time
	err := database.DB.QueryRow(
		`SELECT code_verifier, expires_at FROM sso_states WHERE state = @p1`, state,
	).Scan(&codeVerifier, &expiresAt)

	if err == sql.ErrNoRows {
		log.Println("[SSO ERROR] state tidak ditemukan di DB:", state)
		redirectError("Sesi login sudah tidak valid, silakan klik tombol SSO lagi dari halaman login (bukan reload)")
		return
	} else if err == nil && time.Now().After(expiresAt) {
		log.Println("[SSO ERROR] state sudah expired:", state)
		redirectError("Sesi login sudah kedaluwarsa, silakan coba lagi")
		return
	} else if err != nil {
		log.Println("[SSO ERROR] gagal query sso_states:", err)
		redirectError("Kesalahan server saat validasi sesi: " + truncateError(err, 150))
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

	tokenStart := time.Now()
	tokenResp, err := utils.DoWithRetry(httpClient, tokenReq, 3)
	log.Println("[SSO INFO] Durasi request token_endpoint:", time.Since(tokenStart))

	if err != nil {
		log.Println("[SSO ERROR] gagal request ke token_endpoint setelah retry:", err)
		redirectError("Gagal menghubungi server SSO Kemenkeu (timeout jaringan)")
		return
	}
	defer tokenResp.Body.Close()

	tokenBody, _ := io.ReadAll(tokenResp.Body)

	if tokenResp.StatusCode != http.StatusOK {
		log.Println("[SSO ERROR] token_endpoint status:", tokenResp.StatusCode, "| body:", string(tokenBody))
		redirectError(fmt.Sprintf("Server SSO menolak token (status %d): %s", tokenResp.StatusCode, truncateString(string(tokenBody), 100)))
		return
	}

	var tokenData dto.SSOTokenResponse
	if err := json.Unmarshal(tokenBody, &tokenData); err != nil || tokenData.AccessToken == "" {
		log.Println("[SSO ERROR] gagal parse token response:", string(tokenBody))
		redirectError("Format respons token tidak valid")
		return
	}

	userInfoReq, _ := http.NewRequest(http.MethodGet, endpoints.UserinfoEndpoint, nil)
	userInfoReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
	userInfoReq.Header.Set("User-Agent", "PASTI-V3-Backend/1.0")

	userInfoStart := time.Now()
	userInfoResp, err := utils.DoWithRetry(httpClient, userInfoReq, 3)
	log.Println("[SSO INFO] Durasi request userinfo_endpoint:", time.Since(userInfoStart))

	if err != nil {
		log.Println("[SSO ERROR] gagal request ke userinfo_endpoint setelah retry:", err)
		redirectError("Gagal mengambil data profil dari SSO Kemenkeu")
		return
	}
	defer userInfoResp.Body.Close()

	userInfoBody, _ := io.ReadAll(userInfoResp.Body)

	if userInfoResp.StatusCode != http.StatusOK {
		log.Println("[SSO ERROR] userinfo_endpoint status:", userInfoResp.StatusCode, "| body:", string(userInfoBody))
		redirectError(fmt.Sprintf("Server SSO menolak profil (status %d)", userInfoResp.StatusCode))
		return
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(userInfoBody, &claims); err != nil {
		log.Println("[SSO ERROR] gagal parse userinfo JSON:", err)
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
		log.Println("[SSO ERROR] claim 'sub' kosong. Full claims:", claims)
		redirectError("Data identitas pegawai tidak lengkap")
		return
	}

	email := getClaim("email")
	nip := getClaim("nip")
	nip9 := getClaim("nip9")

	log.Printf("[SSO DEBUG] sub=%q email=%q nip=%q nip9=%q", sub, email, nip, nip9)

	employeeID, err := upsertEmployee(claims, sub, nip, nip9, email)
	if err != nil {
		log.Println("[SSO ERROR] gagal upsertEmployee:", err)
		redirectError("Gagal menyimpan data pegawai: " + truncateError(err, 200))
		return
	}

	accessToken, expiresIn, err := upsertUserFromSSO(employeeID, sub, email, nip, getClaim("name"))
	if err != nil {
		log.Println("[SSO ERROR] gagal upsertUserFromSSO:", err)
		redirectError("Gagal membuat sesi akun: " + truncateError(err, 200))
		return
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

// upsertEmployee: kolom "id" (UNIQUEIDENTIFIER) di-scan ke mssql.UniqueIdentifier,
// lalu dikonversi ke string via .String() agar formatnya valid dipakai kembali
// sebagai parameter WHERE id=... pada query UPDATE.
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

func upsertUserFromSSO(employeeID, sub, email, nip, fullName string) (string, int, error) {
	isProtected := utils.IsProtectedIdentity(email, nip)

	var userIDRaw mssql.UniqueIdentifier
	var role string
	err := database.DB.QueryRow(
		`SELECT id, role FROM users WHERE employee_id = @p1`, employeeID,
	).Scan(&userIDRaw, &role)

	var userID string

	if err == sql.ErrNoRows {
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
			return "", 0, fmt.Errorf("insert user gagal: %w", err)
		}
	} else if err != nil {
		return "", 0, fmt.Errorf("query cek user existing gagal: %w", err)
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
			return "", 0, fmt.Errorf("update user gagal: %w", err)
		}
	}

	accessToken, expiresIn, err := utils.GenerateAccessToken(userID, email, role)
	if err != nil {
		return "", 0, fmt.Errorf("generate JWT gagal: %w", err)
	}
	return accessToken, expiresIn, nil
}
