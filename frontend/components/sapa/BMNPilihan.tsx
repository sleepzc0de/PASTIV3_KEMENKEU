"use client";

import { useCallback, useEffect, useId, useState } from "react";
import { Check } from "lucide-react";
import { SapaRefBMN, getSapaRefBMN } from "@/lib/sapa";
import { NoticeBox, SecondaryButton, SelectInput } from "./fields";
import { errorInfo, periksaBMN, pilihJenisBMN } from "./sapa";

// Daftar jenis BMN aktif beserta satuan yang diizinkan untuk tiap jenis (dikelola admin di Pengaturan SAPA).
export function useRefBMN() {
  const [daftar, setDaftar] = useState<SapaRefBMN | null>(null);
  const [error, setError] = useState<string | null>(null);

  const muat = useCallback(async () => {
    setError(null);
    try {
      const res = await getSapaRefBMN();
      // Daftar kosong dikirim sebagai null oleh Go; formulir butuh larik.
      setDaftar({ jenis: (res.data.jenis ?? []).map((j) => ({ ...j, satuan: j.satuan ?? [] })), satuan: res.data.satuan ?? [] });
    } catch (err) {
      setError(errorInfo(err, "Gagal memuat daftar jenis BMN").message);
    }
  }, []);

  useEffect(() => {
    void muat();
  }, [muat]);

  return { daftar, error, muat };
}

export interface PasanganBMN {
  jenis: string;
  satuan: string;
}

// Pilihan jenis BMN (satu jenis per usulan) dan satuan jumlahnya. Satuan hanya menawarkan yang sesuai untuk jenis terpilih, dan
// berganti otomatis ke satuan bawaan bila jenis diganti ke yang tidak mengizinkannya, sehingga pasangan yang tidak masuk akal
// (Peralatan dan Mesin dalam "meter", Tanah dalam "unit") tidak bisa dibuat. Backend menegakkan aturan yang sama.
export function BMNPilihan({
  daftar,
  error,
  onCoba,
  nilai,
  onChange,
}: {
  daftar: SapaRefBMN | null;
  error: string | null;
  onCoba: () => void;
  nilai: PasanganBMN;
  onChange: (p: PasanganBMN) => void;
}) {
  const nama = useId();
  const periksa = periksaBMN(daftar, nilai.jenis, nilai.satuan);

  // Nilai tersimpan yang cocok dengan daftar dirapikan: huruf besar/kecil mengikuti daftar, dan satuan bawaan terisi bila kosong.
  useEffect(() => {
    if (!daftar || !periksa.jenis) return;
    const p = pilihJenisBMN(daftar, nilai.jenis, nilai.satuan);
    const satuanBaru = nilai.satuan.trim() === "" ? p.satuan : nilai.satuan;
    if (p.jenis !== nilai.jenis || satuanBaru !== nilai.satuan) onChange({ jenis: p.jenis, satuan: satuanBaru });
  }, [daftar, periksa.jenis, nilai.jenis, nilai.satuan, onChange]);

  if (error) {
    return (
      <div className="space-y-2 sm:col-span-2">
        <NoticeBox tone="warn">{error}</NoticeBox>
        <SecondaryButton onClick={onCoba}>Coba lagi</SecondaryButton>
      </div>
    );
  }
  if (!daftar) return <div role="status" aria-label="Memuat daftar jenis BMN" className="h-24 animate-pulse rounded-xl bg-slate-100 sm:col-span-2" />;
  if (daftar.jenis.length === 0) {
    return (
      <div className="sm:col-span-2">
        <NoticeBox tone="warn">Daftar jenis BMN belum diatur. Hubungi admin SAPA untuk menambahkannya di Pengaturan SAPA.</NoticeBox>
      </div>
    );
  }

  return (
    <>
      <fieldset className="sm:col-span-2">
        <legend className="mb-1.5 text-xs font-medium text-slate-600">
          Jenis BMN
          <span aria-hidden="true" className="ml-0.5 text-red-500">
            *
          </span>
        </legend>
        <div role="radiogroup" aria-label="Jenis BMN" className="flex flex-wrap gap-2">
          {daftar.jenis.map((j) => {
            const dipilih = periksa.jenis?.nama === j.nama;
            return (
              <label key={j.nama} className="cursor-pointer">
                <input
                  type="radio"
                  name={nama}
                  value={j.nama}
                  checked={dipilih}
                  onChange={() => onChange(pilihJenisBMN(daftar, j.nama, nilai.satuan))}
                  className="peer sr-only"
                />
                <span
                  className={`inline-flex items-center gap-1.5 rounded-full border px-3.5 py-2 text-sm font-medium transition peer-focus-visible:ring-2 peer-focus-visible:ring-blue-500 peer-focus-visible:ring-offset-1 ${
                    dipilih
                      ? "border-blue-600 bg-blue-600 text-white shadow-sm shadow-blue-600/20"
                      : "border-slate-300 bg-white text-slate-700 hover:border-blue-400 hover:bg-blue-50"
                  }`}
                >
                  {dipilih && <Check className="h-3.5 w-3.5" aria-hidden="true" />}
                  {j.nama}
                </span>
              </label>
            );
          })}
        </div>
        {periksa.pesanJenis && (
          <div className="mt-2">
            <NoticeBox tone="warn">{periksa.pesanJenis}</NoticeBox>
          </div>
        )}
      </fieldset>

      <SelectInput
        label="Satuan jumlah BMN"
        required
        value={periksa.satuanSah}
        onChange={(satuan) => onChange({ jenis: nilai.jenis, satuan })}
        options={periksa.jenis?.satuan ?? []}
        disabled={!periksa.jenis}
        placeholder={periksa.jenis ? "Pilih satuan…" : "Pilih jenis BMN dulu"}
        hint={periksa.jenis ? `Hanya satuan yang sesuai untuk ${periksa.jenis.nama} yang bisa dipilih; dipakai pada terbilang jumlah BMN.` : undefined}
      />
      {periksa.pesanSatuan && (
        <div className="sm:col-span-2">
          <NoticeBox tone="warn">{periksa.pesanSatuan}</NoticeBox>
        </div>
      )}
    </>
  );
}
