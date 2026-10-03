"use client";

import { ReactNode, createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { AlertCircle, CheckCircle2, Info, TriangleAlert, X } from "lucide-react";

type Tone = "success" | "error" | "info" | "warning";

interface ToastItem {
  id: number;
  tone: Tone;
  message: string;
  title?: string;
}

interface ToastApi {
  success: (message: string, title?: string) => void;
  error: (message: string, title?: string) => void;
  info: (message: string, title?: string) => void;
  warning: (message: string, title?: string) => void;
}

const NOOP: ToastApi = { success() {}, error() {}, info() {}, warning() {} };
const ToastContext = createContext<ToastApi>(NOOP);

// Pemberitahuan ringan yang hilang sendiri. Dipakai untuk umpan balik singkat (tersimpan, tersalin, gagal) agar pengguna
// tidak perlu mencari pesan di dalam halaman.
export function useToast(): ToastApi {
  return useContext(ToastContext);
}

const TONE: Record<Tone, { icon: typeof Info; ring: string; iconCls: string }> = {
  success: { icon: CheckCircle2, ring: "ring-emerald-200", iconCls: "text-emerald-600 bg-emerald-50" },
  error: { icon: AlertCircle, ring: "ring-red-200", iconCls: "text-red-600 bg-red-50" },
  warning: { icon: TriangleAlert, ring: "ring-amber-200", iconCls: "text-amber-600 bg-amber-50" },
  info: { icon: Info, ring: "ring-blue-200", iconCls: "text-blue-600 bg-blue-50" },
};

const DURATION: Record<Tone, number> = { success: 3800, info: 4500, warning: 6000, error: 7000 };
const MAX_VISIBLE = 4;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const seq = useRef(0);

  const dismiss = useCallback((id: number) => setItems((cur) => cur.filter((t) => t.id !== id)), []);

  const push = useCallback((tone: Tone, message: string, title?: string) => {
    const id = ++seq.current;
    setItems((cur) => [...cur.slice(-(MAX_VISIBLE - 1)), { id, tone, message, title }]);
  }, []);

  const api = useMemo<ToastApi>(
    () => ({
      success: (m, t) => push("success", m, t),
      error: (m, t) => push("error", m, t),
      info: (m, t) => push("info", m, t),
      warning: (m, t) => push("warning", m, t),
    }),
    [push]
  );

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div
        aria-live="polite"
        className="pointer-events-none fixed inset-x-0 bottom-0 z-[60] flex flex-col items-center gap-2 p-4 sm:inset-x-auto sm:right-0 sm:items-end"
      >
        {items.map((t) => (
          <ToastCard key={t.id} item={t} onDismiss={dismiss} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

function ToastCard({ item, onDismiss }: { item: ToastItem; onDismiss: (id: number) => void }) {
  const [paused, setPaused] = useState(false);
  const { icon: Icon, ring, iconCls } = TONE[item.tone];

  // Hitung mundur berhenti saat kursor di atas kartu atau tab tidak terlihat, supaya pesan tidak hilang sebelum terbaca.
  useEffect(() => {
    if (paused) return;
    const t = setTimeout(() => onDismiss(item.id), DURATION[item.tone]);
    return () => clearTimeout(t);
  }, [paused, item.id, item.tone, onDismiss]);

  return (
    <div
      role={item.tone === "error" ? "alert" : "status"}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      className={`pointer-events-auto flex w-full max-w-sm animate-slide-in-right items-start gap-3 rounded-2xl bg-white p-3.5 shadow-lg ring-1 ${ring}`}
    >
      <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-xl ${iconCls}`}>
        <Icon className="h-4 w-4" aria-hidden="true" />
      </span>
      <div className="min-w-0 flex-1 pt-0.5">
        {item.title && <p className="text-sm font-semibold text-slate-900">{item.title}</p>}
        <p className={`break-words text-sm ${item.title ? "text-slate-600" : "font-medium text-slate-800"}`}>{item.message}</p>
      </div>
      <button
        type="button"
        onClick={() => onDismiss(item.id)}
        aria-label="Tutup pemberitahuan"
        className="shrink-0 rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}
