"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  DatabaseZap,
  Users,
  ChevronsLeft,
  ChevronsRight,
  ShieldCheck,
  X,
  Users2,
  FileClock,
  Wallet
} from "lucide-react";
import { useDashboard } from "@/lib/dashboard-context";

interface NavItem {
  label: string;
  href: string;
  icon: React.ElementType;
  roles?: string[]; // kalau undefined, terlihat semua role
}

const NAV_ITEMS: NavItem[] = [
  { label: "Dashboard", href: "/dashboard", icon: LayoutDashboard },
  { label: "Data Aset (SLDK)", href: "/dashboard/assets", icon: DatabaseZap },
  { label: "Kaji Ulang RUP (Inaproc)", href: "/dashboard/pengadaan", icon: FileClock },
  { label: "Paket Anggaran (Inaproc)", href: "/dashboard/pengadaan/paket-anggaran", icon: Wallet },
  { label: "Cari Pegawai (HRIS2)", href: "/dashboard/pegawai", icon: Users2, roles: ["admin", "superadmin"] },
  { label: "Manajemen Pengguna", href: "/dashboard/users", icon: Users, roles: ["admin", "superadmin"] },
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

  const visibleItems = NAV_ITEMS.filter((item) => !item.roles || (profile && item.roles.includes(profile.role)));

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
        {visibleItems.map((item) => {
          const isActive = pathname === item.href;
          const Icon = item.icon;
          return (
            <Link
              key={item.href}
              href={item.href}
              onClick={onCloseMobile}
              title={collapsed ? item.label : undefined}
              className={`flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                isActive
                  ? "bg-blue-600 text-white"
                  : "text-slate-300 hover:bg-slate-800 hover:text-white"
              } ${collapsed ? "justify-center" : ""}`}
            >
              <Icon className="h-5 w-5 shrink-0" />
              {!collapsed && <span className="truncate">{item.label}</span>}
            </Link>
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