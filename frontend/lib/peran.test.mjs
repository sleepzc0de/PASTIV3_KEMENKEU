import test from "node:test";
import assert from "node:assert/strict";

// Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/peran.test.mjs
const { labelPeran, panjangKode, kodePeranSah, namaPeranBerkode, teksCakupan, bolehSemuaData, opsiPeran, perluPemilih, peranTampil, saranBaru } = await import(
  new URL("./peran.ts", import.meta.url).href
);

const info = (o = {}) => ({
  akun_role: "user", role: "user", peran: "", peran_label: "", peran_id: 0, kode: "", cakupan: { tingkat: "semua" }, tersedia: [], bawaan: false, wajib: false, ...o,
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

test("pilihan peran: admin punya peran bawaan di depan; pengguna biasa hanya peran datanya; yang berlaku ditandai aktif", () => {
  const admin = info({ akun_role: "admin", role: "admin", peran: "admin", peran_id: 0, bawaan: true, tersedia: [baris(4, "ue1", "01504"), baris(7, "satker", "119091")] });
  assert.deepEqual(opsiPeran(admin), [
    { id: null, label: "Admin", aktif: true },
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
  assert.equal(perluPemilih(info({ akun_role: "admin", bawaan: true, tersedia: [] })), false); // admin tanpa peran data: tidak ada yang dipilih
  assert.equal(perluPemilih(info({ akun_role: "admin", bawaan: true, tersedia: [baris(1, "ue1", "01504")] })), true);
});

test("peran yang tampil di samping nama", () => {
  assert.equal(peranTampil(null, "admin"), "Admin");
  assert.equal(peranTampil(undefined, "superadmin"), "Super Admin");
  assert.equal(peranTampil(info(), "user"), "Pengguna");
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
