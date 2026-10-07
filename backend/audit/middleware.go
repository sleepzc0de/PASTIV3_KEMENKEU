package audit

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"pasti-v3-backend/peran"
)

const (
	kunciTanda     = "audit_tanda"
	kunciRequestID = "audit_request_id"
)

// Tanda: keterangan tambahan dari handler yang menggantikan isian bawaan middleware pada entri permintaan ini. Dipakai bila handler tahu hal yang tidak diketahui
// middleware: pengguna yang baru login (belum ada di konteks), alasan gagal, atau objek yang baru dibuat. Jangan mengisinya dengan kata sandi, token, atau badan permintaan.
type Tanda struct {
	Kategori  string
	Aksi      string
	Label     string
	UserID    string
	Username  string
	ObjekTipe string
	ObjekID   string
	Sukses    *bool
	Detail    map[string]interface{}
	Abaikan   bool // jangan catat permintaan ini
}

// Tandai menambahkan keterangan pada entri audit permintaan ini; pemanggilan berulang digabung (isian yang lebih akhir menggantikan, rincian digabung).
func Tandai(c *gin.Context, t Tanda) {
	ada, _ := c.Get(kunciTanda)
	lama, _ := ada.(Tanda)
	if t.Kategori != "" {
		lama.Kategori = t.Kategori
	}
	if t.Aksi != "" {
		lama.Aksi = t.Aksi
	}
	if t.Label != "" {
		lama.Label = t.Label
	}
	if t.UserID != "" {
		lama.UserID = t.UserID
	}
	if t.Username != "" {
		lama.Username = t.Username
	}
	if t.ObjekTipe != "" {
		lama.ObjekTipe = t.ObjekTipe
	}
	if t.ObjekID != "" {
		lama.ObjekID = t.ObjekID
	}
	if t.Sukses != nil {
		lama.Sukses = t.Sukses
	}
	if t.Abaikan {
		lama.Abaikan = true
	}
	if len(t.Detail) > 0 {
		gabung := map[string]interface{}{}
		for k, v := range lama.Detail {
			gabung[k] = v
		}
		for k, v := range t.Detail {
			gabung[k] = v
		}
		lama.Detail = gabung
	}
	c.Set(kunciTanda, lama)
}

// RequestID: pengenal permintaan yang dipasang middleware (juga dikirim sebagai header X-Request-ID).
func RequestID(c *gin.Context) string { return c.GetString(kunciRequestID) }

// Entri tanpa pengguna (login gagal, percobaan tanpa token) bisa dibanjiri penyerang; batasi laju penulisannya supaya tabel tidak dipenuhi dan antrean tidak
// menyisihkan entri pengguna sungguhan.
type pembatas struct {
	mu     sync.Mutex
	mulai  time.Time
	hitung int
	maks   int
}

func (p *pembatas) boleh(sekarang time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if sekarang.Sub(p.mulai) >= time.Second {
		p.mulai, p.hitung = sekarang, 0
	}
	if p.hitung >= p.maks {
		return false
	}
	p.hitung++
	return true
}

var tanpaPengguna = &pembatas{maks: 30}

func metodeUbah(m string) bool {
	return m == http.MethodPost || m == http.MethodPut || m == http.MethodPatch || m == http.MethodDelete
}

// Middleware mencatat aktivitas setelah permintaan selesai: perubahan data oleh pengguna yang login, GET yang ada di katalog dengan tanda Baca (ekspor, unduhan, pencarian
// pegawai), semua permintaan yang ditandai handler (login, dsb.), dan setiap akses yang ditolak (403) bagi pengguna yang login. Tanpa perekam terpasang tidak mencatat apa pun.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		mulai := time.Now()
		id := uuid.NewString()
		c.Set(kunciRequestID, id)
		c.Header("X-Request-ID", id)
		c.Next()
		p := perekam()
		if p == nil {
			return
		}
		if e, ok := susun(c, mulai, id); ok {
			p.Catat(e)
		}
	}
}

