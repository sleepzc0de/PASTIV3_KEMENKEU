import test from "node:test";
import assert from "node:assert/strict";

// Tes fungsi murni fitur API Akses Data. Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/apiAkses.test.mjs
const A = await import(new URL("./apiAksesFormat.ts", import.meta.url).href);

const formSah = () => ({
  nama_aplikasi: "Aplikasi Contoh",
  instansi: "Direktorat Contoh",
  unit: "",
  pic_nama: "Budi Santoso",
  pic_jabatan: "",
  pic_email: "budi@contoh.go.id",
  pic_telepon: "",
  tujuan: "Menampilkan jumlah dan sebaran aset tanah pada dasbor internal.",
  dataset_diminta: ["aset.tanah"],
});

test("teksPeriode memilih satuan terbesar tanpa pecahan", () => {
  assert.equal(A.teksPeriode(60), "60 detik");
  assert.equal(A.teksPeriode(90), "90 detik");
  assert.equal(A.teksPeriode(120), "2 menit");
  assert.equal(A.teksPeriode(3600), "1 jam");
  assert.equal(A.teksPeriode(7200), "2 jam");
  assert.equal(A.teksPeriode(86400), "1 hari");
  assert.equal(A.teksPeriode(5400), "90 menit");
  assert.equal(A.teksPeriode(0), "-");
});

test("teksBatas menampilkan bawaan 1000 per 60 detik dan kuota 5000 per jam", () => {
  const t = A.teksBatas({ limit_permintaan: 1000, limit_jendela_detik: 60, kuota_permintaan: 5000, kuota_periode_detik: 3600 });
  assert.match(t, /1\.000 permintaan \/ 60 detik/);
  assert.match(t, /kuota 5\.000 \/ 1 jam/);
  assert.equal(A.teksBatas(undefined), "-");
});

test("teksOverride kosong bila ikut bawaan", () => {
  assert.equal(A.teksOverride({ limit_permintaan: null, limit_jendela_detik: null, kuota_permintaan: null, kuota_periode_detik: null }), "");
  assert.match(A.teksOverride({ limit_permintaan: 5000, limit_jendela_detik: null, kuota_permintaan: null, kuota_periode_detik: 86400 }), /laju 5\.000 \/ bawaan · kuota bawaan \/ 1 hari/);
});

test("tingkatKuota", () => {
  assert.equal(A.tingkatKuota(0), "aman");
  assert.equal(A.tingkatKuota(79.9), "aman");
  assert.equal(A.tingkatKuota(80), "waspada");
  assert.equal(A.tingkatKuota(100), "kritis");
  assert.equal(A.tingkatKuota(130), "kritis");
});

test("kelas dan label hasil", () => {
  assert.equal(A.kelasHasilAkses("ok"), "baik");
  assert.equal(A.kelasHasilAkses("batas_laju"), "peringatan");
  assert.equal(A.kelasHasilAkses("tidak_sah"), "peringatan");
  assert.equal(A.kelasHasilAkses("galat_server"), "buruk");
  assert.equal(A.labelHasil("kuota_habis"), "Kuota habis (429)");
  assert.equal(A.labelHasil("tidak-dikenal"), "tidak-dikenal");
});

test("pecahDaftar: koma, spasi, baris baru; tanpa kosong dan duplikat", () => {
  assert.deepEqual(A.pecahDaftar("10.0.0.1, 10.0.0.2\n10.0.0.1;  "), ["10.0.0.1", "10.0.0.2"]);
  assert.deepEqual(A.pecahDaftar("   "), []);
});

test("galatUE1 dan galatIP", () => {
  assert.equal(A.galatUE1("01504, 01505"), "");
  assert.equal(A.galatUE1(""), "");
  assert.notEqual(A.galatUE1("1504"), "");
  assert.notEqual(A.galatUE1("01504 abc"), "");
  assert.equal(A.galatIP("10.1.2.0/24, 203.0.113.9, 2001:db8::/32"), "");
  assert.notEqual(A.galatIP("300.1.1.1"), "");
  assert.notEqual(A.galatIP("10.1.2.3/33"), "");
  assert.notEqual(A.galatIP("0.0.0.0/0"), "");
  assert.notEqual(A.galatIP("bukan-ip"), "");
});

test("validasiPendaftaran menerima formulir sah dan menolak isian buruk", () => {
  assert.deepEqual(A.validasiPendaftaran(formSah()), {});
  const kosong = A.validasiPendaftaran({ ...formSah(), nama_aplikasi: " ", instansi: "", pic_nama: "", pic_email: "", tujuan: "", dataset_diminta: [] });
  for (const k of ["nama_aplikasi", "instansi", "pic_nama", "pic_email", "tujuan", "dataset_diminta"]) assert.ok(kosong[k], `kolom ${k} harus bergalat`);
  assert.ok(A.validasiPendaftaran({ ...formSah(), pic_email: "bukan-email" }).pic_email);
  assert.ok(A.validasiPendaftaran({ ...formSah(), pic_telepon: "abc" }).pic_telepon);
  assert.equal(A.validasiPendaftaran({ ...formSah(), pic_telepon: "+62 21 555-1234" }).pic_telepon, undefined);
  assert.ok(A.validasiPendaftaran({ ...formSah(), tujuan: "singkat" }).tujuan);
  assert.ok(A.validasiPendaftaran({ ...formSah(), tujuan: "x".repeat(2001) }).tujuan);
  assert.ok(A.validasiPendaftaran({ ...formSah(), nama_aplikasi: "a".repeat(151) }).nama_aplikasi);
});

