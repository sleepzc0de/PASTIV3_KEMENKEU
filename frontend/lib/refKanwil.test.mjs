import test from "node:test";
import assert from "node:assert/strict";

// Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/refKanwil.test.mjs
const { petaKanwil, kodeKanwilDariSatker, labelKanwil, namaKanwil, kodeKanwilDenganUraian, saringKanwil, ringkasHasilTarik } = await import(new URL("./refKanwil.ts", import.meta.url).href);

const daftar = [
  { kode: "015040199", kode_ue1: "01504", nama: "KANTOR WILAYAH DJP JAKARTA PUSAT", singkatan: "KW DJP JKT", urutan: 100, aktif: true, sumber: "sldk" },
  { kode: "015050199", kode_ue1: "01505", nama: "KANTOR WILAYAH DJBC UTAMA", singkatan: "", urutan: 100, aktif: true, sumber: "satker" },
  { kode: "015090299", kode_ue1: "01509", nama: "KANWIL DJKN JAWA TIMUR", singkatan: "KANWIL JATIM", urutan: 100, aktif: false, sumber: "manual" },
];
const peta = petaKanwil(daftar);

test("kode Kanwil dari kode satker lengkap: 9 karakter pertama yang semuanya angka", () => {
  assert.equal(kodeKanwilDariSatker("015040199119091000KP"), "015040199");
  assert.equal(kodeKanwilDariSatker(" 015040199119091000KP "), "015040199");
  assert.equal(kodeKanwilDariSatker("015040199"), "015040199");
  assert.equal(kodeKanwilDariSatker("01504019"), ""); // terlalu pendek
  assert.equal(kodeKanwilDariSatker("01504X199119091"), ""); // huruf pada 9 karakter pertama
  for (const k of ["", null, undefined]) assert.equal(kodeKanwilDariSatker(k), "");
});

test("label: kode dan singkatan, atau uraian bila singkatan kosong, atau 'Kanwil kode' bila belum terdaftar", () => {
  assert.equal(labelKanwil("015040199", peta), "015040199 · KW DJP JKT");
  assert.equal(labelKanwil("015050199", peta), "015050199 · KANTOR WILAYAH DJBC UTAMA");
  assert.equal(labelKanwil("015060199", peta), "Kanwil 015060199");
  assert.equal(labelKanwil("015040199", {}), "Kanwil 015040199"); // referensi gagal dimuat: tetap tampil kodenya
});

test("kode kosong atau '(kosong)' tampil '(kosong)' dan tidak pernah menjadi 'Kanwil undefined'", () => {
  for (const k of ["", null, undefined, "(kosong)"]) {
    assert.equal(labelKanwil(k, peta), "(kosong)");
    assert.equal(namaKanwil(k, peta), "(kosong)");
    assert.equal(kodeKanwilDenganUraian(k, peta), "(kosong)");
  }
});

test("nama dan kode dengan uraian", () => {
  assert.equal(namaKanwil("015040199", peta), "KANTOR WILAYAH DJP JAKARTA PUSAT");
  assert.equal(namaKanwil("015060199", peta), "015060199");
  assert.equal(kodeKanwilDenganUraian("015040199", peta), "015040199 · KANTOR WILAYAH DJP JAKARTA PUSAT (KW DJP JKT)");
  assert.equal(kodeKanwilDenganUraian("015050199", peta), "015050199 · KANTOR WILAYAH DJBC UTAMA");
  assert.equal(kodeKanwilDenganUraian("015060199", peta), "015060199");
});

test("pencarian: kode (bagian), uraian, atau singkatan; semua kata harus cocok; kosong mengembalikan semuanya", () => {
  const kode = (l) => l.map((r) => r.kode);
  assert.deepEqual(kode(saringKanwil(daftar, "")), ["015040199", "015050199", "015090299"]);
  assert.deepEqual(kode(saringKanwil(daftar, "   ")), ["015040199", "015050199", "015090299"]);
  assert.deepEqual(kode(saringKanwil(daftar, "djp")), ["015040199"]);
  assert.deepEqual(kode(saringKanwil(daftar, "kantor wilayah")), ["015040199", "015050199"]);
  assert.deepEqual(kode(saringKanwil(daftar, "KANWIL jatim")), ["015090299"]);
  assert.deepEqual(kode(saringKanwil(daftar, "01505")), ["015050199"]);
  assert.deepEqual(kode(saringKanwil(daftar, "djp djbc")), []);
  assert.deepEqual(kode(saringKanwil(daftar, "tidak ada")), []);
});

test("ringkasan hasil penarikan menyebut yang dilewati hanya bila ada", () => {
  assert.equal(
    ringkasHasilTarik({ dibaca: 10, ditambahkan: 3, diperbarui: 2, tanpa_perubahan: 5, dilewati_manual: 0, tidak_sah: 0, dipotong: 0 }),
    "10 baris dibaca dari SLDK, 3 ditambahkan, 2 diperbarui, 5 tanpa perubahan."
  );
  const s = ringkasHasilTarik({ dibaca: 6, ditambahkan: 1, diperbarui: 1, tanpa_perubahan: 0, dilewati_manual: 1, tidak_sah: 3, dipotong: 1 });
  assert.match(s, /1 diisi manual \(tidak ditimpa\)/);
  assert.match(s, /3 dilewati karena tidak sah/);
  assert.match(s, /1 uraian dipotong/);
});
