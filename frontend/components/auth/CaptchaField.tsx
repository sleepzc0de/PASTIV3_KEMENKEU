"use client";

import { useEffect, useState, forwardRef } from "react";
import { RefreshCw, Loader2 } from "lucide-react";
import { fetchCaptcha } from "@/lib/api";

interface CaptchaFieldProps {
  onCaptchaIdChange: (id: string) => void;
  error?: string;
  value: string;
  onChange: (value: string) => void;
}

export const CaptchaField = forwardRef<HTMLInputElement, CaptchaFieldProps>(
  ({ onCaptchaIdChange, error, value, onChange }, ref) => {
    const [captchaImage, setCaptchaImage] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(false);

    const loadCaptcha = async () => {
      setIsLoading(true);
      try {
        const res = await fetchCaptcha();
        setCaptchaImage(res.data.captcha_image);
        onCaptchaIdChange(res.data.captcha_id);
        onChange(""); // reset input jawaban setiap captcha di-refresh
      } catch {
        setCaptchaImage(null);
      } finally {
        setIsLoading(false);
      }
    };

    useEffect(() => {
      loadCaptcha();
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    return (
      <div className="w-full">
        <label className="mb-1.5 block text-sm font-medium text-slate-700">
          Kode Keamanan
        </label>
        {/* flex-wrap + min-w pada input: di layar sempit input turun ke baris sendiri,
            bukan melebihi lebar form (lebar bawaan <input> sekitar 190px). */}
        <div className="flex flex-wrap items-center gap-2">
          <div className="flex h-[50px] w-[150px] shrink-0 items-center justify-center overflow-hidden rounded-xl border border-slate-300 bg-slate-50">
            {isLoading ? (
              <Loader2 className="h-4 w-4 animate-spin text-slate-400" />
            ) : captchaImage ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={captchaImage} alt="Captcha" className="h-full w-full object-cover" />
            ) : (
              <span className="text-xs text-slate-400">Gagal memuat</span>
            )}
          </div>
          <button
            type="button"
            onClick={loadCaptcha}
            disabled={isLoading}
            className="flex h-[50px] w-[50px] shrink-0 items-center justify-center rounded-xl border border-slate-300 bg-white text-slate-500 shadow-sm hover:border-slate-400 hover:bg-slate-50 hover:text-blue-600 active:scale-95 disabled:opacity-50"
            title="Muat ulang captcha"
            aria-label="Muat ulang captcha"
          >
            <RefreshCw className={`h-4 w-4 ${isLoading ? "animate-spin" : ""}`} />
          </button>
          <input
            ref={ref}
            type="text"
            inputMode="numeric"
            placeholder="Masukkan kode"
            value={value}
            onChange={(e) => onChange(e.target.value)}
            aria-label="Kode keamanan"
            className="h-[50px] min-w-[8rem] flex-1 rounded-xl border border-slate-300 bg-white px-3.5 text-sm text-slate-900 shadow-sm outline-none placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow"
          />
        </div>
        {error && <p className="mt-1.5 animate-fade-in text-xs font-medium text-red-600">{error}</p>}
      </div>
    );
  }
);

CaptchaField.displayName = "CaptchaField";