test("kueriPantau: rentang bernama atau tanggal kustom, penyaring kosong tidak dikirim", () => {
  assert.deepEqual(A.kueriPantau(A.FILTER_PANTAU_BAWAAN), { rentang: "24j" });
  assert.deepEqual(A.kueriPantau({ ...A.FILTER_PANTAU_BAWAAN, rentang: "7h", klien_id: "12", endpoint: "/aset/:dataset" }), { rentang: "7h", klien_id: "12", endpoint: "/aset/:dataset" });
  assert.deepEqual(A.kueriPantau({ ...A.FILTER_PANTAU_BAWAAN, rentang: "kustom", dari: "2026-10-01", sampai: "2026-10-05" }), { dari: "2026-10-01", sampai: "2026-10-05" });
  assert.deepEqual(A.kueriPantau({ ...A.FILTER_PANTAU_BAWAAN, rentang: "kustom", dari: "salah", sampai: "" }), {});
  assert.deepEqual(A.kueriPantau({ ...A.FILTER_PANTAU_BAWAAN, klien_id: "abc" }), { rentang: "24j" }); // bukan angka tidak dikirim
});

test("galatRentangPantau", () => {
  assert.equal(A.galatRentangPantau({ rentang: "24j", dari: "", sampai: "" }), "");
  assert.notEqual(A.galatRentangPantau({ rentang: "kustom", dari: "", sampai: "" }), "");
  assert.equal(A.galatRentangPantau({ rentang: "kustom", dari: "2026-10-01", sampai: "" }), "");
  assert.notEqual(A.galatRentangPantau({ rentang: "kustom", dari: "2026-10-05", sampai: "2026-10-01" }), "");
});

test("angkaOpsional: kosong = bawaan, bilangan bulat positif saja", () => {
  assert.equal(A.angkaOpsional(""), null);
  assert.equal(A.angkaOpsional("  "), null);
  assert.equal(A.angkaOpsional("5000"), 5000);
  assert.equal(A.angkaOpsional("0"), undefined);
  assert.equal(A.angkaOpsional("-5"), undefined);
  assert.equal(A.angkaOpsional("1.5"), undefined);
  assert.equal(A.angkaOpsional("abc"), undefined);
});

test("jumlahGalat dan jumlahDibatasi", () => {
  assert.equal(A.jumlahGalat({ galat_klien: 2, galat_server: 1, ditolak: 4 }), 7);
  assert.equal(A.jumlahDibatasi({ batas_laju: 3, kuota_habis: 2 }), 5);
  assert.equal(A.teksPersen(1, 4), "25,0%");
  assert.equal(A.teksPersen(1, 0), "0,0%");
});

test("contohCurl memuat kunci dan alamat", () => {
  const c = A.contohCurl("https://pasti.example/api/data/v1", "pasti_abc_xyz");
  assert.match(c, /Authorization: Bearer pasti_abc_xyz/);
  assert.match(c, /\/api\/data\/v1\/aset\/tanah/);
});

const klienContoh = () => ({
  dataset_diminta: ["aset.tanah", "aset.satker"],
  dataset_diberikan: ["aset.tanah"],
  ip_diizinkan: ["10.0.0.0/8"],
  ue1_diizinkan: ["01504"],
  limit_permintaan: 2000,
  limit_jendela_detik: null,
  kuota_permintaan: null,
  kuota_periode_detik: null,
  catatan: "catatan lama",
});

test("nilaiAwalAkses: usulan dari dataset diminta untuk yang menunggu, yang diberikan untuk yang disetujui", () => {
  const a = A.nilaiAwalAkses(klienContoh(), true);
  assert.deepEqual(a.dataset, ["aset.tanah", "aset.satker"]);
  assert.equal(a.catatan, "");
  const b = A.nilaiAwalAkses(klienContoh(), false);
  assert.deepEqual(b.dataset, ["aset.tanah"]);
  assert.equal(b.ip, "10.0.0.0/8");
  assert.equal(b.limit, "2000");
  assert.equal(b.jendela, "");
  assert.equal(b.catatan, "catatan lama");
});

test("susunBodyAkses: menyusun badan sah dan menolak isian buruk", () => {
  const nilai = { ...A.nilaiAwalAkses(klienContoh(), false), ip: "10.0.0.0/8\n203.0.113.9", ue1: "01504 01505", kuota: "9000", periode: "7200" };
  const r = A.susunBodyAkses(nilai);
  assert.deepEqual(r.galat, {});
  assert.deepEqual(r.body, {
    dataset_diberikan: ["aset.tanah"],
    ip_diizinkan: "10.0.0.0/8, 203.0.113.9",
    ue1_diizinkan: "01504, 01505",
    limit_permintaan: 2000,
    limit_jendela_detik: null,
    kuota_permintaan: 9000,
    kuota_periode_detik: 7200,
    catatan: "catatan lama",
  });
  const buruk = A.susunBodyAkses({ ...nilai, dataset: [], ip: "x", ue1: "1", limit: "0", jendela: "abc" });
  assert.equal(buruk.body, undefined);
  for (const k of ["dataset", "ip", "ue1", "limit", "jendela"]) assert.ok(buruk.galat[k], `kolom ${k}`);
});
