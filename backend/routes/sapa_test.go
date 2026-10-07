package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"pasti-v3-backend/handlers"
	"pasti-v3-backend/peran"
	"pasti-v3-backend/sapa"
)

// Tes HTTP SAPA dengan tabel rute yang asli (RegisterSapa) dan penyimpanan dalam memori. Autentikasi diganti
// middleware tes yang membaca pengguna dari header X-User.

const (
	kodeA = "015010199409294002"
	kodeB = "015040199119091000"

	gAdmin  = "00000000-0000-4000-8000-000000000001"
	gSatkA  = "00000000-0000-4000-8000-000000000002"
	gSatkB  = "00000000-0000-4000-8000-000000000003"
	gKanwil = "00000000-0000-4000-8000-000000000004"
	gUE1    = "00000000-0000-4000-8000-000000000005"
	gTanpa  = "00000000-0000-4000-8000-000000000006"
	gBarang = "00000000-0000-4000-8000-000000000007"
	gKwlLn  = "00000000-0000-4000-8000-000000000008"
)

// pengguna uji: role akun, ditambah peran data aplikasi (peran + kode) yang menjadi peran SAPA-nya. SAPA tidak menetapkan peran sendiri: Satker 6 digit
// (karakter ke-10 sampai ke-15 kode satker), Kanwil 9 digit (9 karakter pertama), UE1 5 digit, Pengguna Barang, atau admin.
type pengguna struct{ id, nama, roleApp, peran, kode string }

var daftarPengguna = map[string]pengguna{
	"admin":      {gAdmin, "admin", "superadmin", "", ""},
	"satkA":      {gSatkA, "satker.a", "user", "satker", "409294"},       // kodeA
	"satkB":      {gSatkB, "satker.b", "user", "satker", "119091"},       // kodeB
	"kanwil":     {gKanwil, "kanwil", "user", "kanwil", "015010199"},     // Kanwil kodeA
	"kanwilLain": {gKwlLn, "kanwil.lain", "user", "kanwil", "015040199"}, // Kanwil kodeB
	"ue1":        {gUE1, "ue1", "user", "ue1", "01501"},
	"barang":     {gBarang, "pengguna.barang", "user", "pengguna_barang", ""},
	"tanpa":      {gTanpa, "tanpa", "user", "", ""},
}

type lingkungan struct {
	t    *testing.T
	r    *gin.Engine
	repo *sapa.MemRepo
}

func siapkan(t *testing.T, repoWrap func(*sapa.MemRepo) sapa.Repo) *lingkungan {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := sapa.NewMemRepo()
	repo.Satker[kodeA] = sapa.SatkerInfo{Kode: kodeA + "KP", Nama: "KPKNL Jakarta I", KabKota: "KOTA JAKARTA PUSAT", KodeUE1: "01501"}
	for _, p := range daftarPengguna {
		repo.Nama[p.id] = p.nama
	}
	ctx := context.Background()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(repo.SimpanRefUE1(ctx, sapa.RefUE1{Kode: "01501", Nama: "Ditjen Contoh", Sekretaris: "Sekretaris Direktorat Jenderal Contoh"}, "seed"))

	var rp sapa.Repo = repo
	if repoWrap != nil {
		rp = repoWrap(repo)
	}
	pulihkan := handlers.GunakanSapaLayanan(&sapa.Layanan{Repo: rp})
	t.Cleanup(pulihkan)

	r := gin.New()
	g := r.Group("/api/v1/sapa", func(c *gin.Context) {
		p, ok := daftarPengguna[c.GetHeader("X-User")]
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "message": "tidak login"})
			return
		}
		c.Set("user_id", p.id)
		c.Set("username", p.nama)
		// Peran yang berlaku seperti dipasang middleware autentikasi (peran data aplikasi yang aktif).
		var baris []peran.Baris
		if p.peran != "" {
			baris = []peran.Baris{{ID: 1, Peran: p.peran, Kode: p.kode}}
		}
		peran.Pasang(c, peran.Selesaikan(p.roleApp, baris))
		c.Next()
	})
	RegisterSapa(g)
	return &lingkungan{t: t, r: r, repo: repo}
}

type jawaban struct {
	kode int
	w    *httptest.ResponseRecorder
	m    map[string]interface{}
}

func (j jawaban) data() map[string]interface{} {
	d, _ := j.m["data"].(map[string]interface{})
	return d
}

func (e *lingkungan) kirim(user, method, path string, body []byte, tipe string) jawaban {
	e.t.Helper()
	req := httptest.NewRequest(method, "/api/v1/sapa"+path, bytes.NewReader(body))
	if tipe == "" {
		tipe = "application/json"
	}
	req.Header.Set("Content-Type", tipe)
	req.Header.Set("X-User", user)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var m map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return jawaban{w.Code, w, m}
}

func (e *lingkungan) json(user, method, path string, v interface{}) jawaban {
	e.t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		e.t.Fatal(err)
	}
	return e.kirim(user, method, path, b, "")
}

func (e *lingkungan) harap(j jawaban, kode int) jawaban {
	e.t.Helper()
	if j.kode != kode {
		e.t.Fatalf("status = %d, want %d; badan: %s", j.kode, kode, j.w.Body.String())
	}
	return j
}

