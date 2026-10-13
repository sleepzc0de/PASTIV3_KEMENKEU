// Versi aplikasi yang ditampilkan di footer dan di halaman login. Satu sumber: field "version" di package.json, dibaca saat build oleh next.config.ts
// dan disisipkan sebagai NEXT_PUBLIC_APP_VERSION. Cara menaikkannya ada di docs/versi-aplikasi.md.

export const VERSI_APLIKASI: string = process.env.NEXT_PUBLIC_APP_VERSION ?? "";

// Bentuk yang dipakai: tiga angka, mis. 3.0.0, 3.0.1, 3.1.0.
export function versiSah(v: string): boolean {
  return /^\d+\.\d+\.\d+$/.test(v);
}

// "Versi 3.0.0"; kosong bila versinya tidak ada atau bentuknya tidak sah (lebih baik tidak menampilkan apa pun daripada menampilkan yang salah).
export function teksVersi(v: string = VERSI_APLIKASI): string {
  return versiSah(v) ? `Versi ${v}` : "";
}
