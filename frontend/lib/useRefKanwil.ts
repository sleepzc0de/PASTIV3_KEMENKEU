"use client";

import { useCallback, useEffect, useState } from "react";
import { getRefKanwil, type RefKanwil } from "./api";
import { kodeKanwilDenganUraian, labelKanwil, namaKanwil, petaKanwil, type PetaKanwil } from "./refKanwil";

// Referensi Kanwil dibaca sekali lalu dibagikan ke semua komponen, seperti referensi UE1 (lib/useRefUE1.ts). Setelah superadmin mengubahnya, panggil
// segarkanRefKanwil() supaya label di seluruh layar ikut berubah tanpa memuat ulang halaman.
let daftarCache: RefKanwil[] | null = null;
let petaCache: PetaKanwil = {};
let sedangMuat: Promise<void> | null = null;
const pendengar = new Set<() => void>();

function beritahu() {
  pendengar.forEach((p) => p());
}

export function muatRefKanwil(): Promise<void> {
  if (!sedangMuat) {
    sedangMuat = getRefKanwil()
      .then((res) => {
        daftarCache = res.data.daftar;
        petaCache = petaKanwil(res.data.daftar);
        beritahu();
      })
      .catch(() => {
        // Gagal memuat tidak boleh mengganggu halaman: kode tetap tampil apa adanya ("Kanwil 015040199").
        if (daftarCache === null) {
          daftarCache = [];
          petaCache = {};
          beritahu();
        }
      })
      .finally(() => {
        sedangMuat = null;
      });
  }
  return sedangMuat;
}

// Paksa baca ulang (mis. setelah superadmin menyimpan perubahan).
export function segarkanRefKanwil(): Promise<void> {
  return muatRefKanwil();
}

export function useRefKanwil() {
  const [, paksaGambar] = useState(0);
  useEffect(() => {
    const p = () => paksaGambar((n) => n + 1);
    pendengar.add(p);
    if (daftarCache === null) void muatRefKanwil();
    return () => {
      pendengar.delete(p);
    };
  }, []);

  // Fungsi baru setiap kali referensi berubah, supaya useMemo/useEffect yang memakainya ikut dihitung ulang.
  const peta = petaCache;
  const label = useCallback((kode: string | null | undefined) => labelKanwil(kode, peta), [peta]);
  const nama = useCallback((kode: string | null | undefined) => namaKanwil(kode, peta), [peta]);
  const kodeUraian = useCallback((kode: string | null | undefined) => kodeKanwilDenganUraian(kode, peta), [peta]);
  return { siap: daftarCache !== null, daftar: daftarCache ?? [], peta, label, nama, kodeUraian };
}
