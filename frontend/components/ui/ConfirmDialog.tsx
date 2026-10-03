"use client";

import { ReactNode, useEffect, useRef } from "react";
import { AlertTriangle, Trash2 } from "lucide-react";
import { Button } from "./Button";

interface ConfirmDialogProps {
  title: string;
  message: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  tone?: "danger" | "warning";
  busy?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

// Pengganti window.confirm: konsisten dengan tampilan aplikasi, bisa ditutup dengan Escape, dan fokus awal ada di tombol
// "Batal" supaya tindakan merusak tidak terpicu tanpa sengaja oleh Enter.
export function ConfirmDialog({ title, message, confirmLabel = "Ya, lanjutkan", cancelLabel = "Batal", tone = "danger", busy, onConfirm, onCancel }: ConfirmDialogProps) {
  const cancelRef = useRef<HTMLButtonElement>(null);
  const onCancelRef = useRef(onCancel);
  useEffect(() => {
    onCancelRef.current = onCancel;
  });

  useEffect(() => {
    cancelRef.current?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancelRef.current();
    };
    document.addEventListener("keydown", onKey);
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = overflow;
    };
  }, []);

  const Icon = tone === "danger" ? Trash2 : AlertTriangle;
  const iconCls = tone === "danger" ? "bg-red-50 text-red-600" : "bg-amber-50 text-amber-600";

  return (
    <div
      className="fixed inset-0 z-[55] !mt-0 flex animate-fade-in items-end justify-center bg-slate-950/50 backdrop-blur-sm sm:items-center sm:p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && !busy) onCancel();
      }}
    >
      <div role="alertdialog" aria-modal="true" aria-label={title} className="w-full max-w-md animate-scale-in rounded-t-3xl bg-white p-6 shadow-xl sm:rounded-2xl">
        <div className="flex items-start gap-4">
          <span className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl ${iconCls}`}>
            <Icon className="h-5 w-5" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-slate-900">{title}</h2>
            <div className="mt-1.5 text-sm leading-relaxed text-slate-600">{message}</div>
          </div>
        </div>
        <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            ref={cancelRef}
            type="button"
            onClick={onCancel}
            disabled={busy}
            className="rounded-lg border border-slate-300 bg-white px-4 py-2.5 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-60"
          >
            {cancelLabel}
          </button>
          <Button variant={tone === "danger" ? "danger" : "primary"} fullWidth={false} isLoading={busy} onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}
