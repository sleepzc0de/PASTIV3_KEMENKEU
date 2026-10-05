"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  AnalitikResponse,
  DataHalaman,
  PenarikanAktif,
  PenarikanKuota,
  PenarikanStatus,
  PenyaringData,
  getAnalitik,
  getInaprocData,
  getPenarikanAktif,
  getPenarikanStatus,
} from "@/lib/api";
import { AsyncState, errorMessage } from "../digitalisasi/useDigitalisasi";

const POLL_MS = 2000;

export interface PenarikanState extends AsyncState<PenarikanStatus> {
  aktif: PenarikanAktif | null;
  // Kuota terbaru: dari status penuh, diperbarui tiap polling selama ada antrean.
  kuota: PenarikanKuota | null;
  // Dipanggil setelah penarikan dimulai (respons start) supaya pembacaan berkala langsung berjalan tanpa menunggu muat ulang penuh.
  setAktif: (a: PenarikanAktif | null) => void;
}

// Status halaman Penarikan Data. Selama ada antrean aktif, kemajuannya dibaca ulang tiap 2 detik lewat endpoint ringan; begitu antrean
// selesai, status penuh dimuat ulang (jumlah baris, riwayat) dan onSelesai dipanggil sekali.
export function usePenarikan(onSelesai: () => void): PenarikanState {
  const [data, setData] = useState<PenarikanStatus | null>(null);
  const [aktif, setAktifState] = useState<PenarikanAktif | null>(null);
  const [kuotaTerbaru, setKuotaTerbaru] = useState<PenarikanKuota | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const sebelumnyaAktif = useRef(false);
  const selesai = useRef(onSelesai);
  useEffect(() => {
    selesai.current = onSelesai;
  });

  // Muat penuh.
  useEffect(() => {
    let batal = false;
    getPenarikanStatus()
      .then((res) => {
        if (batal) return;
        setData(res.data);
        setAktifState(res.data.aktif);
        setKuotaTerbaru(res.data.kuota);
        sebelumnyaAktif.current = res.data.aktif !== null;
        setError(null);
      })
      .catch((err) => {
        if (!batal) setError(errorMessage(err, "Gagal memuat status penarikan data"));
      })
      .finally(() => {
        if (!batal) setIsLoading(false);
      });
    return () => {
      batal = true;
    };
  }, [tick]);

  // Pembacaan berkala hanya saat ada antrean.
  const berjalan = aktif !== null;
  useEffect(() => {
    if (!berjalan) return;
    let batal = false;
    const timer = setInterval(() => {
      getPenarikanAktif()
        .then((res) => {
          if (batal) return;
          setAktifState(res.data.aktif);
          setKuotaTerbaru(res.data.kuota);
          if (res.data.aktif === null && sebelumnyaAktif.current) {
            sebelumnyaAktif.current = false;
            setTick((t) => t + 1);
            selesai.current();
          }
        })
        .catch(() => {
          // Gangguan jaringan sesaat: pembacaan berikutnya mencoba lagi.
        });
    }, POLL_MS);
    return () => {
      batal = true;
      clearInterval(timer);
    };
  }, [berjalan]);

  const setAktif = useCallback((a: PenarikanAktif | null) => {
    sebelumnyaAktif.current = a !== null;
    setAktifState(a);
  }, []);
  const reload = useCallback(() => setTick((t) => t + 1), []);
  return { data, isLoading, error, reload, aktif, setAktif, kuota: kuotaTerbaru ?? data?.kuota ?? null };
}

export interface HalamanData {
  dataset: string;
  filter: PenyaringData;
  halaman: number;
  perHalaman?: number;
}

// Satu halaman data lokal sebuah dataset. Permintaan lama yang terlambat tiba dibuang (urutan terakhir yang menang).
export function useDataHalaman(p: HalamanData | null, versi: number): AsyncState<DataHalaman> {
  const [data, setData] = useState<DataHalaman | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const kunci = p ? JSON.stringify(p) : "";

  useEffect(() => {
    if (!p) {
      setData(null);
      return;
    }
    let batal = false;
    setIsLoading(true);
    setError(null);
    getInaprocData(p.dataset, { ...p.filter, halaman: p.halaman, per_halaman: p.perHalaman })
      .then((res) => {
        if (!batal) setData(res.data);
      })
      .catch((err) => {
        if (!batal) setError(errorMessage(err, "Gagal memuat data"));
      })
      .finally(() => {
        if (!batal) setIsLoading(false);
      });
    return () => {
      batal = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kunci, versi, tick]);

  const reload = useCallback(() => setTick((t) => t + 1), []);
  return { data, isLoading, error, reload };
}

// Dasbor analitik untuk satu KLPD dan tahun; berubah filter = muat ulang. `segarkan` meminta penghitungan ulang (melewati cache server).
export function useAnalitik(tahun: string, klpd: string, versi: number): AsyncState<AnalitikResponse> & { segarkan: () => void } {
  const [data, setData] = useState<AnalitikResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const paksa = useRef(false);

  useEffect(() => {
    let batal = false;
    setIsLoading(true);
    setError(null);
    const segarkan = paksa.current;
    paksa.current = false;
    getAnalitik({ tahun, kode_klpd: klpd, segarkan })
      .then((res) => {
        if (!batal) setData(res.data);
      })
      .catch((err) => {
        if (!batal) setError(errorMessage(err, "Gagal memuat dasbor pengadaan"));
      })
      .finally(() => {
        if (!batal) setIsLoading(false);
      });
    return () => {
      batal = true;
    };
  }, [tahun, klpd, versi, tick]);

  const reload = useCallback(() => setTick((t) => t + 1), []);
  const segarkan = useCallback(() => {
    paksa.current = true;
    setTick((t) => t + 1);
  }, []);
  return { data, isLoading, error, reload, segarkan };
}
