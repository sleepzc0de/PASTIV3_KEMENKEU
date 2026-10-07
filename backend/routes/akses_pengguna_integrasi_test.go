package routes

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"pasti-v3-backend/config"
	"pasti-v3-backend/handlers"
	"pasti-v3-backend/persetujuan"
)

// Tes integrasi model akses baru lewat router asli dan SQL Server sungguhan (database uji yang sama dengan peran_integrasi_test.go, sudah dimigrasi sampai 055):
// siapa melihat dan mengelola siapa, tamu dan pernyataan penggunaan aplikasi, migrasi 055, dan sinkronisasi superadmin dengan .env.

// buatPegawaiUji: pengguna SSO (sudah menyetujui pernyataan) dengan kode satker lengkap dari SSO. kode kosong = tanpa data pegawai.
func buatPegawaiUji(t *testing.T, db *sql.DB, nama, role, kodeSatker string) penggunaUji {
	t.Helper()
	p := buatPenggunaUji(t, db, nama, role)
	if _, err := db.Exec(`UPDATE users SET auth_provider = N'sso' WHERE id = @p1`, p.id); err != nil {
		t.Fatal(err)
	}
	if kodeSatker != "" {
		emp := strings.ToUpper(uuid.New().String())
		if _, err := db.Exec(`INSERT INTO employees (id, sso_sub, nip, kode_satker) VALUES (@p1, @p2, N'199001012015011001', @p3)`, emp, "uji-peran-emp-"+nama, kodeSatker); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE users SET employee_id = @p1 WHERE id = @p2`, emp, p.id); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

// namaDalamDaftar: nama pengguna uji ("uji-peran-t...") yang muncul pada GET /users, terurut tanpa awalan.
func namaDalamDaftar(t *testing.T, p penggunaUji, awalan string) []string {
	t.Helper()
	code, body, _ := panggil(t, "GET", "/users", p.token, "")
	if code != 200 {
		t.Fatalf("GET /users oleh %s = %d %v", p.username, code, body)
	}
	var out []string
	for _, x := range body["data"].([]interface{}) {
		if u := x.(map[string]interface{})["username"].(string); strings.HasPrefix(u, "uji-peran-"+awalan) {
			out = append(out, strings.TrimPrefix(u, "uji-peran-"))
		}
	}
	sort.Strings(out)
	return out
}

func TestAksesPenggunaDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUjiPeran(db)
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(func() { bersihkanUjiPeran(db) })
	}
	seedAsetUji(t, db)

	sa := buatPenggunaUji(t, db, "sa", "superadmin")
	pb := buatPenggunaUji(t, db, "pb", "user")
	pilihPeran(t, pb, beriPeran(t, sa, pb, "pengguna_barang", ""))
	ue1 := buatPenggunaUji(t, db, "v-ue1", "user")
	pilihPeran(t, ue1, beriPeran(t, sa, ue1, "ue1", ue1A))
	kanwil := buatPenggunaUji(t, db, "v-kanwil", "user")
	pilihPeran(t, kanwil, beriPeran(t, sa, kanwil, "kanwil", ue1A+"0199"))
	satker1 := buatPenggunaUji(t, db, "v-satker1", "user")
	pilihPeran(t, satker1, beriPeran(t, sa, satker1, "satker", "971001"))
	satker2 := buatPenggunaUji(t, db, "v-satker2", "user")
	pilihPeran(t, satker2, beriPeran(t, sa, satker2, "satker", "971002"))

	// pengguna sasaran (SSO, dengan kode satker): t1 971001, t2 971002 (kanwil 099710199), t3 971003 (kanwil 099710299), t4 972001 (UE1 lain); t5 non-SSO
	t1 := buatPegawaiUji(t, db, "t1", "user", satkerA1)
	t2 := buatPegawaiUji(t, db, "t2", "user", satkerA2)
	t3 := buatPegawaiUji(t, db, "t3", "user", satkerA3)
	t4 := buatPegawaiUji(t, db, "t4", "user", satkerB1)
	t5 := buatPegawaiUji(t, db, "t5", "user", "")
	// superadmin lain (SSO) di dalam cakupan UE1 A: tidak boleh terlihat oleh siapa pun selain superadmin
	tsa := buatPegawaiUji(t, db, "tsa", "superadmin", satkerA1)
	db.Exec(`UPDATE users SET is_protected = 1 WHERE id = @p1`, tsa.id)

	t.Run("daftar pengguna menurut peran", func(t *testing.T) {
		for _, c := range []struct {
			nama string
			p    penggunaUji
			want string
		}{
			{"superadmin melihat semua termasuk superadmin", sa, "t1,t2,t3,t4,t5,tsa"},
			{"pengguna barang: semua kecuali superadmin", pb, "t1,t2,t3,t4,t5"},
			{"ue1 09971: kode satker berawalan 5 digit sama", ue1, "t1,t2,t3"},
			{"kanwil 099710199: 9 digit sama", kanwil, "t1,t2"},
			{"satker 971001: karakter 10-15 sama", satker1, "t1"},
			{"satker 971002", satker2, "t2"},
		} {
			if got := strings.Join(namaDalamDaftar(t, c.p, "t"), ","); got != c.want {
				t.Errorf("%s: %s, want %s", c.nama, got, c.want)
			}
		}
		// pengguna non-SSO (tanpa kode satker) hanya terlihat oleh yang tidak dibatasi cakupan
		for nama, p := range map[string]penggunaUji{"ue1": ue1, "kanwil": kanwil, "satker": satker1} {
			if strings.Contains(strings.Join(namaDalamDaftar(t, p, "t"), ","), "t5") {
				t.Errorf("%s melihat akun non-SSO t5", nama)
			}
		}
	})

	t.Run("daftar memuat satker aset, peran data, tamu, dan status persetujuan", func(t *testing.T) {
		db.Exec(`INSERT INTO user_roles (user_id, role, kode) VALUES (@p1, N'satker', N'971001')`, t1.id)
		_, body, _ := panggil(t, "GET", "/users/"+t1.id, sa.token, "")
		d := data(t, body)
		if d["kode_satker"] != satkerA1 || d["satker_aset"] != "SATKER A1" || d["nip"] != "199001012015011001" || d["tamu"] != false || d["setuju"] != true {
			t.Errorf("detail t1: %v", d)
		}
		if pr := d["peran_data"].([]interface{}); len(pr) != 1 || pr[0].(map[string]interface{})["role"] != "satker" {
			t.Errorf("peran_data t1 = %v", d["peran_data"])
		}
		_, body, _ = panggil(t, "GET", "/users/"+t2.id, sa.token, "")
		if d := data(t, body); d["tamu"] != true || len(d["peran_data"].([]interface{})) != 0 {
			t.Errorf("t2 tanpa peran harus tamu: %v", d)
		}
		_, body, _ = panggil(t, "GET", "/users/"+t5.id, sa.token, "")
		if d := data(t, body); d["kode_satker"] != nil || d["satker_aset"] != nil {
			t.Errorf("akun non-SSO tanpa kode satker: %v", d)
		}
	})

	t.Run("UE1, Kanwil, dan Satker hanya melihat: detail dalam cakupan, 404 di luar, tidak ada aksi ubah", func(t *testing.T) {
		if code, _, _ := panggil(t, "GET", "/users/"+t1.id, ue1.token, ""); code != 200 {
			t.Errorf("ue1 melihat pengguna dalam cakupan = %d, want 200", code)
		}
		if code, _, _ := panggil(t, "GET", "/users/"+t4.id, ue1.token, ""); code != 404 {
			t.Errorf("ue1 melihat pengguna UE1 lain = %d, want 404", code)
		}
		if code, _, _ := panggil(t, "GET", "/users/"+t3.id, kanwil.token, ""); code != 404 {
			t.Errorf("kanwil melihat pengguna kanwil lain = %d, want 404", code)
		}
		if code, _, _ := panggil(t, "GET", "/users/"+t2.id, satker1.token, ""); code != 404 {
			t.Errorf("satker melihat pengguna satker lain = %d, want 404", code)
		}
		if code, _, _ := panggil(t, "GET", "/users/"+tsa.id, ue1.token, ""); code != 404 {
			t.Errorf("ue1 melihat superadmin = %d, want 404", code)
		}
		if code, _, _ := panggil(t, "GET", "/users/"+t1.id+"/peran", ue1.token, ""); code != 200 {
			t.Errorf("ue1 melihat peran pengguna dalam cakupan = %d, want 200", code)
		}
		if code, _, _ := panggil(t, "GET", "/users/"+t4.id+"/peran", ue1.token, ""); code != 404 {
			t.Errorf("ue1 melihat peran pengguna di luar cakupan = %d, want 404", code)
		}
		for nama, p := range map[string]penggunaUji{"ue1": ue1, "kanwil": kanwil, "satker": satker1} {
			for _, c := range []struct{ metode, path, badan string }{
				{"POST", "/users", `{"source":"manual","username":"uji-peran-x","password":"rahasia123","email":"uji-peran-x@example.test","full_name":"X"}`},
				{"PUT", "/users/" + t1.id, `{"full_name":"X","email":"x@example.test","is_active":true}`},
				{"PUT", "/users/" + t1.id + "/deactivate", ""},
				{"DELETE", "/users/" + t1.id, ""},
				{"POST", "/users/" + t1.id + "/peran", `{"role":"pengguna_barang"}`},
				{"DELETE", "/users/" + t1.id + "/peran/1", ""},
			} {
				if code, _, _ := panggil(t, c.metode, c.path, p.token, c.badan); code != 403 {
					t.Errorf("%s: %s %s = %d, want 403 (hanya melihat)", nama, c.metode, c.path, code)
				}
			}
		}
	})

	t.Run("role akun tidak dapat dijadikan admin atau superadmin lewat API", func(t *testing.T) {
		for _, role := range []string{"admin", "superadmin"} {
			badan := `{"source":"manual","username":"uji-peran-r","password":"rahasia123","email":"uji-peran-r@example.test","full_name":"R","role":"` + role + `"}`
			if code, _, _ := panggil(t, "POST", "/users", sa.token, badan); code != 400 {
				t.Errorf("membuat pengguna role %s = %d, want 400", role, code)
			}
			badan = `{"full_name":"X","email":"x@example.test","is_active":true,"role":"` + role + `"}`
			if code, _, _ := panggil(t, "PUT", "/users/"+t1.id, sa.token, badan); code != 400 {
				t.Errorf("mengubah pengguna menjadi role %s = %d, want 400", role, code)
			}
		}
		if code, _, _ := panggil(t, "PUT", "/users/"+t1.id+"/role", sa.token, `{"role":"superadmin"}`); code != 404 {
			t.Errorf("rute ubah role (sudah dihapus) = %d, want 404", code)
		}
		var role string
		db.QueryRow(`SELECT role FROM users WHERE id = @p1`, t1.id).Scan(&role)
		if role != "user" {
			t.Errorf("role t1 = %q, want user", role)
		}
		// pengguna yang dibuat superadmin berstatus user dan tamu
		if code, _, _ := panggil(t, "POST", "/users", sa.token, `{"source":"manual","username":"uji-peran-baru","password":"rahasia123","email":"uji-peran-baru@example.test","full_name":"Baru"}`); code != 201 {
			t.Errorf("membuat pengguna tanpa role = %d, want 201", code)
		}
		var rl string
		var setuju sql.NullTime
		db.QueryRow(`SELECT role, persetujuan_at FROM users WHERE username = N'uji-peran-baru'`).Scan(&rl, &setuju)
		if rl != "user" || setuju.Valid {
			t.Errorf("pengguna baru: role=%q persetujuan=%v, want user dan belum setuju", rl, setuju)
		}
	})

	t.Run("profil: superadmin tidak wajib setuju dan bukan tamu", func(t *testing.T) {
		_, body, _ := panggil(t, "GET", "/auth/me", sa.token, "")
		d := data(t, body)
		ps := d["persetujuan"].(map[string]interface{})
		if d["tamu"] != false || ps["sudah"] != true || d["role"] != "superadmin" {
			t.Errorf("me superadmin: %v", d)
		}
	})
}

func TestPersetujuanDanTamuDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUjiPeran(db)
	lamaEmail, lamaNIP := "", ""
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(func() { bersihkanUjiPeran(db) })
	}
	seedAsetUji(t, db)
	sa := buatPenggunaUji(t, db, "sa", "superadmin")

	baca := func(p penggunaUji) map[string]interface{} {
		t.Helper()
		code, body, _ := panggil(t, "GET", "/auth/persetujuan", p.token, "")
		if code != 200 {
			t.Fatalf("GET /auth/persetujuan = %d %v", code, body)
		}
		return data(t, body)
	}
	kirim := func(p penggunaUji, badan map[string]interface{}) (int, map[string]interface{}) {
		t.Helper()
		b, _ := json.Marshal(badan)
		code, body, _ := panggil(t, "POST", "/auth/persetujuan", p.token, string(b))
		return code, body
	}

	t.Run("pengguna SSO baru: hanya modal; setelah setuju tetap tamu sampai diberi peran", func(t *testing.T) {
		p := buatPenggunaUjiBelumSetuju(t, db, "sso-baru", "user")
		db.Exec(`UPDATE users SET auth_provider = N'sso' WHERE id = @p1`, p.id)

		// semua fitur tertutup dengan alasan persetujuan, kecuali profil dan pernyataan
		for _, path := range []string{"/digitalisasi/ringkasan", "/satker/keterhubungan", "/sapa/saya", "/inaproc/analitik", "/referensi/ue1", "/referensi/kanwil", "/users"} {
			code, body, _ := panggil(t, "GET", path, p.token, "")
			if code != 403 || body["code"] != "persetujuan_diperlukan" {
				t.Errorf("GET %s sebelum menyetujui = %d %v, want 403 persetujuan_diperlukan", path, code, body["code"])
			}
		}
		for _, path := range []string{"/auth/me", "/auth/peran", "/auth/persetujuan"} {
			if code, _, _ := panggil(t, "GET", path, p.token, ""); code != 200 {
				t.Errorf("GET %s oleh tamu = %d, want 200", path, code)
			}
		}
		d := baca(p)
		if d["sudah"] != false || d["frasa"] != persetujuan.Frasa || d["versi"] != persetujuan.Versi || d["isi_profil"] != false || len(d["paragraf"].([]interface{})) < 3 {
			t.Errorf("pernyataan SSO: %v", d)
		}
		_, body, _ := panggil(t, "GET", "/auth/me", p.token, "")
		me := data(t, body)
		if me["tamu"] != true || me["persetujuan"].(map[string]interface{})["sudah"] != false {
			t.Errorf("me sebelum setuju: %v", me)
		}

		// frasa salah atau kotak tidak dicentang ditolak
		for _, b := range []map[string]interface{}{
			{"frasa": "", "setuju": true}, {"frasa": "saya tidak setuju", "setuju": true}, {"frasa": persetujuan.Frasa, "setuju": false}, {"setuju": true},
		} {
			if code, _ := kirim(p, b); code != 400 {
				t.Errorf("persetujuan %v = %d, want 400", b, code)
			}
		}
		if code, _ := kirim(p, map[string]interface{}{"frasa": "  saya   setuju ", "setuju": true}); code != 200 {
			t.Fatalf("persetujuan sah = %d", code)
		}
		if baca(p)["sudah"] != true {
			t.Error("setelah menyetujui, sudah harus true")
		}
		// tetap tamu: tidak ada fitur yang terbuka
		code, body, _ := panggil(t, "GET", "/digitalisasi/ringkasan", p.token, "")
		if code != 403 || body["code"] != "tamu" {
			t.Errorf("setelah setuju tetapi tanpa peran = %d %v, want 403 tamu", code, body["code"])
		}
		// diberi peran: fitur terbuka sesuai cakupan
		beriPeran(t, sa, p, "pengguna_barang", "")
		if code, _, _ := panggil(t, "GET", "/digitalisasi/ringkasan", p.token, ""); code != 200 {
			t.Errorf("setelah diberi peran = %d, want 200", code)
		}
		// menyetujui lagi tidak mengubah apa pun
		if code, _ := kirim(p, map[string]interface{}{"frasa": persetujuan.Frasa, "setuju": true}); code != 200 {
			t.Errorf("persetujuan ulang = %d, want 200", code)
		}
	})

	t.Run("versi pernyataan berubah: pengguna diminta menyetujui ulang", func(t *testing.T) {
		p := buatPenggunaUji(t, db, "versi", "user")
		beriPeran(t, sa, p, "pengguna_barang", "")
		if code, _, _ := panggil(t, "GET", "/digitalisasi/ringkasan", p.token, ""); code != 200 {
			t.Fatalf("sebelum versi berubah = %d", code)
		}
		db.Exec(`UPDATE users SET persetujuan_versi = N'0' WHERE id = @p1`, p.id)
		code, body, _ := panggil(t, "GET", "/digitalisasi/ringkasan", p.token, "")
		if code != 403 || body["code"] != "persetujuan_diperlukan" {
			t.Errorf("versi lama = %d %v, want 403 persetujuan_diperlukan", code, body["code"])
		}
	})

	t.Run("akun non-SSO mengisi nama lengkap, NIP, dan email bersama persetujuan", func(t *testing.T) {
		p := buatPenggunaUjiBelumSetuju(t, db, "lokal", "user") // auth_provider bawaan 'local'
		d := baca(p)
		if d["isi_profil"] != true || d["auth_provider"] != "local" {
			t.Fatalf("akun non-SSO harus mengisi profil: %v", d)
		}
		pencatat := buatPenggunaUji(t, db, "pemilik-email", "user")
		db.Exec(`UPDATE users SET email = N'dipakai@kemenkeu.go.id' WHERE id = @p1`, pencatat.id)
		db.Exec(`UPDATE users SET username = N'uji-peran-nama-login' WHERE id = @p1`, pencatat.id)

		dasar := func(m map[string]interface{}) map[string]interface{} {
			b := map[string]interface{}{"frasa": persetujuan.Frasa, "setuju": true, "nama": "Budi Santoso", "nip": "199609102018011005", "email": "budi@kemenkeu.go.id"}
			for k, v := range m {
				b[k] = v
			}
			return b
		}
		// isian tidak sah: 400 dengan galat per isian, dan tidak ada yang tersimpan
		for _, c := range []struct {
			nama  string
			badan map[string]interface{}
			kunci string
		}{
			{"nama kosong", dasar(map[string]interface{}{"nama": ""}), "nama"},
			{"nip bukan angka", dasar(map[string]interface{}{"nip": "19960910201801100X"}), "nip"},
			{"nip pendek", dasar(map[string]interface{}{"nip": "1996"}), "nip"},
			{"email salah", dasar(map[string]interface{}{"email": "bukan-email"}), "email"},
			{"semua kosong", map[string]interface{}{"frasa": persetujuan.Frasa, "setuju": true}, "nama"},
		} {
			code, body := kirim(p, c.badan)
			g, _ := body["galat"].(map[string]interface{})
			if code != 400 || g[c.kunci] == nil {
				t.Errorf("%s = %d galat=%v, want 400 dengan galat %s", c.nama, code, body["galat"], c.kunci)
			}
		}
		if baca(p)["sudah"] != false {
			t.Error("isian tidak sah tidak boleh menyimpan persetujuan")
		}
		// email sudah dipakai akun lain (email maupun nama login)
		for _, em := range []string{"dipakai@kemenkeu.go.id", "DIPAKAI@Kemenkeu.GO.ID", "uji-peran-nama-login"} {
			if code, _ := kirim(p, dasar(map[string]interface{}{"email": em})); code != 400 && code != 409 {
				t.Errorf("email %q yang bentrok = %d, want 409 (atau 400 bila bukan email sah)", em, code)
			}
		}
		if code, _ := kirim(p, dasar(map[string]interface{}{"email": "dipakai@kemenkeu.go.id"})); code != 409 {
			t.Errorf("email bentrok = %d, want 409", code)
		}
		// identitas superadmin dicadangkan (.env) tidak boleh dipakai akun non-SSO
		lamaEmail, lamaNIP = cfgSuperadmin()
		aturSuperadmin("bos@kemenkeu.go.id", "198001012005011001")
		defer aturSuperadmin(lamaEmail, lamaNIP)
		if code, _ := kirim(p, dasar(map[string]interface{}{"email": "bos@kemenkeu.go.id"})); code != 400 {
			t.Errorf("email superadmin dicadangkan = %d, want 400", code)
		}
		if code, _ := kirim(p, dasar(map[string]interface{}{"nip": "198001012005011001"})); code != 400 {
			t.Errorf("NIP superadmin dicadangkan = %d, want 400", code)
		}
		// isian sah tersimpan (nama dirapikan, email huruf kecil)
		if code, body := kirim(p, dasar(map[string]interface{}{"nama": "  Budi   Santoso ", "email": " Budi@Kemenkeu.GO.ID "})); code != 200 {
			t.Fatalf("persetujuan sah akun non-SSO = %d %v", code, body)
		}
		var nama, nip, email string
		var versi sql.NullString
		db.QueryRow(`SELECT full_name, nip, email, persetujuan_versi FROM users WHERE id = @p1`, p.id).Scan(&nama, &nip, &email, &versi)
		if nama != "Budi Santoso" || nip != "199609102018011005" || email != "budi@kemenkeu.go.id" || versi.String != persetujuan.Versi {
			t.Errorf("tersimpan: %q %q %q %q", nama, nip, email, versi.String)
		}
		d = baca(p)
		if d["sudah"] != true || d["profil"].(map[string]interface{})["nip"] != "199609102018011005" {
			t.Errorf("setelah menyimpan: %v", d)
		}
		// masih tamu sampai diberi peran
		if code, body, _ := panggil(t, "GET", "/digitalisasi/ringkasan", p.token, ""); code != 403 || body["code"] != "tamu" {
			t.Errorf("akun non-SSO tanpa peran = %d %v, want 403 tamu", code, body["code"])
		}
	})

	t.Run("pengguna tanpa token atau dinonaktifkan tetap ditolak", func(t *testing.T) {
		if code, _, _ := panggil(t, "GET", "/auth/persetujuan", "", ""); code != 401 {
			t.Errorf("tanpa token = %d, want 401", code)
		}
		p := buatPenggunaUjiBelumSetuju(t, db, "nonaktif", "user")
		db.Exec(`UPDATE users SET is_active = 0 WHERE id = @p1`, p.id)
		if code, _ := kirim(p, map[string]interface{}{"frasa": persetujuan.Frasa, "setuju": true}); code != 401 {
			t.Errorf("akun nonaktif menyetujui = %d, want 401", code)
		}
	})
}

// Migrasi 055: akun admin lama menjadi pengguna biasa dengan peran Pengguna Barang; superadmin permanen tidak berubah; aman dijalankan ulang.
func TestMigrasi055AdminLamaMenjadiPenggunaBarang(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUjiPeran(db)
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(func() { bersihkanUjiPeran(db) })
	}
	adminLama := buatPenggunaUji(t, db, "admin-lama", "admin")
	adminPunya := buatPenggunaUji(t, db, "admin-punya", "admin")
	protected := buatPenggunaUji(t, db, "admin-protected", "admin")
	biasa := buatPenggunaUji(t, db, "biasa", "user")
	db.Exec(`UPDATE users SET is_protected = 1 WHERE id = @p1`, protected.id)
	db.Exec(`INSERT INTO user_roles (user_id, role, kode, dibuat_oleh) VALUES (@p1, N'pengguna_barang', N'', N'manual')`, adminPunya.id)

	skrip, err := os.ReadFile("../migrations/055_persetujuan_dan_akses_tamu.sql")
	if err != nil {
		t.Fatal(err)
	}
	jalankan := func() {
		t.Helper()
		for i, batch := range regexp.MustCompile(`(?im)^[ \t]*GO[ \t]*\r?$`).Split(string(skrip), -1) {
			if strings.TrimSpace(batch) == "" {
				continue
			}
			if _, err := db.Exec(batch); err != nil {
				t.Fatalf("batch %d: %v", i+1, err)
			}
		}
	}
	keadaan := func(p penggunaUji) string {
		var role string
		var n int
		db.QueryRow(`SELECT role FROM users WHERE id = @p1`, p.id).Scan(&role)
		db.QueryRow(`SELECT COUNT(*) FROM user_roles WHERE user_id = @p1 AND role = N'pengguna_barang'`, p.id).Scan(&n)
		return role + "/" + string(rune('0'+n))
	}
	for i := 0; i < 2; i++ { // dua kali: idempotent
		jalankan()
		for nama, c := range map[string]struct {
			p    penggunaUji
			want string
		}{
			"admin lama":             {adminLama, "user/1"},
			"admin yang sudah punya": {adminPunya, "user/1"}, // tidak digandakan
			"admin protected":        {protected, "admin/0"},
			"pengguna biasa":         {biasa, "user/0"},
		} {
			if got := keadaan(c.p); got != c.want {
				t.Errorf("putaran %d, %s: %s, want %s", i+1, nama, got, c.want)
			}
		}
	}
	// bekas admin langsung bekerja sebagai Pengguna Barang: mengelola pengguna biasa, tetapi tidak memakai fitur superadmin (token lama bertuliskan admin tidak berarti)
	if code, _, _ := panggil(t, "GET", "/users", adminLama.token, ""); code != http.StatusOK {
		t.Errorf("bekas admin melihat pengguna = %d, want 200", code)
	}
	if code, _, _ := panggil(t, "POST", "/inaproc/penarikan", adminLama.token, `{}`); code != http.StatusForbidden {
		t.Errorf("bekas admin memulai penarikan = %d, want 403", code)
	}
}

// Sinkronisasi superadmin dengan .env saat server dimulai: yang cocok dinaikkan, yang lain diturunkan, dan .env kosong tidak mengubah apa pun.
func TestSinkronkanSuperadminDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUjiPeran(db)
	lamaEmail, lamaNIP := cfgSuperadmin()
	t.Cleanup(func() { aturSuperadmin(lamaEmail, lamaNIP) })
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(func() { bersihkanUjiPeran(db) })
	}
	cocokEmail := buatPegawaiUji(t, db, "cocok-email", "user", "")
	cocokNIP := buatPegawaiUji(t, db, "cocok-nip", "user", satkerA1)
	mantan := buatPegawaiUji(t, db, "mantan", "superadmin", "")
	db.Exec(`UPDATE users SET is_protected = 1 WHERE id = @p1`, mantan.id)
	lokal := buatPenggunaUji(t, db, "lokal-super", "superadmin")
	biasa := buatPegawaiUji(t, db, "biasa", "user", satkerA2)
	nonaktif := buatPegawaiUji(t, db, "cocok-nonaktif", "superadmin", "")
	db.Exec(`UPDATE users SET is_protected = 1, is_active = 0, email = N'uji-peran-nonaktif@kemenkeu.go.id' WHERE id = @p1`, nonaktif.id)
	db.Exec(`UPDATE users SET email = N'uji-peran-cocok@kemenkeu.go.id' WHERE id = @p1`, cocokEmail.id)
	db.Exec(`UPDATE employees SET nip = N'198001012005011001' WHERE sso_sub = N'uji-peran-emp-cocok-nip'`)

	keadaan := func(p penggunaUji) string {
		var role string
		var prot, aktif bool
		db.QueryRow(`SELECT role, is_protected, is_active FROM users WHERE id = @p1`, p.id).Scan(&role, &prot, &aktif)
		s := role
		if prot {
			s += "+protected"
		}
		if !aktif {
			s += "+nonaktif"
		}
		return s
	}
	semua := map[string]penggunaUji{"cocokEmail": cocokEmail, "cocokNIP": cocokNIP, "mantan": mantan, "lokal": lokal, "biasa": biasa, "nonaktif": nonaktif}
	awal := map[string]string{}
	for k, p := range semua {
		awal[k] = keadaan(p)
	}

	// .env kosong: tidak ada yang diubah
	aturSuperadmin("", "")
	handlers.SinkronkanSuperadmin()
	for k, p := range semua {
		if got := keadaan(p); got != awal[k] {
			t.Errorf("tanpa konfigurasi %s berubah: %s -> %s", k, awal[k], got)
		}
	}

	aturSuperadmin("uji-peran-cocok@kemenkeu.go.id, uji-peran-nonaktif@kemenkeu.go.id", "198001012005011001")
	for i := 0; i < 2; i++ { // dua kali: idempotent
		handlers.SinkronkanSuperadmin()
		for k, want := range map[string]string{
			"cocokEmail": "superadmin+protected", // dinaikkan lewat email
			"cocokNIP":   "superadmin+protected", // dinaikkan lewat NIP pegawai SSO
			"nonaktif":   "superadmin+protected", // cocok: diaktifkan kembali
			"mantan":     "user",                 // tidak lagi ada di .env: diturunkan
			"lokal":      "user",                 // akun non-SSO tidak pernah superadmin
			"biasa":      "user",
		} {
			if got := keadaan(semua[k]); got != want {
				t.Errorf("putaran %d, %s: %s, want %s", i+1, k, got, want)
			}
		}
	}
	// hak langsung berlaku: token lama "superadmin" milik mantan tidak lagi membuka fitur superadmin (role dibaca dari database)
	if code, _, _ := panggil(t, "GET", "/users", mantan.token, ""); code != 403 {
		t.Errorf("mantan superadmin membuka /users = %d, want 403 (tamu)", code)
	}
}

func cfgSuperadmin() (string, string) {
	return config.Cfg.SuperadminProtectedEmail, config.Cfg.SuperadminProtectedNIP
}

func aturSuperadmin(email, nip string) {
	config.Cfg.SuperadminProtectedEmail, config.Cfg.SuperadminProtectedNIP = email, nip
}

// Pendaftaran lokal (POST /auth/register) tidak boleh memakai email atau username yang dicadangkan untuk superadmin di .env, dan akun yang didaftarkan berstatus tamu.
func TestRegisterTidakBolehMemakaiIdentitasSuperadmin(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUjiPeran(db)
	lamaEmail, lamaNIP := cfgSuperadmin()
	t.Cleanup(func() { aturSuperadmin(lamaEmail, lamaNIP) })
	t.Cleanup(func() { db.Exec(`DELETE FROM users WHERE username LIKE N'uji-peran-reg%'`) })
	aturSuperadmin("uji-peran-reg-bos@kemenkeu.go.id", "")

	badan := func(user, email string) string {
		return `{"username":"` + user + `","email":"` + email + `","password":"rahasia123","full_name":"Uji Daftar"}`
	}
	if code, _, _ := panggil(t, "POST", "/auth/register", "", badan("uji-peran-reg-a", "UJI-PERAN-REG-BOS@kemenkeu.go.id")); code != 400 {
		t.Errorf("daftar dengan email superadmin = %d, want 400", code)
	}
	if code, _, _ := panggil(t, "POST", "/auth/register", "", badan("uji-peran-reg-bos@kemenkeu.go.id", "lain@example.test")); code != 400 {
		t.Errorf("daftar dengan username sama dengan email superadmin = %d, want 400", code)
	}
	if code, _, _ := panggil(t, "POST", "/auth/register", "", badan("uji-peran-reg-ok", "uji-peran-reg-ok@example.test")); code != 201 {
		t.Errorf("daftar biasa = %d, want 201", code)
	}
	var role string
	var setuju sql.NullTime
	db.QueryRow(`SELECT role, persetujuan_at FROM users WHERE username = N'uji-peran-reg-ok'`).Scan(&role, &setuju)
	if role != "user" || setuju.Valid {
		t.Errorf("akun terdaftar: role=%q persetujuan=%v, want user dan belum setuju (tamu)", role, setuju)
	}
}