func (e *lingkungan) buatUsulan() string {
	e.t.Helper()
	j := e.harap(e.json("satkA", "POST", "/penjualan", map[string]string{"kode_satker": kodeA}), http.StatusCreated)
	return j.data()["id"].(string) // UUID usulan (pengenal publik)
}

func TestSapaTanpaLoginDitolak(t *testing.T) {
	e := siapkan(t, nil)
	e.harap(e.kirim("siapa", "GET", "/saya", nil, ""), http.StatusUnauthorized)
}

func TestSapaTanpaPeran(t *testing.T) {
	e := siapkan(t, nil)
	j := e.harap(e.kirim("tanpa", "GET", "/saya", nil, ""), 200)
	if j.data()["punya_akses"] != false || j.data()["alasan"] == "" {
		t.Errorf("saya = %v", j.data())
	}
	for _, p := range []string{"/penjualan", "/referensi/satker?kode=" + kodeA, "/penjualan/1", "/dokumen/1/unduh"} {
		j := e.harap(e.kirim("tanpa", "GET", p, nil, ""), http.StatusForbidden)
		if j.m["code"] != "sapa_tanpa_akses" {
			t.Errorf("%s: code = %v", p, j.m["code"])
		}
	}
	e.harap(e.json("tanpa", "POST", "/penjualan", map[string]string{"kode_satker": kodeA}), http.StatusForbidden)
}

func TestSapaSaya(t *testing.T) {
	e := siapkan(t, nil)
	j := e.harap(e.kirim("satkA", "GET", "/saya", nil, ""), 200)
	d := j.data()
	if d["peran"] != "satker" || d["boleh_membuat"] != true || d["kode_satker"] != kodeA || len(d["tahap"].([]interface{})) != 10 {
		t.Errorf("saya satker = %v", d)
	}
	if len(d["jenis_tim"].([]interface{})) != 4 || len(d["bentuk"].([]interface{})) != 4 || len(d["item_dokumen"].([]interface{})) != len(sapa.DaftarItemDokumen) {
		t.Errorf("pilihan formulir = %v %v %v", d["jenis_tim"], d["bentuk"], d["item_dokumen"])
	}
	// Kode satker lengkap (18 digit) untuk peran Satker diturunkan dari data Digitalisasi Aset menurut kode 6 digit perannya.
	if d["kode_satker6"] != "409294" || len(d["satker_pilihan"].([]interface{})) != 1 {
		t.Errorf("satker saya = %v %v", d["kode_satker6"], d["satker_pilihan"])
	}
	if d := e.harap(e.kirim("kanwil", "GET", "/saya", nil, ""), 200).data(); d["boleh_membuat"] != false || d["punya_akses"] != true || d["peran"] != "kanwil" || d["kode_kanwil"] != "015010199" {
		t.Errorf("saya kanwil = %v", d)
	}
	if d := e.harap(e.kirim("ue1", "GET", "/saya", nil, ""), 200).data(); d["peran"] != "ue1" || d["kode_ue1"] != "01501" || d["boleh_membuat"] != false {
		t.Errorf("saya ue1 = %v", d)
	}
	if d := e.harap(e.kirim("barang", "GET", "/saya", nil, ""), 200).data(); d["peran"] != "pengguna_barang" || d["peran_label"] != "Pengguna Barang" || d["punya_akses"] != true || d["boleh_membuat"] != false || d["admin"] != false {
		t.Errorf("saya pengguna barang = %v", d)
	}
	if d := e.harap(e.kirim("admin", "GET", "/saya", nil, ""), 200).data(); d["admin"] != true || d["boleh_membuat"] != true {
		t.Errorf("saya admin = %v", d)
	}
}

func TestSapaRuteAdminDijaga(t *testing.T) {
	e := siapkan(t, nil)
	id := e.buatUsulan()
	for _, c := range []struct{ method, path string }{
		{"GET", "/template"}, {"POST", "/template/nd_satker"}, {"GET", "/template/nd_satker/unduh"},
		{"GET", "/ref-ue1"}, {"PUT", "/ref-ue1/01501"}, {"DELETE", "/ref-ue1/01501"},
		{"GET", "/bmn"}, {"PUT", "/bmn/satuan"}, {"DELETE", "/bmn/satuan?nama=unit"}, {"PUT", "/bmn/jenis"}, {"DELETE", "/bmn/jenis?nama=Tanah"},
		{"POST", fmt.Sprintf("/penjualan/%s/tahap/tim/buka-ulang", id)},
	} {
		for _, u := range []string{"satkA", "kanwil", "ue1", "barang", "tanpa"} {
			if j := e.kirim(u, c.method, c.path, []byte("{}"), ""); j.kode != http.StatusForbidden {
				t.Errorf("%s %s oleh %s: status %d, want 403", c.method, c.path, u, j.kode)
			}
		}
	}
}

