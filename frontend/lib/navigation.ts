import {
  LayoutDashboard,
  Users,
  Users2,
  ShoppingCart,
  Building2,
  MapPinned,
  Landmark,
  Settings2,
  DatabaseBackup,
  ListTree,
} from "lucide-react";

export interface NavItem {
  label: string;
  href: string;
  icon: React.ElementType;
  description?: string;
  roles?: string[];
  // Menu tetap aktif di halaman di bawahnya (mis. detail usulan). Tanpa ini, hanya alamat yang persis sama yang aktif.
  matchPrefix?: boolean;
  // Halaman lain milik menu ini yang tidak punya baris sendiri di sidebar (mis. Pengaturan SAPA). Menu aktif di halaman-halaman itu, dan
  // halaman-halamannya tetap muncul di pencarian menu (Ctrl+K) dan remah roti.
  halaman?: NavItem[];
}

export function isNavActive(pathname: string, item: Pick<NavItem, "href" | "matchPrefix" | "halaman">): boolean {
  if (pathname === item.href || (item.matchPrefix === true && pathname.startsWith(item.href + "/"))) return true;
  return item.halaman?.some((h) => isNavActive(pathname, h)) ?? false;
}

export interface NavGroup {
  label: string;
  icon: React.ElementType;
  roles?: string[];
  // Hanya untuk peran yang melihat seluruh data (data Pengadaan lengkap belum bisa dibatasi per satker); disembunyikan bagi peran UE1/Kanwil/Satker.
  hanyaSemuaData?: boolean;
  children: NavItem[];
}

export type NavEntry =
  | ({ type: "item" } & NavItem)
  | ({ type: "group" } & NavGroup);

// Satu-satunya sumber daftar menu: dipakai Sidebar, pencarian menu (Ctrl+K), dan remah roti supaya ketiganya tidak pernah berbeda.
export const NAV_ENTRIES: NavEntry[] = [
  {
    type: "item",
    label: "Dashboard",
    href: "/dashboard",
    icon: LayoutDashboard,
    description: "Dashboard analitik aset dan pengadaan, lengkap dengan wawasan berbasis data",
  },
  {
    type: "group",
    label: "Aset",
    icon: Building2,
    children: [
      {
        label: "Digitalisasi Aset",
        href: "/dashboard/digitalisasi",
        icon: MapPinned,
        description: "Peta, daftar (dengan unduhan Excel, CSV, dan PDF), dan sinkronisasi data aset KL 015 dari SLDK",
      },
      {
        label: "SAPA",
        href: "/dashboard/sapa/penjualan",
        icon: Landmark,
        description: "Usulan penjualan BMN: pembentukan tim, berita acara, dan nota dinas dari satker hingga UE1",
        matchPrefix: true,
        halaman: [
          {
            label: "Pengaturan SAPA",
            href: "/dashboard/sapa/pengaturan",
            icon: Settings2,
            description: "Template dokumen Word, jenis dan satuan BMN, dan referensi Unit Eselon I",
            roles: ["admin", "superadmin"],
          },
        ],
      },
    ],
  },
  {
    type: "group",
    label: "Pengadaan",
    icon: ShoppingCart,
    hanyaSemuaData: true,
    children: [
      {
        label: "Pengadaan Terpadu",
        href: "/dashboard/pengadaan-terpadu/penarikan",
        icon: DatabaseBackup,
        description: "Tarik data Pengadaan, Tender, dan E-Katalog dari Inaproc secara manual atau otomatis, lalu lihat dan ekspor ke Excel, CSV, atau PDF",
        matchPrefix: true,
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
    label: "Referensi UE1",
    href: "/dashboard/referensi/ue1",
    icon: ListTree,
    description: "Kelola kode Unit Eselon I: uraian dan singkatan yang tampil di Digitalisasi Aset, Dashboard, dan berkas unduhan",
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

// Daftar datar semua menu yang boleh dilihat peran ini (untuk palet perintah dan remah roti), termasuk halaman turunan.
export function flattenNav(role?: string, semuaData = true): FlatNavItem[] {
  const ok = (roles?: string[]) => !roles || (role !== undefined && roles.includes(role));
  const out: FlatNavItem[] = [];
  for (const e of NAV_ENTRIES) {
    if (!ok(e.roles)) continue;
    if (e.type === "group" && e.hanyaSemuaData && !semuaData) continue;
    if (e.type === "item") {
      out.push(e);
    } else {
      for (const c of e.children) {
        if (!ok(c.roles)) continue;
        out.push({ ...c, group: e.label });
        for (const h of c.halaman ?? []) if (ok(h.roles)) out.push({ ...h, group: e.label });
      }
    }
  }
  return out;
}

export interface Crumb {
  label: string;
  href?: string;
}

// Remah roti dari alamat saat ini: "Beranda > Grup > Menu [> Halaman turunan] [> Detail]". Halaman di bawah menu yang `matchPrefix`
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
        const sub = child.halaman?.find((h) => isNavActive(pathname, h));
        if (sub) {
          crumbs.push({ label: child.label, href: child.href });
          crumbs.push({ label: sub.label, href: pathname === sub.href ? undefined : sub.href });
          if (pathname !== sub.href) crumbs.push({ label: "Detail" });
          return crumbs;
        }
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
