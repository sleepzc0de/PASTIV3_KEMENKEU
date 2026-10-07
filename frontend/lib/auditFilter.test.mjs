import test from "node:test";
import assert from "node:assert/strict";

// Tes fungsi murni halaman Log Audit. Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/auditFilter.test.mjs
const A = await import(new URL("./auditFilter.ts", import.meta.url).href);

const SEKARANG = new Date("2026-10-07T08:00:00Z");

test("batasWaktu: rentang berjalan memakai waktu sekarang, kustom memakai tanggal apa adanya", () => {
  assert.deepEqual(A.batasWaktu({ rentang: "24j", dari: "", sampai: "" }, SEKARANG), { dari: "2026-10-06T08:00:00.000Z" });
  assert.deepEqual(A.batasWaktu({ rentang: "1j", dari: "", sampai: "" }, SEKARANG), { dari: "2026-10-07T07:00:00.000Z" });
  assert.deepEqual(A.batasWaktu({ rentang: "7h", dari: "", sampai: "" }, SEKARANG), { dari: "2026-09-30T08:00:00.000Z" });
  assert.deepEqual(A.batasWaktu({ rentang: "semua", dari: "2026-01-01", sampai: "2026-02-01" }, SEKARANG), {}); // tanggal diabaikan bila bukan kustom
  assert.deepEqual(A.batasWaktu({ rentang: "kustom", dari: "2026-10-01", sampai: "2026-10-05" }, SEKARANG), { dari: "2026-10-01", sampai: "2026-10-05" });
  assert.deepEqual(A.batasWaktu({ rentang: "kustom", dari: "2026-10-01", sampai: "" }, SEKARANG), { dari: "2026-10-01" });
  assert.deepEqual(A.batasWaktu({ rentang: "kustom", dari: "01/10/2026", sampai: "besok" }, SEKARANG), {}); // tidak sah diabaikan
});

test("galatRentang menolak rentang kustom yang terbalik atau tidak sah", () => {
  assert.equal(A.galatRentang({ rentang: "24j", dari: "x", sampai: "y" }), "");
  assert.equal(A.galatRentang({ rentang: "kustom", dari: "2026-10-01", sampai: "2026-10-05" }), "");
  assert.equal(A.galatRentang({ rentang: "kustom", dari: "2026-10-05", sampai: "2026-10-01" }), "Tanggal akhir tidak boleh sebelum tanggal awal");
  assert.equal(A.galatRentang({ rentang: "kustom", dari: "salah", sampai: "" }), "Tanggal awal tidak valid");
  assert.equal(A.galatRentang({ rentang: "kustom", dari: "", sampai: "salah" }), "Tanggal akhir tidak valid");
});

test("kueriAudit hanya mengirim isian yang terisi", () => {
  assert.deepEqual(A.kueriAudit(A.FILTER_BAWAAN, SEKARANG), { dari: "2026-10-06T08:00:00.000Z" });
  const f = { ...A.FILTER_BAWAAN, rentang: "semua", kategori: "auth", aksi: "auth.login.gagal", hasil: "gagal", username: "  budi ", ip: "10.1", q: "ekspor", user_id: "abc", halaman: 3 };
  assert.deepEqual(A.kueriAudit(f, SEKARANG, 50), { kategori: "auth", aksi: "auth.login.gagal", hasil: "gagal", username: "budi", ip: "10.1", q: "ekspor", user_id: "abc", halaman: "3", per_halaman: "50" });
  // halaman tidak sah menjadi 1; tanpa perHalaman (ekspor) halaman tidak dikirim
  assert.equal(A.kueriAudit({ ...A.FILTER_BAWAAN, halaman: 0 }, SEKARANG, 50).halaman, "1");
  assert.equal(A.kueriAudit({ ...A.FILTER_BAWAAN, halaman: 4 }, SEKARANG).halaman, undefined);
  assert.equal(A.kueriAudit({ ...A.FILTER_BAWAAN, hasil: "semua" }, SEKARANG).hasil, undefined);
});

test("filterBerubah: halaman tidak dihitung sebagai filter", () => {
  assert.equal(A.filterBerubah(A.FILTER_BAWAAN), false);
  assert.equal(A.filterBerubah({ ...A.FILTER_BAWAAN, halaman: 5 }), false);
  assert.equal(A.filterBerubah({ ...A.FILTER_BAWAAN, q: "x" }), true);
  assert.equal(A.filterBerubah({ ...A.FILTER_BAWAAN, q: "   " }), false);
  assert.equal(A.filterBerubah({ ...A.FILTER_BAWAAN, rentang: "7h" }), true);
  assert.equal(A.filterBerubah({ ...A.FILTER_BAWAAN, user_id: "u" }), true);
});

