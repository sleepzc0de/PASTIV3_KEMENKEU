// Pernyataan penggunaan aplikasi: pemeriksaan isian di sisi klien (fungsi murni, diuji lewat lib/persetujuan.test.mjs). Aturannya sama dengan backend/persetujuan;
// backend tetap memeriksa ulang, ini hanya untuk membantu pengguna sebelum mengirim.

export const FRASA_SETUJU = "SAYA SETUJU";

// Frasa dibandingkan tanpa membedakan huruf besar/kecil dan spasi berlebih.
export function frasaSah(s: string): boolean {
  return s.trim().split(/\s+/).join(" ").toUpperCase() === FRASA_SETUJU;
}

export interface ProfilIsian {
  nama: string;
  nip: string;
  email: string;
}

export type GalatProfil = Partial<Record<keyof ProfilIsian, string>>;

// Nama tanpa spasi berlebih, NIP tanpa spasi, email huruf kecil tanpa spasi tepi.
export function rapikanProfil(p: ProfilIsian): ProfilIsian {
  return { nama: p.nama.trim().split(/\s+/).join(" "), nip: p.nip.replace(/\s+/g, ""), email: p.email.trim().toLowerCase() };
}

export function emailSah(email: string): boolean {
  if (email === "" || email.length > 100) return false;
  const m = /^[^\s@<>()",;:]+@([^\s@<>()",;:]+)$/.exec(email);
  if (!m) return false;
  const domain = m[1];
  return domain.includes(".") && !domain.startsWith(".") && !domain.endsWith(".") && !domain.includes("..");
}

// Galat per isian bagi profil yang sudah dirapikan (kosong = sah). Pesan sama dengan backend.
export function validasiProfil(p: ProfilIsian): GalatProfil {
  const g: GalatProfil = {};
  const n = [...p.nama].length;
  if (n < 3) g.nama = "Nama lengkap wajib diisi (minimal 3 karakter)";
  else if (n > 100) g.nama = "Nama lengkap maksimal 100 karakter";
  if (!/^[0-9]{18}$/.test(p.nip) && !/^[0-9]{9}$/.test(p.nip)) g.nip = "NIP wajib diisi dengan 18 digit angka (atau 9 digit untuk NIP lama)";
  if (!emailSah(p.email)) g.email = "Email wajib diisi dengan alamat email yang sah (email kedinasan atau pribadi)";
  return g;
}

// Tombol Setuju aktif bila frasa benar, kotak dicentang, dan (akun non-SSO) profil sah.
export function bolehKirimPersetujuan(o: { frasa: string; dicentang: boolean; isiProfil: boolean; profil: ProfilIsian }): boolean {
  if (!o.dicentang || !frasaSah(o.frasa)) return false;
  return !o.isiProfil || Object.keys(validasiProfil(rapikanProfil(o.profil))).length === 0;
}
