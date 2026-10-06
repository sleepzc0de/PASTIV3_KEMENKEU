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

test("alamat usulan harus berbentuk UUID", () => {
  assert.equal(H.adalahUUID("6f9619ff-8b86-d011-b42d-00c04fc964ff"), true);
  assert.equal(H.adalahUUID("6F9619FF-8B86-D011-B42D-00C04FC964FF"), true);
  for (const salah of ["", "1", "12345", "6f9619ff8b86d011b42d00c04fc964ff", "{6f9619ff-8b86-d011-b42d-00c04fc964ff}", "6f9619ff-8b86-d011-b42d-00c04fc964ff/x", "6f9619ff-8b86-d011-b42d-00c04fc964fg", " 6f9619ff-8b86-d011-b42d-00c04fc964ff"]) {
    assert.equal(H.adalahUUID(salah), false, salah);
  }
});

test("label peran dan status", () => {
  assert.equal(H.peranLabel("ue1"), "Unit Eselon I");
  assert.equal(H.peranLabel(""), "Belum ada peran");
  assert.equal(H.peranLabel("tamu"), "tamu");
  assert.equal(H.STATUS_META.selesai.tone, "ok");
});

const alur = [
  { kunci: "tim", label: "Tim", peran: "satker" },
  { kunci: "ba", label: "BA", peran: "satker" },
  { kunci: "nd_satker", label: "ND Satker", peran: "satker" },
  { kunci: "siman_kanwil", label: "Penelitian", peran: "kanwil" },
  { kunci: "nd_ue1", label: "ND UE1", peran: "ue1" },
  { kunci: "siman_ue1", label: "Teruskan", peran: "ue1" },
];

test("tampilan menurut peran: pengguna fokus ke tahap perannya, admin melihat semua", () => {
  assert.equal(H.lihatAwal(false), "saya");
  assert.equal(H.lihatAwal(true), "semua");
  const kunci = (l, p) => H.tahapDilihat(alur, l, p).map((t) => t.kunci);
  assert.deepEqual(kunci("saya", "satker"), ["tim", "ba", "nd_satker"]);
  assert.deepEqual(kunci("saya", "kanwil"), ["siman_kanwil"]);
  assert.deepEqual(kunci("saya", "ue1"), ["nd_ue1", "siman_ue1"]);
  assert.equal(kunci("semua", "satker").length, 6);
  // Admin menyaring per peran, tidak bergantung pada peran SAPA-nya.
  assert.deepEqual(kunci("ue1", ""), ["nd_ue1", "siman_ue1"]);
  assert.deepEqual(kunci("kanwil", "satker"), ["siman_kanwil"]);
  // Tanpa peran: semua tahap (halaman tidak boleh kosong).
  assert.equal(kunci("saya", "").length, 6);
  // Tidak mengubah daftar asal.
  assert.equal(alur.length, 6);
});

test("Pengguna Barang hanya melihat: tanpa tahap sendiri, tampilan awal semua tahap", () => {
  assert.equal(H.punyaTahap("satker"), true);
  assert.equal(H.punyaTahap("kanwil"), true);
  assert.equal(H.punyaTahap("ue1"), true);
  assert.equal(H.punyaTahap("pengguna_barang"), false);
  assert.equal(H.punyaTahap(""), false);
  assert.equal(H.lihatAwal(false, "pengguna_barang"), "semua");
  assert.equal(H.lihatAwal(false, "kanwil"), "saya");
  assert.equal(H.lihatAwal(true, "satker"), "semua");
  // Pilihan "saya" tidak boleh menghasilkan daftar kosong bagi Pengguna Barang.
  assert.equal(H.tahapDilihat(alur, "saya", "pengguna_barang").length, 6);
  assert.equal(H.peranLabel("pengguna_barang"), "Pengguna Barang");
});

test("giliran saat ini", () => {
  assert.deepEqual(H.giliranSaatIni(alur, "", "satker", false), { jenis: "selesai" });
  assert.deepEqual(H.giliranSaatIni(alur, "zzz", "satker", false), { jenis: "selesai" });
  assert.equal(H.giliranSaatIni(alur, "ba", "satker", false).jenis, "saya");
  const lain = H.giliranSaatIni(alur, "siman_kanwil", "satker", false);
  assert.equal(lain.jenis, "lain");
  assert.equal(lain.peran, "kanwil");
  assert.equal(lain.label, "Penelitian");
  // Admin selalu dianggap boleh mengerjakan tahap berjalan.
  assert.equal(H.giliranSaatIni(alur, "siman_kanwil", "", true).jenis, "saya");
});

