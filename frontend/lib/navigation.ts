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
} from "lucide-react";

export interface NavItem {
  label: string;
  href: string;
  icon: React.ElementType;
  description?: string;
  roles?: string[];
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
