package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/dto"
	"pasti-v3-backend/utils"
)

const ssoStateTTL = 10 * time.Minute

// ============ STEP 1: Redirect user ke halaman login SSO Kemenkeu ============
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

// ============ STEP 2: Callback dari SSO Kemenkeu setelah user login ============
func SSOCallback(c *gin.Context) {
	cfg := config.Cfg
	frontendErrorURL := cfg.FrontendURL + "/login?error=sso_failed"

	code := c.Query("code")
	state := c.Query("state")
	errParam := c.Query("error")

	if errParam != "" {
		log.Println("[SSO ERROR] Query param error dari Kemenkeu:", errParam, "| description:", c.Query("error_description"))
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	}
	if code == "" || state == "" {
		log.Println("[SSO ERROR] code atau state kosong. code:", code, "state:", state)
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	}

	var codeVerifier string
	var expiresAt time.Time
	err := database.DB.QueryRow(
		`SELECT code_verifier, expires_at FROM sso_states WHERE state = @p1`, state,
	).Scan(&codeVerifier, &expiresAt)

	if err == sql.ErrNoRows {
		log.Println("[SSO ERROR] state tidak ditemukan di DB:", state)
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	} else if err == nil && time.Now().After(expiresAt) {
		log.Println("[SSO ERROR] state sudah expired:", state, "expiresAt:", expiresAt)
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	} else if err != nil {
		log.Println("[SSO ERROR] gagal query sso_states:", err)
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	}
	database.DB.Exec(`DELETE FROM sso_states WHERE state = @p1`, state)

	endpoints := config.GetSSOEndpoints()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", cfg.SSORedirectURI)
	form.Set("client_id", cfg.SSOClientID)
	form.Set("client_secret", cfg.SSOClientSecret)
	form.Set("code_verifier", codeVerifier)

	tokenReq, _ := http.NewRequest(http.MethodPost, endpoints.TokenEndpoint, strings.NewReader(form.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	tokenResp, err := httpClient.Do(tokenReq)
	if err != nil {
		log.Println("[SSO ERROR] gagal request ke token_endpoint:", err)
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	}
	defer tokenResp.Body.Close()

	tokenBody, _ := io.ReadAll(tokenResp.Body)

	if tokenResp.StatusCode != http.StatusOK {
		log.Println("[SSO ERROR] token_endpoint status:", tokenResp.StatusCode, "| response body:", string(tokenBody))
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	}

	var tokenData dto.SSOTokenResponse
	if err := json.Unmarshal(tokenBody, &tokenData); err != nil || tokenData.AccessToken == "" {
		log.Println("[SSO ERROR] gagal parse token response atau access_token kosong:", string(tokenBody))
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	}

	userInfoReq, _ := http.NewRequest(http.MethodGet, endpoints.UserinfoEndpoint, nil)
	userInfoReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)

	userInfoResp, err := httpClient.Do(userInfoReq)
	if err != nil {
		log.Println("[SSO ERROR] gagal request ke userinfo_endpoint:", err)
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	}
	defer userInfoResp.Body.Close()

	userInfoBody, _ := io.ReadAll(userInfoResp.Body)

	if userInfoResp.StatusCode != http.StatusOK {
		log.Println("[SSO ERROR] userinfo_endpoint status:", userInfoResp.StatusCode, "| response body:", string(userInfoBody))
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(userInfoBody, &claims); err != nil {
		log.Println("[SSO ERROR] gagal parse userinfo JSON:", err, "| body:", string(userInfoBody))
		c.Redirect(http.StatusFound, frontendErrorURL)
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
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	}

	email := getClaim("email")
	nip := getClaim("nip")
	nip9 := getClaim("nip9")

	employeeID, err := upsertEmployee(claims, sub, nip, nip9, email)
	if err != nil {
		log.Println("[SSO ERROR] gagal upsertEmployee:", err)
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	}

	accessToken, expiresIn, err := upsertUserFromSSO(employeeID, sub, email, nip, getClaim("name"))
	if err != nil {
		log.Println("[SSO ERROR] gagal upsertUserFromSSO:", err)
		c.Redirect(http.StatusFound, frontendErrorURL)
		return
	}

	log.Println("[SSO SUCCESS] Login berhasil untuk sub:", sub, "email:", email)

	redirectURL := cfg.FrontendURL + "/sso/callback#token=" + accessToken + "&expires_in=" + urlItoa(expiresIn)
	c.Redirect(http.StatusFound, redirectURL)
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

	var existingID string
	err := database.DB.QueryRow(`SELECT id FROM employees WHERE sso_sub = @p1`, sub).Scan(&existingID)

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
			return "", err
		}
		return newID, nil
	} else if err != nil {
		return "", err
	}

	// Sudah ada -> update data terbaru setiap kali login (data pegawai bisa berubah: jabatan, satker, dll)
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
	return existingID, err
}

func upsertUserFromSSO(employeeID, sub, email, nip, fullName string) (string, int, error) {
	isProtected := utils.IsProtectedIdentity(email, nip)

	var userID, role string
	err := database.DB.QueryRow(
		`SELECT id, role FROM users WHERE employee_id = @p1`, employeeID,
	).Scan(&userID, &role)

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
			return "", 0, err
		}
	} else if err != nil {
		return "", 0, err
	} else {
		// User sudah ada — kalau identitas ini termasuk protected, pastikan role & flag tetap terjaga
		if isProtected {
			database.DB.Exec(
				`UPDATE users SET role='superadmin', is_protected=1, is_active=1, full_name=@p1, last_login=SYSUTCDATETIME() WHERE id=@p2`,
				fullName, userID,
			)
			role = "superadmin"
		} else {
			database.DB.Exec(
				`UPDATE users SET full_name=@p1, last_login=SYSUTCDATETIME() WHERE id=@p2`,
				fullName, userID,
			)
		}
	}

	accessToken, expiresIn, err := utils.GenerateAccessToken(userID, email, role)
	return accessToken, expiresIn, err
}
