import test from "node:test";
import assert from "node:assert/strict";

// Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/satker.test.mjs
const { saringSatker, urutkanSatker, kodeUE1Daftar, hitungPerStatus, persenTerhubung, petunjukKeterhubungan, jumlahPengadaan } = await import(new URL("./satker.ts", import.meta.url).href);

const peng = (rup = 0, tender = 0, non = 0, pagu = 0) => ({ rup_paket: rup, rup_pagu: pagu, tender, tender_pagu: 0, non_tender: non, non_tender_pagu: 0 });
const satker = (kode, nama, o = {}) => ({
  kode, nama, kode_ue1: "01504", ue1: "01504 · DJP", jenis: "INDUK SATKER", jumlah_kode_aset: 1, aset: {}, jumlah_aset: 0, kdj: 0, kdo: 0,
  pengadaan: peng(), status: "hanya_aset", ...o,
});

const DAFTAR = [
  satker("119091", "KPP Pratama Jakarta Gambir", { jumlah_aset: 10, pengadaan: peng(3, 1, 2, 5_000), status: "terhubung" }),
  satker("200200", "Kantor Wilayah DJKN Jawa Timur", { kode_ue1: "01509", ue1: "01509 · DJKN", jumlah_aset: 40, pengadaan: peng(1, 0, 0, 9_000), status: "terhubung" }),
  satker("300300", "KPKNL Surabaya", { kode_ue1: "01509", ue1: "01509 · DJKN", jumlah_aset: 25 }),
  satker("999999", "SATKER TANPA ASET", { kode_ue1: "", ue1: "", jumlah_aset: 0, pengadaan: peng(5, 0, 0, 1_000), status: "hanya_pengadaan" }),
  satker("012345", "", { nama_pengadaan: "Satker Kode Nol", kode_ue1: "", ue1: "", jumlah_aset: 3, status: "hanya_aset" }),
];

test("penyaring kata kunci: semua kata harus cocok pada kode, nama, nama pengadaan, atau label UE1, tanpa memedulikan huruf besar/kecil", () => {
  const cari = (q) => saringSatker(DAFTAR, { q, status: "", ue1: "" }).map((s) => s.kode);
  assert.deepEqual(cari(""), ["119091", "200200", "300300", "999999", "012345"]);
  assert.deepEqual(cari("   "), cari(""));
  assert.deepEqual(cari("gambir"), ["119091"]);
  assert.deepEqual(cari("KPP jakarta"), ["119091"]);
  assert.deepEqual(cari("jakarta kpknl"), []); // semua kata harus ada
  assert.deepEqual(cari("200200"), ["200200"]);
  assert.deepEqual(cari("djkn"), ["200200", "300300"]); // dari label UE1
  assert.deepEqual(cari("kode nol"), ["012345"]); // dari nama menurut Inaproc
  assert.deepEqual(cari("012345"), ["012345"]);
});

test("penyaring status dan UE1 digabung dengan kata kunci", () => {
  const ambil = (s) => saringSatker(DAFTAR, s).map((x) => x.kode);
  assert.deepEqual(ambil({ q: "", status: "terhubung", ue1: "" }), ["119091", "200200"]);
  assert.deepEqual(ambil({ q: "", status: "", ue1: "01509" }), ["200200", "300300"]);
  assert.deepEqual(ambil({ q: "", status: "hanya_aset", ue1: "01509" }), ["300300"]);
  assert.deepEqual(ambil({ q: "surabaya", status: "terhubung", ue1: "" }), []);
  // satker tanpa UE1 dipilih lewat "(kosong)"
  assert.deepEqual(ambil({ q: "", status: "", ue1: "(kosong)" }), ["999999", "012345"]);
});

