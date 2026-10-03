import type { UserListItem } from "@/lib/api";

// Pencarian pengguna dilakukan di sisi klien: daftar pengguna dimuat utuh (tanpa paginasi),
// jadi hasilnya instan dan tidak perlu perubahan API.

// Kata pencarian: huruf kecil, dipisah spasi. Setiap kata harus cocok (AND), sehingga
// "budi biro" menemukan pengguna bernama Budi di satker yang memuat "biro".
export function parseTerms(query: string): string[] {
  return query.toLowerCase().split(/\s+/).filter(Boolean);
}

export function matchesUser(
  u: Pick<UserListItem, "full_name" | "username" | "email" | "nip" | "jabatan" | "satker">,
  terms: string[]
): boolean {
  if (terms.length === 0) return true;
  const haystack = [u.full_name, u.username, u.email, u.nip, u.jabatan, u.satker]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
  return terms.every((term) => haystack.includes(term));
}
