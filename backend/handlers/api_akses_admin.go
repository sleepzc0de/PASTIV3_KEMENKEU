package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/apiakses"
	"pasti-v3-backend/audit"
	"pasti-v3-backend/utils"
)

// Pengelolaan API Akses Data oleh superadmin (rute dipasang dengan AuthRequired + RequireSuperadmin): daftar dan rincian klien, persetujuan, penolakan, pencabutan,
// scope dan batas per klien, penerbitan/rotasi/pencabutan kunci, dan pengaturan global batas laju/kuota. Setiap perubahan tercatat di Log Audit (katalog audit).
// Kunci lengkap hanya muncul pada respons penerbitan (sekali) dan tidak pernah disimpan, dicatat, atau dapat dibaca ulang.

// kunciBaruRes: kunci yang baru diterbitkan. Respons yang memuatnya tidak boleh disimpan perantara (no-store).
type kunciBaruRes struct {
	Kunci string             `json:"kunci"`
	Info  apiakses.KunciInfo `json:"info"`
}

func kunciTidakDisimpan(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func aksesGalatStore(c *gin.Context, konteks string, err error) {
	switch {
	case errors.Is(err, apiakses.ErrTidakAda):
		utils.ErrorResponse(c, http.StatusNotFound, "Klien atau kunci tidak ditemukan")
	case errors.Is(err, apiakses.ErrStatusTidakSesuai):
		utils.ErrorResponse(c, http.StatusConflict, "Status klien saat ini tidak memungkinkan tindakan ini")
	case errors.Is(err, apiakses.ErrTerlaluBanyakKunci):
		utils.ErrorResponse(c, http.StatusConflict, "Jumlah kunci aktif sudah mencapai batas. Cabut kunci yang tidak dipakai atau gunakan rotasi")
	default:
		aksesGagal(c, konteks, err)
	}
}

// klienDenganBatas melengkapi klien dengan batas yang berlaku.
func klienDenganBatas(c *gin.Context, k apiakses.Klien) apiakses.Klien {
	b := apiAkses.BatasKlien(c.Request.Context(), k.Override())
	k.Berlaku = &b
	return k
}

// GET /api-akses/klien?status=&q=&halaman=&per_halaman=
func GetApiAksesKlien(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	ks, total, err := apiAkses.Store.DaftarKlien(ctx, apiakses.PenyaringKlien{
		Status: c.Query("status"), Q: c.Query("q"), Halaman: intQuery(c, "halaman"), PerHalaman: intQuery(c, "per_halaman"),
	})
	if err != nil {
		aksesGagal(c, "daftar klien", err)
		return
	}
	hitung, err := apiAkses.Store.HitungStatus(ctx)
	if err != nil {
		aksesGagal(c, "hitung status klien", err)
		return
	}
	for i := range ks {
		ks[i] = klienDenganBatas(c, ks[i])
	}
	halaman := intQuery(c, "halaman")
	if halaman < 1 {
		halaman = 1
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"klien": ks, "total": total, "halaman": halaman, "per_status": hitung, "dataset": daftarInfoDataset()})
}

// GET /api-akses/klien/:id
func GetApiAksesKlienDetail(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	id, ok := aksesID(c, "id")
	if !ok {
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	k, err := apiAkses.Store.AmbilKlien(ctx, id)
	if err != nil {
		aksesGalatStore(c, "ambil klien", err)
		return
	}
	k = klienDenganBatas(c, k)
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"klien": k, "pemakaian": apiAkses.PemakaianSatu(ctx, k.ID, k.Kode, k.NamaAplikasi, k.Override())})
}

// cakupanReq: scope dan batas klien yang diatur superadmin. Daftar IP dan UE1 berupa teks (dipisah koma/spasi/baris baru); angka null = ikut bawaan global.
type cakupanReq struct {
	DatasetDiberikan  []string `json:"dataset_diberikan"`
	IPDiizinkan       string   `json:"ip_diizinkan"`
	UE1Diizinkan      string   `json:"ue1_diizinkan"`
	LimitPermintaan   *int     `json:"limit_permintaan"`
	LimitJendelaDetik *int     `json:"limit_jendela_detik"`
	KuotaPermintaan   *int     `json:"kuota_permintaan"`
	KuotaPeriodeDetik *int     `json:"kuota_periode_detik"`
	Catatan           string   `json:"catatan"`
}

