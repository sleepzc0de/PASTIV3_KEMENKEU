"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import axios from "axios";
import { DGOverview, DGSyncOverview, getDGOverview, getDGSyncStatus } from "@/lib/api";

export function errorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError(err)) {
    if (err.response) return err.response.data?.message || fallback;
    if (err.code === "ECONNABORTED") return "Permintaan terlalu lama dan dihentikan";
  }
  return "Tidak dapat terhubung ke server";
}

export interface AsyncState<T> {
  data: T | null;
  isLoading: boolean;
  error: string | null;
  reload: () => void;
}

// Ringkasan dimuat sekali dan dipakai bersama tab Ringkasan dan Peta. `version` dinaikkan induk setelah sinkronisasi
// selesai supaya data dimuat ulang. Saat memuat ulang, data lama tetap ditampilkan (hanya diredupkan).
export function useDGOverview(enabled: boolean, version: number): AsyncState<DGOverview> {
  const [data, setData] = useState<DGOverview | null>(null);
  const [isLoading, setIsLoading] = useState(enabled);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    setIsLoading(true);
    setError(null);
    getDGOverview()
      .then((res) => {
        if (!cancelled) setData(res.data);
      })
      .catch((err) => {
        if (!cancelled) setError(errorMessage(err, "Gagal memuat ringkasan"));
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [enabled, version, tick]);

  const reload = useCallback(() => setTick((t) => t + 1), []);
  return { data, isLoading, error, reload };
}

const POLL_MS = 3000;

// Status sinkronisasi. Selama ada antrean aktif, status dibaca ulang tiap 3 detik; begitu antrean selesai,
// onFinished dipanggil sekali (induk memuat ulang data yang baru disinkronkan).
export function useDGSync(enabled: boolean, onFinished: () => void): AsyncState<DGSyncOverview> {
  const [data, setData] = useState<DGSyncOverview | null>(null);
  const [isLoading, setIsLoading] = useState(enabled);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const wasActive = useRef(false);
  const finished = useRef(onFinished);
  useEffect(() => {
    finished.current = onFinished;
  });

  const active = data?.aktif != null;

  // Pembacaan berkala hanya saat ada antrean.
  useEffect(() => {
    if (!enabled || !active) return;
    const timer = setInterval(() => setTick((t) => t + 1), POLL_MS);
    return () => clearInterval(timer);
  }, [enabled, active]);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    getDGSyncStatus()
      .then((res) => {
        if (cancelled) return;
        setData(res.data);
        setError(null);
        const nowActive = res.data.aktif != null;
        if (wasActive.current && !nowActive) finished.current();
        wasActive.current = nowActive;
      })
      .catch((err) => {
        if (!cancelled) setError(errorMessage(err, "Gagal memuat status sinkronisasi"));
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [enabled, tick]);

  const reload = useCallback(() => setTick((t) => t + 1), []);
  return { data, isLoading, error, reload };
}
