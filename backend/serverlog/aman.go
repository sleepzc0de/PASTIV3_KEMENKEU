package serverlog

import (
	"fmt"
	"runtime/debug"
)

// Panik pada goroutine latar belakang (pengukur, pembersih, penjadwal) TIDAK ditangkap oleh pemulih permintaan HTTP dan mematikan seluruh proses. Aman menjalankan fn dan
// mengubah panik menjadi catatan error (dengan jejak tumpukan), sehingga satu putaran pekerjaan latar belakang yang rusak tidak menjatuhkan server. Mengembalikan true bila fn
// selesai tanpa panik.
func Aman(nama string, fn func()) (selesai bool) {
	defer func() {
		if r := recover(); r != nil {
			Catat(Entri{Level: Error, Sumber: "PANIK", Pesan: fmt.Sprintf("%s: %v", nama, r), Detail: potongJejak(string(debug.Stack()))})
			selesai = false
		}
	}()
	fn()
	return true
}