func TestSapaAlurSatkerSampaiUnduh(t *testing.T) {
	e := siapkan(t, nil)
	id := e.buatUsulan()
	base := fmt.Sprintf("/penjualan/%s", id)

	// Nama satker diisi dari data Digitalisasi Aset; Noreg bernomor.
	d := e.harap(e.kirim("satkA", "GET", base, nil, ""), 200).data()
	u := d["usulan"].(map[string]interface{})
	if u["nama_satker"] != "KPKNL Jakarta I" || !strings.HasPrefix(u["noreg"].(string), "PJ-") || d["tahap_saat_ini"] != "tim" {
		t.Errorf("detail = %v", d)
	}
	// Bentuk JSON tahap yang dipakai frontend: "dokumen" = daftar dokumen hasil (bukan jenis), jenis ada di "jenis_dokumen".
	tahap := d["tahap"].([]interface{})
	nd := tahap[2].(map[string]interface{})
	if nd["kunci"] != "nd_satker" || nd["jenis_dokumen"] != "nd_satker" || nd["template_tersedia"].(map[string]interface{})["nd_satker"] != true {
		t.Errorf("tahap nd_satker = %v", nd)
	}
	if docs, ok := nd["dokumen"].([]interface{}); !ok || len(docs) != 0 {
		t.Errorf("dokumen harus berupa daftar kosong: %v", nd["dokumen"])
	}
	if sar, ok := nd["saran"].(map[string]interface{}); !ok || sar["tujuan_surat"] != "Sekretaris Direktorat Jenderal Contoh" {
		t.Errorf("saran = %v", nd["saran"])
	}
	if ext := tahap[3].(map[string]interface{}); ext["jenis"] != "eksternal" || ext["jenis_dokumen"] != nil || ext["wajib_nomor_tanggal"] != true {
		t.Errorf("tahap eksternal = %v", ext)
	}

	// Tahap tidak boleh dilompati.
	e.harap(e.json("satkA", "POST", base+"/tahap/nd_satker/dokumen", sapa.ContohNDSatker()), http.StatusConflict)
	for _, k := range []string{"tim", "ba"} {
		e.harap(e.json("satkA", "POST", base+"/tahap/"+k+"/lewati", map[string]string{"catatan": "dibuat di luar aplikasi"}), 200)
	}

	// Draf, lalu dokumen: data kosong ditolak dengan rincian per bidang.
	e.harap(e.json("satkA", "PUT", base+"/tahap/nd_satker", map[string]string{"alasan": "rusak berat"}), 200)
	bad := e.harap(e.json("satkA", "POST", base+"/tahap/nd_satker/dokumen", map[string]string{"alasan": "rusak berat"}), http.StatusBadRequest)
	if errs, _ := bad.m["errors"].([]interface{}); len(errs) < 2 || bad.m["success"] != false {
		t.Errorf("respons validasi = %v", bad.m)
	}

	ok := e.harap(e.json("satkA", "POST", base+"/tahap/nd_satker/dokumen", sapa.ContohNDSatker()), http.StatusCreated)
	dokID := int(ok.data()["dokumen"].(map[string]interface{})["id"].(float64))

	unduh := e.harap(e.kirim("satkA", "GET", fmt.Sprintf("/dokumen/%d/unduh", dokID), nil, ""), 200)
	if !strings.Contains(unduh.w.Header().Get("Content-Type"), "wordprocessingml") ||
		!strings.HasPrefix(unduh.w.Header().Get("Content-Disposition"), "attachment") ||
		unduh.w.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!bytes.HasPrefix(unduh.w.Body.Bytes(), []byte("PK")) {
		t.Errorf("unduhan: %v %q", unduh.w.Header(), unduh.w.Body.Bytes()[:4])
	}
	if !strings.Contains(unduh.w.Header().Get("Content-Disposition"), ".docx") {
		t.Errorf("nama berkas: %s", unduh.w.Header().Get("Content-Disposition"))
	}

	// Satker lain tidak melihat usulan maupun dokumennya (404, bukan 403, supaya keberadaannya tidak bocor).
	e.harap(e.kirim("satkB", "GET", base, nil, ""), http.StatusNotFound)
	e.harap(e.kirim("satkB", "GET", fmt.Sprintf("/dokumen/%d/unduh", dokID), nil, ""), http.StatusNotFound)
	if l := e.harap(e.kirim("satkB", "GET", "/penjualan", nil, ""), 200).data(); l["total"] != float64(0) {
		t.Errorf("daftar satker lain = %v", l)
	}
	// UE1 di bawahnya dan kanwil melihatnya; UE1 tidak boleh mengerjakan tahap satker.
	e.harap(e.kirim("ue1", "GET", base, nil, ""), 200)
	e.harap(e.kirim("kanwil", "GET", base, nil, ""), 200)
	e.harap(e.json("ue1", "POST", base+"/tahap/nadine_satker/selesai", map[string]string{"nomor": "ND-1", "tanggal": "2026-10-01"}), http.StatusForbidden)

	// Tahap eksternal: nomor dan tanggal Nadine wajib.
	e.harap(e.json("satkA", "POST", base+"/tahap/nadine_satker/selesai", map[string]string{"nomor": ""}), http.StatusBadRequest)
	e.harap(e.json("satkA", "POST", base+"/tahap/nadine_satker/selesai", map[string]string{"nomor": "ND-1/2026", "tanggal": "2026-10-01"}), 200)
	e.harap(e.json("satkA", "POST", base+"/tahap/siman_satker/selesai", map[string]string{}), 200)
	e.harap(e.json("kanwil", "POST", base+"/tahap/siman_kanwil/selesai", map[string]string{"catatan": "ok"}), 200)

	// ND UE1 setelah UE1 menerima.
	e.harap(e.json("ue1", "POST", base+"/tahap/nd_ue1/dokumen", sapa.ContohNDUE1()), http.StatusConflict)
	e.harap(e.json("ue1", "POST", base+"/tahap/ue1_terima/selesai", map[string]string{}), 200)
	e.harap(e.json("ue1", "POST", base+"/tahap/nd_ue1/dokumen", sapa.ContohNDUE1()), http.StatusCreated)

	// Buka ulang hanya admin.
	e.harap(e.json("admin", "POST", base+"/tahap/ue1_terima/buka-ulang", nil), http.StatusConflict) // nd_ue1 sudah selesai
	e.harap(e.json("admin", "POST", base+"/tahap/nd_ue1/buka-ulang", nil), 200)
}

