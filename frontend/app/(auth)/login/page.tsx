"use client";

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { LoginForm } from "@/components/auth/LoginForm";
import { Alert } from "@/components/ui/Alert";
import { BarChart3, DatabaseZap, FileCheck2, MapPinned, ShieldCheck } from "lucide-react";

function LoginErrorBanner() {
  const searchParams = useSearchParams();
  const error = searchParams.get("error");
  const reason = searchParams.get("reason");

  if (error === "sso_failed") {
    return (
      <div className="mb-5">
        <Alert message={reason ? `Login SSO gagal: ${reason}` : "Login SSO gagal, silakan coba lagi."} />
      </div>
    );
  }

  if (error === "session_expired") {
    return (
      <div className="mb-5">
        <Alert tone="warning" message={reason || "Sesi Anda telah berakhir, silakan masuk kembali."} />
      </div>
    );
  }

  return null;
}

const FEATURES = [
  { icon: DatabaseZap, title: "Data aset terpadu", text: "Aset SLDK, pengadaan Inaproc, dan data pegawai dalam satu tempat." },
  { icon: MapPinned, title: "Peta dan analitik", text: "Sebaran aset, kondisi, dan kelengkapan data yang mudah dibaca." },
  { icon: FileCheck2, title: "Administrasi otomatis", text: "SAPA menyusun dokumen penjualan BMN dari formulir menjadi Word." },
  { icon: BarChart3, title: "Pemantauan berkelanjutan", text: "Temukan anomali data lebih dini dengan ringkasan yang selalu mutakhir." },
];

export default function LoginPage() {
  return (
    <div className="flex min-h-dvh bg-white">
      {/* Panel merek (desktop) */}
      <div className="relative hidden w-[52%] flex-col justify-between overflow-hidden bg-slate-950 p-12 text-white lg:flex xl:p-16">
        <div aria-hidden="true" className="absolute inset-0 bg-[radial-gradient(60rem_40rem_at_0%_0%,rgba(79,115,242,0.55),transparent),radial-gradient(40rem_30rem_at_100%_100%,rgba(51,88,224,0.45),transparent)]" />
        <div
          aria-hidden="true"
          className="absolute inset-0 opacity-[0.07] [background-image:linear-gradient(white_1px,transparent_1px),linear-gradient(90deg,white_1px,transparent_1px)] [background-size:44px_44px] [mask-image:radial-gradient(ellipse_at_center,black_30%,transparent_75%)]"
        />
        <div aria-hidden="true" className="absolute -bottom-40 -right-24 h-[28rem] w-[28rem] rounded-full bg-blue-500/20 blur-3xl" />

        <div className="relative z-10 flex items-center gap-3">
          <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-white/10 ring-1 ring-white/20 backdrop-blur">
            <ShieldCheck className="h-6 w-6" />
          </span>
          <div>
            <p className="text-lg font-bold leading-tight tracking-tight">PASTI V3</p>
            <p className="text-xs text-blue-200">Pemantauan Aset Terintegrasi</p>
          </div>
        </div>

        <div className="relative z-10 max-w-lg">
          <h1 className="text-balance text-4xl font-bold leading-[1.15] tracking-tight xl:text-5xl">Kelola aset negara dengan lebih tenang.</h1>
          <p className="mt-4 text-base leading-relaxed text-blue-100/90">Kelola, pantau, dan lindungi seluruh aset dalam satu platform yang aman dan terintegrasi.</p>

          <ul className="stagger mt-10 grid gap-3.5 sm:grid-cols-2">
            {FEATURES.map(({ icon: Icon, title, text }) => (
              <li key={title} className="rounded-2xl bg-white/[0.06] p-4 ring-1 ring-white/10 backdrop-blur-sm transition hover:bg-white/10">
                <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-500/25 text-blue-200">
                  <Icon className="h-[18px] w-[18px]" aria-hidden="true" />
                </span>
                <p className="mt-3 text-sm font-semibold">{title}</p>
                <p className="mt-1 text-xs leading-relaxed text-blue-100/70">{text}</p>
              </li>
            ))}
          </ul>
        </div>

        <p className="relative z-10 text-xs text-blue-200/80">© {new Date().getFullYear()} PASTI V3. Seluruh hak cipta dilindungi.</p>
      </div>

      {/* Formulir */}
      <div className="relative flex w-full flex-col items-center justify-center bg-gradient-to-b from-slate-50 to-white px-6 py-10 sm:py-12 lg:w-[48%]">
        <div className="w-full max-w-sm animate-fade-up">
          <div className="mb-8">
            {/* Panel merek di kiri tersembunyi di bawah lg, jadi tampilkan identitas di sini. */}
            <div className="mb-8 flex items-center gap-3 lg:hidden">
              <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-gradient-to-br from-blue-500 to-blue-700 text-white shadow-lg shadow-blue-600/30">
                <ShieldCheck className="h-6 w-6" />
              </span>
              <div>
                <p className="text-lg font-bold leading-tight tracking-tight text-slate-900">PASTI V3</p>
                <p className="text-xs text-slate-500">Pemantauan Aset Terintegrasi</p>
              </div>
            </div>
            <h2 className="text-2xl font-bold tracking-tight text-slate-900">Selamat datang kembali</h2>
            <p className="mt-1.5 text-sm text-slate-500">Masuk ke akun Anda untuk melanjutkan ke dashboard.</p>
          </div>

          <Suspense fallback={null}>
            <LoginErrorBanner />
          </Suspense>

          <LoginForm />
        </div>
      </div>
    </div>
  );
}
