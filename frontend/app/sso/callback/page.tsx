"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Cookies from "js-cookie";
import { Loader2, ShieldAlert } from "lucide-react";

export default function SSOCallbackPage() {
  const router = useRouter();
  const [error, setError] = useState(false);

  useEffect(() => {
    const hash = window.location.hash.replace("#", "");
    const params = new URLSearchParams(hash);
    const token = params.get("token");
    const expiresIn = params.get("expires_in");

    if (!token) {
      setError(true);
      return;
    }

    const expiresInDays = expiresIn ? Number(expiresIn) / 86400 : 1 / 96; // fallback ~15 menit

    Cookies.set("pasti_access_token", token, {
      expires: expiresInDays,
      secure: process.env.NODE_ENV === "production",
      sameSite: "strict",
    });

    // Bersihkan fragment dari URL demi keamanan (agar token tidak tersimpan di history)
    window.history.replaceState(null, "", window.location.pathname);

    router.replace("/dashboard");
  }, [router]);

  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-3 bg-slate-50 px-6 text-center">
        <ShieldAlert className="h-10 w-10 text-red-500" />
        <h1 className="text-lg font-semibold text-slate-900">Login SSO Gagal</h1>
        <p className="max-w-sm text-sm text-slate-500">
          Terjadi kesalahan saat memproses login SSO Kemenkeu. Silakan coba lagi.
        </p>
        <a href="/login" className="mt-2 text-sm font-medium text-blue-600 hover:underline">
          Kembali ke halaman login
        </a>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-3 bg-slate-50">
      <Loader2 className="h-8 w-8 animate-spin text-blue-600" />
      <p className="text-sm text-slate-500">Memproses login SSO Kemenkeu...</p>
    </div>
  );
}