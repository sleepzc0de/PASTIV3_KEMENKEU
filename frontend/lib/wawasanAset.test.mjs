import test from "node:test";
import assert from "node:assert/strict";

// Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/wawasanAset.test.mjs
const { wawasanAset } = await import(new URL("./wawasanAset.ts", import.meta.url).href);

const SEKARANG = Date.parse("2026-10-06T00:00:00Z");
const hariLalu = (n) => new Date(SEKARANG - n * 24 * 3600 * 1000).toISOString();
const KUNCI = ["satker", "tanah", "gedung_kantor_utama", "gedung_lainnya", "rusunara", "rumah_negara", "mess_rumah_negara"];

function stat(key, o = {}) {
  return {
    key, label: key, geo: key !== "satker", punya_luas: true, punya_nilai: key !== "rumah_negara",
    jumlah: 100, luas: 1000, nilai: 1_000_000_000, bertitik: 100, tanpa_koordinat: 0, di_luar_indonesia: 0, tanpa_foto: 0, tanpa_kondisi: 0, ...o,
  };
}

function dasar(ubah = {}) {
  const d = {
    tersedia: true,
    sinkron: KUNCI.map((dataset) => ({ dataset, label: dataset, jumlah_baris: 10, terakhir_sukses: hariLalu(2) })),
    satker: { total: 120, induk: 100, anak: 20, kdj: 40, kdo: 5 },
    aset: ["tanah", "gedung_kantor_utama", "gedung_lainnya", "rusunara", "rumah_negara", "mess_rumah_negara"].map((k) => stat(k)),
    hunian: { rusun_kamar_e: 10, rusun_kamar_d: 20 },
    ue1: [
      { kode: "01504", label: "01504 · DJP", satker: 50, kdj: 10, kdo: 1, per: { tanah: { jumlah: 60, luas: 600, nilai: 600e9 } } },
      { kode: "01505", label: "01505 · DJBC", satker: 30, kdj: 10, kdo: 1, per: { tanah: { jumlah: 40, luas: 400, nilai: 400e9 } } },
    ],
    provinsi: [
      { nama: "DKI Jakarta", per: { tanah: { jumlah: 50, luas: 0, nilai: 0 } } },
      { nama: "Jawa Barat", per: { tanah: { jumlah: 30, luas: 0, nilai: 0 } } },
      { nama: "Bali", per: { tanah: { jumlah: 10, luas: 0, nilai: 0 } } },
      { nama: "Papua", per: { tanah: { jumlah: 10, luas: 0, nilai: 0 } } },
    ],
    kondisi: { tanah: [{ k: "Baik", jumlah: 100, nilai: 1e9 }] },
    status_hukum: {},
    asuransi: {},
    status_penghuni: [],
    kelengkapan: { satker_induk: 100, induk_tanpa_kantor_utama: 10, induk_tanpa_tanah: 5 },
  };
  return { ...d, ...ubah };
}

const cari = (w, bagian, kataJudul) => w.find((x) => x.bagian === bagian && (!kataJudul || x.judul.includes(kataJudul)));

test("ikhtisar menyebut jumlah satker, aset, dan nilai", () => {
  const w = wawasanAset(dasar(), SEKARANG);
  const i = cari(w, "ikhtisar");
  assert.equal(i.tingkat, "info");
  assert.match(i.isi, /100 satker induk \(20 anak satker\)/);
  assert.match(i.isi, /600 aset: 100 tanah, 200 gedung, dan 300 hunian/);
  assert.match(i.isi, /Rp 5 miliar/); // lima dataset bernilai x Rp 1 miliar (rumah negara tanpa nilai)
});

test("nilai aset terpusat pada satu UE1 menjadi perhatian, selain itu info", () => {
  let w = wawasanAset(dasar(), SEKARANG); // 60% : 40%
  let s = cari(w, "sebaran", "unit eselon I");
  assert.equal(s.tingkat, "perhatian");
  assert.match(s.judul, /terpusat/);
  assert.match(s.isi, /01504 · DJP memegang 60% nilai aset/);

  w = wawasanAset(dasar({ ue1: [
    { kode: "a", label: "A", satker: 1, kdj: 0, kdo: 0, per: { tanah: { jumlah: 1, luas: 1, nilai: 55 } } },
    { kode: "b", label: "B", satker: 1, kdj: 0, kdo: 0, per: { tanah: { jumlah: 1, luas: 1, nilai: 45 } } },
  ] }), SEKARANG);
  s = cari(w, "sebaran", "unit eselon I");
  assert.equal(s.tingkat, "info");
  assert.doesNotMatch(s.judul, /terpusat/);
});

