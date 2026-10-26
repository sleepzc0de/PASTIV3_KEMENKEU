package handlers

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"pasti-v3-backend/apiakses"
)

func bacaSumber(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("baca %s: %v", path, err)
	}
	return string(b)
}

// jawabanDokumentasi: isi jawaban setelah melewati JSON, sama dengan yang diterima frontend.
func jawabanDokumentasi(t *testing.T) map[string]any {
	t.Helper()
	b, err := json.Marshal(susunDokumentasi(apiakses.PengaturanBawaan()))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDokumentasiMemuatSemuaDatasetTanpaKolomTertutup(t *testing.T) {
	m := jawabanDokumentasi(t)
	daftar, _ := m["dataset"].([]any)
	if want := len(apiakses.Katalog()) + len(apiakses.KatalogPengadaan()); len(daftar) != want {
		t.Fatalf("dataset di dokumentasi %d, katalog %d", len(daftar), want)
	}
	lihat := map[string]bool{}
	for _, v := range daftar {
		d := v.(map[string]any)
		kode := d["kode"].(string)
		if lihat[kode] {
			t.Errorf("dataset %s dobel", kode)
		}
		lihat[kode] = true
		if d["alamat"] == "" || d["label"] == "" {
			t.Errorf("%s: alamat atau label kosong", kode)
		}
		kolom := map[string]bool{}
		for _, k := range d["kolom"].([]any) {
			kolom[k.(map[string]any)["nama"].(string)] = true
		}
		// kolom yang tidak pernah diberikan tidak boleh muncul sebagai kolom yang dapat ditarik
		for n := range kolom {
			if strings.HasPrefix(n, "foto") || n == "nama_pengguna" {
				t.Errorf("%s: kolom %s tidak boleh muncul di dokumentasi sebagai kolom yang diberikan", kode, n)
			}
		}
		if tertutup, ok := d["tertutup"].([]any); ok {
			for _, x := range tertutup {
				if kolom[x.(string)] {
					t.Errorf("%s: kolom %v tertutup tetapi tercantum sebagai kolom yang diberikan", kode, x)
				}
			}
		}
	}
	// dataset tanpa scope apa pun tetap tampil: dokumentasi milik superadmin, bukan milik satu kunci
	for _, kode := range apiakses.DaftarKode() {
		if !lihat[kode] {
			t.Errorf("dataset %s tidak ada di dokumentasi", kode)
		}
	}
}

// Penyaring dataset yang tercantum harus nama parameter yang diuraikan; tidak ada penyaring yang tidak dijelaskan.
func TestDokumentasiPenyaringTerurai(t *testing.T) {
	m := jawabanDokumentasi(t)
	uraian := map[string]bool{}
	for _, p := range m["parameter"].([]any) {
		uraian[p.(map[string]any)["nama"].(string)] = true
	}
	for _, v := range m["dataset"].([]any) {
		d := v.(map[string]any)
		for _, p := range d["penyaring"].([]any) {
			if !uraian[p.(string)] {
				t.Errorf("%s: penyaring %v tidak ada di uraian parameter", d["kode"], p)
			}
		}
	}
}

// Setiap parameter yang dibaca handler daftar data harus ada di uraian, dan sebaliknya: uraian tidak boleh menyebut parameter yang tidak ada.
func TestDokumentasiParameterSamaDenganHandler(t *testing.T) {
	baca := map[string]bool{}
	re := regexp.MustCompile(`c\.Query\("(\w+)"\)`)
	for _, f := range []string{"api_akses_data.go", "api_akses_pengadaan.go"} {
		for _, mm := range re.FindAllStringSubmatch(bacaSumber(t, f), -1) {
			baca[mm[1]] = true
		}
	}
	// /saya/penggunaan membaca parameter sendiri (rentang), bukan parameter daftar data
	delete(baca, "rentang")
	uraian := map[string]bool{}
	for _, p := range parameterKatalog() {
		uraian[p["nama"].(string)] = true
	}
	for n := range baca {
		if !uraian[n] {
			t.Errorf("parameter %q dibaca handler tetapi tidak diuraikan di dokumentasi", n)
		}
	}
	for n := range uraian {
		if !baca[n] {
			t.Errorf("parameter %q diuraikan tetapi tidak dibaca handler mana pun", n)
		}
	}
}

// Setiap kode galat gerbang (konstanta Kode* di apiakses/gerbang.go) harus terdokumentasi, dengan status HTTP yang benar-benar dipakai gerbang.
func TestDokumentasiKodeGalatLengkap(t *testing.T) {
	sumber := bacaSumber(t, "../apiakses/gerbang.go")
	kode := map[string]bool{}
	for _, mm := range regexp.MustCompile(`(?m)^\s+Kode\w+\s*=\s*"(\w+)"`).FindAllStringSubmatch(sumber, -1) {
		kode[mm[1]] = true
	}
	if len(kode) < 7 {
		t.Fatalf("hanya %d kode galat terbaca dari gerbang.go", len(kode))
	}
	ada := map[string]bool{}
	for _, g := range galatGerbang() {
		if g.Arti == "" || g.HTTP < 400 {
			t.Errorf("galat %d %q tidak lengkap", g.HTTP, g.Kode)
		}
		if g.Kode != "" {
			ada[g.Kode] = true
			if !kode[g.Kode] {
				t.Errorf("kode %q didokumentasikan tetapi bukan kode gerbang", g.Kode)
			}
		}
	}
	for k := range kode {
		if !ada[k] {
			t.Errorf("kode galat %q ada di gerbang tetapi tidak didokumentasikan", k)
		}
	}
}

// Setiap header yang dikirim gerbang atau dibaca dari permintaan harus terdokumentasi, dan sebaliknya.
func TestDokumentasiHeaderSamaDenganKode(t *testing.T) {
	gerbang := bacaSumber(t, "../apiakses/gerbang.go")
	kunci := bacaSumber(t, "../apiakses/kunci.go")
	dikirim := map[string]bool{"X-Request-ID": true} // dipasang middleware audit untuk semua rute, termasuk rute klien
	for _, mm := range regexp.MustCompile(`(?:h\.Set|c\.Header)\("([\w-]+)"`).FindAllStringSubmatch(gerbang, -1) {
		dikirim[mm[1]] = true
	}
	dibaca := map[string]bool{}
	for _, mm := range regexp.MustCompile(`h\.Get\("([\w-]+)"\)`).FindAllStringSubmatch(kunci, -1) {
		dibaca[mm[1]] = true
	}
	ada := map[string]string{}
	for _, h := range headerGerbang() {
		if h.Arti == "" {
			t.Errorf("header %s tanpa uraian", h.Nama)
		}
		ada[h.Nama] = h.Arah
	}
	for n := range dikirim {
		if ada[n] != "respons" {
			t.Errorf("header respons %s tidak terdokumentasi sebagai respons", n)
		}
	}
	for n := range dibaca {
		if ada[n] != "permintaan" {
			t.Errorf("header permintaan %s tidak terdokumentasi sebagai permintaan", n)
		}
	}
	for n, arah := range ada {
		if arah == "respons" && !dikirim[n] {
			t.Errorf("header respons %s didokumentasikan tetapi tidak dikirim", n)
		}
		if arah == "permintaan" && !dibaca[n] {
			t.Errorf("header permintaan %s didokumentasikan tetapi tidak dibaca", n)
		}
	}
}

func TestDokumentasiBawaanMengikutiPengaturan(t *testing.T) {
	p := apiakses.PengaturanBawaan()
	p.HalamanMaks = 250
	p.LimitPermintaan = 77
	b, _ := json.Marshal(susunDokumentasi(p))
	var m struct {
		AlamatDasar string `json:"alamat_dasar"`
		Bawaan      struct {
			LimitPermintaan int `json:"limit_permintaan"`
			HalamanBawaan   int `json:"halaman_bawaan"`
			HalamanMaks     int `json:"halaman_maks"`
			MinPencarian    int `json:"min_pencarian"`
		} `json:"bawaan"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.AlamatDasar != "/api/data/v1" {
		t.Errorf("alamat dasar %q", m.AlamatDasar)
	}
	if m.Bawaan.LimitPermintaan != 77 || m.Bawaan.HalamanMaks != 250 {
		t.Errorf("bawaan tidak mengikuti pengaturan: %+v", m.Bawaan)
	}
	if m.Bawaan.HalamanBawaan != apiakses.HalamanBawaan || m.Bawaan.MinPencarian != apiakses.MinPencarian {
		t.Errorf("bawaan konstanta salah: %+v", m.Bawaan)
	}
}

// Daftar endpoint di dokumentasi harus sama dengan rute klien yang benar-benar dipasang (metode, alamat, dan apakah menghabiskan kuota).
func TestDokumentasiEndpointSamaDenganRute(t *testing.T) {
	rute := map[string]bool{}
	re := regexp.MustCompile(`dataKlien\.(\w+)\("([^"]+)", handlers\.ApiAksesGerbang\((true|false)\)`)
	param := regexp.MustCompile(`:(\w+)`)
	for _, mm := range re.FindAllStringSubmatch(bacaSumber(t, "../routes/routes.go"), -1) {
		rute[mm[1]+" "+param.ReplaceAllString(mm[2], "{$1}")+" kuota="+mm[3]] = true
	}
	if len(rute) < 7 {
		t.Fatalf("hanya %d rute klien terbaca dari routes.go", len(rute))
	}
	ada := map[string]bool{}
	for _, e := range endpointGerbang() {
		if e.Isi == "" {
			t.Errorf("endpoint %s tanpa uraian", e.Path)
		}
		k := e.Metode + " " + e.Path + " kuota=" + map[bool]string{true: "true", false: "false"}[e.Kuota]
		ada[k] = true
		if !rute[k] {
			t.Errorf("endpoint %q didokumentasikan tetapi tidak ada di routes.go", k)
		}
	}
	for k := range rute {
		if !ada[k] {
			t.Errorf("rute %q ada di routes.go tetapi tidak didokumentasikan", k)
		}
	}
}
