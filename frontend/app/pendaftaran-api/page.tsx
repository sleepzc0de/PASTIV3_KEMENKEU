import type { Metadata } from "next";
import Link from "next/link";
import { ClipboardCheck, FileSearch, KeyRound, ShieldCheck } from "lucide-react";
import { PendaftaranForm } from "@/components/apiakses/PendaftaranForm";
import { teksVersi } from "@/lib/versi";

export const metadata: Metadata = {
  title: "Pendaftaran API Akses Data · PASTI V3",
  description: "Daftarkan aplikasi Anda untuk menarik data PASTI lewat API.",
};

const LANGKAH = [
  { ikon: ClipboardCheck, judul: "Daftar", teks: "Isi formulir ini: aplikasi, instansi, PIC, tujuan, dan dataset yang dibutuhkan." },
  { ikon: FileSearch, judul: "Dinilai", teks: "Administrator PASTI menilai dan menentukan dataset serta batas pemakaian yang diberikan." },
  { ikon: KeyRound, judul: "Terima kunci", teks: "Bila disetujui, kunci API diserahkan kepada PIC lewat jalur yang aman. Kunci hanya ditampilkan sekali." },
];

// Halaman pendaftaran publik (tanpa login) untuk aplikasi yang ingin memakai API Akses Data PASTI.
export default function PendaftaranApiPage() {
  return (
    <div className="min-h-dvh bg-gradient-to-b from-slate-50 to-white">
      <header className="border-b border-slate-200/70 bg-white/80 backdrop-blur">
        <div className="mx-auto flex max-w-3xl items-center gap-3 px-5 py-4">
          <span className="flex h-10 w-10 items-center justify-center rounded-2xl bg-gradient-to-br from-blue-500 to-blue-700 text-white shadow-md shadow-blue-600/25">
            <ShieldCheck className="h-5 w-5" aria-hidden="true" />
          </span>
          <div>
            <p className="text-base font-bold leading-tight tracking-tight text-slate-900">PASTI V3</p>
            <p className="text-xs text-slate-500">Pemantauan Aset Terintegrasi</p>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-3xl px-5 py-8 sm:py-10">
        <h1 className="text-balance text-2xl font-bold tracking-tight text-slate-900 sm:text-3xl">Pendaftaran API Akses Data</h1>
        <p className="mt-2 max-w-2xl text-sm leading-relaxed text-slate-600">
          Aplikasi lain yang ingin menarik data PASTI harus mendaftar terlebih dahulu. Setelah disetujui, aplikasi Anda memakai kunci API yang diterbitkan PASTI, dengan hak akses dan batas pemakaian yang
          ditentukan administrator.
        </p>

        <ol className="mt-6 grid gap-3 sm:grid-cols-3">
          {LANGKAH.map(({ ikon: Ikon, judul, teks }, i) => (
            <li key={judul} className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
              <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-50 text-blue-700">
                <Ikon className="h-[18px] w-[18px]" aria-hidden="true" />
              </span>
              <p className="mt-3 text-sm font-semibold text-slate-900">
                {i + 1}. {judul}
              </p>
              <p className="mt-1 text-xs leading-relaxed text-slate-500">{teks}</p>
            </li>
          ))}
        </ol>

        <div className="mt-8 rounded-2xl border border-slate-200 bg-white p-5 shadow-card sm:p-7">
          <PendaftaranForm />
        </div>

        <p className="mt-6 text-center text-xs text-slate-400">
          Sudah punya akun PASTI?{" "}
          <Link href="/" className="font-medium text-blue-700 hover:underline">
            Masuk ke PASTI
          </Link>
          {teksVersi() && <> · PASTI V3 · {teksVersi()}</>}
        </p>
      </main>
    </div>
  );
}
