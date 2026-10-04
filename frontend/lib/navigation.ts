import {
  LayoutDashboard,
  DatabaseZap,
  Users,
  Users2,
  FileClock,
  Wallet,
  WalletCards,
  Package,
  PackageCheck,
  Boxes,
  ClipboardCheck,
  LayoutList,
  ShoppingCart,
  Gavel,
  CalendarClock,
  CalendarRange,
  FileSignature,
  ScrollText,
  Megaphone,
  BadgeCheck,
  ClipboardList,
  Receipt,
  NotebookPen,
  ReceiptText,
  Newspaper,
  UsersRound,
  FileCheck2,
  FileText,
  Trophy,
  Coins,
  MapPinned,
  Landmark,
  HandCoins,
  Settings2,
} from "lucide-react";

export interface NavItem {
  label: string;
  href: string;
  icon: React.ElementType;
  description?: string;
  roles?: string[];
  // Menu tetap aktif di halaman di bawahnya (mis. detail usulan). Tanpa ini, hanya alamat yang persis sama yang aktif.
  matchPrefix?: boolean;
}

export function isNavActive(pathname: string, item: Pick<NavItem, "href" | "matchPrefix">): boolean {
  return pathname === item.href || (item.matchPrefix === true && pathname.startsWith(item.href + "/"));
}

export interface NavGroup {
  label: string;
  icon: React.ElementType;
  roles?: string[];
  children: NavItem[];
}

export type NavEntry =
  | ({ type: "item" } & NavItem)
  | ({ type: "group" } & NavGroup);

// Satu-satunya sumber daftar menu: dipakai Sidebar dan kartu "Akses Cepat"
// di halaman dashboard supaya keduanya tidak pernah berbeda.
export const NAV_ENTRIES: NavEntry[] = [
  { type: "item", label: "Dashboard", href: "/dashboard", icon: LayoutDashboard },
  {
    type: "item",
    label: "Data Aset (SLDK)",
    href: "/dashboard/assets",
    icon: DatabaseZap,
    description: "Cari dan telusuri data aset dari SLDK",
  },
  {
    type: "item",
    label: "Digitalisasi Aset",
    href: "/dashboard/digitalisasi",
    icon: MapPinned,
    description: "Peta, analitik, dan sinkronisasi data aset KL 015 dari SLDK",
  },
  {
    type: "group",
    label: "SAPA",
    icon: Landmark,
    children: [
      {
        label: "Penjualan",
        href: "/dashboard/sapa/penjualan",
        icon: HandCoins,
        description: "Usulan penjualan BMN: pembentukan tim, berita acara, dan nota dinas dari satker hingga UE1",
        matchPrefix: true,
      },
      {
        label: "Pengaturan SAPA",
        href: "/dashboard/sapa/pengaturan",
        icon: Settings2,
        description: "Template dokumen Word, peran pengguna SAPA, dan referensi Unit Eselon I",
        roles: ["admin", "superadmin"],
      },
    ],
  },
  {
    type: "group",
    label: "Pengadaan (Inaproc)",
    icon: ShoppingCart,
    children: [
      {
        label: "Kaji Ulang RUP",
        href: "/dashboard/pengadaan",
        icon: FileClock,
        description: "Riwayat kaji ulang paket RUP",
      },
      {
        label: "Paket Anggaran",
        href: "/dashboard/pengadaan/paket-anggaran",
        icon: Wallet,
        description: "Anggaran paket penyedia",
      },
      {
        label: "Anggaran Swakelola",
        href: "/dashboard/pengadaan/anggaran-swakelola",
        icon: WalletCards,
        description: "Anggaran paket swakelola",
      },
      {
        label: "Paket Penyedia",
        href: "/dashboard/pengadaan/paket-penyedia",
        icon: Package,
        description: "Daftar paket pengadaan penyedia",
      },
      {
        label: "Penyedia Terumumkan",
        href: "/dashboard/pengadaan/penyedia-terumumkan",
        icon: PackageCheck,
        description: "Paket penyedia yang telah diumumkan",
      },
      {
        label: "Paket Swakelola",
        href: "/dashboard/pengadaan/paket-swakelola",
        icon: Boxes,
        description: "Daftar paket swakelola",
      },
      {
        label: "Swakelola Terumumkan",
        href: "/dashboard/pengadaan/swakelola-terumumkan",
        icon: ClipboardCheck,
        description: "Paket swakelola yang telah diumumkan",
      },
      {
        label: "Program Master",
        href: "/dashboard/pengadaan/program-master",
        icon: LayoutList,
        description: "Master program dan pagu program",
      },
    ],
  },
  {
    type: "group",
    label: "Tender (Inaproc)",
    icon: Gavel,
    children: [
      {
        label: "Jadwal Non Tender",
        href: "/dashboard/tender/jadwal-non-tender",
        icon: CalendarClock,
        description: "Jadwal tahapan pengadaan non tender",
      },
      {
        label: "Jadwal Tender",
        href: "/dashboard/tender/jadwal-tender",
        icon: CalendarRange,
        description: "Jadwal tahapan tender",
      },
      {
        label: "Non Tender E-Kontrak",
        href: "/dashboard/tender/non-tender-ekontrak",
        icon: FileSignature,
        description: "Riwayat BAP/BAST, SPMK/SPP, dan penilaian kinerja penyedia",
      },
      {
        label: "Kontrak Non Tender",
        href: "/dashboard/tender/non-tender-ekontrak-kontrak",
        icon: ScrollText,
        description: "Data kontrak, nilai, penyedia, dan PPK",
      },
      {
        label: "Pengumuman Non Tender",
        href: "/dashboard/tender/non-tender-pengumuman",
        icon: Megaphone,
        description: "Pengumuman paket non tender: pagu, HPS, metode, dan status",
      },
      {
        label: "Non Tender Selesai",
        href: "/dashboard/tender/non-tender-selesai",
        icon: BadgeCheck,
        description: "Paket non tender yang selesai: penyedia, nilai penawaran, negosiasi, dan kontrak",
      },
      {
        label: "Pencatatan Non Tender",
        href: "/dashboard/tender/pencatatan-non-tender",
        icon: ClipboardList,
        description: "Paket non tender yang dicatat: pagu, total realisasi, PPK, dan status",
      },
      {
        label: "Realisasi Pencatatan",
        href: "/dashboard/tender/pencatatan-non-tender-realisasi",
        icon: Receipt,
        description: "Realisasi pencatatan non tender: nomor, jenis, nilai, penyedia, dan tanggal",
      },
      {
        label: "Pencatatan Swakelola",
        href: "/dashboard/tender/pencatatan-swakelola",
        icon: NotebookPen,
        description: "Paket swakelola yang dicatat: tipe, pagu, total realisasi, PPK, dan status",
      },
      {
        label: "Realisasi Swakelola",
        href: "/dashboard/tender/pencatatan-swakelola-realisasi",
        icon: ReceiptText,
        description: "Realisasi pencatatan swakelola: nomor, jenis, nilai, pelaksana, dan tanggal",
      },
      {
        label: "Pengumuman Tender",
        href: "/dashboard/tender/pengumuman",
        icon: Newspaper,
        description: "Pengumuman tender: pagu, HPS, metode, PPK/Pokja, dan status; bisa dicari dengan kode tender",
      },
      {
        label: "Peserta Tender",
        href: "/dashboard/tender/peserta-tender",
        icon: UsersRound,
        description: "Peserta tender: penyedia, nilai penawaran, dan pemenang",
      },
      {
        label: "Tender E-Kontrak",
        href: "/dashboard/tender/tender-ekontrak",
        icon: FileCheck2,
        description: "Kontrak tender beserta riwayat BAP/BAST, SPMK/SPP, dan penilaian kinerja penyedia",
      },
      {
        label: "Kontrak Tender",
        href: "/dashboard/tender/tender-ekontrak-kontrak",
        icon: FileText,
        description: "Data kontrak tender: nilai, penyedia, dan PPK",
      },
      {
        label: "Tender Selesai",
        href: "/dashboard/tender/tender-selesai",
        icon: Trophy,
        description: "Tender yang selesai: pagu, HPS, metode, status, dan tanggal penetapan pemenang",
      },
      {
        label: "Nilai Tender Selesai",
        href: "/dashboard/tender/tender-selesai-nilai",
        icon: Coins,
        description: "Nilai penawaran, negosiasi, dan kontrak per penyedia pemenang",
      },
    ],
  },
  {
    type: "item",
    label: "Cari Pegawai (HRIS2)",
    href: "/dashboard/pegawai",
    icon: Users2,
    description: "Pencarian dan detail data pegawai",
    roles: ["admin", "superadmin"],
  },
  {
    type: "item",
    label: "Manajemen Pengguna",
    href: "/dashboard/users",
    icon: Users,
    description: "Kelola akun dan peran pengguna",
    roles: ["admin", "superadmin"],
  },
];