func TestSapaMasukanTidakValid(t *testing.T) {
	e := siapkan(t, nil)
	id := e.buatUsulan()
	base := fmt.Sprintf("/penjualan/%s", id)

	for _, p := range []string{"/penjualan/abc", "/penjualan/0", "/penjualan/-1", "/penjualan/999", "/dokumen/abc/unduh", "/dokumen/999/unduh"} {
		e.harap(e.kirim("satkA", "GET", p, nil, ""), http.StatusNotFound)
	}
	e.harap(e.json("satkA", "POST", base+"/tahap/aneh/lewati", map[string]string{"catatan": "alasan lengkap"}), http.StatusNotFound)
	e.harap(e.kirim("satkA", "POST", "/penjualan", []byte("{bukan json"), ""), http.StatusBadRequest)
	e.harap(e.json("satkA", "POST", "/penjualan", map[string]string{"kode_satker": "123"}), http.StatusBadRequest)
	e.harap(e.json("satkA", "POST", "/penjualan", map[string]string{"kode_satker": kodeB}), http.StatusForbidden)
	e.harap(e.kirim("satkA", "GET", "/penjualan?status=aneh", nil, ""), http.StatusBadRequest)

	// Isian terlalu besar.
	besar := bytes.Repeat([]byte("a"), sapa.MaksDataTahap+4096)
	e.harap(e.kirim("satkA", "PUT", base+"/tahap/tim", besar, ""), http.StatusRequestEntityTooLarge)
	e.harap(e.kirim("satkA", "POST", base+"/tahap/tim/dokumen", besar, ""), http.StatusRequestEntityTooLarge)
}

func TestSapaDaftarDanPencarian(t *testing.T) {
	e := siapkan(t, nil)
	e.buatUsulan()
	e.buatUsulan()
	j := e.harap(e.kirim("admin", "GET", "/penjualan?per_halaman=1&halaman=2", nil, ""), 200).data()
	if j["total"] != float64(2) || len(j["usulan"].([]interface{})) != 1 || j["halaman"] != float64(2) {
		t.Errorf("halaman = %v", j)
	}
	if j := e.harap(e.kirim("admin", "GET", "/penjualan?q=KPKNL", nil, ""), 200).data(); j["total"] != float64(2) {
		t.Errorf("cari = %v", j)
	}
	if j := e.harap(e.kirim("admin", "GET", "/penjualan?q=tidak-ada", nil, ""), 200).data(); j["total"] != float64(0) {
		t.Errorf("cari kosong = %v", j)
	}
	// per_halaman di luar batas dikembalikan ke bawaan, bukan galat.
	if j := e.harap(e.kirim("admin", "GET", "/penjualan?per_halaman=100000&halaman=-3", nil, ""), 200).data(); j["per_halaman"] != float64(20) || j["halaman"] != float64(1) {
		t.Errorf("batas = %v", j)
	}
	r := e.harap(e.kirim("satkA", "GET", "/referensi/satker?kode="+kodeA+"KP", nil, ""), 200).data()
	if s, _ := r["satker"].(map[string]interface{}); s["nama"] != "KPKNL Jakarta I" || r["kode_ue1"] != "01501" || r["ue1"] == nil {
		t.Errorf("referensi satker = %v", r)
	}
	e.harap(e.kirim("satkA", "GET", "/referensi/satker?kode=12", nil, ""), http.StatusBadRequest)
}

