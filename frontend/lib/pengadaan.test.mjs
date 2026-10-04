import test from "node:test";
import assert from "node:assert/strict";

// Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/pengadaan.test.mjs
const p = await import(new URL("./pengadaan.ts", import.meta.url).href);

test("rupiahRingkas memakai satuan T/M/jt dengan koma desimal", () => {
  assert.equal(p.rupiahRingkas(1_250_000_000_000), "Rp 1,25 T");
  assert.equal(p.rupiahRingkas(3_400_000_000), "Rp 3,4 M");
  assert.equal(p.rupiahRingkas(12_500_000), "Rp 12,5 jt");
  assert.equal(p.rupiahRingkas(950_000), "Rp 950.000");
  assert.equal(p.rupiahRingkas(0), "Rp 0");
  assert.equal(p.rupiahRingkas(-2_500_000_000), "-Rp 2,5 M");
  assert.equal(p.rupiahRingkas(null), "-");
  assert.equal(p.rupiahRingkas(Number.NaN), "-");
});

test("sumbuRingkas dan batasSumbu menghasilkan angka sumbu yang bulat", () => {
  assert.equal(p.sumbuRingkas(2_500_000_000), "2,5 M");
  assert.equal(p.sumbuRingkas(450_000_000), "450 jt");
  assert.equal(p.sumbuRingkas(12_000), "12 rb");
  assert.equal(p.sumbuRingkas(0), "0");
  assert.equal(p.batasSumbu(0), 1);
  assert.equal(p.batasSumbu(-5), 1);
  assert.equal(p.batasSumbu(1), 1);
  assert.equal(p.batasSumbu(1.3), 2);
  assert.equal(p.batasSumbu(2.4), 2.5);
  assert.equal(p.batasSumbu(3.2), 5);
  assert.equal(p.batasSumbu(7), 10);
  assert.equal(p.batasSumbu(870_000_000), 1_000_000_000);
  // Batas atas selalu >= nilai terbesar, supaya batang tidak keluar dari bidang gambar.
  for (const v of [0.4, 1.01, 2.01, 2.51, 5.01, 9.99, 123, 4_567_890_123]) assert.ok(p.batasSumbu(v) >= v, `batas untuk ${v}`);
});

test("formatPersen dan bagi aman terhadap pembagian dengan nol", () => {
  assert.equal(p.formatPersen(12.345), "12,3%");
  assert.equal(p.formatPersen(0), "0%");
  assert.equal(p.formatPersen(Number.POSITIVE_INFINITY), "-");
  assert.equal(p.bagi(1, 0), 0);
  assert.equal(p.bagi(25, 100), 25);
});

test("waktuRelatif dan durasi", () => {
  const now = Date.parse("2026-10-04T10:00:00Z");
  assert.equal(p.waktuRelatif(null, now), "belum pernah");
  assert.equal(p.waktuRelatif("2026-10-04T09:59:40Z", now), "baru saja");
  assert.equal(p.waktuRelatif("2026-10-04T09:30:00Z", now), "30 menit lalu");
  assert.equal(p.waktuRelatif("2026-10-04T05:00:00Z", now), "5 jam lalu");
  assert.equal(p.waktuRelatif("2026-10-01T10:00:00Z", now), "3 hari lalu");
  assert.equal(p.waktuRelatif("2026-10-05T10:00:00Z", now), "baru saja"); // jam perangkat mundur
  assert.equal(p.durasi("2026-10-04T10:00:00Z", "2026-10-04T10:02:05Z"), 125);
  assert.equal(p.durasi("2026-10-04T10:00:00Z", null), null);
  assert.equal(p.durasi("2026-10-04T10:00:10Z", "2026-10-04T10:00:00Z"), null);
  assert.equal(p.formatDurasi(45), "45 dtk");
  assert.equal(p.formatDurasi(125), "2 mnt 5 dtk");
  assert.equal(p.formatDurasi(3725), "1 jam 2 mnt");
});

