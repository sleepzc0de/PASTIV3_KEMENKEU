// Pencacah permintaan API yang sedang berjalan. TopLoader mendengarkan kejadian "pasti:net" untuk menampilkan bilah
// kemajuan di atas halaman. Hanya hitungan yang dikirim: tidak ada data permintaan.

let inflight = 0;

function emit() {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("pasti:net", { detail: inflight }));
  }
}

export function netStart(): void {
  inflight++;
  emit();
}

export function netEnd(): void {
  inflight = Math.max(0, inflight - 1);
  emit();
}

export function netCount(): number {
  return inflight;
}
