"use client";

import { useRef, useState } from "react";
import { ClipboardPaste, Database, FileDown, FileSpreadsheet, FileUp, Plus, Trash2, X } from "lucide-react";
import { SapaBarang, SapaHasilImpor, eksporSapaBarang, imporSapaBarang, imporSapaBarangSIMAN, unduhSapaTemplateBarang } from "@/lib/sapa";
import { simpanBlob } from "./download";
import { ErrorBox, NoticeBox, PrimaryButton, SecondaryButton, TextField, inputCls } from "./fields";
import {
  ErrorInfo,
  MAKS_BARANG,
  MAKS_BYTE_XLSX,
  ModeImpor,
  URUTAN_KOLOM_TEMPEL,
  barangKosong,
  errorInfo,
  formatRupiah,
  formatUkuran,
  gabungBarang,
  kosongBarang,
  parseBarangTempel,
  totalBarang,
} from "./sapa";

interface Laporan {
  hasil: SapaHasilImpor;
  nama: string;
  mode: ModeImpor;
  terbuang: number;
}

// Daftar barang objek penjualan. Jumlah dan total nilai di bawah hanya pratinjau: angka resmi di dokumen dihitung
// backend dari daftar yang sama, sehingga selalu cocok dengan tabel lampiran. Daftar bisa diisi satu per satu, ditempel dari
// Excel, atau diunggah dari template Excel (diunduh dari tombol "Unduh template").
export function BarangEditor({ barang, onChange, satuan }: { barang: SapaBarang[]; onChange: (b: SapaBarang[]) => void; satuan?: string }) {
  const [tempel, setTempel] = useState<string | null>(null);
  const [pesanTempel, setPesanTempel] = useState("");
  const [unduh, setUnduh] = useState<"template" | "daftar" | null>(null);
  const [mengunggah, setMengunggah] = useState<"excel" | "siman" | null>(null);
  const [galat, setGalat] = useState<ErrorInfo | null>(null);
  const [menunggu, setMenunggu] = useState<{ hasil: SapaHasilImpor; nama: string } | null>(null);
  const [laporan, setLaporan] = useState<Laporan | null>(null);
  const inputBerkas = useRef<HTMLInputElement>(null);
  const jenisBerkas = useRef<"excel" | "siman">("excel"); // tombol yang membuka pilihan berkas: template SAPA atau ekspor data aset SIMAN
  const total = totalBarang(barang);
  const terisi = barang.filter((b) => !barangKosong(b));

  const ubah = (i: number, k: keyof SapaBarang, v: string) => onChange(barang.map((b, j) => (j === i ? { ...b, [k]: v } : b)));
  const sisa = MAKS_BARANG - barang.length;

  const terapkanTempel = () => {
    const hasil = parseBarangTempel(tempel ?? "");
    if (hasil.barang.length === 0) {
      setPesanTempel("Tidak ada baris yang dapat dibaca. Salin sel dari Excel (kolom dipisah tab) lalu tempel di kotak ini.");
      return;
    }
    // Baris contoh kosong di awal daftar tidak perlu dipertahankan saat data ditempel.
    const g = gabungBarang(barang, hasil.barang, "tambah");
    const terbuang = g.terbuang + hasil.dilewati;
    onChange(g.barang);
    setPesanTempel(terbuang > 0 ? `${hasil.barang.length - g.terbuang} baris ditambahkan; ${terbuang} baris di luar batas ${MAKS_BARANG} barang diabaikan.` : "");
    if (terbuang === 0) setTempel(null);
  };

  const unduhTemplate = async () => {
    setUnduh("template");
    setGalat(null);
    try {
      const { blob, disposition } = await unduhSapaTemplateBarang();
      simpanBlob(blob, disposition, "Template Daftar Barang SAPA.xlsx");
    } catch (err) {
      setGalat(errorInfo(err, "Gagal mengunduh template"));
    } finally {
      setUnduh(null);
    }
  };

  const unduhDaftar = async () => {
    setUnduh("daftar");
    setGalat(null);
    try {
      const { blob, disposition } = await eksporSapaBarang(terisi);
      simpanBlob(blob, disposition, "Daftar Barang SAPA.xlsx");
    } catch (err) {
      setGalat(errorInfo(err, "Gagal mengunduh daftar barang"));
    } finally {
      setUnduh(null);
    }
  };

  const terapkanImpor = (hasil: SapaHasilImpor, nama: string, mode: ModeImpor) => {
    const g = gabungBarang(barang, hasil.barang, mode);
    onChange(g.barang);
    setLaporan({ hasil, nama, mode, terbuang: g.terbuang });
    setMenunggu(null);
  };

  const bukaPilihan = (jenis: "excel" | "siman") => {
    jenisBerkas.current = jenis;
    inputBerkas.current?.click();
  };

  const pilihBerkas = async (f: File | null) => {
    if (inputBerkas.current) inputBerkas.current.value = ""; // berkas yang sama boleh dipilih lagi
    if (!f) return;
    const dariSIMAN = jenisBerkas.current === "siman";
    setGalat(null);
    setLaporan(null);
    setMenunggu(null);
    if (!f.name.toLowerCase().endsWith(".xlsx")) {
      setGalat({
        message: dariSIMAN
          ? "Berkas harus berformat .xlsx: gunakan berkas hasil ekspor data aset dari SIMAN tanpa mengubah formatnya."
          : "Berkas harus berformat .xlsx. Simpan dari Excel sebagai Excel Workbook, atau gunakan template dari tombol Unduh template.",
        errors: [],
      });
      return;
    }
    if (f.size > MAKS_BYTE_XLSX) {
      setGalat({ message: `Berkas terlalu besar (${formatUkuran(f.size)}); maksimal ${formatUkuran(MAKS_BYTE_XLSX)}.`, errors: [] });
      return;
    }
    setMengunggah(dariSIMAN ? "siman" : "excel");
    try {
      const res = dariSIMAN ? await imporSapaBarangSIMAN(f) : await imporSapaBarang(f);
      const hasil = { ...res.data, barang: res.data.barang ?? [], galat: res.data.galat ?? [], peringatan: res.data.peringatan ?? [] };
      // Daftar yang masih kosong langsung diisi; bila sudah ada isinya, pengguna memilih mengganti atau menambahkan.
      if (terisi.length === 0) terapkanImpor(hasil, f.name, "ganti");
      else setMenunggu({ hasil, nama: f.name });
    } catch (err) {
      setGalat(errorInfo(err, dariSIMAN ? "Gagal membaca data SIMAN" : "Gagal membaca berkas Excel"));
    } finally {
      setMengunggah(null);
    }
  };

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <SecondaryButton onClick={() => onChange([...barang, kosongBarang()])} disabled={sisa <= 0}>
          <Plus className="h-4 w-4" aria-hidden="true" />
          Tambah barang
        </SecondaryButton>
        <SecondaryButton
          onClick={() => {
            setTempel(tempel === null ? "" : null);
            setPesanTempel("");
          }}
        >
          <ClipboardPaste className="h-4 w-4" aria-hidden="true" />
          Tempel dari Excel
        </SecondaryButton>
        <span className="mx-1 hidden h-6 w-px bg-slate-200 sm:block" aria-hidden="true" />
        <SecondaryButton onClick={unduhTemplate} busy={unduh === "template"} disabled={mengunggah !== null}>
          <FileDown className="h-4 w-4" aria-hidden="true" />
          Unduh template Excel
        </SecondaryButton>
        <SecondaryButton onClick={() => bukaPilihan("excel")} busy={mengunggah === "excel"} disabled={unduh !== null || mengunggah === "siman"}>
          <FileUp className="h-4 w-4" aria-hidden="true" />
          Unggah Excel
        </SecondaryButton>
        <SecondaryButton onClick={() => bukaPilihan("siman")} busy={mengunggah === "siman"} disabled={unduh !== null || mengunggah === "excel"}>
          <Database className="h-4 w-4" aria-hidden="true" />
          Unggah Data SIMAN
        </SecondaryButton>
        {terisi.length > 0 && (
          <SecondaryButton onClick={unduhDaftar} busy={unduh === "daftar"} disabled={mengunggah !== null}>
            <FileSpreadsheet className="h-4 w-4" aria-hidden="true" />
            Unduh daftar ini
          </SecondaryButton>
        )}
        <input
          ref={inputBerkas}
          type="file"
          accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          className="sr-only"
          tabIndex={-1}
          aria-label="Pilih berkas Excel daftar barang (template SAPA atau ekspor data aset SIMAN)"
          onChange={(e) => void pilihBerkas(e.target.files?.[0] ?? null)}
        />
      </div>
      <p className="text-xs text-slate-500">
        Cara cepat: unduh template Excel, isi daftar barang, lalu unggah kembali. Atau pilih <span className="font-medium text-slate-700">Unggah Data SIMAN</span> untuk memuat daftar langsung dari berkas hasil
        ekspor data aset SIMAN (Nama, Kode, NUP, Merk, Kondisi, tahun Tanggal Perolehan, Nilai Perolehan, Nilai Permohonan sebagai nilai limit, dan Keterangan). Baris yang bermasalah tetap dimuat sehingga
        dapat diperbaiki di bawah.
      </p>

      <ErrorBox error={galat} />

      {menunggu && (
        <div role="alertdialog" aria-label="Pilih cara memuat daftar dari Excel" className="space-y-3 rounded-lg border border-blue-200 bg-blue-50/60 p-3.5">
          <p className="text-sm text-slate-700">
            <span className="font-semibold">{menunggu.nama}</span> memuat <span className="font-semibold">{menunggu.hasil.barang.length}</span> barang. Daftar saat ini sudah berisi{" "}
            <span className="font-semibold">{terisi.length}</span> barang.
          </p>
          <div className="flex flex-wrap gap-2">
            <PrimaryButton onClick={() => terapkanImpor(menunggu.hasil, menunggu.nama, "ganti")}>Ganti daftar saat ini</PrimaryButton>
            <SecondaryButton onClick={() => terapkanImpor(menunggu.hasil, menunggu.nama, "tambah")}>Tambahkan di bawahnya</SecondaryButton>
            <SecondaryButton onClick={() => setMenunggu(null)}>Batal</SecondaryButton>
          </div>
        </div>
      )}

      {laporan && <LaporanImpor laporan={laporan} onTutup={() => setLaporan(null)} />}

      {tempel !== null && (
        <div className="space-y-2 rounded-lg border border-blue-200 bg-blue-50/50 p-3">
          <p className="text-xs text-slate-600">
            Salin sel dari Excel/Sheets (tanpa mengubah urutan kolom) lalu tempel di sini. Urutan kolom: <span className="font-medium">{URUTAN_KOLOM_TEMPEL}</span>.
            Baris judul dikenali dan dilewati.
          </p>
          <textarea
            aria-label="Tempel data barang dari Excel"
            value={tempel}
            onChange={(e) => setTempel(e.target.value)}
            rows={5}
            className={inputCls + " font-mono text-xs"}
            placeholder="Tempel di sini…"
          />
          {pesanTempel && <NoticeBox tone="warn">{pesanTempel}</NoticeBox>}
          <div className="flex justify-end gap-2">
            <SecondaryButton onClick={() => setTempel(null)}>Batal</SecondaryButton>
            <PrimaryButton onClick={terapkanTempel} disabled={tempel.trim() === ""}>
              Tambahkan ke daftar
            </PrimaryButton>
          </div>
        </div>
      )}

      {barang.length === 0 ? (
        <p className="rounded-lg border border-dashed border-slate-300 px-3 py-6 text-center text-sm text-slate-500">
          Belum ada barang. Tambahkan satu per satu, tempel dari Excel, unggah berkas template Excel, atau unggah data SIMAN.
        </p>
      ) : (
        <ol className="space-y-3">
          {barang.map((b, i) => (
            <li key={i} className="rounded-lg border border-slate-200 bg-slate-50/60 p-3">
              <div className="mb-2 flex items-center justify-between">
                <span className="text-xs font-semibold text-slate-600">Barang {i + 1}</span>
                <button
                  type="button"
                  onClick={() => onChange(barang.filter((_, j) => j !== i))}
                  aria-label={`Hapus barang ${i + 1}`}
                  className="rounded p-1 text-slate-400 hover:bg-red-50 hover:text-red-600"
                >
                  <Trash2 className="h-4 w-4" aria-hidden="true" />
                </button>
              </div>
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <TextField label="Nama barang" required value={b.nama} onChange={(v) => ubah(i, "nama", v)} maxLength={300} className="sm:col-span-2" />
                <TextField label="Kode barang" value={b.kode} onChange={(v) => ubah(i, "kode", v)} maxLength={30} inputMode="numeric" />
                <TextField label="NUP" value={b.nup} onChange={(v) => ubah(i, "nup", v)} maxLength={30} inputMode="numeric" />
                <TextField label="Lokasi / merk / tipe" value={b.lokasi} onChange={(v) => ubah(i, "lokasi", v)} maxLength={500} className="sm:col-span-2" />
                <TextField label="Kondisi" value={b.kondisi} onChange={(v) => ubah(i, "kondisi", v)} maxLength={50} placeholder="mis. Rusak Berat" />
                <TextField label="Tahun perolehan" value={b.tahun_perolehan} onChange={(v) => ubah(i, "tahun_perolehan", v)} maxLength={4} inputMode="numeric" />
                <TextField
                  label="Nilai perolehan (Rp)"
                  required
                  value={b.nilai_perolehan}
                  onChange={(v) => ubah(i, "nilai_perolehan", v)}
                  inputMode="decimal"
                  placeholder="mis. 1500000 atau 1.500.000,50"
                />
                <TextField label="Nilai limit (Rp)" required value={b.nilai_limit} onChange={(v) => ubah(i, "nilai_limit", v)} inputMode="decimal" />
                <TextField label="Keterangan" value={b.keterangan} onChange={(v) => ubah(i, "keterangan", v)} maxLength={300} className="sm:col-span-2" />
              </div>
            </li>
          ))}
        </ol>
      )}

      <dl className="grid gap-px overflow-hidden rounded-lg border border-slate-200 bg-slate-200 text-sm sm:grid-cols-3">
        <div className="bg-white px-3 py-2.5">
          <dt className="text-xs text-slate-500">Jumlah BMN</dt>
          <dd className="font-semibold text-slate-900">
            {total.jumlah}
            {satuan ? ` ${satuan}` : ""}
          </dd>
        </div>
        <div className="bg-white px-3 py-2.5">
          <dt className="text-xs text-slate-500">Total nilai perolehan</dt>
          <dd className="break-words font-semibold text-slate-900">{formatRupiah(total.perolehan)}</dd>
        </div>
        <div className="bg-white px-3 py-2.5">
          <dt className="text-xs text-slate-500">Total nilai limit</dt>
          <dd className="break-words font-semibold text-slate-900">{formatRupiah(total.limit)}</dd>
        </div>
      </dl>
      {total.tidakSah > 0 && (
        <p role="status" className="text-xs text-amber-700">
          {total.tidakSah} barang memiliki nilai yang belum terbaca sebagai angka; belum ikut dijumlahkan.
        </p>
      )}
    </div>
  );
}

