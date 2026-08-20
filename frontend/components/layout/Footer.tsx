export function Footer() {
  return (
    <footer className="shrink-0 border-t border-slate-200 bg-white px-6 py-3">
      <p className="text-center text-xs text-slate-400">
        © {new Date().getFullYear()} PASTI V3 — Pemantauan Aset Terintegrasi. Seluruh hak cipta dilindungi.
      </p>
    </footer>
  );
}