func formBerkas(t *testing.T, bidang, nama string, isi []byte, catatan string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(bidang, nama)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(isi)
	if catatan != "" {
		w.WriteField("catatan", catatan)
	}
	w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

func TestSapaUnggahDanUnduhTemplate(t *testing.T) {
	e := siapkan(t, nil)
	bawaan, _ := sapa.TemplateBawaan(sapa.DokNDSatker)

	// Daftar awal: ND Satker/UE1 bawaan, SK Tim/BA belum ada.
	list := e.harap(e.kirim("admin", "GET", "/template", nil, ""), 200).m["data"].([]interface{})
	sumber := map[string]string{}
	for _, it := range list {
		m := it.(map[string]interface{})
		sumber[m["jenis"].(map[string]interface{})["kunci"].(string)] = m["sumber"].(string)
	}
	if sumber["nd_satker"] != "bawaan" || sumber["sk_tim"] != "belum" || len(sumber) != 4 {
		t.Errorf("sumber = %v", sumber)
	}

	body, tipe := formBerkas(t, "berkas", "Templat ND – Satker.docx", bawaan, "uji")
	j := e.harap(e.kirim("admin", "POST", "/template/nd_satker", body, tipe), http.StatusCreated)
	if tpl := j.data()["template"].(map[string]interface{}); tpl["versi"] != float64(1) || tpl["aktif"] != true {
		t.Errorf("hasil unggah = %v", j.data())
	}
	list = e.harap(e.kirim("admin", "GET", "/template", nil, ""), 200).m["data"].([]interface{})
	for _, it := range list {
		m := it.(map[string]interface{})
		if m["jenis"].(map[string]interface{})["kunci"] == "nd_satker" && m["sumber"] != "unggahan" {
			t.Errorf("setelah unggah: %v", m["sumber"])
		}
	}

	dl := e.harap(e.kirim("admin", "GET", "/template/nd_satker/unduh", nil, ""), 200)
	if !bytes.Equal(dl.w.Body.Bytes(), bawaan) {
		t.Error("isi unduhan template berbeda dari yang diunggah")
	}
	if cd := dl.w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "filename*=utf-8''") {
		t.Errorf("nama berkas non-ASCII harus dikodekan: %q", cd)
	}

	// Penolakan.
	body, tipe = formBerkas(t, "berkas", "template.pdf", bawaan, "")
	e.harap(e.kirim("admin", "POST", "/template/nd_satker", body, tipe), http.StatusBadRequest)
	body, tipe = formBerkas(t, "berkas", "t.docx", []byte("bukan zip"), "")
	e.harap(e.kirim("admin", "POST", "/template/nd_satker", body, tipe), http.StatusBadRequest)
	body, tipe = formBerkas(t, "lain", "t.docx", bawaan, "")
	e.harap(e.kirim("admin", "POST", "/template/nd_satker", body, tipe), http.StatusBadRequest)
	body, tipe = formBerkas(t, "berkas", "t.docx", bawaan, "")
	e.harap(e.kirim("admin", "POST", "/template/aneh", body, tipe), http.StatusNotFound)
	e.harap(e.kirim("admin", "POST", "/template/nd_satker", []byte("{}"), "application/json"), http.StatusBadRequest)
	besar, tipe := formBerkas(t, "berkas", "besar.docx", make([]byte, sapa.MaksTemplate+2<<20), "")
	e.harap(e.kirim("admin", "POST", "/template/nd_satker", besar, tipe), http.StatusRequestEntityTooLarge)
	e.harap(e.kirim("admin", "GET", "/template/ba_penelitian/unduh", nil, ""), http.StatusConflict) // tidak ada template
}

// SAPA tidak lagi menetapkan peran sendiri: rute peran SAPA sudah dicabut (admin pun mendapat 404), dan peran di SAPA mengikuti peran data aplikasi.
func TestSapaTidakAdaLagiPemberianPeranSendiri(t *testing.T) {
	e := siapkan(t, nil)
	e.harap(e.kirim("admin", "GET", "/peran", nil, ""), http.StatusNotFound)
	e.harap(e.json("admin", "PUT", "/peran/"+gTanpa, map[string]string{"peran": "satker", "kode_satker": kodeA}), http.StatusNotFound)
	// Pengguna tanpa peran data di aplikasi tetap tidak bisa masuk.
	e.harap(e.kirim("tanpa", "GET", "/penjualan", nil, ""), http.StatusForbidden)
}

// Cakupan menurut peran data aplikasi: Satker satkernya, Kanwil Kanwil-nya, UE1 UE1-nya, Pengguna Barang semuanya (lihat saja), admin semuanya.
func TestSapaCakupanDanHakMengikutiPeranAplikasi(t *testing.T) {
	e := siapkan(t, nil)
	id := e.buatUsulan() // satkA membuat usulan untuk kodeA (UE1 01501, Kanwil 015010199, satker 409294)
	base := "/penjualan/" + id

	for _, u := range []string{"satkA", "kanwil", "ue1", "barang", "admin"} {
		e.harap(e.kirim(u, "GET", base, nil, ""), 200)
		if l := e.harap(e.kirim(u, "GET", "/penjualan", nil, ""), 200).data(); l["total"] != float64(1) {
			t.Errorf("daftar %s = %v, want 1 usulan", u, l)
		}
	}
	// Satker lain dan Kanwil lain (walau UE1-nya berbeda) tidak melihatnya: 404, tanpa membocorkan keberadaannya.
	for _, u := range []string{"satkB", "kanwilLain"} {
		e.harap(e.kirim(u, "GET", base, nil, ""), http.StatusNotFound)
		if l := e.harap(e.kirim(u, "GET", "/penjualan", nil, ""), 200).data(); l["total"] != float64(0) {
			t.Errorf("daftar %s = %v, want kosong", u, l)
		}
	}
	// Pengguna Barang melihat semuanya tetapi tidak mengerjakan tahap apa pun dan tidak membuat usulan.
	e.harap(e.json("barang", "PUT", base+"/tahap/tim", map[string]string{"alasan": "x"}), http.StatusForbidden)
	e.harap(e.json("barang", "POST", base+"/tahap/tim/lewati", map[string]string{"catatan": "dilewati oleh pengguna barang"}), http.StatusForbidden)
	e.harap(e.json("barang", "POST", "/penjualan", map[string]string{"kode_satker": kodeA}), http.StatusForbidden)
	// Kanwil lain tidak bisa mengerjakan tahap Kanwil pada usulan di luar Kanwil-nya.
	e.harap(e.json("kanwilLain", "POST", base+"/tahap/siman_kanwil/selesai", map[string]string{"catatan": "ok"}), http.StatusNotFound)
}