// ubah memeriksa isian dan menyusun UbahCakupan; pesan galat kosong bila sah.
func (r cakupanReq) ubah() (apiakses.UbahCakupan, string) {
	ds, err := apiakses.NormalisasiDataset(r.DatasetDiberikan, 50)
	if err != nil {
		return apiakses.UbahCakupan{}, err.Error()
	}
	ip, err := apiakses.DaftarIP(r.IPDiizinkan)
	if err != nil {
		return apiakses.UbahCakupan{}, err.Error()
	}
	ue1, err := apiakses.DaftarUE1(r.UE1Diizinkan)
	if err != nil {
		return apiakses.UbahCakupan{}, err.Error()
	}
	o := apiakses.Override{LimitPermintaan: r.LimitPermintaan, LimitJendelaDetik: r.LimitJendelaDetik, KuotaPermintaan: r.KuotaPermintaan, KuotaPeriodeDetik: r.KuotaPeriodeDetik}
	if err := o.Validasi(); err != nil {
		return apiakses.UbahCakupan{}, err.Error()
	}
	catatan := strings.TrimSpace(r.Catatan)
	if len([]rune(catatan)) > 1000 {
		return apiakses.UbahCakupan{}, "Catatan terlalu panjang (maksimal 1000 karakter)"
	}
	return apiakses.UbahCakupan{DatasetDiberikan: ds, IPDiizinkan: ip, UE1Diizinkan: ue1, Override: o, Catatan: catatan}, ""
}

func detailCakupan(u apiakses.UbahCakupan) map[string]interface{} {
	d := map[string]interface{}{"dataset": u.DatasetDiberikan, "ip_diizinkan": u.IPDiizinkan, "ue1_diizinkan": u.UE1Diizinkan}
	if u.Override.LimitPermintaan != nil {
		d["limit_permintaan"] = *u.Override.LimitPermintaan
	}
	if u.Override.LimitJendelaDetik != nil {
		d["limit_jendela_detik"] = *u.Override.LimitJendelaDetik
	}
	if u.Override.KuotaPermintaan != nil {
		d["kuota_permintaan"] = *u.Override.KuotaPermintaan
	}
	if u.Override.KuotaPeriodeDetik != nil {
		d["kuota_periode_detik"] = *u.Override.KuotaPeriodeDetik
	}
	return d
}

// PUT /api-akses/klien/:id: ubah scope, allowlist IP, pembatasan UE1, dan override batas.
func PutApiAksesKlien(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	id, ok := aksesID(c, "id")
	if !ok {
		return
	}
	var req cakupanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	u, pesan := req.ubah()
	if pesan != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, pesan)
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	if err := apiAkses.Store.UbahCakupan(ctx, id, u); err != nil {
		aksesGalatStore(c, "ubah klien", err)
		return
	}
	apiAkses.BuangCache()
	audit.Tandai(c, audit.Tanda{Detail: detailCakupan(u)})
	k, err := apiAkses.Store.AmbilKlien(ctx, id)
	if err != nil {
		aksesGalatStore(c, "ambil klien", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Pengaturan klien disimpan", gin.H{"klien": klienDenganBatas(c, k)})
}

type setujuiReq struct {
	cakupanReq
	LabelKunci string `json:"label_kunci"`
}

// POST /api-akses/klien/:id/setujui: menyetujui pendaftaran (status menunggu). Dataset yang diberikan bawaannya yang diminta. Kunci pertama diterbitkan sekaligus dan
// ditampilkan SEKALI pada respons ini.
func PostApiAksesSetujui(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	id, ok := aksesID(c, "id")
	if !ok {
		return
	}
	var req setujuiReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	lama, err := apiAkses.Store.AmbilKlien(ctx, id)
	if err != nil {
		aksesGalatStore(c, "ambil klien", err)
		return
	}
	if len(req.DatasetDiberikan) == 0 {
		req.DatasetDiberikan = lama.DatasetDiminta
	}
	u, pesan := req.cakupanReq.ubah()
	if pesan != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, pesan)
		return
	}
	if len(u.DatasetDiberikan) == 0 {
		utils.ErrorResponse(c, http.StatusBadRequest, "Pilih sedikitnya satu dataset yang diberikan")
		return
	}
	if err := apiAkses.Store.UbahStatus(ctx, id, []string{apiakses.StatusMenunggu}, apiakses.StatusDisetujui, aksesOleh(c), u.Catatan, &u); err != nil {
		aksesGalatStore(c, "setujui klien", err)
		return
	}
	apiAkses.BuangCache()
	audit.Tandai(c, audit.Tanda{Detail: map[string]interface{}{"klien": lama.Kode, "cakupan": detailCakupan(u)}})
	kunciRes, galatKunci := terbitkanKunciRes(c, id, req.LabelKunci, false, false)
	k, err := apiAkses.Store.AmbilKlien(ctx, id)
	if err != nil {
		aksesGalatStore(c, "ambil klien", err)
		return
	}
	kunciTidakDisimpan(c)
	out := gin.H{"klien": klienDenganBatas(c, k)}
	pesanOK := "Pendaftaran disetujui. Kunci API ditampilkan sekali; salin dan serahkan ke PIC lewat jalur yang aman"
	if kunciRes != nil {
		out["kunci_baru"] = kunciRes
	} else {
		out["kunci_gagal"] = true
		pesanOK = "Pendaftaran disetujui, tetapi kunci API gagal dibuat: " + galatKunci + ". Terbitkan kunci dari rincian klien"
	}
	utils.SuccessResponse(c, http.StatusOK, pesanOK, out)
}

