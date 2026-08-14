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
        <div className="flex items-center gap-2">
          <div className="flex h-[50px] w-[150px] shrink-0 items-center justify-center overflow-hidden rounded-lg border border-slate-300 bg-slate-50">
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
            className="flex h-[50px] w-[50px] shrink-0 items-center justify-center rounded-lg border border-slate-300 text-slate-500 hover:bg-slate-50 disabled:opacity-50"
            title="Muat ulang captcha"
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
            className="h-[50px] flex-1 rounded-lg border border-slate-300 px-3.5 text-sm text-slate-900 outline-none placeholder:text-slate-400 focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
          />
        </div>
        {error && <p className="mt-1.5 text-xs font-medium text-red-500">{error}</p>}
      </div>
    );
  }
);

CaptchaField.displayName = "CaptchaField";