import test from "node:test";
import assert from "node:assert/strict";

// Tes fungsi murni sumber data sinkronisasi Digitalisasi Aset. Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/sumberSinkron.test.mjs
const S = await import(new URL("./sumberSinkron.ts", import.meta.url).href);

const SEKARANG = Date.parse("2026-10-25T00:00:00Z");
const fmt = (iso) => `<${iso.slice(0, 10)}>`;
const info = (o = {}) => ({
  mode: "otomatis",
  sldk: true,
  datamart: true,
  tersedia: true,
  cadangan: true,
  datamart_disegarkan: "2026-10-23T03:00:00Z",
  datamart_terjangkau: true,
  ...o,
});
const kunci = (l) => l.map((x) => x.kunci);

test("backend lama tanpa info sumber: tidak ada pemberitahuan, dan sldk_tersedia menentukan", () => {
  assert.deepEqual(S.pemberitahuanSumber(undefined, SEKARANG, fmt), []);
  assert.equal(S.bisaMulai(undefined, true), true);
  assert.equal(S.bisaMulai(undefined, false), false);
  assert.equal(S.judulKonfirmasi(undefined), "Mulai sinkronisasi dari SLDK?");
});

test("cadangan aktif dan sehat: satu pemberitahuan informasi dengan kesegaran datamart", () => {
  const l = S.pemberitahuanSumber(info(), SEKARANG, fmt);
  assert.deepEqual(kunci(l), ["cadangan"]);
  assert.equal(l[0].nada, "info");
  assert.match(l[0].teks, /data per <2026-10-23>/);
  assert.match(l[0].teks, /hanya dipakai bila lebih baru/);
});

test("tanpa datamart: tidak ada pemberitahuan sama sekali", () => {
  assert.deepEqual(S.pemberitahuanSumber(info({ datamart: false, cadangan: false, datamart_disegarkan: null, datamart_terjangkau: false }), SEKARANG, fmt), []);
});

test("tidak ada sumber: peringatan yang menyebut penyebab menurut mode", () => {
  const a = S.pemberitahuanSumber(info({ tersedia: false, sldk: false, datamart: false, cadangan: false }), SEKARANG, fmt);
  assert.deepEqual(kunci(a), ["tidak_tersedia"]);
  assert.equal(a[0].nada, "warning");
  assert.match(a[0].teks, /SLDK_DB_\*/);
  assert.match(S.pemberitahuanSumber(info({ tersedia: false, mode: "datamart", datamart: false }), SEKARANG, fmt)[0].teks, /DATAMART_DB_\*/);
  assert.match(S.pemberitahuanSumber(info({ tersedia: false, mode: "sldk", sldk: false }), SEKARANG, fmt)[0].teks, /"sldk"/);
});

test("mode datamart dan SLDK tidak dikonfigurasi: dijelaskan bahwa datamart yang dibaca", () => {
  const m = S.pemberitahuanSumber(info({ mode: "datamart", cadangan: false }), SEKARANG, fmt);
  assert.deepEqual(kunci(m), ["mode_datamart"]);
  assert.match(m[0].teks, /bukan SLDK/);
  const h = S.pemberitahuanSumber(info({ sldk: false, cadangan: false }), SEKARANG, fmt);
  assert.deepEqual(kunci(h), ["hanya_datamart"]);
  assert.equal(S.judulKonfirmasi(info({ mode: "datamart" })), "Mulai sinkronisasi dari datamart?");
  assert.equal(S.judulKonfirmasi(info({ sldk: false, cadangan: false })), "Mulai sinkronisasi dari datamart?");
  assert.equal(S.judulKonfirmasi(info()), "Mulai sinkronisasi dari SLDK?");
});

test("datamart tak terjangkau atau belum pernah disegarkan: peringatan bahwa cadangan tidak bekerja", () => {
  const l = S.pemberitahuanSumber(info({ datamart_terjangkau: false, datamart_disegarkan: null }), SEKARANG, fmt);
  assert.deepEqual(kunci(l), ["cadangan", "datamart_bermasalah"]);
  assert.equal(l[1].nada, "warning");
  assert.doesNotMatch(l[0].teks, /data per/);
});

test("datamart usang melewati batas hari: peringatan dengan umurnya; tepat di batas belum", () => {
  const usang = S.pemberitahuanSumber(info({ datamart_disegarkan: "2026-10-01T00:00:00Z" }), SEKARANG, fmt);
  assert.deepEqual(kunci(usang), ["cadangan", "datamart_usang"]);
  assert.match(usang[1].teks, /24 hari lalu/);
  const batas = new Date(SEKARANG - S.BATAS_USANG_HARI * 24 * 3600 * 1000).toISOString();
  assert.deepEqual(kunci(S.pemberitahuanSumber(info({ datamart_disegarkan: batas }), SEKARANG, fmt)), ["cadangan"]);
});

test("mode sldk: datamart yang ada tidak dipakai, jadi tidak diperingatkan walau usang", () => {
  const l = S.pemberitahuanSumber(info({ mode: "sldk", cadangan: false, datamart_disegarkan: "2026-01-01T00:00:00Z", datamart_terjangkau: false }), SEKARANG, fmt);
  assert.deepEqual(l, []);
});

test("umurHari: hari penuh, tak pernah negatif, null bila tidak terbaca", () => {
  assert.equal(S.umurHari("2026-10-24T00:00:00Z", SEKARANG), 1);
  assert.equal(S.umurHari("2026-10-24T12:00:00Z", SEKARANG), 0);
  assert.equal(S.umurHari("2026-10-26T00:00:00Z", SEKARANG), 0);
  assert.equal(S.umurHari(null, SEKARANG), null);
  assert.equal(S.umurHari("bukan tanggal", SEKARANG), null);
});

test("labelSumber: datamart atau SLDK (riwayat lama tanpa sumber = SLDK)", () => {
  assert.equal(S.labelSumber("datamart"), "Datamart");
  assert.equal(S.labelSumber("sldk"), "SLDK");
  assert.equal(S.labelSumber(null), "SLDK");
  assert.equal(S.labelSumber(undefined), "SLDK");
});
