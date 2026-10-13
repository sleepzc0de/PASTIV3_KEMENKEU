package serverlog

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// Kunci konteks Gin yang dipakai bersama middleware lain.
const (
	// KunciSudahDicatat: permintaan ini sudah dicatat oleh yang menanganinya (mis. ditolak karena beban penuh), jadi Middleware tidak mencatat ulang.
	KunciSudahDicatat = "serverlog_sudah"
	// KunciBerat: permintaan ini rute berat (ekspor, dokumen, analitik); waktu lambatnya wajar sehingga tidak dilaporkan sebagai "lambat".
	KunciBerat = "serverlog_berat"

	kunciUserID = "user_id" // diisi middleware autentikasi
)

func requestID(c *gin.Context) string { return c.Writer.Header().Get("X-Request-ID") }

func entriPermintaan(c *gin.Context, mulai time.Time, level Level, sumber, pesan string) Entri {
	return Entri{
		Waktu: time.Now(), Level: level, Sumber: sumber, Pesan: pesan,
		Rute: c.FullPath(), Metode: c.Request.Method, Status: c.Writer.Status(),
		DurasiMS: int(time.Since(mulai).Milliseconds()), RequestID: requestID(c), UserID: c.GetString(kunciUserID),
	}
}

// Middleware mencatat permintaan yang bermasalah saja, TIDAK ADA log akses per permintaan: respons 5xx (galat server) sebagai error, dan permintaan yang melewati
// batas lambat sebagai warning (rute berat dikecualikan). Dipasang paling luar. lambat <= 0 mematikan catatan permintaan lambat.
func Middleware(lambat time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		mulai := time.Now()
		c.Next()
		if c.GetBool(KunciSudahDicatat) {
			return
		}
		status := c.Writer.Status()
		if c.FullPath() == "/health" {
			return
		}
		switch {
		case status >= 500:
			e := entriPermintaan(c, mulai, Error, "HTTP", "respons galat server: "+http.StatusText(status))
			if len(c.Errors) > 0 {
				e.Detail = c.Errors.String()
			}
			Catat(e)
		case lambat > 0 && time.Since(mulai) > lambat && !c.GetBool(KunciBerat) && c.FullPath() != "":
			Catat(entriPermintaan(c, mulai, Warn, "HTTP", fmt.Sprintf("permintaan lambat (lebih dari %d ms)", lambat.Milliseconds())))
		}
	}
}

// Pemulih menangkap panik pada penangan permintaan: dicatat sebagai error lengkap dengan jejak tumpukan, dan klien menerima 500 yang rapi, sehingga satu permintaan
// yang rusak tidak mematikan server. Dipasang di sebelah dalam middleware lain (audit, pengukur) supaya mereka tetap melihat respons 500-nya.
func Pemulih() gin.HandlerFunc {
	return func(c *gin.Context) {
		mulai := time.Now()
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			// http.ErrAbortHandler adalah cara sah menghentikan respons; teruskan ke net/http.
			if err, ok := r.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(r)
			}
			c.Set(KunciSudahDicatat, true)
			if putus(r) {
				// Klien memutus sambungan di tengah respons: bukan salah server, cukup peringatan.
				c.Abort()
				e := entriPermintaan(c, mulai, Warn, "HTTP", "klien memutus sambungan sebelum respons selesai")
				Catat(e)
				return
			}
			e := entriPermintaan(c, mulai, Error, "PANIK", fmt.Sprint(r))
			e.Status = http.StatusInternalServerError
			e.Detail = potongJejak(string(debug.Stack()))
			Catat(e)
			if !c.Writer.Written() {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Terjadi kesalahan pada server"})
			} else {
				c.Abort()
			}
		}()
		c.Next()
	}
}

// putus: panik akibat klien memutus sambungan (broken pipe / connection reset), yang wajar terjadi saat pengguna menutup tab.
func putus(r interface{}) bool {
	err, ok := r.(error)
	if !ok {
		return false
	}
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) {
		s := strings.ToLower(op.Error())
		return strings.Contains(s, "broken pipe") || strings.Contains(s, "connection reset by peer")
	}
	return false
}

// potongJejak membuang bagian awal jejak tumpukan yang hanya berisi mesin pemulihan sendiri dan membatasi panjangnya.
func potongJejak(s string) string {
	if i := strings.Index(s, "panic("); i >= 0 {
		s = s[i:]
	}
	return s
}