test("pengurutan: terbanyak lebih dulu, seri diputus kode, masukan tidak diubah", () => {
  const salinan = DAFTAR.map((s) => s.kode);
  assert.deepEqual(urutkanSatker(DAFTAR, "aset").map((s) => s.kode), ["200200", "300300", "119091", "012345", "999999"]);
  assert.deepEqual(urutkanSatker(DAFTAR, "pengadaan").map((s) => s.kode), ["119091", "999999", "200200", "012345", "300300"]);
  assert.deepEqual(urutkanSatker(DAFTAR, "nilai").map((s) => s.kode), ["200200", "119091", "999999", "012345", "300300"]);
  assert.deepEqual(urutkanSatker(DAFTAR, "kode").map((s) => s.kode), ["012345", "119091", "200200", "300300", "999999"]);
  // nama kosong ditaruh terakhir
  assert.equal(urutkanSatker(DAFTAR, "nama").at(-1).kode, "012345");
  assert.deepEqual(DAFTAR.map((s) => s.kode), salinan);
  assert.equal(jumlahPengadaan(DAFTAR[0]), 6);
});

test("pilihan UE1 dan hitungan per status", () => {
  assert.deepEqual(kodeUE1Daftar(DAFTAR), ["01504", "01509", "(kosong)"]);
  assert.deepEqual(hitungPerStatus(DAFTAR), { semua: 5, terhubung: 2, hanya_aset: 2, hanya_pengadaan: 1 });
  assert.deepEqual(hitungPerStatus([]), { semua: 0, terhubung: 0, hanya_aset: 0, hanya_pengadaan: 0 });
});

test("persen terhubung dihitung dari satker pada data aset dan aman dari pembagian nol", () => {
  assert.equal(persenTerhubung({ satker_aset: 4, satker_pengadaan: 3, terhubung: 1, hanya_aset: 3, hanya_pengadaan: 2, aset_tanpa_kode: 0 }), 25);
  assert.equal(persenTerhubung({ satker_aset: 0, satker_pengadaan: 3, terhubung: 0, hanya_aset: 0, hanya_pengadaan: 3, aset_tanpa_kode: 0 }), 0);
});

test("petunjuk: data belum ada, kode tidak cocok sama sekali, dan banyak satker pengadaan yang tidak ditemukan", () => {
  const r = (o) => ({ satker_aset: 10, satker_pengadaan: 10, terhubung: 5, hanya_aset: 5, hanya_pengadaan: 5, aset_tanpa_kode: 0, ...o });
  assert.deepEqual(petunjukKeterhubungan(r({}), "2026").map((p) => p.tingkat), ["info"]); // 50% hanya pengadaan -> info
  assert.equal(petunjukKeterhubungan(r({ hanya_pengadaan: 1, terhubung: 9, hanya_aset: 1 }), "2026").length, 0); // sehat
  const nol = petunjukKeterhubungan(r({ terhubung: 0, hanya_aset: 10, hanya_pengadaan: 10 }), "2026");
  assert.equal(nol[0].tingkat, "perhatian");
  assert.match(nol[0].isi, /kd_satker_str/);
  const tanpaAset = petunjukKeterhubungan(r({ satker_aset: 0, terhubung: 0, hanya_aset: 0, hanya_pengadaan: 10 }), "2026");
  assert.match(tanpaAset[0].isi, /Data aset belum ada/);
  const tanpaPeng = petunjukKeterhubungan(r({ satker_pengadaan: 0, terhubung: 0, hanya_pengadaan: 0 }), "2025");
  assert.match(tanpaPeng[0].isi, /tahun 2025/);
  const kosong = petunjukKeterhubungan(r({ satker_aset: 0, satker_pengadaan: 0, terhubung: 0, hanya_aset: 0, hanya_pengadaan: 0 }), "2026");
  assert.equal(kosong.length, 1);
  assert.match(petunjukKeterhubungan(r({ terhubung: 9, hanya_aset: 1, hanya_pengadaan: 1, aset_tanpa_kode: 1234 }), "2026").at(-1).isi, /1\.234 baris/);
});
