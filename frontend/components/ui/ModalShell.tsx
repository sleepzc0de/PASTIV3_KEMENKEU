"use client";

import { ReactNode, useEffect, useRef, useState } from "react";
import { X } from "lucide-react";

interface ModalShellProps {
  title: string;
  subtitle?: string;
  onClose: () => void;
  children: ReactNode;
}

// Kerangka modal: header menempel, Escape dan klik di luar menutup, fokus pindah ke tombol Tutup saat dibuka
// dan kembali ke elemen pembukanya saat ditutup.
export function ModalShell({ title, subtitle, onClose, children }: ModalShellProps) {
  // Elemen pembuka dicatat saat render pertama, sebelum fokus dipindah.
  const [opener] = useState<HTMLElement | null>(() =>
    typeof document !== "undefined" && document.activeElement instanceof HTMLElement ? document.activeElement : null
  );
  // onClose dari induk biasanya fungsi inline yang berganti tiap render; disimpan di ref agar efek cukup berjalan sekali.
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  });
  const closeButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCloseRef.current();
    };
    document.addEventListener("keydown", onKeyDown);
    closeButtonRef.current?.focus();
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      opener?.focus();
    };
  }, [opener]);

  return (
    // `!mt-0`: modal dirender di dalam wadah `space-y-*` yang memberi margin-top pada setiap anaknya; tanpa ini
    // overlay `fixed inset-0` bergeser dan menyisakan celah di atas.
    <div
      className="fixed inset-0 z-50 !mt-0 flex items-center justify-center bg-black/50 p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="max-h-[90dvh] w-full max-w-3xl overflow-y-auto overscroll-contain rounded-xl bg-white shadow-xl"
      >
        <div className="sticky top-0 z-10 flex items-start justify-between gap-3 border-b border-slate-200 bg-white px-4 py-4 sm:px-6">
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-slate-900">{title}</h2>
            {subtitle && <p className="mt-0.5 break-words text-xs text-slate-500">{subtitle}</p>}
          </div>
          <button
            type="button"
            ref={closeButtonRef}
            onClick={onClose}
            aria-label="Tutup"
            className="shrink-0 rounded-md p-1.5 text-slate-400 hover:bg-slate-100"
          >
            <X className="h-5 w-5" />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}
