import test from "node:test";
import assert from "node:assert/strict";

// Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/refUE1.test.mjs
const { petaUE1, labelUE1, namaUE1, kodeDenganUraian, opsiUE1 } = await import(new URL("./refUE1.ts", import.meta.url).href);

const peta = petaUE1([
  { kode: "01504", nama: "DIREKTORAT JENDERAL PAJAK", singkatan: "DJP", urutan: 4, aktif: true },
  { kode: "01599", nama: "UNIT TANPA SINGKATAN", singkatan: "", urutan: 100, aktif: true },
]);

test("label: kode dan singkatan, atau uraian bila singkatan kosong, atau 'UE1 kode' bila belum terdaftar", () => {
  assert.equal(labelUE1("01504", peta), "01504 · DJP");
  assert.equal(labelUE1("01599", peta), "01599 · UNIT TANPA SINGKATAN");
  assert.equal(labelUE1("01500", peta), "UE1 01500");
  assert.equal(labelUE1("01504", {}), "UE1 01504"); // referensi gagal dimuat: tetap tampil kodenya
});

test("kode kosong atau '(kosong)' tampil '(kosong)' dan tidak pernah menjadi 'UE1 undefined'", () => {
  for (const k of ["", null, undefined, "(kosong)"]) {
    assert.equal(labelUE1(k, peta), "(kosong)");
    assert.equal(namaUE1(k, peta), "(kosong)");
    assert.equal(kodeDenganUraian(k, peta), "(kosong)");
  }
});

test("nama dan kode dengan uraian", () => {
  assert.equal(namaUE1("01504", peta), "DIREKTORAT JENDERAL PAJAK");
  assert.equal(namaUE1("01500", peta), "01500");
  assert.equal(kodeDenganUraian("01504", peta), "01504 · DIREKTORAT JENDERAL PAJAK (DJP)");
  assert.equal(kodeDenganUraian("01599", peta), "01599 · UNIT TANPA SINGKATAN");
  assert.equal(kodeDenganUraian("01500", peta), "01500");
});

test("opsi untuk select: nilai tetap kode asli, label dari referensi, urutan mengikuti masukan", () => {
  assert.deepEqual(opsiUE1(["01599", "01504", "01500"], peta), [
    { value: "01599", label: "01599 · UNIT TANPA SINGKATAN" },
    { value: "01504", label: "01504 · DJP" },
    { value: "01500", label: "UE1 01500" },
  ]);
});
