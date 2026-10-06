"use client";

import { useEffect, useState } from "react";
import { Loader2, MapPin, ShieldAlert } from "lucide-react";
import { DGDatasetKey, DGDetailData, getDGDetail } from "@/lib/api";
import { formatDateTime } from "@/lib/dasbor";
import { Alert } from "@/components/ui/Alert";
import { ModalShell } from "@/components/ui/ModalShell";
import { DATASET_LABEL, TITLE_COLUMN, columnLabel, formatCell } from "./digitalisasi";
import { errorMessage } from "./useDigitalisasi";

interface Props {
  dataset: DGDatasetKey;
  id: number;
  onClose: () => void;
  onShowOnMap?: (lat: number, lng: number) => void;
}

export function RecordDetailModal({ dataset, id, onClose, onShowOnMap }: Props) {
  const [data, setData] = useState<DGDetailData | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setData(null);
    setError("");
    getDGDetail(dataset, id)
      .then((res) => {
        if (!cancelled) setData(res.data);
      })
      .catch((err) => {
        if (!cancelled) setError(errorMessage(err, "Gagal memuat detail"));
      });
    return () => {
      cancelled = true;
    };
  }, [dataset, id]);

  const title = data ? String(data.row[TITLE_COLUMN[dataset]] ?? DATASET_LABEL[dataset]) : DATASET_LABEL[dataset];
  const lat = data && typeof data.row.Latitude === "number" ? data.row.Latitude : null;
  const lng = data && typeof data.row.Longitude === "number" ? data.row.Longitude : null;

  return (
    <ModalShell title={title} subtitle={DATASET_LABEL[dataset]} onClose={onClose}>
      <div className="space-y-4 px-4 py-4 sm:px-6">
        {error && <Alert message={error} />}
        {!data && !error && (
          <div className="flex items-center justify-center py-10">
            <Loader2 className="h-5 w-5 animate-spin text-blue-600" />
          </div>
        )}
        {data && (
          <>
            {lat !== null && lng !== null && onShowOnMap && (
              <button
                type="button"
                onClick={() => onShowOnMap(lat, lng)}
                className="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                <MapPin className="h-4 w-4" />
                Lihat di peta
              </button>
            )}
            <dl className="grid gap-x-6 gap-y-3 sm:grid-cols-2">
              {data.kolom.map((c) => {
                const raw = data.row[c.nama];
                const isCoord = c.nama === "Latitude" || c.nama === "Longitude";
                return (
                  <div key={c.nama} className="min-w-0">
                    <dt className="flex items-center gap-1.5 text-xs text-slate-500">
                      {columnLabel(c.nama)}
                      {c.sensitif && (
                        <span className="inline-flex items-center gap-0.5 rounded bg-amber-50 px-1.5 py-0.5 text-[10px] font-medium text-amber-700">
                          <ShieldAlert className="h-3 w-3" aria-hidden="true" />
                          Data pribadi
                        </span>
                      )}
                    </dt>
                    <dd className="mt-0.5 break-words text-sm text-slate-900">
                      {isCoord && typeof raw === "number"
                        ? raw.toFixed(6)
                        : /^Luas_/.test(c.nama) && typeof raw === "number"
                          ? `${formatCell(c, c.nama, raw)} m²`
                          : formatCell(c, c.nama, raw)}
                    </dd>
                  </div>
                );
              })}
              <div className="min-w-0">
                <dt className="text-xs text-slate-500">{columnLabel("synced_at")}</dt>
                <dd className="mt-0.5 text-sm text-slate-900">{formatDateTime(String(data.row.synced_at ?? ""))}</dd>
              </div>
            </dl>
          </>
        )}
      </div>
    </ModalShell>
  );
}