test("labelParameter merapikan parameter riwayat", () => {
  assert.equal(p.labelParameter("K10/2025"), "K10 · 2025");
  assert.equal(p.labelParameter("L1"), "Tingkat 1");
  assert.equal(p.labelParameter("L3:ABC/XYZ"), "Tingkat 3 · ABC / XYZ");
  assert.equal(p.labelParameter("2025/COMPLETED/K10"), "2025 · COMPLETED · K10");
  assert.equal(p.labelParameter(""), "");
});

const dataset = (o) => ({ id: "tender/x", kelompok: "tender", subkelompok: "Tender", nama: "X", deskripsi: "", mode: "klpd_tahun", otomatis: true, punya_klpd: true,
  punya_tahun: true, perlu_status: false, baris: 10, disinkron_at: null, terakhir: null, terakhir_sukses: null, ...o });
const riwayat = (o) => ({ id: 1, batch_id: "b", dataset: "tender/x", parameter: "K10/2025", pemicu: "manual", status: "sukses", jumlah_baris: 1, baris_gagal: 0, halaman: 1,
  percobaan: 1, pesan: null, dijalankan_oleh: null, dibuat: "2026-10-04T00:00:00Z", mulai: null, selesai: "2026-10-04T00:00:00Z", ...o });

test("kesegaranDataset membedakan kosong, belum, segar, lama, dan gagal", () => {
  const now = Date.parse("2026-10-05T00:00:00Z");
  assert.equal(p.kesegaranDataset(dataset({ baris: 0 }), now), "belum");
  assert.equal(p.kesegaranDataset(dataset({ terakhir_sukses: riwayat({}), terakhir: riwayat({}) }), now), "segar");
  assert.equal(p.kesegaranDataset(dataset({ terakhir_sukses: riwayat({ selesai: "2026-09-01T00:00:00Z" }) }), now), "lama");
  assert.equal(p.kesegaranDataset(dataset({ baris: 0, terakhir_sukses: riwayat({}) }), now), "kosong");
  // Gagal yang lebih baru dari sukses terakhir terlihat; gagal lama yang sudah disusul sukses tidak.
  assert.equal(p.kesegaranDataset(dataset({ terakhir_sukses: riwayat({ id: 1 }), terakhir: riwayat({ id: 2, status: "gagal" }) }), now), "gagal");
  assert.equal(p.kesegaranDataset(dataset({ terakhir_sukses: riwayat({ id: 3 }), terakhir: riwayat({ id: 3 }) }), now), "segar");
  // Ada baris tetapi tidak ada riwayat (data dari penarikan di halaman lama): ditandai lama, bukan "belum".
  assert.equal(p.kesegaranDataset(dataset({ baris: 5 }), now), "lama");
});

test("kelompokkanDataset menjaga urutan server dan membuang kelompok kosong", () => {
  const ds = [
    dataset({ id: "a", kelompok: "tender", subkelompok: "Tender" }),
    dataset({ id: "b", kelompok: "rup", subkelompok: "Penyedia" }),
    dataset({ id: "c", kelompok: "tender", subkelompok: "Non-tender" }),
    dataset({ id: "d", kelompok: "tender", subkelompok: "Tender" }),
  ];
  const out = p.kelompokkanDataset(ds, [{ id: "rup", nama: "RUP" }, { id: "tender", nama: "Tender" }, { id: "kosong", nama: "Kosong" }]);
  assert.deepEqual(out.map((k) => k.id), ["rup", "tender"]);
  assert.deepEqual(out[1].subkelompok.map((s) => [s.nama, s.datasets.map((d) => d.id)]), [["Tender", ["a", "d"]], ["Non-tender", ["c"]]]);
});

