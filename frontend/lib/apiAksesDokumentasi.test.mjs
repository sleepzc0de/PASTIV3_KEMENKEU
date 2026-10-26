import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

// Tes fungsi murni tab Dokumentasi API Akses Data. Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/apiAksesDokumentasi.test.mjs
const D = await import(new URL("./apiAksesDokumentasi.ts", import.meta.url).href);
const F = await import(new URL("./apiAksesFormat.ts", import.meta.url).href);

const PARAMETER = [
  { nama: "per_halaman", uraian: "x" },
  { nama: "kursor", uraian: "x" },
  { nama: "hitung", uraian: "x" },
  { nama: "q", uraian: "x" },
  { nama: "kode_satker", uraian: "x" },
  { nama: "kode_ue1", uraian: "x" },
  { nama: "provinsi", uraian: "x" },
  { nama: "kab_kota", uraian: "x" },
  { nama: "kode_klpd", uraian: "x" },
  { nama: "tahun", uraian: "x" },
];

const aset = {
  kode: "aset.tanah",
  label: "Tanah",
  deskripsi: "Aset tanah milik negara",
  kelompok: "aset",
  alamat: "/api/data/v1/aset/tanah",
  geo: true,
  penyaring: ["q", "kode_satker", "kode_ue1", "provinsi", "kab_kota"],
  kolom: [
    { nama: "id", tipe: "bilangan" },
    { nama: "kode_satker", tipe: "teks" },
    { nama: "luas_tanah", tipe: "desimal" },
  ],
};

const tender = {
  kode: "pengadaan.tender_pengumuman",
  label: "Pengumuman tender",
  deskripsi: "Pengumuman tender Inaproc",
  kelompok: "pengadaan",
  subkelompok: "Tender",
  alamat: "/api/data/v1/pengadaan/tender_pengumuman",
  geo: false,
  penyaring: ["q", "kode_satker", "kode_ue1", "kode_klpd", "tahun"],
  kolom: [
    { nama: "id", tipe: "teks" },
    { nama: "nama_paket", tipe: "teks" },
    { nama: "tgl_pengumuman", tipe: "waktu" },
  ],
  tertutup: ["nama_ppk"],
};

const instansi = {
  kode: "pengadaan.ekatalog_instansi_satker",
  label: "Instansi dan satker e-Katalog",
  deskripsi: "Referensi instansi",
  kelompok: "pengadaan",
  subkelompok: "E-Katalog",
  alamat: "/api/data/v1/pengadaan/ekatalog_instansi_satker",
  geo: false,
  penyaring: ["kode_satker", "kode_ue1"],
  kolom: [{ nama: "id", tipe: "teks" }],
};

const AKAR = "https://pasti.contoh.go.id";
const opsi = (ds, ekstra = {}) => ({ akar: AKAR, ds, tahun: 2025, halamanBawaan: 100, halamanMaks: 500, ...ekstra });

test("alamat dasar: akar dari env, kalau tidak dari alamat halaman, tanpa garis miring ganda", () => {
  assert.equal(D.alamatDasarData("https://pasti.kemenkeu.go.id/", "http://localhost:3000"), "https://pasti.kemenkeu.go.id/api/data/v1");
  assert.equal(D.alamatDasarData("", "http://localhost:3000"), "http://localhost:3000/api/data/v1");
  assert.equal(D.alamatDasarData(undefined, undefined), "/api/data/v1");
  assert.equal(D.alamatDasarData("https://a.id///", "", "/x"), "https://a.id/x");
  assert.equal(D.akarApi("  https://a.id/ ", ""), "https://a.id");
});

test("penyaringDataset memisahkan penyaring yang didukung dan yang ditolak 400", () => {
  const a = D.penyaringDataset(aset, PARAMETER);
  assert.deepEqual(a.didukung, ["q", "kode_satker", "kode_ue1", "provinsi", "kab_kota"]);
  assert.deepEqual(a.ditolak, ["kode_klpd", "tahun"]);
  const t = D.penyaringDataset(tender, PARAMETER);
  assert.deepEqual(t.ditolak, ["provinsi", "kab_kota"]);
  const i = D.penyaringDataset(instansi, PARAMETER);
  assert.deepEqual(i.didukung, ["kode_satker", "kode_ue1"]);
  assert.deepEqual(i.ditolak, ["q", "provinsi", "kab_kota", "kode_klpd", "tahun"]);
  // parameter umum tidak pernah dianggap penyaring
  for (const p of ["per_halaman", "kursor", "hitung"]) {
    assert.ok(!a.didukung.includes(p) && !a.ditolak.includes(p));
  }
  // penyaring dataset yang belum diuraikan backend tetap ditampilkan, bukan dibuang
  assert.deepEqual(D.penyaringDataset({ penyaring: ["baru"] }, PARAMETER).didukung, ["baru"]);
});