test("sebaran provinsi: yang terbanyak dan porsi tiga teratas", () => {
  const p = cari(wawasanAset(dasar(), SEKARANG), "sebaran", "provinsi");
  assert.match(p.isi, /4 provinsi/);
  assert.match(p.isi, /DKI Jakarta terbanyak \(50 aset, 50%\)/);
  assert.match(p.isi, /tiga provinsi teratas memuat 90%/);
});

test("kondisi: tingkat menurut bagian rusak berat dari yang terisi", () => {
  const dengan = (baik, ringan, berat) => cari(wawasanAset(dasar({ kondisi: { tanah: [
    { k: "Baik", jumlah: baik, nilai: 0 }, { k: "Rusak Ringan", jumlah: ringan, nilai: 0 }, { k: "Rusak Berat", jumlah: berat, nilai: 5e9 }, { k: null, jumlah: 40, nilai: 0 },
  ] } }), SEKARANG), "kondisi");
  let k = dengan(80, 8, 12); // 12% dari 100 terisi (yang kosong tidak dihitung)
  assert.equal(k.tingkat, "penting");
  assert.match(k.isi, /12 aset \(12%\) berkondisi rusak berat, bernilai tercatat Rp 5 miliar/);
  assert.match(k.isi, /dari 100 aset yang kondisinya terisi/);
  assert.match(k.isi, /Rusak berat terbanyak pada Tanah \(12\)/);

  assert.equal(dengan(90, 6, 4).tingkat, "perhatian"); // 4%
  k = dengan(95, 5, 0);
  assert.equal(k.tingkat, "baik");
  assert.match(k.isi, /Tidak ada aset berkondisi rusak berat; 5 \(5%\) rusak ringan/);
});

test("koordinat: tingkat menurut bagian yang belum valid, dan aset di luar Indonesia disebut bila ada", () => {
  const dengan = (tanpa, luar) => cari(wawasanAset(dasar({ aset: [stat("tanah", { jumlah: 100, bertitik: 100 - tanpa - luar, tanpa_koordinat: tanpa, di_luar_indonesia: luar })] }), SEKARANG), "kelengkapan", "Koordinat");
  let k = dengan(30, 0);
  assert.equal(k.tingkat, "penting");
  assert.match(k.isi, /70 dari 100 aset \(70%\) punya koordinat valid/);
  assert.doesNotMatch(k.isi, /di luar Indonesia/);
  assert.equal(dengan(8, 2).tingkat, "perhatian"); // 10%
  assert.match(dengan(8, 2).isi, /2 berkoordinat di luar Indonesia/);
  k = dengan(1, 0);
  assert.equal(k.tingkat, "baik");
  assert.match(k.judul, /lengkap/);
});

test("foto dan asuransi gedung", () => {
  const w = wawasanAset(dasar({
    aset: [stat("tanah", { jumlah: 100, tanpa_foto: 40 })],
    asuransi: { gedung_kantor_utama: [{ k: "Y", jumlah: 90, nilai: 0 }], gedung_lainnya: [{ k: "T", jumlah: 5, nilai: 0 }, { k: null, jumlah: 5, nilai: 0 }] },
  }), SEKARANG);
  assert.equal(cari(w, "kelengkapan", "Foto").tingkat, "perhatian"); // 40%
  const a = cari(w, "kelengkapan", "asuransi");
  assert.equal(a.tingkat, "baik"); // 90 dari 100
  assert.match(a.isi, /90 dari 100 gedung \(90%\) tercatat diasuransikan; 5 \(5%\) belum/);

  const rendah = cari(wawasanAset(dasar({ asuransi: { gedung_kantor_utama: [{ k: "Y", jumlah: 10, nilai: 0 }, { k: "T", jumlah: 90, nilai: 0 }] } }), SEKARANG), "kelengkapan", "asuransi");
  assert.equal(rendah.tingkat, "perhatian"); // 10% di bawah 40%
});

