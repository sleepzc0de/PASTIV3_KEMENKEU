// API Inaproc mengirim nip_ppk sebagai angka JSON pada pencatatan-swakelola-realisasi. NIP 18 digit melebihi 2^53, jadi
// JSON.parse membulatkannya dan digit belakangnya berubah diam-diam (198505152010011001 tampil sebagai 198505152010011000).
// Angka 16 digit ke atas dikutip dulu supaya terbaca utuh sebagai teks; angka yang lebih pendek tidak disentuh.
// Dipakai sebagai transformResponse axios, jadi menerima teks respons mentah. Berkas ini sengaja tanpa dependensi supaya
// bisa dites langsung dengan node --test.
export function kutipNipPanjang(data: unknown): unknown {
  if (typeof data !== "string") return data;
  try {
    return JSON.parse(data.replace(/("nip_ppk"\s*:\s*)(\d{16,})/g, '$1"$2"'));
  } catch {
    return data;
  }
}
