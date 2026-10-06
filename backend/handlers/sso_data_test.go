package handlers

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func jwtUji(payload string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." + enc([]byte(payload)) + "." + enc([]byte("tanda-tangan"))
}

func TestBacaKlaimIDToken(t *testing.T) {
	k := bacaKlaimIDToken(jwtUji(`{"sub":"abc","nip":198505152010011001,"iat":1700000000,"roles":["a","b"]}`))
	if k == nil {
		t.Fatal("id_token yang sah tidak terbaca")
	}
	if k["sub"] != "abc" {
		t.Errorf("sub = %v", k["sub"])
	}
	// angka besar tidak boleh kehilangan digit (float64 akan mengubahnya menjadi 198505152010011000)
	if got := klaimTeks(k, "nip"); got != "198505152010011001" {
		t.Errorf("nip = %q, want 198505152010011001", got)
	}
	for _, rusak := range []string{"", "bukan-jwt", "a.b", "a.%%%.c", jwtUji("bukan json"), jwtUji(`[1,2]`)} {
		if got := bacaKlaimIDToken(rusak); got != nil {
			t.Errorf("bacaKlaimIDToken(%q) = %v, want nil", rusak, got)
		}
	}
}

func TestKlaimTeks(t *testing.T) {
	k := map[string]interface{}{"s": "teks", "n": json.Number("12345678901234567890"), "f": 1.5, "b": true, "x": nil}
	if klaimTeks(k, "s") != "teks" || klaimTeks(k, "n") != "12345678901234567890" {
		t.Error("string dan angka harus terbaca apa adanya")
	}
	for _, kunci := range []string{"f", "b", "x", "tidak-ada"} {
		if klaimTeks(k, kunci) != "" {
			t.Errorf("klaimTeks(%q) harus kosong untuk tipe selain string/angka JSON", kunci)
		}
	}
}

func TestHashKlaimStabilDanAbaikanKlaimYangBerubahTiapLogin(t *testing.T) {
	ui := map[string]interface{}{"sub": "1", "jabatan": "Pranata Komputer", "kode_satker": "119091"}
	id1 := map[string]interface{}{"sub": "1", "iat": json.Number("1"), "exp": json.Number("2"), "nonce": "x", "at_hash": "h1"}
	id2 := map[string]interface{}{"sub": "1", "iat": json.Number("100"), "exp": json.Number("200"), "nonce": "y", "at_hash": "h2"}
	if hashKlaim(ui, id1) != hashKlaim(ui, id2) {
		t.Error("hash tidak boleh berubah hanya karena iat/exp/nonce/at_hash")
	}
	// urutan kunci tidak berpengaruh
	ui2 := map[string]interface{}{"kode_satker": "119091", "jabatan": "Pranata Komputer", "sub": "1"}
	if hashKlaim(ui, id1) != hashKlaim(ui2, id1) {
		t.Error("hash harus sama untuk isi yang sama walau urutan kunci berbeda")
	}
	// perubahan data pegawai harus terlihat
	ui3 := map[string]interface{}{"sub": "1", "jabatan": "Kepala Seksi", "kode_satker": "119091"}
	if hashKlaim(ui, id1) == hashKlaim(ui3, id1) {
		t.Error("perubahan jabatan harus mengubah hash")
	}
	// id_token kosong berbeda dari id_token berisi data pegawai
	if hashKlaim(ui, nil) == hashKlaim(ui, map[string]interface{}{"unit": "A"}) {
		t.Error("klaim id_token yang berbeda harus mengubah hash")
	}
	if len(hashKlaim(ui, nil)) != 64 {
		t.Errorf("panjang hash = %d, want 64 (SHA-256 heksadesimal, cocok dengan kolom CHAR(64))", len(hashKlaim(ui, nil)))
	}
}
