package acuan

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Pagar: sinkronisasi Digitalisasi Aset di server tidak boleh mengakses SLDK. Satu-satunya kode yang membaca SLDK untuk dataset ini adalah paket acuan, dan hanya tes yang boleh memakainya.

const jalurAcuan = "pasti-v3-backend/digitalisasi/acuan"

// berkasGoServer: berkas .go non-tes di seluruh backend.
func berkasGoServer(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(filepath.Join("..", ".."), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 50 {
		t.Fatalf("hanya %d berkas Go ditemukan: jalur penelusuran salah", len(out))
	}
	return out
}

func TestKodeServerTidakMengimporAcuan(t *testing.T) {
	for _, p := range berkasGoServer(t) {
		if strings.Contains(filepath.ToSlash(p), "/digitalisasi/acuan/") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, imp := range f.Imports {
			if strings.Trim(imp.Path.Value, `"`) == jalurAcuan {
				t.Errorf("%s mengimpor paket acuan: kode server tidak boleh membaca SLDK untuk Digitalisasi Aset", p)
			}
		}
	}
}

func TestSinkronisasiDigitalisasiTidakMenyentuhSLDK(t *testing.T) {
	// paket digitalisasi dan handler-nya tidak menyebut koneksi SLDK
	larangan := []string{"SLDKDB", "ConnectSLDK", "SLDK_DB_", "DJKN."}
	for _, p := range berkasGoServer(t) {
		slash := filepath.ToSlash(p)
		if !(strings.Contains(slash, "/digitalisasi/") && !strings.Contains(slash, "/acuan/")) && !strings.HasSuffix(slash, "/handlers/digitalisasi_sync.go") {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		kode := stripKomentar(string(b))
		for _, k := range larangan {
			if strings.Contains(kode, k) {
				t.Errorf("%s memuat %q: sinkronisasi Digitalisasi Aset tidak boleh mengakses SLDK", p, k)
			}
		}
	}
}

// stripKomentar membuang komentar baris dan blok sederhana supaya penjelasan tentang SLDK di komentar tidak dianggap pemakaian.
func stripKomentar(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if i := strings.Index(l, "//"); i >= 0 {
			l = l[:i]
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}