test("kelompok peran berurutan", () => {
  const g = H.kelompokPeran(alur);
  assert.deepEqual(g.map((x) => [x.peran, x.items.length]), [["satker", 3], ["kanwil", 1], ["ue1", 2]]);
  // Peran yang sama tetapi tidak berurutan membentuk kelompok terpisah.
  assert.equal(H.kelompokPeran([{ peran: "a" }, { peran: "b" }, { peran: "a" }]).length, 3);
  assert.deepEqual(H.kelompokPeran([]), []);
});

test("anggota tim: NIP ikut dinormalisasi dan baris kosong dibuang", () => {
  const tim = H.normTim({ anggota: [{ nama: "Budi", jabatan: "Kasi", kedudukan: "Ketua", nip: "198001012005011001" }, { nama: "Siti" }] });
  assert.equal(tim.anggota[0].nip, "198001012005011001");
  assert.equal(tim.anggota[1].nip, "");
  assert.deepEqual(H.kosongAnggota(), { nama: "", jabatan: "", kedudukan: "", nip: "" });
  const m = H.muatanTim({ ...tim, anggota: [...tim.anggota, H.kosongAnggota()] });
  assert.equal(m.anggota.length, 2); // baris kosong tidak ikut terkirim
  assert.equal(m.anggota[0].nip, "198001012005011001");
});

test("kata kunci pencarian pegawai", () => {
  assert.equal(H.kataKunciPegawai("ab"), null);
  assert.equal(H.kataKunciPegawai("  a   b "), null);
  assert.equal(H.kataKunciPegawai("  Budi   Santoso "), "Budi Santoso");
  assert.equal(H.kataKunciPegawai("abc"), "abc");
  assert.equal(H.kataKunciPegawai("Évr"), "Évr"); // huruf bertanda dihitung satu
  assert.equal(H.kataKunciPegawai(""), null);
});

test("NIP terpakai dan ganda", () => {
  const a = (nip) => ({ nama: "x", jabatan: "y", kedudukan: "z", nip });
  const daftar = [a("1980 0101"), a(""), a("19800101"), a("199001012015022002")];
  assert.deepEqual([...H.nipTerpakai(daftar)].sort(), ["19800101", "199001012015022002"]);
  assert.deepEqual([...H.nipGanda(daftar)], ["19800101"]);
  assert.equal(H.nipGanda([a(""), a("")]).size, 0);
});

// Daftar contoh yang sama bentuknya dengan jawaban GET /sapa/referensi/bmn.
const REF = {
  satuan: [
    { nama: "bidang", aktif: true, urutan: 1 },
    { nama: "unit", aktif: true, urutan: 2 },
    { nama: "buah", aktif: true, urutan: 3 },
    { nama: "set", aktif: true, urutan: 4 },
  ],
  jenis: [
    { nama: "Tanah", aktif: true, urutan: 1, satuan: ["bidang"], satuan_bawaan: "bidang" },
    { nama: "Peralatan dan Mesin", aktif: true, urutan: 2, satuan: ["unit", "buah", "set"], satuan_bawaan: "unit" },
    { nama: "Tanah dan Bangunan", aktif: true, urutan: 3, satuan: ["bidang", "unit"], satuan_bawaan: "unit" },
  ],
};

test("memilih jenis BMN tidak pernah menghasilkan pasangan yang tidak masuk akal", () => {
  // Satuan bawaan dipakai saat belum ada satuan.
  assert.deepEqual(H.pilihJenisBMN(REF, "Tanah", ""), { jenis: "Tanah", satuan: "bidang" });
  assert.deepEqual(H.pilihJenisBMN(REF, "Peralatan dan Mesin", ""), { jenis: "Peralatan dan Mesin", satuan: "unit" });
  // Satuan yang sedang dipakai dipertahankan bila masih diizinkan untuk jenis baru ...
  assert.deepEqual(H.pilihJenisBMN(REF, "Peralatan dan Mesin", "set"), { jenis: "Peralatan dan Mesin", satuan: "set" });
  assert.deepEqual(H.pilihJenisBMN(REF, "Tanah dan Bangunan", "BIDANG"), { jenis: "Tanah dan Bangunan", satuan: "bidang" });
  // ... dan diganti satuan bawaan bila tidak (Peralatan dan Mesin dalam "set" lalu pindah ke Tanah).
  assert.deepEqual(H.pilihJenisBMN(REF, "Tanah", "set"), { jenis: "Tanah", satuan: "bidang" });
  assert.deepEqual(H.pilihJenisBMN(REF, "Tanah dan Bangunan", "buah"), { jenis: "Tanah dan Bangunan", satuan: "unit" });
  // Nama jenis dikanonkan menurut daftar; jenis yang tidak dikenal tidak mengubah satuan.
  assert.equal(H.pilihJenisBMN(REF, "tanah", "").jenis, "Tanah");
  assert.deepEqual(H.pilihJenisBMN(REF, "Pesawat", "unit"), { jenis: "Pesawat", satuan: "unit" });
  assert.deepEqual(H.pilihJenisBMN(null, "Tanah", "x"), { jenis: "Tanah", satuan: "x" });
});

