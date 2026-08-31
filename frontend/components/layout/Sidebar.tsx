"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
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
  ChevronsLeft,
  ChevronsRight,
  ChevronDown,
  ShieldCheck,
  X,
  Gavel,
  CalendarClock,
} from "lucide-react";
import { useDashboard } from "@/lib/dashboard-context";

interface NavItem {
  label: string;
  href: string;
  icon: React.ElementType;
  roles?: string[];
}

interface NavGroup {
  label: string;
  icon: React.ElementType;
  roles?: string[];
  children: NavItem[];
}

type NavEntry =
  | ({ type: "item" } & NavItem)
  | ({ type: "group" } & NavGroup);

const NAV_ENTRIES: NavEntry[] = [
  { type: "item", label: "Dashboard", href: "/dashboard", icon: LayoutDashboard },
  { type: "item", label: "Data Aset (SLDK)", href: "/dashboard/assets", icon: DatabaseZap },
  {
    type: "group",
    label: "Pengadaan (Inaproc)",
    icon: ShoppingCart,
    children: [
      { label: "Kaji Ulang RUP", href: "/dashboard/pengadaan", icon: FileClock },
      { label: "Paket Anggaran", href: "/dashboard/pengadaan/paket-anggaran", icon: Wallet },
      { label: "Anggaran Swakelola", href: "/dashboard/pengadaan/anggaran-swakelola", icon: WalletCards },
      { label: "Paket Penyedia", href: "/dashboard/pengadaan/paket-penyedia", icon: Package },
      { label: "Penyedia Terumumkan", href: "/dashboard/pengadaan/penyedia-terumumkan", icon: PackageCheck },
      { label: "Paket Swakelola", href: "/dashboard/pengadaan/paket-swakelola", icon: Boxes },
      { label: "Swakelola Terumumkan", href: "/dashboard/pengadaan/swakelola-terumumkan", icon: ClipboardCheck },
      { label: "Program Master", href: "/dashboard/pengadaan/program-master", icon: LayoutList },
    ],
  },
  {
    type: "group",
    label: "Tender (Inaproc)",
    icon: Gavel,
    children: [
      { label: "Jadwal Non Tender", href: "/dashboard/tender/jadwal-non-tender", icon: CalendarClock },
    ],
  },
  { type: "item", label: "Cari Pegawai (HRIS2)", href: "/dashboard/pegawai", icon: Users2, roles: ["admin", "superadmin"] },
  { type: "item", label: "Manajemen Pengguna", href: "/dashboard/users", icon: Users, roles: ["admin", "superadmin"] },
];

interface SidebarProps {
  collapsed: boolean;
  onToggleCollapse: () => void;
  mobileOpen: boolean;
  onCloseMobile: () => void;
}

