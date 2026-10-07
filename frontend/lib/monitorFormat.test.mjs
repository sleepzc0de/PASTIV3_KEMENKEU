import test from "node:test";
import assert from "node:assert/strict";

// Tes fungsi murni halaman Monitor Resource. Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/monitorFormat.test.mjs
const M = await import(new URL("./monitorFormat.ts", import.meta.url).href);

test("format angka gaya Indonesia", () => {
  assert.equal(M.formatAngka(1234.567, 1), "1.234,6");
  assert.equal(M.formatAngka(0, 0), "0");
  assert.equal(M.formatAngka(null), "-");
  assert.equal(M.formatAngka(Number.NaN), "-");
  assert.equal(M.formatRingkas(12), "12");
  assert.equal(M.formatRingkas(12.34), "12,3");
  assert.equal(M.formatPersen(37.456), "37,5%");
  assert.equal(M.formatPersen(undefined), "-");
});

test("formatBytes memilih satuan yang sesuai", () => {
  assert.equal(M.formatBytes(0), "0 B");
  assert.equal(M.formatBytes(512), "512 B");
  assert.equal(M.formatBytes(1536), "1,5 KB");
  assert.equal(M.formatBytes(5 * 1024 * 1024), "5,0 MB");
  assert.equal(M.formatBytes(8 * 1024 ** 3), "8,0 GB");
  assert.equal(M.formatBytes(2.5 * 1024 ** 4), "2,5 TB");
  assert.equal(M.formatBytes(-5), "0 B");
  assert.equal(M.formatBytes(null), "-");
  assert.equal(M.formatMB(2048), "2,0 GB");
  assert.equal(M.formatMB(null), "-");
});

test("formatMs dan formatDurasi", () => {
  assert.equal(M.formatMs(4.2), "4,2 ms");
  assert.equal(M.formatMs(250), "250 ms");
  assert.equal(M.formatMs(1500), "1,5 dtk");
  assert.equal(M.formatMs(25000), "25 dtk");
  assert.equal(M.formatMs(null), "-");
  assert.equal(M.formatDurasi(30), "30 detik");
  assert.equal(M.formatDurasi(90061), "1 hari 1 jam 1 menit");
  assert.equal(M.formatDurasi(3600), "1 jam");
  assert.equal(M.formatDurasi(7 * 86400 + 5 * 60), "7 hari 5 menit");
  assert.equal(M.formatDurasi(null), "-");
});

test("label status membedakan resource dan kinerja", () => {
  assert.equal(M.labelStatus("cukup"), "Cukup");
  assert.equal(M.labelStatus("kritis", "resource"), "Perlu ditambah");
  assert.equal(M.labelStatus("kritis", "kinerja"), "Perlu diperiksa");
  assert.equal(M.labelStatus("perhatian", "kinerja"), "Perlu perhatian");
  assert.equal(M.labelStatus("tidak_ada_data"), "Belum dapat dinilai");
  for (const s of ["cukup", "perhatian", "kritis", "tidak_ada_data"]) {
    assert.ok(M.KELAS_STATUS[s].lencana && M.KELAS_STATUS[s].titik && M.KELAS_STATUS[s].garis && M.KELAS_STATUS[s].kartu, s);
  }
  assert.equal(M.statusDariPersen(95, 80, 90), "kritis");
  assert.equal(M.statusDariPersen(85, 80, 90), "perhatian");
  assert.equal(M.statusDariPersen(10, 80, 90), "cukup");
  assert.equal(M.statusDariPersen(null, 80, 90), "tidak_ada_data");
});

test("ringkasanSpanduk: perlu ditambah mengalahkan perlu perhatian", () => {
  const k = (nama, status, kelompok = "resource") => ({ nama, status, kelompok });
  assert.match(M.ringkasanSpanduk("kritis", ["CPU server", "Disk server"], []).judul, /^Perlu ditambah: CPU server, Disk server$/);
  assert.match(M.ringkasanSpanduk("kritis", [], [k("Waktu respons aplikasi", "kritis", "kinerja")]).judul, /^Perlu diperiksa: Waktu respons aplikasi$/);
  assert.match(M.ringkasanSpanduk("perhatian", [], [k("Memori (RAM) server", "perhatian"), k("CPU server", "cukup")]).judul, /^Perlu perhatian: Memori \(RAM\) server$/);
  assert.equal(M.ringkasanSpanduk("cukup", [], [k("CPU server", "cukup")]).judul, "Semua resource cukup");
  assert.equal(M.ringkasanSpanduk("tidak_ada_data", [], []).judul, "Belum cukup data untuk menilai");
});

test("batasAtas membulatkan sumbu Y", () => {
  assert.equal(M.batasAtas([[10, 20]], 100), 100);
  assert.equal(M.batasAtas([[3, null, 7]]), 10);
  assert.equal(M.batasAtas([[0.8, 1.2]]), 2);
  assert.equal(M.batasAtas([[130, 90]]), 200);
  assert.equal(M.batasAtas([[420]]), 500);
  assert.equal(M.batasAtas([[600]]), 1000);
  assert.equal(M.batasAtas([[null, null]]), 1);
  assert.equal(M.batasAtas([[]]), 1);
  assert.equal(M.batasAtas([[1], [250]]), 500); // beberapa deret berbagi satu sumbu
});

test("jalurGaris memutus garis pada data kosong dan menjepit nilai di dalam bidang", () => {
  assert.equal(M.jalurGaris([], 100, 50, 10), "");
  assert.equal(M.jalurGaris([0, 10], 100, 50, 10), "M0.0 50.0 L100.0 0.0");
  assert.equal(M.jalurGaris([5], 100, 50, 10), "M50.0 25.0"); // satu titik di tengah
  // celah: dua potongan garis, bukan satu garis yang menyeberangi celah
  assert.equal(M.jalurGaris([0, null, 10], 100, 50, 10), "M0.0 50.0 M100.0 0.0");
  // di luar rentang dijepit
  assert.equal(M.jalurGaris([-5, 50], 100, 50, 10), "M0.0 50.0 L100.0 0.0");
  // ruang kiri untuk label sumbu
  assert.equal(M.jalurGaris([0, 10], 100, 50, 10, 20), "M20.0 50.0 L100.0 0.0");
  assert.equal(M.jumlahTitik([1, null, 2, null]), 2);
});

test("ambilKolom membaca satu kolom dengan skala", () => {
  const titik = [{ rss: 2 * 1024 * 1024, cpu_rata: 5 }, { rss: null, cpu_rata: 10 }, { rss: "x" }];
  assert.deepEqual(M.ambilKolom(titik, "rss", 1024 * 1024), [2, null, null]);
  assert.deepEqual(M.ambilKolom(titik, "cpu_rata"), [5, 10, null]);
});