test("periksaBMN menandai data lama yang tidak sesuai daftar", () => {
  assert.deepEqual(H.periksaBMN(REF, "", ""), { jenis: null, satuanSah: "", pesanJenis: null, pesanSatuan: null });
  const ok = H.periksaBMN(REF, "tanah", "BIDANG");
  assert.equal(ok.jenis.nama, "Tanah");
  assert.equal(ok.satuanSah, "bidang");
  assert.equal(ok.pesanJenis, null);
  assert.equal(ok.pesanSatuan, null);

  const asing = H.periksaBMN(REF, "Pesawat Terbang", "unit");
  assert.equal(asing.jenis, null);
  assert.match(asing.pesanJenis, /tidak ada di daftar/);

  const salah = H.periksaBMN(REF, "Tanah", "unit");
  assert.equal(salah.satuanSah, "");
  assert.match(salah.pesanSatuan, /tidak sesuai untuk Tanah.*bidang/);
  assert.match(H.periksaBMN(REF, "Peralatan dan Mesin", "meter").pesanSatuan, /unit, buah, set/);

  // Satuan belum dipilih bukan galat di sini (formulir menandainya wajib).
  assert.equal(H.periksaBMN(REF, "Tanah", "").pesanSatuan, null);
  // Daftar belum termuat: tidak ada penilaian.
  assert.equal(H.periksaBMN(null, "Tanah", "unit").pesanSatuan, null);
});

test("satuanDiizinkan dan cariJenisBMN tidak membedakan huruf besar/kecil", () => {
  assert.equal(H.satuanDiizinkan(REF.jenis[1], " Unit "), "unit");
  assert.equal(H.satuanDiizinkan(REF.jenis[0], "unit"), null);
  assert.equal(H.satuanDiizinkan(null, "unit"), null);
  assert.equal(H.cariJenisBMN(REF, "  peralatan DAN mesin ").nama, "Peralatan dan Mesin");
  assert.equal(H.cariJenisBMN(REF, ""), null);
});

test("pengaturan admin: satuan jenis disusun menurut urutan daftar satuan", () => {
  // Urutan centang tidak dipakai: urutan mengikuti daftar satuan.
  assert.deepEqual(H.susunSatuanJenis(REF.satuan, ["set", "unit"], "set"), { satuan: ["unit", "set"], bawaan: "set" });
  // Bawaan yang tidak dicentang lagi jatuh ke satuan pertama.
  assert.deepEqual(H.susunSatuanJenis(REF.satuan, ["buah", "set"], "unit"), { satuan: ["buah", "set"], bawaan: "buah" });
  // Tidak ada yang dicentang: kosong (backend menolak jenis tanpa satuan).
  assert.deepEqual(H.susunSatuanJenis(REF.satuan, [], "unit"), { satuan: [], bawaan: "" });
  assert.equal(H.urutanBerikut([]), 1);
  assert.equal(H.urutanBerikut([{ urutan: 3 }, { urutan: 7 }, { urutan: 5 }]), 8);
});

test("gabungBarang: ganti, tambah, dan batas jumlah", () => {
  const b = (nama) => ({ ...H.kosongBarang(), nama, nilai_perolehan: "1", nilai_limit: "1" });
  const lama = [b("A"), H.kosongBarang(), b("B")];
  const baru = [b("C"), b("D")];
  assert.deepEqual(H.gabungBarang(lama, baru, "ganti").barang.map((x) => x.nama), ["C", "D"]);
  // Baris kosong pada daftar lama dibuang saat menambahkan.
  assert.deepEqual(H.gabungBarang(lama, baru, "tambah").barang.map((x) => x.nama), ["A", "B", "C", "D"]);
  assert.deepEqual(H.gabungBarang([H.kosongBarang()], baru, "tambah").barang.map((x) => x.nama), ["C", "D"]);

  const banyak = Array.from({ length: 498 }, (_, i) => b(`L${i}`));
  const r = H.gabungBarang(banyak, [b("X"), b("Y"), b("Z"), b("W")], "tambah");
  assert.equal(r.barang.length, 500);
  assert.equal(r.terbuang, 2);
  assert.equal(r.barang[499].nama, "Y");
  assert.equal(H.gabungBarang(lama, baru, "ganti").terbuang, 0);
  assert.equal(H.MAKS_BYTE_XLSX, 2 * 1024 * 1024);
});