test("kelengkapan satker induk dan status hukum tanah", () => {
  let w = wawasanAset(dasar({ kelengkapan: { satker_induk: 100, induk_tanpa_kantor_utama: 45, induk_tanpa_tanah: 2 } }), SEKARANG);
  let k = cari(w, "kelengkapan", "satker induk");
  assert.equal(k.tingkat, "perhatian");
  assert.match(k.isi, /45 dari 100 satker induk \(45%\) belum punya data gedung kantor utama/);
  assert.equal(cari(wawasanAset(dasar(), SEKARANG), "kelengkapan", "satker induk").tingkat, "info");

  w = wawasanAset(dasar({ status_hukum: { tanah: [{ k: "Sertipikat HP", jumlah: 70, nilai: 0 }, { k: null, jumlah: 30, nilai: 0 }] } }), SEKARANG);
  const h = cari(w, "kelengkapan", "Status hukum");
  assert.equal(h.tingkat, "perhatian");
  assert.match(h.isi, /30 dari 100 tanah \(30%\)/);
  assert.equal(cari(wawasanAset(dasar({ status_hukum: { tanah: [{ k: "Sertipikat HP", jumlah: 95, nilai: 0 }, { k: null, jumlah: 5, nilai: 0 }] } }), SEKARANG), "kelengkapan", "Status hukum"), undefined);
});

test("kesegaran data: belum pernah, sebagian, lama, dan segar", () => {
  const sinkron = (fn) => KUNCI.map((dataset, i) => ({ dataset, label: dataset, jumlah_baris: 1, terakhir_sukses: fn(i) }));
  let w = wawasanAset(dasar({ sinkron: sinkron(() => null) }), SEKARANG);
  assert.equal(cari(w, "data").tingkat, "penting");
  assert.match(cari(w, "data").judul, /belum pernah disinkronkan/);

  w = wawasanAset(dasar({ sinkron: sinkron((i) => (i < 2 ? null : hariLalu(1))) }), SEKARANG);
  assert.equal(cari(w, "data").tingkat, "perhatian");
  assert.match(cari(w, "data").isi, /2 dari 7 dataset \(Satuan Kerja, Tanah\) belum pernah disalin/);

  w = wawasanAset(dasar({ sinkron: sinkron((i) => hariLalu(i === 3 ? 20 : 1)) }), SEKARANG);
  assert.equal(cari(w, "data").tingkat, "perhatian");
  assert.match(cari(w, "data").isi, /Gedung Lainnya\) terakhir disalin 20 hari lalu/);

  w = wawasanAset(dasar(), SEKARANG);
  assert.equal(cari(w, "data").tingkat, "baik");
  assert.match(cari(w, "data").isi, /dalam 2 hari terakhir/);
});

test("urutan: penting, perhatian, info, lalu baik; urutan semula dipertahankan untuk tingkat sama", () => {
  const w = wawasanAset(dasar({ kondisi: { tanah: [{ k: "Baik", jumlah: 80, nilai: 0 }, { k: "Rusak Berat", jumlah: 20, nilai: 0 }] } }), SEKARANG);
  const rank = { penting: 0, perhatian: 1, info: 2, baik: 3 };
  for (let i = 1; i < w.length; i++) assert.ok(rank[w[i - 1].tingkat] <= rank[w[i].tingkat], `urutan salah di ${i}: ${w.map((x) => x.tingkat)}`);
  assert.equal(w[0].tingkat, "penting");
  assert.equal(w[0].bagian, "kondisi");
  assert.equal(w.at(-1).tingkat, "baik");
});

test("data kosong atau tidak lengkap tidak melempar galat dan tidak memuat NaN, Infinity, atau undefined", () => {
  const kosong = dasar({
    sinkron: [], satker: { total: 0, induk: 0, anak: 0, kdj: 0, kdo: 0 }, aset: [], hunian: {}, ue1: [], provinsi: [], kondisi: {}, status_hukum: {}, asuransi: {},
    status_penghuni: [], kelengkapan: { satker_induk: 0, induk_tanpa_kantor_utama: 0, induk_tanpa_tanah: 0 },
  });
  assert.deepEqual(wawasanAset(kosong, SEKARANG), []);

  const nol = dasar({ aset: [stat("tanah", { jumlah: 0, luas: 0, nilai: 0, bertitik: 0 })], ue1: [], provinsi: [] });
  for (const d of [dasar(), nol, dasar({ kondisi: undefined === 1 ? {} : { tanah: [] } })]) {
    for (const w of wawasanAset(d, SEKARANG)) {
      assert.doesNotMatch(`${w.judul} ${w.isi}`, /NaN|Infinity|undefined|null/, `${w.judul}: ${w.isi}`);
      assert.ok(w.judul && w.isi);
    }
  }
});
