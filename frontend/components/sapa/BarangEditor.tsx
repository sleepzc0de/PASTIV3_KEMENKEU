"use client";

import { useState } from "react";
import { ClipboardPaste, Plus, Trash2 } from "lucide-react";
import { SapaBarang } from "@/lib/sapa";
import { NoticeBox, PrimaryButton, SecondaryButton, TextField, inputCls } from "./fields";
import { MAKS_BARANG, URUTAN_KOLOM_TEMPEL, formatRupiah, kosongBarang, parseBarangTempel, totalBarang } from "./sapa";

// Daftar barang objek penjualan. Jumlah dan total nilai di bawah hanya pratinjau: angka resmi di dokumen dihitung
// backend dari daftar yang sama, sehingga selalu cocok dengan tabel lampiran.
export function BarangEditor({ barang, onChange }: { barang: SapaBarang[]; onChange: (b: SapaBarang[]) => void }) {
  const [tempel, setTempel] = useState<string | null>(null);
  const [pesanTempel, setPesanTempel] = useState("");
  const total = totalBarang(barang);

  const ubah = (i: number, k: keyof SapaBarang, v: string) => onChange(barang.map((b, j) => (j === i ? { ...b, [k]: v } : b)));
  const sisa = MAKS_BARANG - barang.length;

  const terapkanTempel = () => {
    const hasil = parseBarangTempel(tempel ?? "");
    if (hasil.barang.length === 0) {
      setPesanTempel("Tidak ada baris yang dapat dibaca. Salin sel dari Excel (kolom dipisah tab) lalu tempel di kotak ini.");
      return;
    }
    const diambil = hasil.barang.slice(0, Math.max(sisa, 0));
    const terbuang = hasil.barang.length - diambil.length + hasil.dilewati;
    // Baris contoh kosong di awal daftar tidak perlu dipertahankan saat data ditempel.
    const dasar = barang.length === 1 && Object.values(barang[0]).every((x) => x === "") ? [] : barang;
    onChange([...dasar, ...diambil]);
    setPesanTempel(terbuang > 0 ? `${diambil.length} baris ditambahkan; ${terbuang} baris di luar batas ${MAKS_BARANG} barang diabaikan.` : "");
    if (terbuang === 0) setTempel(null);
  };

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-2">
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
      </div>

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
          Belum ada barang. Tambahkan satu per satu atau tempel dari Excel.
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
          <dd className="font-semibold text-slate-900">{total.jumlah}</dd>
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