test("kalimatJadwal menjelaskan keadaan penarikan otomatis", () => {
  const o = { aktif: true, token_ada: true, berikutnya: null, jumlah_tugas: 56, jatuh_tempo: 0, terakhir_otomatis: null, zona: "WIB" };
  const pg = { aktif: true, interval_hari: 2, jam_mulai: 1, jam_akhir: 5, kode_klpd: "K10", jumlah_tahun: 2, jeda_detik: 2, dataset: [], diubah: null, diubah_oleh: "", bawaan_server: true };
  assert.equal(p.kalimatJadwal(o, pg), "Otomatis setiap 2 hari, dimulai antara pukul 01.00 dan 05.00 WIB, untuk tahun berjalan dan 1 tahun sebelumnya.");
  assert.match(p.kalimatJadwal(o, { ...pg, interval_hari: 1, jam_akhir: 1, jumlah_tahun: 1 }), /^Otomatis setiap hari, kapan saja, untuk tahun berjalan\.$/);
  assert.match(p.kalimatJadwal(o, { ...pg, aktif: false }), /dimatikan/);
  assert.match(p.kalimatJadwal({ ...o, token_ada: false }, pg), /token Inaproc belum dikonfigurasi/);
});

test("daftarTahun dan tahunValid", () => {
  assert.deepEqual(p.daftarTahun(2026, 3), ["2026", "2025", "2024"]);
  assert.equal(p.tahunValid("2025"), true);
  assert.equal(p.tahunValid(" 2025 "), true);
  assert.equal(p.tahunValid("25"), false);
  assert.equal(p.tahunValid("3025"), false);
});

test("bangunPermintaan: matriks untuk dataset biasa, tugas eksplisit untuk yang butuh isian", () => {
  const ds = [
    dataset({ id: "tender/pengumuman", nama: "Pengumuman Tender" }),
    dataset({ id: "ekatalog-archive/instansi-satker", nama: "Instansi", mode: "klpd" }),
    dataset({ id: "ekatalog/penyedia-detail", nama: "Penyedia V6", mode: "kode", otomatis: false }),
    dataset({ id: "ekatalog/list-kategori-produk", nama: "Kategori", mode: "kategori" }),
    dataset({ id: "ekatalog/e-purchasing-by-produk", nama: "Transaksi", mode: "transaksi", perlu_status: true }),
    dataset({ id: "rup/paket-penyedia", nama: "Paket Penyedia", perlu_status: true }),
  ];
  const dasar = { kodeKLPD: " K10 ", tahun: ["2025", "2024"], dataset: [], kode: {}, opsi: {} };

  // Dataset per KLPD+tahun x 2 tahun, per KLPD x1, kategori tingkat 1 x1 (eksplisit).
  let r = p.bangunPermintaan(ds, { ...dasar, dataset: ["tender/pengumuman", "ekatalog-archive/instansi-satker", "ekatalog/list-kategori-produk"] });
  assert.deepEqual(r.masalah, []);
  assert.equal(r.jumlahTugas, 4);
  assert.deepEqual(r.permintaan.datasets, ["tender/pengumuman", "ekatalog-archive/instansi-satker"]);
  assert.equal(r.permintaan.kode_klpd, "K10");
  assert.deepEqual(r.permintaan.tahun, ["2025", "2024"]);
  assert.deepEqual(r.permintaan.tugas, [{ dataset: "ekatalog/list-kategori-produk", kode_klpd: "K10" }]);

  // Beberapa kode sekaligus; kode ganda digabung.
  r = p.bangunPermintaan(ds, { ...dasar, dataset: ["ekatalog/penyedia-detail"], kode: { "ekatalog/penyedia-detail": "A1, B2\nA1  C3" } });
  assert.deepEqual(r.permintaan.tugas.map((t) => t.kode), ["A1", "B2", "C3"]);
  assert.equal(r.jumlahTugas, 3);
  assert.equal(r.permintaan.datasets, undefined);

  // Status khusus mengubah dataset matriks menjadi tugas per tahun.
  r = p.bangunPermintaan(ds, { ...dasar, dataset: ["ekatalog/e-purchasing-by-produk", "rup/paket-penyedia"], opsi: { "ekatalog/e-purchasing-by-produk": { status: "ON_PROCESS" } } });
  assert.equal(r.jumlahTugas, 4); // transaksi per tahun (2) + paket penyedia per tahun (2)
  assert.deepEqual(r.permintaan.datasets, ["rup/paket-penyedia"]);
  assert.deepEqual(r.permintaan.tugas.map((t) => [t.dataset, t.tahun, t.status]), [["ekatalog/e-purchasing-by-produk", "2025", "ON_PROCESS"], ["ekatalog/e-purchasing-by-produk", "2024", "ON_PROCESS"]]);
});