export function Sidebar({ collapsed, onToggleCollapse, mobileOpen, onCloseMobile }: SidebarProps) {
  const pathname = usePathname();
  const { profile } = useDashboard();

  const pengadaanGroup = NAV_ENTRIES.find(
    (e): e is { type: "group" } & NavGroup => e.type === "group"
  );
  const isInPengadaanRoute =
    pengadaanGroup?.children.some((c) => pathname === c.href) ?? false;

  const [pengadaanOpen, setPengadaanOpen] = useState(isInPengadaanRoute);

  // Auto-expand grup kalau user sedang berada di salah satu halaman
  // Inaproc, supaya konteks navigasi tetap terlihat.
  useEffect(() => {
    if (isInPengadaanRoute) {
      setPengadaanOpen(true);
    }
  }, [isInPengadaanRoute]);

  const canSee = (roles?: string[]) => !roles || (profile && roles.includes(profile.role));

  const handleGroupClick = () => {
    // Kalau sidebar sedang diciutkan, buka dulu sidebar-nya supaya
    // submenu bisa terlihat, baru toggle grup.
    if (collapsed) {
      onToggleCollapse();
      setPengadaanOpen(true);
      return;
    }
    setPengadaanOpen((prev) => !prev);
  };

  const sidebarContent = (
    <div className="flex h-full flex-col bg-slate-900 text-slate-200">
      {/* Header / Logo */}
      <div className={`flex items-center gap-2.5 border-b border-slate-800 px-4 py-5 ${collapsed ? "justify-center" : ""}`}>
        <ShieldCheck className="h-7 w-7 shrink-0 text-blue-400" />
        {!collapsed && (
          <div className="min-w-0">
            <p className="truncate text-sm font-bold text-white">PASTI V3</p>
            <p className="truncate text-[11px] text-slate-400">Pemantauan Aset Terintegrasi</p>
          </div>
        )}
        <button
          onClick={onCloseMobile}
          className="ml-auto shrink-0 rounded-md p-1 text-slate-400 hover:bg-slate-800 hover:text-white md:hidden"
        >
          <X className="h-5 w-5" />
        </button>
      </div>

      {/* Nav Items */}
      <nav className="flex-1 space-y-1 overflow-y-auto px-2.5 py-4">
        {NAV_ENTRIES.map((entry) => {
          if (entry.type === "item") {
            if (!canSee(entry.roles)) return null;
            const isActive = pathname === entry.href;
            const Icon = entry.icon;
            return (
              <Link
                key={entry.href}
                href={entry.href}
                onClick={onCloseMobile}
                title={collapsed ? entry.label : undefined}
                className={`flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                  isActive ? "bg-blue-600 text-white" : "text-slate-300 hover:bg-slate-800 hover:text-white"
                } ${collapsed ? "justify-center" : ""}`}
              >
                <Icon className="h-5 w-5 shrink-0" />
                {!collapsed && <span className="truncate">{entry.label}</span>}
              </Link>
            );
          }

          // entry.type === "group"
          if (!canSee(entry.roles)) return null;
          const GroupIcon = entry.icon;
          const hasActiveChild = entry.children.some((c) => pathname === c.href);

          return (
            <div key={entry.label}>
              <button
                onClick={handleGroupClick}
                title={collapsed ? entry.label : undefined}
                className={`flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                  hasActiveChild && !pengadaanOpen
                    ? "bg-blue-600/20 text-blue-300"
                    : "text-slate-300 hover:bg-slate-800 hover:text-white"
                } ${collapsed ? "justify-center" : ""}`}
              >
                <GroupIcon className="h-5 w-5 shrink-0" />
                {!collapsed && (
                  <>
                    <span className="flex-1 truncate text-left">{entry.label}</span>
                    <ChevronDown
                      className={`h-4 w-4 shrink-0 transition-transform ${pengadaanOpen ? "rotate-180" : ""}`}
                    />
                  </>
                )}
              </button>

              {!collapsed && pengadaanOpen && (
                <div className="mt-1 space-y-0.5 border-l border-slate-800 pl-3.5 ml-3.5">
                  {entry.children.map((child) => {
                    const isActive = pathname === child.href;
                    const ChildIcon = child.icon;
                    return (
                      <Link
                        key={child.href}
                        href={child.href}
                        onClick={onCloseMobile}
                        className={`flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
                          isActive ? "bg-blue-600 text-white" : "text-slate-400 hover:bg-slate-800 hover:text-white"
                        }`}
                      >
                        <ChildIcon className="h-4 w-4 shrink-0" />
                        <span className="truncate">{child.label}</span>
                      </Link>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
      </nav>

      {/* Collapse Toggle (desktop only) */}
      <div className="hidden border-t border-slate-800 p-2.5 md:block">
        <button
          onClick={onToggleCollapse}
          className={`flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-slate-300 transition-colors hover:bg-slate-800 hover:text-white ${
            collapsed ? "justify-center" : ""
          }`}
        >
          {collapsed ? <ChevronsRight className="h-5 w-5 shrink-0" /> : <ChevronsLeft className="h-5 w-5 shrink-0" />}
          {!collapsed && <span>Ciutkan Menu</span>}
        </button>
      </div>
    </div>
  );

  return (
    <>
      {/* Desktop Sidebar */}
      <aside
        className={`fixed inset-y-0 left-0 z-30 hidden shrink-0 transition-all duration-200 md:block ${
          collapsed ? "w-[72px]" : "w-64"
        }`}
      >
        {sidebarContent}
      </aside>

      {/* Mobile Sidebar (overlay) */}
      {mobileOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="absolute inset-0 bg-black/50" onClick={onCloseMobile} />
          <aside className="absolute inset-y-0 left-0 w-64">{sidebarContent}</aside>
        </div>
      )}
    </>
  );
}