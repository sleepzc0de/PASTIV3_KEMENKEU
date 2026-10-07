package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/peran"
	"pasti-v3-backend/utils"
)

// Siapa yang boleh melihat dan mengelola pengguna lain:
//
//   - Superadmin: semua pengguna.
//   - Pengguna Barang: semua pengguna KECUALI superadmin (tidak melihat dan tidak dapat mengubahnya).
//   - UE1, Kanwil, Satker: hanya MELIHAT pengguna dalam cakupan kode satker SSO-nya, juga tanpa superadmin: kode satker pegawai berawalan 5 digit (UE1) atau 9 digit
//     (Kanwil) yang sama, atau karakter ke-10 sampai ke-15 yang sama (Satker, lihat peran.Cakupan.KondisiSQL). Akun non-SSO tidak punya kode satker, jadi
//     tidak tampak bagi mereka.
//   - Peran lain (tamu): tidak melihat siapa pun.
//
// Mengubah pengguna (membuat, mengubah, menonaktifkan, menghapus, memberi dan mencabut peran) hanya untuk superadmin dan Pengguna Barang (middleware
// RequireKelolaPengguna), dan Pengguna Barang tidak dapat menyentuh superadmin.

// kondisiPengguna: potongan WHERE (alias u = users, e = employees) yang membatasi pengguna yang boleh dilihat pemanggil. Kode cakupan diperiksa ulang sebagai
// angka murni oleh peran.Cakupan.KondisiSQL sehingga aman ditulis langsung ke SQL.
func kondisiPengguna(c *gin.Context) string {
	if c.GetString("role") == peran.AkunSuperadmin {
		return "1 = 1"
	}
	const tanpaSuperadmin = "u.role <> N'superadmin' AND u.is_protected = 0"
	switch c.GetString(peran.KunciGinPeran) {
	case peran.PenggunaBarang:
		return tanpaSuperadmin
	case peran.UE1, peran.Kanwil, peran.Satker:
		return tanpaSuperadmin + " AND (" + peran.DariGin(c).KondisiSQL("e.kode_satker") + ")"
	}
	return "1 = 0"
}

// penggunaTerlihat memastikan pengguna :id boleh dilihat pemanggil; bila tidak (tidak ada, superadmin bagi non-superadmin, atau di luar cakupan) menjawab 404 dan
// mengembalikan false. Jawabannya sama untuk "tidak ada" dan "tidak boleh", supaya keberadaan akun lain tidak bocor.
func penggunaTerlihat(c *gin.Context, ctx context.Context, id string) bool {
	var n int
	err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u LEFT JOIN employees e ON e.id = u.employee_id WHERE u.id = @p1 AND (`+kondisiPengguna(c)+`)`, id).Scan(&n)
	if err != nil {
		log.Println("[PENGGUNA ERROR] cek akses pengguna:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan server")
		return false
	}
	if n == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "User tidak ditemukan")
		return false
	}
	return true
}

// penggunaTerlihatID: seperti penggunaTerlihat dengan batas waktu sendiri, untuk handler yang belum punya context.
func penggunaTerlihatID(c *gin.Context, id string) bool {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	return penggunaTerlihat(c, ctx, id)
}
