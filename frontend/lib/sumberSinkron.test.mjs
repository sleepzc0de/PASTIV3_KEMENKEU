import test from "node:test";
import assert from "node:assert/strict";

// Tes fungsi murni sumber data sinkronisasi Digitalisasi Aset (datamart). Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/sumberSinkron.test.mjs
const S = await import(new URL("./sumberSinkron.ts", import.meta.url).href);

const SEKARANG = Date.parse("2026-10-25T00:00:00Z");
const fmt = (iso) => `<${iso.slice(0, 10)}>`;
const info = (o = {}) => ({
  tersedia: true,
  terjangkau: true,
  data_per: "2026-10-23T03:00:00Z",
  penyegaran_terakhir: "sukses",
  ...o,
});
const kunci = (l) => l.map((x) => x.kunci);

test("tanpa info sumber (respons belum dimuat): tidak ada pemberitahuan dan tidak bisa mulai", () => {
  assert.deepEqual(S.pemberitahuanSumber(undefined, SEKARANG, fmt), []);
  assert.equal(S.bisaMulai(undefined), false);
});

test("datamart sehat: satu keterangan bahwa data disalin dari datamart beserta data per", () => {
  const l = S.pemberitahuanSumber(info(), SEKARANG, fmt);
  assert.deepEqual(kunci(l), ["datamart"]);
  assert.equal(l[0].nada, "info");
  assert.match(l[0].teks, /data per <2026-10-23>/);
  assert.match(l[0].teks, /bukan langsung dari SLDK/);
  assert.equal(S.bisaMulai(info()), true);
});

test("datamart tidak dikonfigurasi: peringatan yang menyebut DATAMART_DB_*, dan tidak bisa mulai", () => {
  const l = S.pemberitahuanSumber(info({ tersedia: false, terjangkau: false, data_per: null, penyegaran_terakhir: "" }), SEKARANG, fmt);
  assert.deepEqual(kunci(l), ["tidak_tersedia"]);
  assert.equal(l[0].nada, "warning");
  assert.match(l[0].teks, /DATAMART_DB_\*/);
  assert.doesNotMatch(l[0].teks, /SLDK_DB_/);
  assert.equal(S.bisaMulai(info({ tersedia: false })), false);
});

test("datamart tak terjangkau: peringatan bahwa sinkronisasi akan gagal", () => {
  const l = S.pemberitahuanSumber(info({ terjangkau: false, data_per: null, penyegaran_terakhir: "" }), SEKARANG, fmt);
  assert.deepEqual(kunci(l), ["datamart", "tidak_terjangkau"]);
  assert.equal(l[1].nada, "warning");
  assert.doesNotMatch(l[0].teks, /data per/);
});

test("datamart belum pernah berhasil disegarkan: peringatan bahwa belum ada data", () => {
  const l = S.pemberitahuanSumber(info({ data_per: null, penyegaran_terakhir: "gagal" }), SEKARANG, fmt);
  assert.deepEqual(kunci(l), ["datamart", "belum_disegarkan"]);
});

test("penyegaran terakhir gagal: peringatan bahwa data berasal dari penyegaran sukses sebelumnya", () => {
  const l = S.pemberitahuanSumber(info({ penyegaran_terakhir: "gagal" }), SEKARANG, fmt);
  assert.deepEqual(kunci(l), ["datamart", "penyegaran_gagal"]);
  assert.match(l[1].teks, /sebelumnya/);
  // sedang berjalan bukan masalah
  assert.deepEqual(kunci(S.pemberitahuanSumber(info({ penyegaran_terakhir: "berjalan" }), SEKARANG, fmt)), ["datamart"]);
});

test("datamart usang melewati batas hari: peringatan dengan umurnya; tepat di batas belum", () => {
  const usang = S.pemberitahuanSumber(info({ data_per: "2026-10-01T00:00:00Z" }), SEKARANG, fmt);
  assert.deepEqual(kunci(usang), ["datamart", "datamart_usang"]);
  assert.match(usang[1].teks, /24 hari lalu/);
  const batas = new Date(SEKARANG - S.BATAS_USANG_HARI * 24 * 3600 * 1000).toISOString();
  assert.deepEqual(kunci(S.pemberitahuanSumber(info({ data_per: batas }), SEKARANG, fmt)), ["datamart"]);
});

test("umurHari: hari penuh, tak pernah negatif, null bila tidak terbaca", () => {
  assert.equal(S.umurHari("2026-10-24T00:00:00Z", SEKARANG), 1);
  assert.equal(S.umurHari("2026-10-24T12:00:00Z", SEKARANG), 0);
  assert.equal(S.umurHari("2026-10-26T00:00:00Z", SEKARANG), 0);
  assert.equal(S.umurHari(null, SEKARANG), null);
  assert.equal(S.umurHari("bukan tanggal", SEKARANG), null);
});

test("labelSumberLama: hanya riwayat dari masa membaca SLDK langsung yang diberi tanda", () => {
  assert.equal(S.labelSumberLama("sldk"), "SLDK (lama)");
  assert.equal(S.labelSumberLama("datamart"), null);
  assert.equal(S.labelSumberLama(null), null);
  assert.equal(S.labelSumberLama(undefined), null);
});

test("judul konfirmasi menyebut datamart", () => {
  assert.match(S.JUDUL_KONFIRMASI, /datamart/);
});