func TestSapaReferensiUE1(t *testing.T) {
	e := siapkan(t, nil)
	e.harap(e.json("admin", "PUT", "/ref-ue1/01502", map[string]string{"nama": "Ditjen Lain", "sekretaris": "Sekretaris Ditjen Lain"}), 200)
	e.harap(e.json("admin", "PUT", "/ref-ue1/1", map[string]string{"nama": "X", "sekretaris": "Y"}), http.StatusBadRequest)
	if l := e.harap(e.kirim("admin", "GET", "/ref-ue1", nil, ""), 200).m["data"].([]interface{}); len(l) != 2 {
		t.Errorf("ref ue1 = %v", l)
	}
	e.harap(e.kirim("admin", "DELETE", "/ref-ue1/01502", nil, ""), 200)
	if l := e.harap(e.kirim("admin", "GET", "/ref-ue1", nil, ""), 200).m["data"].([]interface{}); len(l) != 1 {
		t.Errorf("ref ue1 setelah hapus = %v", l)
	}
}

// repoRusak membuat satu operasi gagal dengan galat berisi rahasia, untuk memastikan galat internal tidak bocor ke klien.
type repoRusak struct{ *sapa.MemRepo }

func (repoRusak) DaftarPenjualan(context.Context, sapa.Scope, sapa.FilterDaftar) ([]sapa.KasusInfo, int, error) {
	return nil, 0, errors.New("mssql: login failed for user 'sa' password=RAHASIA")
}

func TestSapaGalatInternalTidakBocor(t *testing.T) {
	e := siapkan(t, func(m *sapa.MemRepo) sapa.Repo { return repoRusak{m} })
	j := e.harap(e.kirim("admin", "GET", "/penjualan", nil, ""), http.StatusInternalServerError)
	if strings.Contains(j.w.Body.String(), "RAHASIA") || strings.Contains(j.w.Body.String(), "mssql") {
		t.Errorf("galat internal bocor: %s", j.w.Body.String())
	}
	if j.m["message"] != "Terjadi kesalahan pada server" {
		t.Errorf("message = %v", j.m["message"])
	}
}

