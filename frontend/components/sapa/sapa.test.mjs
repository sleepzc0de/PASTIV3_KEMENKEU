import test from "node:test";
import assert from "node:assert/strict";

// Tes fungsi murni halaman SAPA. Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test components/sapa/sapa.test.mjs
const H = await import(new URL("./sapa.ts", import.meta.url).href);

test("parseUang meniru ParseUang backend", () => {
  const sen = (s) => H.parseUang(s)?.toString() ?? null;
  assert.equal(sen("1500000"), "150000000");
  assert.equal(sen("1500000.50"), "150000050");
  assert.equal(sen("1.500.000,50"), "150000050");
  assert.equal(sen("Rp1.500.000"), "150000000");
  assert.equal(sen("rp 2 500"), "250000");
  assert.equal(sen("1.500"), "150000"); // pemisah ribuan
  assert.equal(sen("1500.5"), "150050");
  assert.equal(sen("12.50"), "1250"); // dua angka di belakang titik = desimal
  assert.equal(sen("0"), "0");
  assert.equal(sen(".5"), "50");
  assert.equal(sen(""), null);
  assert.equal(sen("  "), null);
  assert.equal(sen("abc"), null);
  assert.equal(sen("1.5.5.5x"), null);
  assert.equal(sen("1,234"), null); // tiga angka di belakang koma
  assert.equal(sen("-5"), null);
  assert.equal(sen("12345678901234567"), null); // 17 digit
  assert.equal(sen("8999999999999999.99"), "899999999999999999"); // sen melewati 2^53: BigInt menjaga ketepatan
  assert.equal(sen("9007199254740993"), null); // di atas batas backend
  assert.equal(sen("9000000000000000"), "900000000000000000"); // tepat batas
  assert.equal(sen("9000000000000000.01"), null); // lewat batas
});

test("formatRupiah", () => {
  assert.equal(H.formatRupiah(150000050n), "Rp1.500.000,50");
  assert.equal(H.formatRupiah(0n), "Rp0,00");
  assert.equal(H.formatRupiah(5n), "Rp0,05");
  assert.equal(H.formatRupiah(100000n), "Rp1.000,00");
  assert.equal(H.formatRupiah(123456789012n), "Rp1.234.567.890,12");
});

test("totalBarang", () => {
  const b = (p, l) => ({ ...H.kosongBarang(), nama: "x", nilai_perolehan: p, nilai_limit: l });
  const t = H.totalBarang([b("1000000000", "900000000"), b("2500000000.50", "1500000000"), b("oops", "10"), b("50000000", "")]);
  assert.equal(t.jumlah, 4);
  assert.equal(t.perolehan, (100000000000n + 250000000050n + 5000000000n));
  assert.equal(t.limit, (90000000000n + 150000000000n + 1000n));
  assert.equal(t.tidakSah, 2);
  assert.equal(H.totalBarang([]).jumlah, 0);
});

test("formatTanggal tidak bergeser zona waktu", () => {
  assert.equal(H.formatTanggal("2026-10-03"), "3 Oktober 2026");
  assert.equal(H.formatTanggal("2026-01-01"), "1 Januari 2026");
  assert.equal(H.formatTanggal("2026-12-31T23:59:59Z"), "31 Desember 2026");
  assert.equal(H.formatTanggal(""), "-");
  assert.equal(H.formatTanggal(undefined), "-");
  assert.equal(H.formatTanggal("2026-13-01"), "2026-13-01");
  assert.equal(H.formatTanggal("bukan tanggal"), "bukan tanggal");
});

test("formatUkuran", () => {
  assert.equal(H.formatUkuran(512), "512 B");
  assert.equal(H.formatUkuran(1536), "1,5 KB");
  assert.equal(H.formatUkuran(5 * 1024 * 1024), "5,0 MB");
  assert.equal(H.formatUkuran(-1), "-");
  assert.equal(H.formatUkuran(NaN), "-");
});

test("bersihKodeSatker", () => {
  assert.equal(H.bersihKodeSatker("015010199409294002KP"), "015010199409294002");
  assert.equal(H.bersihKodeSatker(" 0150 1019 9409 2940 02 "), "015010199409294002");
  assert.equal(H.bersihKodeSatker("01501019940929400299999"), "015010199409294002");
  assert.equal(H.bersihKodeSatker("abc"), "");
});

test("errorInfo", () => {
  const ax = (status, data) => ({ isAxiosError: true, response: { status, data } });
  assert.deepEqual(H.errorInfo(ax(400, { message: "Data belum lengkap", errors: ["a", "b"] }), "x"), { message: "Data belum lengkap", errors: ["a", "b"] });
  assert.deepEqual(H.errorInfo(ax(400, { message: "Nilai salah", errors: ["Nilai salah"] }), "x"), { message: "Nilai salah", errors: [] });
  assert.deepEqual(H.errorInfo(ax(500, {}), "Gagal"), { message: "Gagal", errors: [] });
  assert.deepEqual(H.errorInfo(ax(500, { message: 5, errors: "x" }), "Gagal"), { message: "Gagal", errors: [] });
  assert.deepEqual(H.errorInfo({ isAxiosError: true, code: "ECONNABORTED" }, "x").message, "Permintaan terlalu lama dan dihentikan");
  assert.equal(H.errorInfo(new Error("boom"), "x").message, "Tidak dapat terhubung ke server");
  assert.equal(H.errorInfo(null, "x").message, "Tidak dapat terhubung ke server");
  assert.equal(H.errorStatus(ax(409, {})), 409);
  assert.equal(H.errorStatus(new Error("x")), null);
});

