"use client";

import { useCallback, useEffect, useState } from "react";
import { getRefUE1, type RefUE1 } from "./api";
import { kodeDenganUraian, labelUE1, namaUE1, opsiUE1, petaUE1, type PetaUE1 } from "./refUE1";

// Referensi UE1 dibaca sekali lalu dibagikan ke semua komponen (puluhan baris saja). Setelah admin mengubahnya, panggil segarkanRefUE1() supaya
// label di seluruh layar ikut berubah tanpa memuat ulang halaman.
let daftarCache: RefUE1[] | null = null;
let petaCache: PetaUE1 = {};
let sedangMuat: Promise<void> | null = null;
const pendengar = new Set<() => void>();

function beritahu() {
  pendengar.forEach((p) => p());
}

export function muatRefUE1(): Promise<void> {
  if (!sedangMuat) {
    sedangMuat = getRefUE1()
      .then((res) => {
        daftarCache = res.data.daftar;
        petaCache = petaUE1(res.data.daftar);
        beritahu();
      })
      .catch(() => {
        // Gagal memuat tidak boleh mengganggu halaman: kode tetap tampil apa adanya ("UE1 01504").
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

// Paksa baca ulang (mis. setelah admin menyimpan perubahan).
export function segarkanRefUE1(): Promise<void> {
  return muatRefUE1();
}

export function useRefUE1() {
  const [, paksaGambar] = useState(0);
  useEffect(() => {
    const p = () => paksaGambar((n) => n + 1);
    pendengar.add(p);
    if (daftarCache === null) void muatRefUE1();
    return () => {
      pendengar.delete(p);
    };
  }, []);

  // Fungsi baru setiap kali referensi berubah, supaya useMemo/useEffect yang memakainya ikut dihitung ulang.
  const peta = petaCache;
  const label = useCallback((kode: string | null | undefined) => labelUE1(kode, peta), [peta]);
  const nama = useCallback((kode: string | null | undefined) => namaUE1(kode, peta), [peta]);
  const kodeUraian = useCallback((kode: string | null | undefined) => kodeDenganUraian(kode, peta), [peta]);
  const opsi = useCallback((kode: string[]) => opsiUE1(kode, peta), [peta]);
  return { siap: daftarCache !== null, daftar: daftarCache ?? [], peta, label, nama, kodeUraian, opsi };
}
