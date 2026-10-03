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

	"pasti-v3-backend/handlers"
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
)

type pengguna struct{ id, nama, roleApp string }

var daftarPengguna = map[string]pengguna{
	"admin":  {gAdmin, "admin", "admin"},
	"satkA":  {gSatkA, "satker.a", "user"},
	"satkB":  {gSatkB, "satker.b", "user"},
	"kanwil": {gKanwil, "kanwil", "user"},
	"ue1":    {gUE1, "ue1", "user"},
	"tanpa":  {gTanpa, "tanpa", "user"},
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
		repo.Pengguna = append(repo.Pengguna, sapa.PeranRow{UserID: p.id, Username: p.nama, Nama: p.nama, PeranApp: p.roleApp})
	}
	ctx := context.Background()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(repo.SimpanPeran(ctx, gSatkA, sapa.PeranSatker, kodeA, "", "seed"))
	must(repo.SimpanPeran(ctx, gSatkB, sapa.PeranSatker, kodeB, "", "seed"))
	must(repo.SimpanPeran(ctx, gKanwil, sapa.PeranKanwil, "", "", "seed"))
	must(repo.SimpanPeran(ctx, gUE1, sapa.PeranUE1, "", "01501", "seed"))
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
		c.Set("role", p.roleApp)
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

func (e *lingkungan) buatUsulan() int {
	e.t.Helper()
	j := e.harap(e.json("satkA", "POST", "/penjualan", map[string]string{"kode_satker": kodeA}), http.StatusCreated)
	return int(j.data()["id"].(float64))
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
	if d := e.harap(e.kirim("kanwil", "GET", "/saya", nil, ""), 200).data(); d["boleh_membuat"] != false || d["punya_akses"] != true {
		t.Errorf("saya kanwil = %v", d)
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
		{"GET", "/peran"}, {"PUT", "/peran/" + gSatkA}, {"GET", "/ref-ue1"}, {"PUT", "/ref-ue1/01501"}, {"DELETE", "/ref-ue1/01501"},
		{"POST", fmt.Sprintf("/penjualan/%d/tahap/tim/buka-ulang", id)},
	} {
		for _, u := range []string{"satkA", "kanwil", "ue1", "tanpa"} {
			if j := e.kirim(u, c.method, c.path, []byte("{}"), ""); j.kode != http.StatusForbidden {
				t.Errorf("%s %s oleh %s: status %d, want 403", c.method, c.path, u, j.kode)
			}
		}
	}
}

func TestSapaAlurSatkerSampaiUnduh(t *testing.T) {
	e := siapkan(t, nil)
	id := e.buatUsulan()
	base := fmt.Sprintf("/penjualan/%d", id)

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
	base := fmt.Sprintf("/penjualan/%d", id)

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

func TestSapaPeranDanReferensiUE1(t *testing.T) {
	e := siapkan(t, nil)
	e.harap(e.json("admin", "PUT", "/peran/"+gTanpa, map[string]string{"peran": "satker", "kode_satker": kodeA + "KP"}), 200)
	if d := e.harap(e.kirim("tanpa", "GET", "/saya", nil, ""), 200).data(); d["peran"] != "satker" || d["kode_satker"] != kodeA {
		t.Errorf("peran setelah ditetapkan = %v", d)
	}
	e.harap(e.json("admin", "PUT", "/peran/"+gTanpa, map[string]string{"peran": "satker", "kode_satker": "1"}), http.StatusBadRequest)
	e.harap(e.json("admin", "PUT", "/peran/"+gTanpa, map[string]string{"peran": "tamu"}), http.StatusBadRequest)
	e.harap(e.json("admin", "PUT", "/peran/bukan-guid", map[string]string{"peran": "kanwil"}), http.StatusBadRequest)
	e.harap(e.json("admin", "PUT", "/peran/00000000-0000-4000-8000-0000000000ff", map[string]string{"peran": "kanwil"}), http.StatusNotFound)
	e.harap(e.json("admin", "PUT", "/peran/"+gTanpa, map[string]string{"peran": ""}), 200)
	e.harap(e.kirim("tanpa", "GET", "/penjualan", nil, ""), http.StatusForbidden)
	if l := e.harap(e.kirim("admin", "GET", "/peran?q=satker", nil, ""), 200).m["data"].([]interface{}); len(l) != 2 {
		t.Errorf("daftar peran = %v", l)
	}

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