func TestSapaJenisDanSatuanBMN(t *testing.T) {
	e := siapkan(t, nil)

	// Pengguna SAPA melihat daftar aktif; Tanah hanya boleh bersatuan bidang.
	j := e.harap(e.kirim("satkA", "GET", "/referensi/bmn", nil, ""), 200).data()
	jenis := j["jenis"].([]interface{})
	var tanah map[string]interface{}
	for _, x := range jenis {
		if m := x.(map[string]interface{}); m["nama"] == "Tanah" {
			tanah = m
		}
	}
	if len(jenis) != 7 || tanah == nil || len(tanah["satuan"].([]interface{})) != 1 || tanah["satuan_bawaan"] != "bidang" {
		t.Errorf("referensi BMN = %v", j)
	}
	e.harap(e.kirim("tanpa", "GET", "/referensi/bmn", nil, ""), http.StatusForbidden)

	// Admin menambah satuan dan jenis yang namanya memuat koma, spasi, dan huruf non-ASCII.
	e.harap(e.json("admin", "PUT", "/bmn/satuan", map[string]interface{}{"nama": "meter lari", "aktif": true, "urutan": 7}), 200)
	e.harap(e.json("admin", "PUT", "/bmn/jenis", map[string]interface{}{"nama": "Pagar, Jalan & Taman – Kantor", "aktif": true, "urutan": 8, "satuan": []string{"meter lari", "unit"}}), 200)
	all := e.harap(e.kirim("admin", "GET", "/bmn", nil, ""), 200).data()
	if len(all["jenis"].([]interface{})) != 8 || len(all["satuan"].([]interface{})) != 7 {
		t.Errorf("daftar lengkap = %v", all)
	}
	// Dihapus lewat query (bukan segmen jalur) walau namanya memuat karakter khusus.
	e.harap(e.kirim("admin", "DELETE", "/bmn/jenis?nama="+"Pagar%2C+Jalan+%26+Taman+%E2%80%93+Kantor", nil, ""), 200)
	e.harap(e.kirim("admin", "DELETE", "/bmn/jenis?nama=Pagar%2C+Jalan+%26+Taman+%E2%80%93+Kantor", nil, ""), http.StatusNotFound)

	// Validasi dan konflik dipetakan ke 400 dan 409.
	e.harap(e.json("admin", "PUT", "/bmn/satuan", map[string]interface{}{"nama": "a/b", "aktif": true}), http.StatusBadRequest)
	e.harap(e.json("admin", "PUT", "/bmn/jenis", map[string]interface{}{"nama": "Kosong", "aktif": true, "satuan": []string{}}), http.StatusBadRequest)
	e.harap(e.kirim("admin", "PUT", "/bmn/jenis", []byte("{bukan json"), ""), http.StatusBadRequest)
	k := e.harap(e.kirim("admin", "DELETE", "/bmn/satuan?nama=bidang", nil, ""), http.StatusConflict) // satu-satunya satuan Tanah
	if msg, _ := k.m["message"].(string); !strings.Contains(msg, "Tanah") {
		t.Errorf("pesan konflik = %q", msg)
	}

	// Nota Dinas dengan pasangan tidak masuk akal ditolak (400) dan menyebut satuan yang boleh.
	id := e.buatUsulan()
	base := fmt.Sprintf("/penjualan/%s", id)
	for _, kt := range []string{"tim", "ba"} {
		e.harap(e.json("satkA", "POST", base+"/tahap/"+kt+"/lewati", map[string]string{"catatan": "dibuat di luar aplikasi"}), 200)
	}
	nd := sapa.ContohNDSatker()
	nd.JenisBMN, nd.Satuan = "Peralatan dan Mesin", "meter lari"
	bad := e.harap(e.json("satkA", "POST", base+"/tahap/nd_satker/dokumen", nd), http.StatusBadRequest)
	if msg, _ := bad.m["message"].(string); !strings.Contains(msg, "tidak sesuai") || !strings.Contains(msg, "unit") {
		t.Errorf("pesan = %q", msg)
	}
	nd.JenisBMN, nd.Satuan = "kendaraan bermotor", "UNIT"
	e.harap(e.json("satkA", "POST", base+"/tahap/nd_satker/dokumen", nd), http.StatusCreated)
}

func TestSapaUsulanDikenaliLewatUUID(t *testing.T) {
	e := siapkan(t, nil)
	a := e.buatUsulan()
	b := e.buatUsulan()
	for _, id := range []string{a, b} {
		u, err := uuid.Parse(id)
		if err != nil || u.Version() != 4 || id != strings.ToLower(id) {
			t.Fatalf("id usulan %q harus UUID v4 huruf kecil (err %v)", id, err)
		}
	}
	if a == b {
		t.Fatal("dua usulan harus punya UUID berbeda")
	}

	// Detail, dengan UUID huruf besar sekalipun.
	d := e.harap(e.kirim("satkA", "GET", "/penjualan/"+a, nil, ""), 200)
	usulan, _ := d.data()["usulan"].(map[string]interface{})
	if usulan["id"] != a {
		t.Errorf("usulan.id = %v, want %v", usulan["id"], a)
	}
	e.harap(e.kirim("satkA", "GET", "/penjualan/"+strings.ToUpper(a), nil, ""), 200)

	// Nomor id berurutan tidak lagi dikenali, dan tidak pernah dikirim ke klien.
	for _, p := range []string{"/penjualan/1", "/penjualan/2", "/penjualan/" + strings.ReplaceAll(a, "-", ""), "/penjualan/{" + a + "}", "/penjualan/" + a + "x"} {
		e.harap(e.kirim("satkA", "GET", p, nil, ""), http.StatusNotFound)
	}
	if body := d.w.Body.String(); strings.Contains(body, `"penjualan_id"`) || strings.Contains(body, `"id":1,`) || strings.Contains(body, `"id":2,`) {
		t.Errorf("badan memuat nomor id internal: %s", body)
	}

	// Daftar memakai UUID yang sama.
	l := e.harap(e.kirim("satkA", "GET", "/penjualan", nil, ""), 200).data()
	var ids []string
	for _, x := range l["usulan"].([]interface{}) {
		ids = append(ids, x.(map[string]interface{})["id"].(string))
	}
	if len(ids) != 2 || !((ids[0] == a && ids[1] == b) || (ids[0] == b && ids[1] == a)) {
		t.Errorf("daftar id = %v, want %v dan %v", ids, a, b)
	}

	// Usulan milik satker lain dijawab sama dengan usulan yang tidak ada: tidak ada petunjuk bahwa UUID itu benar.
	asing := e.kirim("satkB", "GET", "/penjualan/"+a, nil, "")
	tidakAda := e.kirim("satkB", "GET", "/penjualan/00000000-0000-4000-8000-000000000000", nil, "")
	if asing.kode != http.StatusNotFound || tidakAda.kode != http.StatusNotFound || asing.w.Body.String() != tidakAda.w.Body.String() {
		t.Errorf("usulan milik satker lain = %d %s; usulan tidak ada = %d %s", asing.kode, asing.w.Body.String(), tidakAda.kode, tidakAda.w.Body.String())
	}
	// Semua aksi tahap memakai UUID yang sama dan tetap menolak satker lain.
	for _, p := range []string{"/tahap/tim", "/tahap/tim/lewati", "/tahap/tim/selesai", "/tahap/tim/dokumen"} {
		method := "POST"
		if p == "/tahap/tim" {
			method = "PUT"
		}
		if j := e.json("satkB", method, "/penjualan/"+a+p, map[string]string{}); j.kode != http.StatusNotFound {
			t.Errorf("%s %s oleh satker lain: status %d, want 404", method, p, j.kode)
		}
	}
}

