"use client";

import { useEffect, useState } from "react";
import { AlarmClock } from "lucide-react";
import { Button } from "@/components/ui/Button";

interface IdleWarningModalProps {
  open: boolean;
  countdownSeconds: number;
  onStayLoggedIn: () => void;
  onLogoutNow: () => void;
}

export function IdleWarningModal({
  open,
  countdownSeconds,
  onStayLoggedIn,
  onLogoutNow,
}: IdleWarningModalProps) {
  const [remaining, setRemaining] = useState(countdownSeconds);

  useEffect(() => {
    if (!open) {
      setRemaining(countdownSeconds);
      return;
    }

    setRemaining(countdownSeconds);
    const interval = setInterval(() => {
      setRemaining((prev) => {
        if (prev <= 1) {
          clearInterval(interval);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(interval);
  }, [open, countdownSeconds]);

  if (!open) return null;

  const minutes = Math.floor(remaining / 60);
  const seconds = remaining % 60;
  // Cincin hitung mundur: penuh di awal, habis saat waktu tiba.
  const R = 28;
  const C = 2 * Math.PI * R;
  const fraction = countdownSeconds > 0 ? remaining / countdownSeconds : 0;

  return (
    <div className="fixed inset-0 z-[80] flex animate-fade-in items-end justify-center bg-slate-950/60 backdrop-blur-sm sm:items-center sm:px-4">
      <div role="alertdialog" aria-modal="true" aria-label="Sesi akan berakhir" className="w-full max-w-sm animate-scale-in rounded-t-3xl bg-white p-6 shadow-xl sm:rounded-2xl">
        <div className="flex flex-col items-center text-center">
          <div className="relative flex h-20 w-20 items-center justify-center">
            <svg className="absolute inset-0 -rotate-90" viewBox="0 0 64 64" aria-hidden="true">
              <circle cx="32" cy="32" r={R} fill="none" stroke="#fef3c7" strokeWidth="5" />
              <circle
                cx="32"
                cy="32"
                r={R}
                fill="none"
                stroke="#f59e0b"
                strokeWidth="5"
                strokeLinecap="round"
                strokeDasharray={C}
                strokeDashoffset={C * (1 - fraction)}
                style={{ transition: "stroke-dashoffset 1s linear" }}
              />
            </svg>
            <AlarmClock className="h-7 w-7 text-amber-600" aria-hidden="true" />
          </div>

          <h2 className="mt-4 text-lg font-semibold text-slate-900">Sesi Anda akan berakhir</h2>
          <p className="mt-1 text-sm text-slate-500">Karena tidak ada aktivitas</p>

          <p className="mt-4 text-sm text-slate-600">
            Anda akan otomatis keluar dalam{" "}
            <span aria-live="polite" className="inline-block min-w-[3ch] rounded-md bg-amber-50 px-1.5 py-0.5 text-base font-bold tabular-nums text-amber-700">
              {minutes}:{seconds.toString().padStart(2, "0")}
            </span>
            .
          </p>
        </div>

        <div className="mt-6 flex gap-3">
          <button
            onClick={onLogoutNow}
            className="flex-1 rounded-lg border border-slate-300 bg-white px-4 py-2.5 text-sm font-medium text-slate-700 shadow-sm hover:bg-slate-50 active:scale-[0.97]"
          >
            Keluar Sekarang
          </button>
          <Button onClick={onStayLoggedIn} className="flex-1">
            Tetap Masuk
          </Button>
        </div>
      </div>
    </div>
  );
}