// terbitkanKunciRes menerbitkan kunci dan mencatatnya di Log Audit (hanya prefix, tidak pernah kuncinya). tandaiAksi: jadikan aksi permintaan ini penerbitan/rotasi
// kunci (rute kunci); bila false (persetujuan dan pengaktifan kembali), aksi permintaan tetap dan prefix kunci hanya ditambahkan ke rincian.
func terbitkanKunciRes(c *gin.Context, klienID int64, label string, rotasi, tandaiAksi bool) (*kunciBaruRes, string) {
	label = strings.TrimSpace(label)
	if len([]rune(label)) > 100 {
		label = string([]rune(label)[:100])
	}
	kunci, info, err := apiAkses.TerbitkanKunci(c.Request.Context(), klienID, label, aksesOleh(c), rotasi)
	if err != nil {
		switch {
		case errors.Is(err, apiakses.ErrTerlaluBanyakKunci):
			return nil, "jumlah kunci aktif sudah mencapai batas"
		case errors.Is(err, apiakses.ErrStatusTidakSesuai):
			return nil, "klien belum/tidak lagi berstatus disetujui"
		}
		log.Printf("[API-AKSES ERROR] terbitkan kunci: %v", err)
		return nil, galatServer
	}
	t := audit.Tanda{Detail: map[string]interface{}{"kunci_prefix": info.Prefix, "kunci_id": info.ID, "rotasi": rotasi, "kedaluwarsa": info.KedaluwarsaAt}}
	if tandaiAksi {
		t.Aksi, t.Label = "apiakses.kunci.terbit", "Menerbitkan kunci API"
		if rotasi {
			t.Aksi, t.Label = "apiakses.kunci.rotasi", "Merotasi kunci API"
		}
	}
	audit.Tandai(c, t)
	return &kunciBaruRes{Kunci: kunci, Info: info}, ""
}

// galatServer: penanda galat internal dari terbitkanKunciRes (pesannya tidak ditampilkan apa adanya).
const galatServer = "terjadi kesalahan pada server"

type catatanReq struct {
	Catatan string `json:"catatan"`
}

func bacaCatatan(c *gin.Context, wajib bool) (string, bool) {
	var req catatanReq
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
			return "", false
		}
	}
	cat := strings.TrimSpace(req.Catatan)
	if wajib && cat == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Alasan wajib diisi")
		return "", false
	}
	if len([]rune(cat)) > 1000 {
		utils.ErrorResponse(c, http.StatusBadRequest, "Catatan terlalu panjang (maksimal 1000 karakter)")
		return "", false
	}
	return cat, true
}

