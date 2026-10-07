import test from "node:test";
import assert from "node:assert/strict";

// Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/peran.test.mjs
const { labelPeran, panjangKode, kodePeranSah, namaPeranBerkode, teksCakupan, bolehSemuaData, bolehLihatPengadaan, peranEfektif, adalahSuperadmin, adalahTamu, bolehKelolaPengguna, bolehLihatPengguna, bolehLihatSinkronisasi, PERAN_LIHAT_PENGGUNA, opsiPeran, perluPemilih, peranTampil, saranBaru } = await import(
  new URL("./peran.ts", import.meta.url).href
);

const info = (o = {}) => ({
  akun_role: "user", role: "user", peran: "", peran_label: "", peran_id: 0, kode: "", cakupan: { tingkat: "semua" }, tersedia: [], bawaan: false, tamu: false, ...o,
});
const baris = (id, role, kode = "") => ({ id, role, kode, aktif: false, label: role });

test("kode peran sah: Pengguna Barang tanpa kode, lainnya angka sepanjang tingkatnya", () => {
  assert.equal(kodePeranSah("pengguna_barang", ""), true);
  assert.equal(kodePeranSah("pengguna_barang", "01504"), false);
  assert.equal(kodePeranSah("ue1", "01504"), true);
  assert.equal(kodePeranSah("ue1", "1504"), false);
  assert.equal(kodePeranSah("ue1", "0150A"), false);
  assert.equal(kodePeranSah("kanwil", "015040199"), true);
  assert.equal(kodePeranSah("kanwil", "01504"), false);
  assert.equal(kodePeranSah("satker", "119091"), true);
  assert.equal(kodePeranSah("satker", "11909"), false);
  assert.equal(kodePeranSah("superadmin", ""), false); // diberikan lewat akun, bukan peran data
  assert.equal(kodePeranSah("", ""), false);
  assert.deepEqual(["pengguna_barang", "ue1", "kanwil", "satker", "x"].map(panjangKode), [0, 5, 9, 6, -1]);
});

test("label dan nama peran berkode", () => {
  assert.equal(labelPeran("superadmin"), "Super Admin");
  assert.equal(labelPeran("pengguna_barang"), "Pengguna Barang");
  assert.equal(labelPeran(""), "");
  assert.equal(labelPeran("lain"), "lain");
  assert.equal(namaPeranBerkode("ue1", "01504"), "UE1 01504");
  assert.equal(namaPeranBerkode("pengguna_barang", ""), "Pengguna Barang");
});

test("teks cakupan", () => {
  assert.equal(teksCakupan({ tingkat: "semua" }), "Seluruh data");
  assert.equal(teksCakupan(undefined), "Seluruh data");
  assert.equal(teksCakupan({ tingkat: "ue1", kode: "01504" }), "UE1 01504");
  assert.equal(teksCakupan({ tingkat: "kanwil", kode: "015040199" }), "Kanwil 015040199");
  assert.equal(teksCakupan({ tingkat: "satker", kode: "119091" }), "Satker 119091");
  assert.match(teksCakupan({ tingkat: "kosong" }), /belum diberi peran/);
});

test("boleh semua data: hanya cakupan semua (atau info belum ada)", () => {
  assert.equal(bolehSemuaData(undefined), true);
  assert.equal(bolehSemuaData(null), true);
  assert.equal(bolehSemuaData(info()), true);
  assert.equal(bolehSemuaData(info({ cakupan: { tingkat: "ue1", kode: "01504" } })), false);
  assert.equal(bolehSemuaData(info({ cakupan: { tingkat: "kosong" } })), false);
});

test("pengadaan terbuka bagi semua peran yang punya data, termasuk yang dibatasi per satker; hanya cakupan kosong yang ditolak", () => {
  assert.equal(bolehLihatPengadaan(undefined), true);
  assert.equal(bolehLihatPengadaan(null), true);
  assert.equal(bolehLihatPengadaan(info()), true);
  assert.equal(bolehLihatPengadaan(info({ cakupan: { tingkat: "ue1", kode: "01504" } })), true);
  assert.equal(bolehLihatPengadaan(info({ cakupan: { tingkat: "kanwil", kode: "015040199" } })), true);
  assert.equal(bolehLihatPengadaan(info({ cakupan: { tingkat: "satker", kode: "119091" } })), true);
  assert.equal(bolehLihatPengadaan(info({ cakupan: { tingkat: "kosong" } })), false);
});

test("peran efektif untuk hak akses menu dan fitur", () => {
  assert.equal(peranEfektif(info({ akun_role: "superadmin", role: "superadmin", peran: "superadmin" })), "superadmin");
  assert.equal(peranEfektif(info({ peran: "pengguna_barang" })), "pengguna_barang");
  assert.equal(peranEfektif(info({ peran: "satker", kode: "119091" })), "satker");
  assert.equal(peranEfektif(info()), ""); // tamu
  assert.equal(peranEfektif(null, "superadmin"), "superadmin"); // peran belum dimuat: role akun superadmin tetap superadmin
  assert.equal(peranEfektif(undefined, "user"), "");
  assert.equal(peranEfektif(undefined), "");
  // superadmin yang bertindak sebagai peran data kehilangan hak superadmin
  assert.equal(adalahSuperadmin(info({ akun_role: "superadmin", role: "user", peran: "satker" })), false);
  assert.equal(adalahSuperadmin(info({ akun_role: "superadmin", role: "superadmin", peran: "superadmin" })), true);
  assert.equal(adalahSuperadmin(info({ peran: "pengguna_barang" })), false);
});

