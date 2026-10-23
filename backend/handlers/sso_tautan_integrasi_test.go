package handlers

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"pasti-v3-backend/config"
)

// Tes integrasi dengan SQL Server sungguhan (lihat inaproc_integrasi_mssql_test.go; dilewati bila PASTI_UJI_MSSQL_DSN kosong) untuk penautan akun saat login
// SSO. Pengguna uji berakhiran email "@uji-tautan.test" dan pegawai uji ber-sub "uji-sso-tautan-"; keduanya dibersihkan sebelum serta sesudah tes.

const (
	emailTautanUji     = "budi@uji-tautan.test"
	emailProtectedUji  = "protected@uji-tautan.test"
	subTautanLama      = "uji-sso-tautan-lama"
	subTautanBaru      = "uji-sso-tautan-baru"
	subTautanAmbigu    = "uji-sso-tautan-ambigu"
	namaTautanBaru     = "BUDI SANTOSO"
	nipTautanUji       = "199001012015011001"
	perananTautanLama  = "admin"
	domainEmailTautan  = "%@uji-tautan.test"
	polaSubTautanUji   = "uji-sso-tautan-%"
	namaPenggunaTautan = "Nama Lama"
)

func bersihkanTautanUji(t *testing.T, db *sql.DB) {
	t.Helper()
	// users lebih dulu: users.employee_id mereferensikan employees.
	if _, err := db.Exec(`DELETE FROM users WHERE email LIKE @p1 OR username LIKE @p1`, domainEmailTautan); err != nil {
		t.Fatalf("bersihkan pengguna uji: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM employees WHERE sso_sub LIKE @p1`, polaSubTautanUji); err != nil {
		t.Fatalf("bersihkan pegawai uji: %v", err)
	}
}

func siapkanTautanUji(t *testing.T) *sql.DB {
	t.Helper()
	db := dbIntegrasi(t)
	lama := config.Cfg
	config.Cfg = &config.Config{JWTSecret: "rahasia-uji-tautan", JWTAccessExpireMin: 15, SuperadminProtectedEmail: emailProtectedUji}
	t.Cleanup(func() { config.Cfg = lama })
	bersihkanTautanUji(t, db)
	if !biarkanDataUji() {
		t.Cleanup(func() { bersihkanTautanUji(t, db) })
	}
	return db
}

func pegawaiTautanUji(t *testing.T, sub, email string) string {
	t.Helper()
	claims := map[string]interface{}{"sub": sub, "name": namaTautanBaru, "email": email, "nip": nipTautanUji}
	id, err := upsertEmployee(claims, sub, nipTautanUji, "", email)
	if err != nil {
		t.Fatalf("upsertEmployee %s: %v", sub, err)
	}
	return id
}

// penggunaTautanUji menyisipkan satu baris users dan mengembalikan id-nya. employeeID boleh kosong (NULL).
func penggunaTautanUji(t *testing.T, db *sql.DB, username, email, provider, role string, aktif, protected bool, employeeID string) string {
	t.Helper()
	id := uuid.New().String()
	var emp interface{}
	if employeeID != "" {
		emp = employeeID
	}
	if _, err := db.Exec(
		`INSERT INTO users (id, username, email, full_name, role, is_active, auth_provider, is_protected, employee_id) VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9)`,
		id, username, email, namaPenggunaTautan, role, aktif, provider, protected, emp,
	); err != nil {
		t.Fatalf("sisip pengguna uji %s: %v", username, err)
	}
	return id
}

type barisPenggunaUji struct {
	ID, Role, Provider, FullName string
	Aktif, Protected             bool
	EmployeeID                   sql.NullString
	LastLogin                    sql.NullTime
}

func bacaPenggunaTautanUji(t *testing.T, db *sql.DB, email string) []barisPenggunaUji {
	t.Helper()
	rows, err := db.Query(`SELECT CONVERT(NVARCHAR(36), id), role, auth_provider, full_name, is_active, is_protected, CONVERT(NVARCHAR(36), employee_id), last_login FROM users WHERE email = @p1`, email)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var hasil []barisPenggunaUji
	for rows.Next() {
		var b barisPenggunaUji
		if err := rows.Scan(&b.ID, &b.Role, &b.Provider, &b.FullName, &b.Aktif, &b.Protected, &b.EmployeeID, &b.LastLogin); err != nil {
			t.Fatal(err)
		}
		hasil = append(hasil, b)
	}
	return hasil
}

func samaUUID(a, b string) bool { return uuid.MustParse(a) == uuid.MustParse(b) }

// Kasus yang dilaporkan: pengguna sudah ada dari identitas SSO lain (sub lama), lalu login lewat SSO yang menerbitkan sub baru dengan email yang sama.
func TestTautkanAkunSSOYangEmailnyaSudahAdaDenganSQLServer(t *testing.T) {
	db := siapkanTautanUji(t)

	pegawaiLama := pegawaiTautanUji(t, subTautanLama, emailTautanUji)
	idLama := penggunaTautanUji(t, db, emailTautanUji, emailTautanUji, "sso", perananTautanLama, true, false, pegawaiLama)
	pegawaiBaru := pegawaiTautanUji(t, subTautanBaru, emailTautanUji)

	token, expiresIn, userID, err := upsertUserFromSSO(pegawaiBaru, subTautanBaru, emailTautanUji, nipTautanUji, namaTautanBaru)
	if err != nil {
		t.Fatalf("upsertUserFromSSO: %v (sebelum perbaikan: Violation of UNIQUE KEY ...)", err)
	}
	if token == "" || expiresIn <= 0 {
		t.Errorf("token = %q, expiresIn = %d", token, expiresIn)
	}
	if !samaUUID(userID, idLama) {
		t.Errorf("userID = %s, mau akun lama %s", userID, idLama)
	}

	baris := bacaPenggunaTautanUji(t, db, emailTautanUji)
	if len(baris) != 1 {
		t.Fatalf("jumlah pengguna dengan email itu = %d, mau 1 (tidak boleh ada akun ganda)", len(baris))
	}
	b := baris[0]
	if b.Role != perananTautanLama {
		t.Errorf("peran = %q, mau %q (peran akun lama harus tetap)", b.Role, perananTautanLama)
	}
	if !b.EmployeeID.Valid || !samaUUID(b.EmployeeID.String, pegawaiBaru) {
		t.Errorf("employee_id = %v, mau pegawai baru %s", b.EmployeeID, pegawaiBaru)
	}
	if b.FullName != namaTautanBaru || !b.LastLogin.Valid {
		t.Errorf("full_name = %q, last_login valid = %v", b.FullName, b.LastLogin.Valid)
	}

	// Login berikutnya ditemukan lewat employee_id: tetap akun yang sama dan tidak ada akun baru.
	_, _, userID2, err := upsertUserFromSSO(pegawaiBaru, subTautanBaru, emailTautanUji, nipTautanUji, namaTautanBaru)
	if err != nil || !samaUUID(userID2, idLama) {
		t.Fatalf("login kedua: userID = %s, err = %v", userID2, err)
	}
	if n := len(bacaPenggunaTautanUji(t, db, emailTautanUji)); n != 1 {
		t.Errorf("setelah login kedua, jumlah pengguna = %d, mau 1", n)
	}
}

func TestTanpaAkunSamaMembuatPenggunaBaruDenganSQLServer(t *testing.T) {
	db := siapkanTautanUji(t)
	pegawai := pegawaiTautanUji(t, subTautanBaru, emailTautanUji)

	_, _, userID, err := upsertUserFromSSO(pegawai, subTautanBaru, emailTautanUji, nipTautanUji, namaTautanBaru)
	if err != nil {
		t.Fatal(err)
	}
	baris := bacaPenggunaTautanUji(t, db, emailTautanUji)
	if len(baris) != 1 || !samaUUID(baris[0].ID, userID) {
		t.Fatalf("baris = %+v", baris)
	}
	if baris[0].Role != "user" || baris[0].Provider != "sso" || baris[0].Protected || !baris[0].Aktif {
		t.Errorf("pengguna baru = %+v, mau peran user, penyedia sso, aktif, bukan protected", baris[0])
	}
}

// Akun lokal (mis. dibuat dengan sandi) tidak boleh diambil alih oleh identitas SSO yang kebetulan beremail sama.
func TestAkunLokalTidakDitautkanDenganSQLServer(t *testing.T) {
	db := siapkanTautanUji(t)
	idLokal := penggunaTautanUji(t, db, emailTautanUji, emailTautanUji, "local", "user", true, false, "")
	pegawai := pegawaiTautanUji(t, subTautanBaru, emailTautanUji)

	_, _, _, err := upsertUserFromSSO(pegawai, subTautanBaru, emailTautanUji, nipTautanUji, namaTautanBaru)
	if !errors.Is(err, errAkunBentrok) {
		t.Fatalf("err = %v, mau errAkunBentrok", err)
	}
	baris := bacaPenggunaTautanUji(t, db, emailTautanUji)
	if len(baris) != 1 || !samaUUID(baris[0].ID, idLokal) || baris[0].EmployeeID.Valid || baris[0].Provider != "local" || baris[0].FullName != namaPenggunaTautan {
		t.Errorf("akun lokal berubah: %+v", baris)
	}
}

// Akun SSO yang dinonaktifkan administrator tetap tidak bisa masuk, dan tidak ditautkan.
func TestAkunNonaktifDitolakTanpaDitautkanDenganSQLServer(t *testing.T) {
	db := siapkanTautanUji(t)
	pegawaiLama := pegawaiTautanUji(t, subTautanLama, emailTautanUji)
	penggunaTautanUji(t, db, emailTautanUji, emailTautanUji, "sso", "user", false, false, pegawaiLama)
	pegawaiBaru := pegawaiTautanUji(t, subTautanBaru, emailTautanUji)

	_, _, _, err := upsertUserFromSSO(pegawaiBaru, subTautanBaru, emailTautanUji, nipTautanUji, namaTautanBaru)
	if !errors.Is(err, errAccountInactive) {
		t.Fatalf("err = %v, mau errAccountInactive", err)
	}
	baris := bacaPenggunaTautanUji(t, db, emailTautanUji)
	if len(baris) != 1 || baris[0].Aktif || !baris[0].EmployeeID.Valid || !samaUUID(baris[0].EmployeeID.String, pegawaiLama) || baris[0].LastLogin.Valid {
		t.Errorf("akun nonaktif berubah: %+v", baris)
	}
}

// Identitas superadmin permanen selalu diaktifkan kembali dan dinaikkan, juga saat akunnya ditautkan.
func TestAkunProtectedNonaktifDiaktifkanDanDitautkanDenganSQLServer(t *testing.T) {
	db := siapkanTautanUji(t)
	pegawaiLama := pegawaiTautanUji(t, subTautanLama, emailProtectedUji)
	penggunaTautanUji(t, db, emailProtectedUji, emailProtectedUji, "sso", "user", false, false, pegawaiLama)
	pegawaiBaru := pegawaiTautanUji(t, subTautanBaru, emailProtectedUji)

	_, _, _, err := upsertUserFromSSO(pegawaiBaru, subTautanBaru, emailProtectedUji, nipTautanUji, namaTautanBaru)
	if err != nil {
		t.Fatalf("upsertUserFromSSO: %v", err)
	}
	baris := bacaPenggunaTautanUji(t, db, emailProtectedUji)
	if len(baris) != 1 {
		t.Fatalf("jumlah pengguna = %d, mau 1", len(baris))
	}
	b := baris[0]
	if !b.Aktif || b.Role != "superadmin" || !b.Protected || !b.EmployeeID.Valid || !samaUUID(b.EmployeeID.String, pegawaiBaru) {
		t.Errorf("akun protected = %+v, mau aktif, superadmin, protected, tertaut ke pegawai baru", b)
	}
}

// Email cocok dengan satu akun dan username cocok dengan akun lain: ambigu, ditolak.
func TestDuaAkunBerbedaBentrokDenganSQLServer(t *testing.T) {
	db := siapkanTautanUji(t)
	penggunaTautanUji(t, db, "lain@uji-tautan.test", emailTautanUji, "sso", "user", true, false, "")
	penggunaTautanUji(t, db, emailTautanUji, "lain2@uji-tautan.test", "sso", "user", true, false, "")
	pegawai := pegawaiTautanUji(t, subTautanAmbigu, emailTautanUji)

	_, _, _, err := upsertUserFromSSO(pegawai, subTautanAmbigu, emailTautanUji, nipTautanUji, namaTautanBaru)
	if !errors.Is(err, errAkunBentrok) {
		t.Fatalf("err = %v, mau errAkunBentrok", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE employee_id IS NOT NULL AND email LIKE @p1`, domainEmailTautan).Scan(&n); err != nil || n != 0 {
		t.Errorf("ada akun yang tertaut padahal bentrok: n = %d, err = %v", n, err)
	}
}