// ubahStatusKlien: bagian bersama penolakan dan pencabutan.
func ubahStatusKlien(c *gin.Context, dari []string, ke, pesan string) {
	if !aksesSiap(c) {
		return
	}
	id, ok := aksesID(c, "id")
	if !ok {
		return
	}
	cat, ok := bacaCatatan(c, true)
	if !ok {
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	lama, err := apiAkses.Store.AmbilKlien(ctx, id)
	if err != nil {
		aksesGalatStore(c, "ambil klien", err)
		return
	}
	if err := apiAkses.Store.UbahStatus(ctx, id, dari, ke, aksesOleh(c), cat, nil); err != nil {
		aksesGalatStore(c, "ubah status klien", err)
		return
	}
	apiAkses.BuangCache()
	audit.Tandai(c, audit.Tanda{Detail: map[string]interface{}{"klien": lama.Kode, "dari": lama.Status, "ke": ke, "alasan": cat}})
	k, err := apiAkses.Store.AmbilKlien(ctx, id)
	if err != nil {
		aksesGalatStore(c, "ambil klien", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, pesan, gin.H{"klien": klienDenganBatas(c, k)})
}

// POST /api-akses/klien/:id/tolak
func PostApiAksesTolak(c *gin.Context) {
	ubahStatusKlien(c, []string{apiakses.StatusMenunggu}, apiakses.StatusDitolak, "Pendaftaran ditolak")
}

// POST /api-akses/klien/:id/cabut: mencabut akses klien yang disetujui; semua kuncinya ikut dicabut.
func PostApiAksesCabut(c *gin.Context) {
	ubahStatusKlien(c, []string{apiakses.StatusDisetujui}, apiakses.StatusDicabut, "Akses klien dicabut dan semua kuncinya dihentikan")
}

// POST /api-akses/klien/:id/aktifkan: mengaktifkan kembali klien yang dicabut. Kunci lama tidak hidup lagi; kunci baru diterbitkan dan ditampilkan sekali.
func PostApiAksesAktifkan(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	id, ok := aksesID(c, "id")
	if !ok {
		return
	}
	cat, ok := bacaCatatan(c, false)
	if !ok {
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	lama, err := apiAkses.Store.AmbilKlien(ctx, id)
	if err != nil {
		aksesGalatStore(c, "ambil klien", err)
		return
	}
	if err := apiAkses.Store.UbahStatus(ctx, id, []string{apiakses.StatusDicabut}, apiakses.StatusDisetujui, aksesOleh(c), cat, nil); err != nil {
		aksesGalatStore(c, "aktifkan klien", err)
		return
	}
	apiAkses.BuangCache()
	audit.Tandai(c, audit.Tanda{Detail: map[string]interface{}{"klien": lama.Kode, "dari": lama.Status, "ke": apiakses.StatusDisetujui}})
	kunciRes, galatKunci := terbitkanKunciRes(c, id, "", false, false)
	k, err := apiAkses.Store.AmbilKlien(ctx, id)
	if err != nil {
		aksesGalatStore(c, "ambil klien", err)
		return
	}
	kunciTidakDisimpan(c)
	out := gin.H{"klien": klienDenganBatas(c, k)}
	pesan := "Klien diaktifkan kembali. Kunci lama tidak berlaku; kunci baru ditampilkan sekali"
	if kunciRes != nil {
		out["kunci_baru"] = kunciRes
	} else {
		out["kunci_gagal"] = true
		pesan = "Klien diaktifkan kembali, tetapi kunci baru gagal dibuat: " + galatKunci + ". Terbitkan kunci dari rincian klien"
	}
	utils.SuccessResponse(c, http.StatusOK, pesan, out)
}

type kunciReq struct {
	Label  string `json:"label"`
	Rotasi bool   `json:"rotasi"`
}

// POST /api-akses/klien/:id/kunci: menerbitkan kunci baru. rotasi=true: kunci aktif lain dihentikan setelah masa tenggang (pengaturan), atau langsung bila tenggang 0.
func PostApiAksesKunci(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	id, ok := aksesID(c, "id")
	if !ok {
		return
	}
	var req kunciReq
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
			return
		}
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	k, err := apiAkses.Store.AmbilKlien(ctx, id)
	if err != nil {
		aksesGalatStore(c, "ambil klien", err)
		return
	}
	if k.Status != apiakses.StatusDisetujui {
		utils.ErrorResponse(c, http.StatusConflict, "Kunci hanya dapat diterbitkan untuk klien yang disetujui")
		return
	}
	audit.Tandai(c, audit.Tanda{Detail: map[string]interface{}{"klien": k.Kode}})
	res, galat := terbitkanKunciRes(c, id, req.Label, req.Rotasi, true)
	if res == nil {
		if galat == galatServer {
			utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
			return
		}
		utils.ErrorResponse(c, http.StatusConflict, "Kunci tidak dapat diterbitkan: "+galat)
		return
	}
	kunciTidakDisimpan(c)
	utils.SuccessResponse(c, http.StatusCreated, "Kunci API diterbitkan. Kunci hanya ditampilkan sekali; salin sekarang", res)
}

// DELETE /api-akses/klien/:id/kunci/:kunciId?alasan=
func DeleteApiAksesKunci(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	id, ok := aksesID(c, "id")
	if !ok {
		return
	}
	kid, ok := aksesID(c, "kunciId")
	if !ok {
		return
	}
	alasan := strings.TrimSpace(c.Query("alasan"))
	if len([]rune(alasan)) > 200 {
		utils.ErrorResponse(c, http.StatusBadRequest, "Alasan terlalu panjang (maksimal 200 karakter)")
		return
	}
	if alasan == "" {
		alasan = "dicabut superadmin"
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	prefix, err := apiAkses.Store.CabutKunci(ctx, id, kid, aksesOleh(c), alasan)
	if err != nil {
		aksesGalatStore(c, "cabut kunci", err)
		return
	}
	apiAkses.BuangCache()
	audit.Tandai(c, audit.Tanda{Detail: map[string]interface{}{"kunci_prefix": prefix, "kunci_id": kid, "alasan": alasan}})
	utils.SuccessResponse(c, http.StatusOK, "Kunci dicabut", gin.H{"prefix": prefix})
}

// ---------------------------------------------------------------- pengaturan global

type batasNilai struct {
	Min  int `json:"min"`
	Maks int `json:"maks"`
}

func rentangPengaturan() gin.H {
	return gin.H{
		"limit_permintaan":        batasNilai{apiakses.LimitMin, apiakses.LimitMaks},
		"limit_jendela_detik":     batasNilai{apiakses.JendelaMin, apiakses.JendelaMaks},
		"kuota_permintaan":        batasNilai{apiakses.KuotaMin, apiakses.KuotaMaks},
		"kuota_periode_detik":     batasNilai{apiakses.PeriodeMin, apiakses.PeriodeMaks},
		"halaman_maks":            batasNilai{apiakses.HalamanMin, apiakses.HalamanMaksBatas},
		"kunci_masa_berlaku_hari": batasNilai{0, apiakses.MasaBerlakuMaksHari},
		"rotasi_tenggang_jam":     batasNilai{0, apiakses.TenggangMaksJam},
		"log_retensi_hari":        batasNilai{1, apiakses.LogRetensiMaksHari},
		"agregat_retensi_hari":    batasNilai{1, apiakses.AgregatRetensiMaksHari},
	}
}

// GET /api-akses/pengaturan
func GetApiAksesPengaturan(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"pengaturan": apiAkses.Pengaturan(ctx), "bawaan_pabrik": apiakses.PengaturanBawaan(), "rentang": rentangPengaturan()})
}

// PUT /api-akses/pengaturan: mengganti bawaan global; berlaku dalam hitungan detik tanpa deploy ulang. Override per klien tidak berubah.
func PutApiAksesPengaturan(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	var req apiakses.Pengaturan
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	if err := req.Validasi(); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	lama := apiAkses.Pengaturan(ctx)
	if err := apiAkses.TerapkanPengaturan(ctx, req, aksesOleh(c)); err != nil {
		aksesGagal(c, "simpan pengaturan", err)
		return
	}
	audit.Tandai(c, audit.Tanda{Detail: map[string]interface{}{"sebelum": lama, "sesudah": req}})
	utils.SuccessResponse(c, http.StatusOK, "Pengaturan disimpan dan langsung berlaku", gin.H{"pengaturan": req})
}