test("cariDataset: semua kata harus cocok, tidak peka huruf, termasuk nama kolom", () => {
  const semua = [aset, tender, instansi];
  assert.equal(D.cariDataset(semua, "").length, 3);
  assert.equal(D.cariDataset(semua, "   ").length, 3);
  assert.deepEqual(D.cariDataset(semua, "TENDER").map((d) => d.kode), ["pengadaan.tender_pengumuman"]);
  assert.deepEqual(D.cariDataset(semua, "luas_tanah").map((d) => d.kode), ["aset.tanah"]);
  assert.deepEqual(D.cariDataset(semua, "katalog instansi").map((d) => d.kode), ["pengadaan.ekatalog_instansi_satker"]);
  assert.deepEqual(D.cariDataset(semua, "tender tanah"), []);
  assert.deepEqual(D.cariDataset(semua, "e-katalog").map((d) => d.kode), ["pengadaan.ekatalog_instansi_satker"]);
});

test("tipeJson menerjemahkan tipe katalog ke istilah JSON", () => {
  assert.equal(D.tipeJson("teks"), "string");
  assert.equal(D.tipeJson("bilangan"), "integer");
  assert.equal(D.tipeJson("desimal"), "number");
  assert.equal(D.tipeJson("waktu"), "string (ISO 8601)");
  assert.equal(D.tipeJson("logika"), "boolean");
  assert.equal(D.tipeJson("lain"), "lain");
});

test("contohParameter memilih penyaring yang berlaku pada dataset", () => {
  assert.deepEqual(D.contohParameter(aset, 2025), [["kode_ue1", "01504"]]);
  assert.deepEqual(D.contohParameter(tender, 2025), [["kode_klpd", "K10"], ["tahun", "2025"]]);
  assert.deepEqual(D.contohParameter(instansi, 2025), [["kode_ue1", "01504"]]);
  assert.deepEqual(D.contohParameter({ penyaring: ["q"] }, 2025), []);
});

test("urlDataset menyusun alamat dengan kueri yang di-encode", () => {
  assert.equal(D.urlDataset(AKAR, aset), `${AKAR}/api/data/v1/aset/tanah`);
  assert.equal(D.urlDataset(AKAR, aset, [["kode_ue1", "01504"], ["per_halaman", "100"]]), `${AKAR}/api/data/v1/aset/tanah?kode_ue1=01504&per_halaman=100`);
  assert.equal(D.urlDataset(AKAR, aset, [["q", "a b&c"]]), `${AKAR}/api/data/v1/aset/tanah?q=a%20b%26c`);
});

const SEMUA_KOMBINASI = [];
for (const b of ["curl", "python", "node"]) for (const s of ["satu", "semua"]) for (const ds of [aset, tender, instansi, { ...aset, penyaring: ["q"] }]) SEMUA_KOMBINASI.push([b, s, ds]);

test("contohKode: setiap contoh memuat alamat dataset, membaca kunci dari lingkungan, dan tidak membocorkan kunci", () => {
  for (const [b, s, ds] of SEMUA_KOMBINASI) {
    const kode = D.contohKode(b, s, opsi(ds));
    assert.ok(kode.includes(`${AKAR}${ds.alamat}`), `${b}/${s}: alamat tidak ada`);
    assert.ok(kode.includes("PASTI_API_KEY"), `${b}/${s}: kunci tidak dari lingkungan`);
    assert.ok(!/pasti_[a-z2-7]{10}_/.test(kode), `${b}/${s}: ada bentuk kunci`);
    assert.ok(!/[?&]api_?key=/i.test(kode), `${b}/${s}: kunci di alamat`);
    assert.ok(!kode.includes("undefined") && !kode.includes("NaN"), `${b}/${s}: nilai kosong`);
  }
});