test("waktuWIB menggeser ke UTC+7 tanpa bergantung pada zona browser", () => {
  assert.equal(A.waktuWIB("2026-10-07T08:04:05Z"), "7 Okt 2026, 15:04:05");
  assert.equal(A.waktuWIB("2026-10-07T08:04:05Z", false), "7 Okt 2026, 15:04");
  assert.equal(A.waktuWIB("2026-12-31T20:30:00Z"), "1 Jan 2027, 03:30:00"); // lewat tengah malam di WIB
  assert.equal(A.waktuWIB("2026-10-07T08:04:05.123456Z"), "7 Okt 2026, 15:04:05");
  assert.equal(A.waktuWIB(""), "-");
  assert.equal(A.waktuWIB(undefined), "-");
  assert.equal(A.waktuWIB("bukan waktu"), "bukan waktu");
  assert.equal(A.waktuSingkatWIB("2026-10-07T08:04:05Z", false), "15:04");
  assert.equal(A.waktuSingkatWIB("2026-10-07T08:04:05Z", true), "7 Okt 15:04");
  assert.equal(A.tanggalSingkatWIB("2026-10-07T18:00:00Z"), "8 Okt"); // 01:00 WIB hari berikutnya
});

test("teksRelatif", () => {
  const t = (detik) => new Date(SEKARANG.getTime() - detik * 1000).toISOString();
  assert.equal(A.teksRelatif(t(10), SEKARANG), "baru saja");
  assert.equal(A.teksRelatif(t(180), SEKARANG), "3 menit lalu");
  assert.equal(A.teksRelatif(t(7200), SEKARANG), "2 jam lalu");
  assert.equal(A.teksRelatif(t(5 * 86400), SEKARANG), "5 hari lalu");
  assert.equal(A.teksRelatif(t(40 * 86400), SEKARANG), "28 Agu 2026, 15:00");
  assert.equal(A.teksRelatif(undefined, SEKARANG), "-");
  assert.equal(A.teksRelatif(new Date(SEKARANG.getTime() + 600_000).toISOString(), SEKARANG), "7 Okt 2026, 15:10"); // jam server sedikit maju
});

test("kelasHasil dan detailBaris", () => {
  assert.equal(A.kelasHasil({ sukses: true, aksi: "pengguna.ubah" }), "berhasil");
  assert.equal(A.kelasHasil({ sukses: false, aksi: "auth.login.gagal" }), "gagal");
  assert.equal(A.kelasHasil({ sukses: false, aksi: "akses.ditolak" }), "ditolak");

  const baris = A.detailBaris({ alasan: "kata_sandi_salah", percobaan: 3, aktif: true, kata_sandi_diganti: false, parameter: { id: "42" }, kueri: "format=xlsx", tidak_dikenal: null });
  assert.deepEqual(baris, [
    ["Alasan", "Kata sandi salah"],
    ["Percobaan gagal ke-", "3"],
    ["Akun aktif", "Ya"],
    ["Kata sandi diganti", "Tidak"],
    ["Parameter rute", "id = 42"],
    ["Kata kunci / filter", "format=xlsx"],
    ["tidak_dikenal", "-"],
  ]);
  assert.deepEqual(A.detailBaris({ alasan: "alasan_baru" }), [["Alasan", "alasan_baru"]]); // alasan yang belum dikenal tampil apa adanya
  assert.deepEqual(A.detailBaris(undefined), []);
});

test("ringkasUserAgent dan labelPeran", () => {
  assert.equal(A.ringkasUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0 Safari/537.36"), "Chrome · Windows");
  assert.equal(A.ringkasUserAgent("Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 Chrome/141.0 Safari/537.36 Edg/141.0"), "Edge · Windows");
  assert.equal(A.ringkasUserAgent("Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605 Version/17 Safari/604"), "Safari · iOS");
  assert.equal(A.ringkasUserAgent("curl/8.4.0"), "curl");
  assert.equal(A.ringkasUserAgent("x".repeat(100)).length, 41);
  assert.equal(A.ringkasUserAgent(undefined), "-");
  assert.equal(A.labelPeran("kanwil", "015010199"), "Kanwil 015010199");
  assert.equal(A.labelPeran("superadmin"), "Superadmin");
  assert.equal(A.labelPeran(""), "");
});