func TestSapaTemplateImporDanEksporBarang(t *testing.T) {
	e := siapkan(t, nil)
	const tipeXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

	// Hanya pengguna SAPA yang dilayani.
	for _, c := range []struct{ method, path string }{{"GET", "/barang/template"}, {"POST", "/barang/impor"}, {"POST", "/barang/ekspor"}} {
		if j := e.kirim("tanpa", c.method, c.path, []byte("{}"), ""); j.kode != http.StatusForbidden {
			t.Errorf("%s %s tanpa peran: status %d, want 403", c.method, c.path, j.kode)
		}
	}

	// Template: berkas xlsx yang diunduh sebagai lampiran.
	tpl := e.harap(e.kirim("satkA", "GET", "/barang/template", nil, ""), 200)
	if ct := tpl.w.Header().Get("Content-Type"); ct != tipeXLSX {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := tpl.w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, ".xlsx") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if !bytes.HasPrefix(tpl.w.Body.Bytes(), []byte("PK")) {
		t.Error("isi template bukan berkas zip/xlsx")
	}

	// Ekspor lalu impor kembali mengembalikan daftar yang sama.
	daftar := []sapa.Barang{
		{Nama: "Gedung Lama", Kode: "4010101001", NUP: "1", Kondisi: "Rusak Berat", TahunPerolehan: "1999", NilaiPerolehan: "2500000000", NilaiLimit: "1500000000"},
		{Nama: "Truk", NilaiPerolehan: "100", NilaiLimit: "50"},
	}
	eks := e.harap(e.json("satkA", "POST", "/barang/ekspor", map[string]interface{}{"barang": daftar}), 200)
	if ct := eks.w.Header().Get("Content-Type"); ct != tipeXLSX {
		t.Errorf("Content-Type ekspor = %q", ct)
	}
	body, tipe := formBerkas(t, "berkas", "daftar.xlsx", eks.w.Body.Bytes(), "")
	imp := e.harap(e.kirim("satkA", "POST", "/barang/impor", body, tipe), 200).data()
	got, _ := imp["barang"].([]interface{})
	if len(got) != 2 || got[0].(map[string]interface{})["nama"] != "Gedung Lama" || got[1].(map[string]interface{})["nama"] != "Truk" {
		t.Errorf("barang hasil impor = %v", imp["barang"])
	}
	if g, _ := imp["galat"].([]interface{}); len(g) != 0 {
		t.Errorf("galat = %v", g)
	}

	// Berkas yang salah dijawab dengan pesan yang bisa ditindaklanjuti.
	for _, c := range []struct {
		nama   string
		isi    []byte
		status int
		pesan  string
	}{
		{"daftar.csv", []byte("a,b"), http.StatusBadRequest, ".xlsx"},
		{"rusak.xlsx", []byte("bukan zip"), http.StatusBadRequest, "tidak dapat dibaca"},
		{"template.xlsx", tpl.w.Body.Bytes(), http.StatusBadRequest, "Tidak ada baris data"},
	} {
		b, tp := formBerkas(t, "berkas", c.nama, c.isi, "")
		j := e.harap(e.kirim("satkA", "POST", "/barang/impor", b, tp), c.status)
		if msg, _ := j.m["message"].(string); !strings.Contains(msg, c.pesan) {
			t.Errorf("%s: pesan = %q, want memuat %q", c.nama, msg, c.pesan)
		}
	}
	// Tanpa berkas sama sekali.
	b, tp := formBerkas(t, "lain", "daftar.xlsx", []byte("x"), "")
	e.harap(e.kirim("satkA", "POST", "/barang/impor", b, tp), http.StatusBadRequest)
	// Berkas melebihi batas ukuran.
	besar, tp := formBerkas(t, "berkas", "besar.xlsx", bytes.Repeat([]byte("x"), sapa.MaksUkuranXLSX+(2<<20)), "")
	e.harap(e.kirim("satkA", "POST", "/barang/impor", besar, tp), http.StatusRequestEntityTooLarge)

	// Ekspor: JSON rusak dan daftar melebihi batas ditolak.
	e.harap(e.kirim("satkA", "POST", "/barang/ekspor", []byte("{bukan json"), ""), http.StatusBadRequest)
	e.harap(e.json("satkA", "POST", "/barang/ekspor", map[string]interface{}{"barang": make([]sapa.Barang, sapa.MaksBarang+1)}), http.StatusBadRequest)
}