test("namaDariDisposition", () => {
  assert.equal(H.namaDariDisposition("attachment; filename=\"ND Usulan.docx\"", "f.docx"), "ND Usulan.docx");
  assert.equal(H.namaDariDisposition("attachment; filename*=utf-8''Templat%20ND%20%E2%80%93%20Satker.docx", "f.docx"), "Templat ND – Satker.docx");
  assert.equal(H.namaDariDisposition("attachment; filename=plain.docx", "f.docx"), "plain.docx");
  assert.equal(H.namaDariDisposition("", "f.docx"), "f.docx");
  assert.equal(H.namaDariDisposition("attachment; filename*=utf-8''%E0%A4%A", "f.docx"), "f.docx"); // rusak -> cadangan
});

const items = [
  { kunci: "daftar_barang", label: "Daftar", otomatis: true },
  { kunci: "ba", label: "BA", otomatis: false },
  { kunci: "psp", label: "PSP", otomatis: false },
];

test("normalisasi menangani null dan bidang hilang dari backend", () => {
  const tim = H.normTim({ jabatan_pimpinan: "Kepala", anggota: null });
  assert.equal(tim.anggota.length, 1); // satu baris kosong siap diisi
  assert.equal(tim.kota, "");
  assert.equal(H.normTim(undefined).anggota.length, 1);
  assert.equal(H.normTim({ anggota: [{ nama: "A", jabatan: "B" }] }).anggota[0].kedudukan, "");

  const nd = H.normNDSatker({ barang: null, dokumen: null, sudah_rp4: "ya" }, items);
  assert.equal(nd.sudah_rp4, false);
  assert.deepEqual(nd.barang, []);
  assert.deepEqual(nd.dokumen.daftar_barang, { ada: true, nomor: "", tanggal: "" }); // otomatis = ada
  assert.deepEqual(nd.dokumen.ba, { ada: false, nomor: "", tanggal: "" });

  const nd2 = H.normNDSatker({ dokumen: { ba: { ada: true, nomor: "BA-1", tanggal: "2026-09-01" } }, barang: [{ nama: "Tanah" }] }, items);
  assert.equal(nd2.dokumen.ba.nomor, "BA-1");
  assert.equal(nd2.barang[0].nup, "");
  assert.equal(nd2.penandatangan.nip, "");

  assert.equal(H.normNDUE1(null).penandatangan.nama, "");
  assert.equal(H.normBA(null).bentuk, "");
});

test("muatanNDSatker tidak mengirim dokumen otomatis dan membersihkan yang tidak ada", () => {
  const nd = H.normNDSatker({}, items);
  nd.dokumen.ba = { ada: true, nomor: " BA-1 ", tanggal: "2026-09-01" };
  nd.dokumen.psp = { ada: false, nomor: "sisa", tanggal: "2020-01-01" };
  const m = H.muatanNDSatker(nd, items);
  assert.deepEqual(Object.keys(m.dokumen).sort(), ["ba", "psp"]);
  assert.deepEqual(m.dokumen.ba, { ada: true, nomor: "BA-1", tanggal: "2026-09-01" });
  assert.deepEqual(m.dokumen.psp, { ada: false, nomor: "", tanggal: "" });
});

test("parseBarangTempel", () => {
  const tsv = [
    "Nama\tKode\tNUP\tLokasi\tKondisi\tTahun\tNilai Perolehan\tNilai Limit\tKeterangan",
    "Tanah Kantor\t2010101001\t1\tJl. Merdeka 1\tBaik\t2001\t1.000.000.000\t900.000.000\t-",
    "",
    "Gedung\t4010101001\t2\tJl. Merdeka\tRusak Berat\t1999\t2500000000.50\t1500000000",
    "Pos Jaga\t4010101002",
  ].join("\r\n");
  const r = H.parseBarangTempel(tsv);
  assert.equal(r.barang.length, 3);
  assert.equal(r.barang[0].nama, "Tanah Kantor");
  assert.equal(r.barang[0].nilai_perolehan, "1.000.000.000");
  assert.equal(r.barang[1].keterangan, ""); // kolom kurang -> kosong
  assert.equal(r.barang[2].nilai_limit, "");
  assert.equal(r.dilewati, 0);

  // Tanpa baris judul: baris pertama tetap data.
  assert.equal(H.parseBarangTempel("Nama Barang A\tK\t1\tL\tBaik\t2000\t1000\t900\t").barang.length, 1);
  assert.equal(H.parseBarangTempel("\n\n  \n").barang.length, 0);

  // Batas jumlah barang.
  const banyak = Array.from({ length: H.MAKS_BARANG + 7 }, (_, i) => `B${i}\tk\t1\tl\tBaik\t2000\t1\t1`).join("\n");
  const rb = H.parseBarangTempel(banyak);
  assert.equal(rb.barang.length, H.MAKS_BARANG);
  assert.equal(rb.dilewati, 7);
});

test("label peran dan status", () => {
  assert.equal(H.peranLabel("ue1"), "Unit Eselon I");
  assert.equal(H.peranLabel(""), "Belum ada peran");
  assert.equal(H.peranLabel("tamu"), "tamu");
  assert.equal(H.STATUS_META.selesai.tone, "ok");
});