// susun membentuk entri dari permintaan yang sudah selesai; ok=false bila permintaan ini tidak perlu dicatat.
func susun(c *gin.Context, mulai time.Time, reqID string) (Entri, bool) {
	metode := c.Request.Method
	if metode == http.MethodOptions || metode == http.MethodHead {
		return Entri{}, false
	}
	rute := c.FullPath()
	if rute == "" { // tidak ada rute yang cocok (404)
		return Entri{}, false
	}
	var tanda Tanda
	adaTanda := false
	if v, ok := c.Get(kunciTanda); ok {
		tanda, adaTanda = v.(Tanda)
	}
	if adaTanda && tanda.Abaikan {
		return Entri{}, false
	}
	aturan, adaAturan := katalog[metode+" "+rute]
	status := c.Writer.Status()

	userID := c.GetString("user_id")
	username := c.GetString("username")
	if tanda.UserID != "" {
		userID = tanda.UserID
	}
	if tanda.Username != "" {
		username = tanda.Username
	}
	ubah := metodeUbah(metode)
	kategoriAuth := adaAturan && aturan.Kategori == KatAuth || adaTanda && tanda.Kategori == KatAuth
	catat := adaTanda ||
		(adaAturan && (ubah || aturan.Baca) && (userID != "" || kategoriAuth)) ||
		(ubah && userID != "") ||
		(status == http.StatusForbidden && userID != "")
	if !catat {
		return Entri{}, false
	}
	if userID == "" && !tanpaPengguna.boleh(mulai) {
		return Entri{}, false
	}

	e := Entri{
		Waktu: mulai.UTC(), RequestID: reqID, UserID: userID, Username: username, Metode: metode, Rute: rute, Status: status,
		Sukses: status < 400, DurasiMS: int(time.Since(mulai).Milliseconds()), IP: c.ClientIP(), UserAgent: c.Request.UserAgent(),
		Peran: c.GetString(peran.KunciGinPeran), KodePeran: peran.DariGin(c).Kode,
	}
	param := map[string]string{}
	for _, p := range c.Params {
		param[p.Key] = p.Value
	}
	switch {
	case adaAturan:
		e.Kategori, e.Aksi, e.Label = aturan.Kategori, aturan.Aksi, isiLabel(aturan.Label, param)
		e.ObjekTipe = aturan.ObjekTipe
		if aturan.ObjekParam != "" {
			e.ObjekID = param[aturan.ObjekParam]
		}
	default:
		e.Kategori, e.Aksi, e.Label = KategoriDariRute(rute), metode+" "+rute, "Menjalankan "+metode+" "+rute
	}
	detail := map[string]interface{}{}
	if len(param) > 0 {
		detail["parameter"] = param
	}
	if adaAturan && aturan.Kueri {
		if q := bersihkanKueri(c.Request.URL.RawQuery); q != "" {
			detail["kueri"] = q
		}
	}
	if host, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil && host != e.IP {
		detail["remote_addr"] = host // alamat yang tersambung langsung (proxy); IP di atas dari header proxy
	}
	if adaTanda {
		if tanda.Kategori != "" {
			e.Kategori = tanda.Kategori
		}
		if tanda.Aksi != "" {
			e.Aksi = tanda.Aksi
		}
		if tanda.Label != "" {
			e.Label = tanda.Label
		}
		if tanda.ObjekTipe != "" {
			e.ObjekTipe = tanda.ObjekTipe
		}
		if tanda.ObjekID != "" {
			e.ObjekID = tanda.ObjekID
		}
		if tanda.Sukses != nil {
			e.Sukses = *tanda.Sukses
		}
		for k, v := range tanda.Detail {
			detail[k] = v
		}
	}
	// Login yang tidak berhasil (mis. permintaan rusak yang tidak sempat ditandai handler) tidak boleh tercatat sebagai berhasil.
	if e.Aksi == AksiLoginBerhasil && !e.Sukses {
		e.Aksi, e.Label = AksiLoginGagal, "Login gagal"
	}
	// Akses ditolak ditandai tersendiri supaya mudah disaring; aksi aslinya tetap tersimpan di rincian.
	if status == http.StatusForbidden && e.Aksi != AksiLoginGagal {
		detail["aksi_asal"] = e.Aksi
		e.Aksi, e.Label, e.Sukses = AksiDitolak, "Akses ditolak: "+e.Label, false
	}
	if len(detail) > 0 {
		e.Detail = detail
	}
	return e, true
}