test("tamu: sudah masuk tetapi belum punya peran", () => {
  assert.equal(adalahTamu(info({ peran: "", tamu: true })), true);
  assert.equal(adalahTamu(info({ peran: "ue1", kode: "01504" })), false);
  assert.equal(adalahTamu(info({ peran: "superadmin" })), false);
  assert.equal(adalahTamu(undefined), false); // belum dimuat: bukan tamu (yang menentukan hanya backend)
});

test("manajemen pengguna: superadmin dan Pengguna Barang mengelola; UE1, Kanwil, dan Satker hanya melihat; tamu tidak", () => {
  for (const [peran, kelola, lihat] of [
    ["superadmin", true, true],
    ["pengguna_barang", true, true],
    ["ue1", false, true],
    ["kanwil", false, true],
    ["satker", false, true],
    ["", false, false],
    ["admin", false, false], // role lama sudah ditiadakan
  ]) {
    assert.equal(bolehKelolaPengguna(peran), kelola, "kelola " + peran);
    assert.equal(bolehLihatPengguna(peran), lihat, "lihat " + peran);
  }
  assert.deepEqual(PERAN_LIHAT_PENGGUNA, ["superadmin", "pengguna_barang", "ue1", "kanwil", "satker"]);
});

test("pilihan peran: superadmin punya peran bawaan di depan; pengguna biasa hanya peran datanya; yang berlaku ditandai aktif", () => {
  const admin = info({ akun_role: "superadmin", role: "superadmin", peran: "superadmin", peran_id: 0, bawaan: true, tersedia: [baris(4, "ue1", "01504"), baris(7, "satker", "119091")] });
  assert.deepEqual(opsiPeran(admin), [
    { id: null, label: "Super Admin", aktif: true },
    { id: 4, label: "UE1 01504", aktif: false },
    { id: 7, label: "Satker 119091", aktif: false },
  ]);
  const sebagaiSatker = { ...admin, peran: "satker", peran_id: 7, kode: "119091" };
  assert.deepEqual(opsiPeran(sebagaiSatker).map((o) => o.aktif), [false, false, true]);

  const biasa = info({ peran: "ue1", peran_id: 4, kode: "01504", tersedia: [baris(4, "ue1", "01504"), baris(9, "pengguna_barang")] });
  assert.deepEqual(opsiPeran(biasa), [{ id: 4, label: "UE1 01504", aktif: true }, { id: 9, label: "Pengguna Barang", aktif: false }]);
  assert.deepEqual(opsiPeran(null), []);
});

test("pemilih hanya muncul bila ada dua pilihan atau lebih", () => {
  assert.equal(perluPemilih(info()), false);
  assert.equal(perluPemilih(info({ tersedia: [baris(1, "ue1", "01504")], peran_id: 1 })), false);
  assert.equal(perluPemilih(info({ tersedia: [baris(1, "ue1", "01504"), baris(2, "satker", "119091")], peran_id: 1 })), true);
  assert.equal(perluPemilih(info({ akun_role: "superadmin", bawaan: true, tersedia: [] })), false); // superadmin tanpa peran data: tidak ada yang dipilih
  assert.equal(perluPemilih(info({ akun_role: "superadmin", bawaan: true, tersedia: [baris(1, "ue1", "01504")] })), true);
});

test("peran yang tampil di samping nama: tanpa peran adalah Tamu", () => {
  assert.equal(peranTampil(null, "user"), "");
  assert.equal(peranTampil(undefined, "superadmin"), "Super Admin");
  assert.equal(peranTampil(info(), "user"), "Tamu");
  assert.equal(peranTampil(info({ akun_role: "superadmin", role: "superadmin", peran: "superadmin" }), "superadmin"), "Super Admin");
  assert.equal(peranTampil(info({ peran: "kanwil", kode: "015040199" }), "user"), "Kanwil 015040199");
});

test("saran yang sudah dimiliki tidak disarankan lagi", () => {
  const saran = [
    { role: "ue1", kode: "01504", label: "UE1" },
    { role: "kanwil", kode: "015040199", label: "Kanwil" },
    { role: "satker", kode: "119091", label: "Satker" },
  ];
  assert.deepEqual(saranBaru(saran, [baris(1, "ue1", "01504")]).map((s) => s.role), ["kanwil", "satker"]);
  assert.deepEqual(saranBaru(saran, [baris(1, "ue1", "01509")]).map((s) => s.role), ["ue1", "kanwil", "satker"]); // kode lain bukan duplikat
  assert.deepEqual(saranBaru([], [baris(1, "ue1", "01504")]), []);
});

test("tab Sinkronisasi Digitalisasi Aset: hanya superadmin dan Pengguna Barang; UE1, Kanwil, Satker, dan tamu tidak melihatnya", () => {
  for (const p of ["superadmin", "pengguna_barang"]) assert.equal(bolehLihatSinkronisasi(p), true, p);
  for (const p of ["ue1", "kanwil", "satker", "user", "", "admin"]) assert.equal(bolehLihatSinkronisasi(p), false, p);
  // memakai peran efektif: superadmin yang bertindak sebagai Satker tidak melihatnya, kembali ke peran bawaan melihatnya
  assert.equal(bolehLihatSinkronisasi(peranEfektif(info({ peran: "satker" }), "user")), false);
  assert.equal(bolehLihatSinkronisasi(peranEfektif(info({ peran: "superadmin" }), "superadmin")), true);
  assert.equal(bolehLihatSinkronisasi(peranEfektif(undefined, "user")), false); // profil belum dimuat: disembunyikan dulu
});