export interface FlatNavItem extends NavItem {
  group?: string;
}

// Daftar datar semua menu yang boleh dilihat peran ini (untuk palet perintah dan remah roti).
export function flattenNav(role?: string): FlatNavItem[] {
  const ok = (roles?: string[]) => !roles || (role !== undefined && roles.includes(role));
  const out: FlatNavItem[] = [];
  for (const e of NAV_ENTRIES) {
    if (!ok(e.roles)) continue;
    if (e.type === "item") {
      out.push(e);
    } else {
      for (const c of e.children) if (ok(c.roles)) out.push({ ...c, group: e.label });
    }
  }
  return out;
}

export interface Crumb {
  label: string;
  href?: string;
}

// Remah roti dari alamat saat ini: "Beranda > Grup > Menu [> Detail]". Halaman di bawah menu yang `matchPrefix`
// (mis. detail usulan SAPA) mendapat satu remah tambahan "Detail".
export function breadcrumbsFor(pathname: string): Crumb[] {
  if (pathname === "/dashboard") return [{ label: "Beranda" }];
  const crumbs: Crumb[] = [{ label: "Beranda", href: "/dashboard" }];
  for (const e of NAV_ENTRIES) {
    if (e.type === "item") {
      if (isNavActive(pathname, e)) {
        crumbs.push({ label: e.label, href: pathname === e.href ? undefined : e.href });
        if (pathname !== e.href) crumbs.push({ label: "Detail" });
        return crumbs;
      }
    } else {
      const child = e.children.find((c) => isNavActive(pathname, c));
      if (child) {
        crumbs.push({ label: e.label });
        crumbs.push({ label: child.label, href: pathname === child.href ? undefined : child.href });
        if (pathname !== child.href) crumbs.push({ label: "Detail" });
        return crumbs;
      }
    }
  }
  return crumbs;
}

// Judul halaman untuk <title> dan penanda halaman aktif.
export function pageTitleFor(pathname: string): string {
  const c = breadcrumbsFor(pathname);
  return c[c.length - 1]?.label ?? "PASTI V3";
}
