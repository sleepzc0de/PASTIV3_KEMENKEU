// Tanggal dari Inaproc datang dalam beberapa bentuk: ISO UTC ("2021-12-06T00:00:00.000000Z"),
// ISO UTC dengan spasi ("2024-01-10 13:30:00.000000Z"), dan teks lokal tanpa zona
// ("May 7, 2021 12:00:00 AM"). Yang berakhiran Z diformat sebagai UTC supaya
// tanggalnya tidak bergeser; sisanya apa adanya. Bentuk yang tidak bisa dibaca
// ditampilkan mentah.
export function formatDate(value?: string | null): string {
  if (!value) return "-";
  const isUtc = value.endsWith("Z");
  // Sebagian browser menolak pemisah spasi dan pecahan detik lebih dari 3 digit,
  // jadi dinormalkan dulu.
  const normalized = value.replace(/^(\d{4}-\d{2}-\d{2}) /, "$1T").replace(/(\.\d{3})\d+/, "$1");
  const date = new Date(normalized);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString("id-ID", {
    day: "numeric",
    month: "long",
    year: "numeric",
    timeZone: isUtc ? "UTC" : undefined,
  });
}

// Rupiah ditampilkan tanpa desimal; kalau nilainya berpecahan, pecahannya tetap
// ditampilkan (maks. 2 digit) supaya angka yang tampil tidak berbeda dari datanya.
export function formatCurrency(n?: number | null): string {
  if (n === undefined || n === null) return "-";
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
    maximumFractionDigits: Number.isInteger(n) ? 0 : 2,
  }).format(n);
}
