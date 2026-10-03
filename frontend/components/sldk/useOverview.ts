"use client";

import { useCallback, useEffect, useState } from "react";
import axios from "axios";
import { getSLDKOverview, SLDKOverviewData } from "@/lib/api";

export interface OverviewState {
  data: SLDKOverviewData | null;
  isLoading: boolean;
  error: string | null;
  reload: () => void;
}

// Data Ringkasan dan Pemantauan dimuat sekali dan dipakai bersama kedua tab. Saat dimuat ulang, data lama
// tetap ada (tampilan hanya diredupkan) supaya halaman tidak berkedip.
export function useOverview(enabled: boolean): OverviewState {
  const [data, setData] = useState<SLDKOverviewData | null>(null);
  const [isLoading, setIsLoading] = useState(enabled);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    setIsLoading(true);
    setError(null);
    getSLDKOverview()
      .then((res) => {
        if (!cancelled) setData(res.data);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(axios.isAxiosError(err) && err.response ? err.response.data?.message || "Gagal memuat ringkasan" : "Tidak dapat terhubung ke server");
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
