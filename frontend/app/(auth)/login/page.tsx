import { LoginForm } from "@/components/auth/LoginForm";
import { ShieldCheck } from "lucide-react";

export default function LoginPage() {
  return (
    <div className="flex min-h-screen">
      {/* Panel kiri — branding */}
      <div className="relative hidden w-1/2 flex-col justify-between overflow-hidden bg-gradient-to-br from-blue-700 via-blue-800 to-slate-900 p-12 text-white lg:flex">
        <div className="absolute -right-24 -top-24 h-96 w-96 rounded-full bg-white/5" />
        <div className="absolute -bottom-32 -left-10 h-80 w-80 rounded-full bg-white/5" />

        <div className="relative z-10 flex items-center gap-2 text-lg font-bold">
          <ShieldCheck className="h-7 w-7" />
          PASTI V3
        </div>

        <div className="relative z-10 max-w-md">
          <h1 className="text-3xl font-bold leading-tight">
            Pemantauan Aset Terintegrasi
          </h1>
          <p className="mt-4 text-blue-100">
            Kelola, pantau, dan lindungi seluruh aset organisasi Anda dalam satu
            platform yang aman dan terintegrasi.
          </p>
        </div>

        <p className="relative z-10 text-xs text-blue-200">
          © {new Date().getFullYear()} PASTI V3. Seluruh hak cipta dilindungi.
        </p>
      </div>

      {/* Panel kanan — form login */}
      <div className="flex w-full flex-col items-center justify-center bg-slate-50 px-6 py-12 lg:w-1/2">
        <div className="w-full max-w-sm">
          <div className="mb-8 text-center lg:text-left">
            <h2 className="text-2xl font-bold text-slate-900">Selamat Datang Kembali</h2>
            <p className="mt-1.5 text-sm text-slate-500">
              Masuk ke akun Anda untuk melanjutkan ke dashboard
            </p>
          </div>
          <LoginForm />
        </div>
      </div>
    </div>
  );
}