test("contohKode: satu halaman memakai per_halaman bawaan, tarik semua memakai maksimal dan kursor", () => {
  for (const b of ["curl", "python", "node"]) {
    const satu = D.contohKode(b, "satu", opsi(aset, { halamanBawaan: 100, halamanMaks: 250 }));
    assert.match(satu, /per_halaman[^0-9]{1,5}100\b/, `${b}: satu halaman`);
    assert.ok(!satu.includes("kursor"), `${b}: satu halaman tidak perlu kursor`);
    const semua = D.contohKode(b, "semua", opsi(aset, { halamanBawaan: 100, halamanMaks: 250 }));
    assert.match(semua, /per_halaman[^0-9]{1,5}250\b/, `${b}: semua`);
    assert.ok(semua.includes("kursor"), `${b}: semua harus memakai kursor`);
    assert.ok(semua.includes("ada_berikutnya"), `${b}: semua harus berhenti pada ada_berikutnya`);
  }
  // 429 ditangani pada contoh yang mengulang sendiri
  for (const b of ["python", "node"]) assert.ok(D.contohKode(b, "semua", opsi(aset)).includes("Retry-After"), b);
});

test("contohKode: penyaring contoh mengikuti dataset", () => {
  assert.ok(D.contohKode("curl", "satu", opsi(tender)).includes("kode_klpd=K10&tahun=2025&per_halaman=100"));
  assert.ok(D.contohKode("python", "semua", opsi(tender)).includes('tarik(kode_klpd="K10", tahun="2025")'));
  assert.ok(D.contohKode("node", "semua", opsi(tender)).includes('tarik({ kode_klpd: "K10", tahun: "2025" })'));
  assert.ok(D.contohKode("python", "semua", opsi(aset)).includes('tarik(kode_ue1="01504")'));
  // tanpa penyaring contoh: panggilan kosong yang sah
  const polos = { ...aset, penyaring: ["q"] };
  assert.ok(D.contohKode("python", "semua", opsi(polos)).includes("tarik():") || D.contohKode("python", "semua", opsi(polos)).includes("tarik()"));
  assert.ok(D.contohKode("node", "semua", opsi(polos)).includes("tarik({})"));
});

// Contoh Node harus lolos pemeriksaan sintaks sungguhan (berkas .mjs, dengan await tingkat atas dan for await).
test("contohKode: semua contoh Node lolos node --check", () => {
  const dir = mkdtempSync(join(tmpdir(), "dok-api-"));
  try {
    let n = 0;
    for (const [b, s, ds] of SEMUA_KOMBINASI) {
      if (b !== "node") continue;
      const berkas = join(dir, `contoh${n++}.mjs`);
      writeFileSync(berkas, D.contohKode(b, s, opsi(ds)));
      const r = spawnSync(process.execPath, ["--check", berkas], { encoding: "utf8" });
      assert.equal(r.status, 0, `${s}/${ds.kode}: ${r.stderr}`);
    }
    assert.ok(n >= 8);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

// Python hanya diperiksa bila terpasang (alias Microsoft Store tanpa Python tidak dihitung).
const adaPython = (() => {
  const r = spawnSync("python", ["-c", "print(40+2)"], { encoding: "utf8" });
  return r.status === 0 && r.stdout.trim() === "42";
})();

test("contohKode: semua contoh Python lolos pemeriksaan sintaks", { skip: !adaPython && "Python tidak terpasang" }, () => {
  for (const [b, s, ds] of SEMUA_KOMBINASI) {
    if (b !== "python") continue;
    const r = spawnSync("python", ["-c", "import ast,sys; ast.parse(sys.stdin.read())"], { input: D.contohKode(b, s, opsi(ds)), encoding: "utf8" });
    assert.equal(r.status, 0, `${s}/${ds.kode}: ${r.stderr}`);
  }
});

test("kelompokkanDataset menjaga bidang tambahan dataset dokumentasi", () => {
  const g = F.kelompokkanDataset([tender, aset, instansi]);
  assert.deepEqual(g.map((x) => x.kelompok), ["aset", "pengadaan"]);
  assert.deepEqual(g[1].bagian.map((b) => b.judul), ["Tender", "E-Katalog"]);
  assert.deepEqual(g[1].bagian[0].dataset[0].kolom.map((k) => k.nama), ["id", "nama_paket", "tgl_pengumuman"]);
  assert.deepEqual(g[1].bagian[0].dataset[0].tertutup, ["nama_ppk"]);
});
