import test from "node:test";
import assert from "node:assert/strict";

// Tes fungsi murni halaman Log Server. Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/logServerFilter.test.mjs
const L = await import(new URL("./logServerFilter.ts", import.meta.url).href);

const f = (p = {}) => ({ ...L.FILTER_BAWAAN_LOG, ...p });

test("kueriLog: hanya mengirim isian yang terisi, level semua tidak dikirim", () => {
  assert.deepEqual(L.kueriLog(f(), {}), {});
  assert.deepEqual(L.kueriLog(f({ level: "error", sumber: " INAPROC SYNC ", q: " timeout " }), { dari: "2026-10-12T00:00:00.000Z" }), {
    dari: "2026-10-12T00:00:00.000Z",
    level: "error",
    sumber: "INAPROC SYNC",
    q: "timeout",
  });
  assert.deepEqual(L.kueriLog(f({ halaman: 3 }), { dari: "2026-10-01", sampai: "2026-10-07" }, 50), { dari: "2026-10-01", sampai: "2026-10-07", halaman: "3", per_halaman: "50" });
  // halaman tidak sah jatuh ke 1
  assert.equal(L.kueriLog(f({ halaman: 0 }), {}, 50).halaman, "1");
  assert.equal(L.kueriLog(f({ halaman: Number.NaN }), {}, 50).halaman, "1");
});

test("filterLogBerubah", () => {
  assert.equal(L.filterLogBerubah(f()), false);
  assert.equal(L.filterLogBerubah(f({ halaman: 5 })), false); // halaman bukan filter
  assert.equal(L.filterLogBerubah(f({ level: "warn" })), true);
  assert.equal(L.filterLogBerubah(f({ rentang: "7h" })), true);
  assert.equal(L.filterLogBerubah(f({ sumber: "  " })), false);
  assert.equal(L.filterLogBerubah(f({ q: "x" })), true);
});

test("ringkasPesan: memadatkan spasi dan memotong tanpa memenggal kata", () => {
  assert.equal(L.ringkasPesan("gagal   membaca\n\tbaris"), "gagal membaca baris");
  const panjang = "kata ".repeat(100);
  const r = L.ringkasPesan(panjang, 40);
  assert.ok(r.endsWith("…"));
  assert.ok(r.length <= 41);
  assert.ok(!/\s…$/.test(r), "tidak boleh berakhir dengan spasi sebelum elipsis");
  assert.equal(L.ringkasPesan("pendek", 40), "pendek");
});

test("teksJumlah", () => {
  assert.equal(L.teksJumlah(1), "");
  assert.equal(L.teksJumlah(0), "");
  assert.equal(L.teksJumlah(1204), "× 1.204");
});

test("deretPerJam: tinggi relatif terhadap jam tersibuk, jam kosong nol", () => {
  const { maks, batang } = L.deretPerJam([
    { jam: "a", error: 10, warn: 0 },
    { jam: "b", error: 5, warn: 5 },
    { jam: "c", error: 0, warn: 0 },
    { jam: "d", error: 0, warn: 20 },
  ]);
  assert.equal(maks, 20);
  assert.equal(batang[0].tinggiError, 50);
  assert.equal(batang[1].tinggiError, 25);
  assert.equal(batang[1].tinggiWarn, 25);
  assert.equal(batang[2].tinggiError + batang[2].tinggiWarn, 0);
  assert.equal(batang[3].tinggiWarn, 100);
  assert.equal(batang[3].total, 20);
  // semua nol: tidak membagi dengan nol
  const nol = L.deretPerJam([{ jam: "a", error: 0, warn: 0 }]);
  assert.equal(nol.maks, 0);
  assert.equal(nol.batang[0].tinggiError, 0);
  assert.deepEqual(L.deretPerJam([]), { maks: 0, batang: [] });
});

const PENGATURAN = { level: "warn", simpan: "warn", retensi_error_hari: 30, retensi_warn_hari: 7, maks_baris: 50000, lambat_ms: 3000 };
const PEREKAM = { diterima: 10, ditulis: 10, dijatuhkan: 0, gagal: 0, antrean: 0, kapasitas: 2048, tertunda: 0, dibatalkan_klien: 3 };

test("catatanPerekam: tanpa masalah tidak ada catatan; masalah penyimpanan dijelaskan", () => {
  assert.deepEqual(L.catatanPerekam(PEREKAM, PENGATURAN), []);
  const c = L.catatanPerekam({ ...PEREKAM, dijatuhkan: 1500, gagal: 2, tertunda: 4 }, PENGATURAN);
  assert.equal(c.length, 3);
  assert.equal(c[0].tingkat, "warning");
  assert.match(c[0].teks, /1\.500 catatan tidak sempat disimpan/);
  assert.match(c[1].teks, /gagal 2 kali/);
  assert.equal(c[2].tingkat, "info");
  const mati = L.catatanPerekam(PEREKAM, { ...PENGATURAN, simpan: "off" });
  assert.equal(mati[0].tingkat, "warning");
  assert.match(mati[0].teks, /LOG_SIMPAN=off/);
  const hanyaError = L.catatanPerekam(PEREKAM, { ...PENGATURAN, simpan: "error" });
  assert.equal(hanyaError[0].tingkat, "info");
});

const KETAHANAN = { ditolak_laju: 0, ditolak_beban: 0, ditolak_berat: 0, serentak_sekarang: 2, serentak_puncak: 40, berat_sekarang: 0, berat_antri: 0, maks_serentak: 400, maks_berat: 4, laju_aktif: true };

test("catatanKetahanan: penolakan beban memberi petunjuk, kondisi sehat tanpa catatan", () => {
  assert.deepEqual(L.catatanKetahanan(KETAHANAN), []);
  const c = L.catatanKetahanan({ ...KETAHANAN, ditolak_beban: 541, ditolak_berat: 3, ditolak_laju: 9 });
  assert.equal(c.length, 3);
  assert.equal(c[0].tingkat, "warning");
  assert.match(c[0].teks, /541 permintaan ditolak \(503\)/);
  assert.match(c[0].teks, /MAKS_PERMINTAAN_SERENTAK/);
  assert.match(c[1].teks, /MAKS_BERAT_SERENTAK/);
  assert.equal(c[2].tingkat, "info");
  assert.match(c[2].teks, /RATE_LIMIT_UMUM_PER_DETIK/);
});

test("catatanKetahanan: mendekati batas dan pembatas laju mati", () => {
  const dekat = L.catatanKetahanan({ ...KETAHANAN, serentak_puncak: 350 });
  assert.equal(dekat.length, 1);
  assert.match(dekat[0].teks, /mendekati batas 400/);
  // bila sudah ada penolakan, cukup catatan penolakannya (tidak ganda)
  assert.equal(L.catatanKetahanan({ ...KETAHANAN, serentak_puncak: 400, ditolak_beban: 5 }).length, 1);
  const mati = L.catatanKetahanan({ ...KETAHANAN, laju_aktif: false });
  assert.match(mati[0].teks, /RATE_LIMIT_AKTIF=false/);
});

test("teksPenyimpanan", () => {
  assert.match(L.teksPenyimpanan(PENGATURAN), /Galat \(error\) dan peringatan \(warning\) yang disimpan; error disimpan 30 hari, warning 7 hari, paling banyak 50\.000 baris/);
  assert.match(L.teksPenyimpanan({ ...PENGATURAN, simpan: "error" }), /Hanya galat \(error\) yang disimpan; error disimpan 30 hari, paling banyak/);
  assert.equal(L.teksPenyimpanan({ ...PENGATURAN, simpan: "off" }), "Penyimpanan log dimatikan.");
});
