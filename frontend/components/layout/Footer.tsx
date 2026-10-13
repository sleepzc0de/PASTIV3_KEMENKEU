import { teksVersi } from "@/lib/versi";

export function Footer() {
  const versi = teksVersi();
  return (
    <footer className="shrink-0 px-6 pb-5 pt-2">
      <p className="text-center text-xs text-slate-400">
        © {new Date().getFullYear()} PASTI V3 · Pemantauan Aset Terintegrasi{versi && ` · ${versi}`}
      </p>
    </footer>
  );
}
