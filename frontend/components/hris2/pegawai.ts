// Bentuk data HRIS2 belum terdokumentasi lengkap. Daftar hasil pencarian hanya dijamin memuat
// nama, nip18, dan namaSatker; detail (GetPegawai) memuat lebih banyak. Semua field di sini
// dibaca secara longgar: dipakai bila ada, dilewati bila tidak, dan tidak pernah melempar error.

export type Row = Record<string, unknown>;

function isRecord(v: unknown): v is Row {
  return !!v && typeof v === "object" && !Array.isArray(v);
}

function str(v: unknown): string {
  return typeof v === "string" ? v.trim() : "";
}

// Nilai string pertama yang tidak kosong dari beberapa kemungkinan nama field
// (HRIS2 memakai camelCase, tetapi PascalCase pernah muncul di contoh lain).
function pick(row: Row, keys: string[]): string {
  for (const key of keys) {
    const value = str(row[key]);
    if (value) return value;
  }
  return "";
}

const LIST_KEYS = ["items", "result", "results", "data", "list", "rows"];

function findArray(obj: Row): Row[] | null {
  for (const key of LIST_KEYS) {
    const value = obj[key];
    if (Array.isArray(value)) return value.filter(isRecord);
  }
  return null;
}

function looksLikePegawai(row: Row): boolean {
  return pick(row, ["nama", "Nama"]) !== "" || pick(row, ["nip18", "Nip18", "nip", "NIP"]) !== "";
}

// Respons HRIS2 dibungkus { statusCode, isError, message, data }. Isi `data` bisa berupa array
// pegawai, objek berhalaman ({ items: [...] }), atau satu objek pegawai.
export function extractRows(raw: unknown): Row[] {
  if (Array.isArray(raw)) return raw.filter(isRecord);
  if (!isRecord(raw)) return [];
  const inner = "data" in raw ? raw.data : raw;
  if (Array.isArray(inner)) return inner.filter(isRecord);
  if (!isRecord(inner)) return [];
  const list = findArray(inner);
  if (list) return list;
  // Satu objek: hanya dianggap data pegawai bila memang memuat nama/NIP, supaya amplop
  // kosong seperti { totalCount: 0 } tidak tampil sebagai baris palsu.
  return looksLikePegawai(inner) ? [inner] : [];
}

// Pesan galat dari amplop HRIS2 (isError: true) yang tetap dibalas dengan HTTP 200.
export function extractError(raw: unknown): string | null {
  if (!isRecord(raw) || raw.isError !== true) return null;
  return str(raw.message) || "HRIS2 mengembalikan kesalahan";
}

export interface PegawaiRingkas {
  nip: string;
  nama: string;
  gelarDepan: string;
  gelarBelakang: string;
  satker: string;
  jabatan: string;
  golongan: string;
  foto: string;
}

export function normalizePegawai(row: Row): PegawaiRingkas {
  const jabatanPertama = Array.isArray(row.jabatan) && isRecord(row.jabatan[0]) ? row.jabatan[0] : {};
  const pangkat = isRecord(row.pangkat) ? row.pangkat : {};
  return {
    nip: pick(row, ["nip18", "Nip18", "nip", "NIP"]),
    nama: pick(row, ["nama", "Nama"]),
    gelarDepan: pick(row, ["gelarDepan", "GelarDepan"]),
    gelarBelakang: pick(row, ["gelarBelakang", "GelarBelakang"]),
    satker: pick(row, ["namaSatker", "NamaSatker"]),
    jabatan: pick(row, ["namaJabatan", "NamaJabatan"]) || str(jabatanPertama.namaJabatan),
    golongan: pick(row, ["kodeGolongan", "KodeGolongan"]) || str(pangkat.kodeGolongan),
    foto: pick(row, ["gravatar", "Gravatar"]),
  };
}

// "Dr. Budi Santoso, S.Kom., M.M." dari bagian-bagiannya; gelar yang kosong dilewati.
export function namaLengkap(p: Pick<PegawaiRingkas, "nama" | "gelarDepan" | "gelarBelakang">): string {
  const depan = [p.gelarDepan, p.nama].filter(Boolean).join(" ");
  return p.gelarBelakang && depan ? `${depan}, ${p.gelarBelakang}` : depan;
}

// Inisial untuk avatar pengganti foto: huruf pertama dari dua kata pertama.
export function initials(nama: string): string {
  const words = nama.split(/\s+/).filter(Boolean);
  if (words.length === 0) return "?";
  return words
    .slice(0, 2)
    .map((w) => Array.from(w)[0])
    .join("")
    .toUpperCase();
}

export interface Jabatan {
  namaJabatan: string;
  statusJabatan: string;
  jenisJabatan: string;
  tanggalMulai: string;
  // Unit organisasi dari yang tertinggi ke terendah (esl1..esl4, lalu organisasi), tanpa duplikat.
  unit: string[];
}

function toJabatan(raw: unknown): Jabatan[] {
  if (!Array.isArray(raw)) return [];
  return raw.filter(isRecord).map((j) => {
    const seen = new Set<string>();
    const unit: string[] = [];
    for (const key of ["esl1", "esl2", "esl3", "esl4", "organisasi"]) {
      const value = str(j[key]);
      const id = value.toLowerCase();
      if (value && !seen.has(id)) {
        seen.add(id);
        unit.push(value);
      }
    }
    return {
      namaJabatan: str(j.namaJabatan),
      statusJabatan: str(j.statusJabatan),
      jenisJabatan: str(j.jenisJabatan),
      tanggalMulai: str(j.tanggalMulai),
      unit,
    };
  });
}

export interface PegawaiDetail extends PegawaiRingkas {
  tempatLahir: string;
  tanggalLahir: string;
  jenisKelamin: string;
  noHp: string;
  email: string;
  kdSatker: string;
  status: string;
  namaPangkat: string;
  kodeGolongan: string;
  tmtPangkat: string;
  jabatanList: Jabatan[];
}

export function normalizeDetail(row: Row, nipFallback: string): PegawaiDetail {
  const base = normalizePegawai(row);
  const pangkat = isRecord(row.pangkat) ? row.pangkat : {};
  const status = isRecord(row.status) ? row.status : {};
  return {
    ...base,
    nip: base.nip || nipFallback,
    tempatLahir: pick(row, ["tempatLahir"]),
    tanggalLahir: pick(row, ["tanggalLahir"]),
    jenisKelamin: pick(row, ["jenisKelamin"]),
    noHp: pick(row, ["noHp"]),
    email: pick(row, ["email"]),
    kdSatker: pick(row, ["kdSatker"]),
    status: str(status.uraian),
    namaPangkat: str(pangkat.namaPangkat),
    kodeGolongan: str(pangkat.kodeGolongan) || base.golongan,
    tmtPangkat: str(pangkat.tanggalMulai),
    jabatanList: toJabatan(row.jabatan),
  };
}
