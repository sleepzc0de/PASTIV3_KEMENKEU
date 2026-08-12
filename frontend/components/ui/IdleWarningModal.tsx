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

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4">
      <div className="w-full max-w-sm rounded-xl bg-white p-6 shadow-xl">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-amber-100">
            <AlarmClock className="h-5 w-5 text-amber-600" />
          </div>
          <div>
            <h2 className="text-base font-semibold text-slate-900">
              Sesi Anda akan berakhir
            </h2>
            <p className="text-sm text-slate-500">Karena tidak ada aktivitas</p>
          </div>
        </div>

        <p className="mt-4 text-sm text-slate-600">
          Anda akan otomatis keluar dalam{" "}
          <span className="font-semibold text-slate-900">
            {minutes}:{seconds.toString().padStart(2, "0")}
          </span>{" "}
          jika tidak ada aktivitas.
        </p>

        <div className="mt-6 flex gap-3">
          <button
            onClick={onLogoutNow}
            className="flex-1 rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
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