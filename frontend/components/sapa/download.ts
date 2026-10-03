import { namaDariDisposition } from "./sapa";

// Menyimpan hasil unduhan (blob dari axios) sebagai berkas di komputer pengguna.
export function simpanBlob(blob: Blob, disposition: string, cadangan: string): void {
  const nama = namaDariDisposition(disposition, cadangan);
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = nama;
  a.rel = "noopener";
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Pencabutan ditunda: sebagian browser membaca blob setelah klik selesai diproses.
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}