// Hasil memuat berkas Excel: berapa barang yang masuk dan masalah per baris. Nomor baris merujuk ke baris di berkas Excel.
function LaporanImpor({ laporan, onTutup }: { laporan: Laporan; onTutup: () => void }) {
  const { hasil, nama, mode, terbuang } = laporan;
  const masalah = hasil.galat.length > 0;
  return (
    <div role="status" className={`rounded-lg border p-3.5 ${masalah ? "border-amber-200 bg-amber-50" : "border-emerald-200 bg-emerald-50"}`}>
      <div className="flex items-start justify-between gap-3">
        <p className={`text-sm ${masalah ? "text-amber-900" : "text-emerald-900"}`}>
          <span className="font-semibold">{hasil.barang.length} barang</span> dimuat dari <span className="font-medium">{nama}</span>
          {mode === "tambah" ? " dan ditambahkan di bawah daftar." : "."}
          {masalah && " Periksa dan perbaiki hal berikut di daftar di bawah (nomor baris merujuk ke baris di Excel):"}
        </p>
        <button type="button" onClick={onTutup} aria-label="Tutup laporan impor" className="shrink-0 rounded p-1 text-slate-400 hover:bg-white/60 hover:text-slate-700">
          <X className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
      {masalah && (
        <ul className="ml-5 mt-2 list-disc space-y-0.5 text-xs text-amber-900">
          {hasil.galat.map((g, i) => (
            <li key={i}>{g}</li>
          ))}
        </ul>
      )}
      {(hasil.peringatan.length > 0 || terbuang > 0) && (
        <ul className="ml-5 mt-2 list-disc space-y-0.5 text-xs text-amber-900">
          {hasil.peringatan.map((p, i) => (
            <li key={i}>{p}</li>
          ))}
          {terbuang > 0 && <li>{terbuang} barang di luar batas {MAKS_BARANG} barang tidak dimuat.</li>}
        </ul>
      )}
    </div>
  );
}