test("bangunPermintaan melaporkan masalah yang menghalangi", () => {
  const ds = [
    dataset({ id: "tender/pengumuman", nama: "Pengumuman Tender" }),
    dataset({ id: "ekatalog/penyedia-detail", nama: "Penyedia V6", mode: "kode" }),
    dataset({ id: "ekatalog/list-kategori-produk", nama: "Kategori", mode: "kategori" }),
  ];
  const r = p.bangunPermintaan(ds, {
    kodeKLPD: "K10", tahun: [], dataset: ["tender/pengumuman", "ekatalog/penyedia-detail", "ekatalog/list-kategori-produk"], kode: {}, opsi: { "ekatalog/list-kategori-produk": { kd2: "X" } },
  });
  assert.equal(r.masalah.length, 3);
  assert.match(r.masalah[0], /Pengumuman Tender: pilih minimal satu tahun/);
  assert.match(r.masalah[1], /Penyedia V6: isi kode/);
  assert.match(r.masalah[2], /Kategori: kode kategori tingkat 2 harus disertai tingkat 1/);
  assert.equal(r.jumlahTugas, 0);
  // Terlalu banyak kode ditolak.
  const banyak = Array.from({ length: 51 }, (_, i) => `K${i}`).join(" ");
  const r2 = p.bangunPermintaan(ds, { kodeKLPD: "K10", tahun: ["2025"], dataset: ["ekatalog/penyedia-detail"], kode: { "ekatalog/penyedia-detail": banyak }, opsi: {} });
  assert.match(r2.masalah[0], /maksimal 50 kode/);
  // Tanpa pilihan sama sekali: permintaan kosong.
  const r3 = p.bangunPermintaan(ds, { kodeKLPD: "K10", tahun: ["2025"], dataset: [], kode: {}, opsi: {} });
  assert.equal(r3.jumlahTugas, 0);
  assert.deepEqual(r3.permintaan, {});
});

test("pisahKode memisahkan menurut spasi, koma, titik koma, dan baris baru", () => {
  assert.deepEqual(p.pisahKode(" a,b;c\nd  a "), ["a", "b", "c", "d"]);
  assert.deepEqual(p.pisahKode(""), []);
});
test("skalaSumbu: langkah bulat, tanpa label sumbu berulang, batas >= nilai", () => {
  const tick = (maks, bulat) => {
    const { batas, langkah } = p.skalaSumbu(maks, bulat);
    return Array.from({ length: Math.round(batas / langkah) + 1 }, (_, i) => i * langkah);
  };
  assert.deepEqual(tick(2, true), [0, 1, 2]); // jumlah kecil: tidak ada 0,5 / 1,5 yang dibulatkan jadi label ganda
  assert.deepEqual(tick(1, true), [0, 1]);
  assert.deepEqual(tick(7, true), [0, 2, 4, 6, 8]);
  assert.deepEqual(tick(870_000_000, false), [0, 250_000_000, 500_000_000, 750_000_000, 1_000_000_000]);
  assert.deepEqual(tick(0, true), [0, 1]);
  for (const bulat of [true, false]) {
    for (const v of [0.3, 1, 2, 3, 4.2, 9, 11, 37, 123, 4_567_890_123]) {
      const { batas, langkah } = p.skalaSumbu(v, bulat);
      assert.ok(batas >= v, `batas ${batas} < ${v}`);
      const n = Math.round(batas / langkah);
      assert.ok(n >= 1 && n <= 6, `jumlah selang ${n} untuk ${v}`);
      if (bulat) assert.ok(Number.isInteger(langkah), `langkah ${langkah} bukan bilangan bulat untuk ${v}`);
    }
  }
  // Label sumbu jumlah (bilangan bulat) tidak pernah berulang.
  for (const v of [1, 2, 3, 5, 8, 13, 40]) {
    const labels = tick(v, true).map((x) => p.formatAngka(x));
    assert.equal(new Set(labels).size, labels.length, `label berulang untuk ${v}: ${labels}`);
  }
});