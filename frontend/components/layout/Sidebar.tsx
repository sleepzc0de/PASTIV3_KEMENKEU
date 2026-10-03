"use client";

import { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ChevronsLeft,
  ChevronsRight,
  ChevronDown,
  LogOut,
  ShieldCheck,
  User as UserIcon,
  X,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { useDashboard } from "@/lib/dashboard-context";
import { NAV_ENTRIES, isNavActive } from "@/lib/navigation";
import { ROLE_LABEL } from "@/lib/roles";

// Sama dengan breakpoint `md` Tailwind: di bawah ini sidebar jadi drawer.
const DESKTOP_QUERY = "(min-width: 768px)";

interface SidebarProps {
  collapsed: boolean;
  onToggleCollapse: () => void;
  mobileOpen: boolean;
  onCloseMobile: () => void;
}

export function Sidebar({ collapsed, onToggleCollapse, mobileOpen, onCloseMobile }: SidebarProps) {
  const pathname = usePathname();
  const { logout } = useAuth();
  const { profile } = useDashboard();
  const closeButtonRef = useRef<HTMLButtonElement>(null);

  // Grup yang berisi halaman aktif saat ini (kalau ada).
  const activeGroupLabel = NAV_ENTRIES.find(
    (e) => e.type === "group" && e.children.some((c) => isNavActive(pathname, c))
  )?.label;

  // Status buka/tutup disimpan per grup (kunci: label), supaya membuka
  // "Pengadaan" tidak ikut membuka/menutup "Tender".
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>(
    activeGroupLabel ? { [activeGroupLabel]: true } : {}
  );

  // Auto-expand grup yang berisi halaman aktif, supaya konteks navigasi
  // tetap terlihat. Hanya membuka; grup lain tidak disentuh.
  useEffect(() => {
    if (activeGroupLabel) {
      setOpenGroups((prev) => ({ ...prev, [activeGroupLabel]: true }));
    }
  }, [activeGroupLabel]);

  // Selama drawer mobile terbuka: kunci scroll halaman di belakangnya, tutup
  // dengan Escape, dan tutup otomatis kalau layar dilebarkan ke ukuran desktop
  // (kalau tidak, scroll halaman akan tetap terkunci di desktop).
  useEffect(() => {
    if (!mobileOpen) return;

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeButtonRef.current?.focus();

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCloseMobile();
    };
    document.addEventListener("keydown", onKeyDown);

    const mq = window.matchMedia(DESKTOP_QUERY);
    const onBreakpointChange = (e: MediaQueryListEvent) => {
      if (e.matches) onCloseMobile();
    };
    mq.addEventListener("change", onBreakpointChange);

    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", onKeyDown);
      mq.removeEventListener("change", onBreakpointChange);
    };
  }, [mobileOpen, onCloseMobile]);

  const canSee = (roles?: string[]) => !roles || (profile && roles.includes(profile.role));

  const handleGroupClick = (label: string, isCollapsed: boolean) => {
    // Kalau sidebar sedang diciutkan, buka dulu sidebar-nya supaya
    // submenu bisa terlihat, baru buka grup yang diklik.
    if (isCollapsed) {
      onToggleCollapse();
      setOpenGroups((prev) => ({ ...prev, [label]: true }));
      return;
    }
    setOpenGroups((prev) => ({ ...prev, [label]: !prev[label] }));
  };

  // Isi sidebar dipakai dua kali: sidebar tetap di desktop (boleh diciutkan)
  // dan drawer di mobile (selalu penuh, status `collapsed` desktop diabaikan).
  const renderSidebar = (isCollapsed: boolean, isMobile: boolean) => (
    <div className="flex h-full flex-col bg-slate-900 text-slate-200">
      {/* Header / Logo */}
      <div className={`flex items-center gap-2.5 border-b border-slate-800 px-4 py-5 ${isCollapsed ? "justify-center" : ""}`}>
        <ShieldCheck className="h-7 w-7 shrink-0 text-blue-400" />
        {!isCollapsed && (
          <div className="min-w-0">
            <p className="truncate text-sm font-bold text-white">PASTI V3</p>
            <p className="truncate text-[11px] text-slate-400">Pemantauan Aset Terintegrasi</p>
          </div>
        )}
        {isMobile && (
          <button
            ref={closeButtonRef}
            onClick={onCloseMobile}
            aria-label="Tutup menu"
            className="-mr-1 ml-auto shrink-0 rounded-lg p-2 text-slate-400 hover:bg-slate-800 hover:text-white"
          >
            <X className="h-5 w-5" />
          </button>
        )}
      </div>

      {/* Nav Items */}
      <nav aria-label="Menu utama" className="flex-1 space-y-1 overflow-y-auto px-2.5 py-4">
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
                title={isCollapsed ? entry.label : undefined}
                aria-current={isActive ? "page" : undefined}
                className={`flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                  isActive ? "bg-blue-600 text-white" : "text-slate-300 hover:bg-slate-800 hover:text-white"
                } ${isCollapsed ? "justify-center" : ""}`}
              >
                <Icon className="h-5 w-5 shrink-0" />
                {!isCollapsed && <span className="truncate">{entry.label}</span>}
              </Link>
            );
          }

          // entry.type === "group"
          if (!canSee(entry.roles)) return null;
          const GroupIcon = entry.icon;
          const children = entry.children.filter((c) => canSee(c.roles));
          if (children.length === 0) return null;
          const hasActiveChild = children.some((c) => isNavActive(pathname, c));
          const isOpen = Boolean(openGroups[entry.label]);

          return (
            <div key={entry.label}>
              <button
                onClick={() => handleGroupClick(entry.label, isCollapsed)}
                title={isCollapsed ? entry.label : undefined}
                aria-label={isCollapsed ? entry.label : undefined}
                aria-expanded={isCollapsed ? undefined : isOpen}
                className={`flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                  hasActiveChild && !isOpen
                    ? "bg-blue-600/20 text-blue-300"
                    : "text-slate-300 hover:bg-slate-800 hover:text-white"
                } ${isCollapsed ? "justify-center" : ""}`}
              >
                <GroupIcon className="h-5 w-5 shrink-0" />
                {!isCollapsed && (
                  <>
                    <span className="flex-1 truncate text-left">{entry.label}</span>
                    <ChevronDown
                      className={`h-4 w-4 shrink-0 transition-transform ${isOpen ? "rotate-180" : ""}`}
                    />
                  </>
                )}
              </button>

              {!isCollapsed && isOpen && (
                <div className="mt-1 space-y-0.5 border-l border-slate-800 pl-3.5 ml-3.5">
                  {children.map((child) => {
                    const isActive = isNavActive(pathname, child);
                    const ChildIcon = child.icon;
                    return (
                      <Link
                        key={child.href}
                        href={child.href}
                        onClick={onCloseMobile}
                        aria-current={isActive ? "page" : undefined}
                        className={`flex items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors md:py-2 ${
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

      {isMobile ? (
        /* Akun + Keluar (mobile): di layar kecil Navbar hanya memuat ikon. */
        <div className="border-t border-slate-800 p-2.5">
          {profile && (
            <div className="mb-1 flex items-center gap-3 px-3 py-2">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-blue-600/20 text-blue-300">
                <UserIcon className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-white">{profile.full_name}</p>
                <p className="truncate text-xs text-slate-400">{ROLE_LABEL[profile.role] || profile.role}</p>
              </div>
            </div>
          )}
          <button
            onClick={() => logout("manual")}
            className="flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-slate-300 transition-colors hover:bg-red-500/10 hover:text-red-300"
          >
            <LogOut className="h-5 w-5 shrink-0" />
            Keluar
          </button>
        </div>
      ) : (
        /* Collapse Toggle (desktop only) */
        <div className="border-t border-slate-800 p-2.5">
          <button
            onClick={onToggleCollapse}
            title={isCollapsed ? "Perluas menu" : undefined}
            aria-label={isCollapsed ? "Perluas menu" : undefined}
            className={`flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-slate-300 transition-colors hover:bg-slate-800 hover:text-white ${
              isCollapsed ? "justify-center" : ""
            }`}
          >
            {isCollapsed ? <ChevronsRight className="h-5 w-5 shrink-0" /> : <ChevronsLeft className="h-5 w-5 shrink-0" />}
            {!isCollapsed && <span>Ciutkan Menu</span>}
          </button>
        </div>
      )}
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
        {renderSidebar(collapsed, false)}
      </aside>

      {/* Mobile Sidebar (drawer) */}
      {mobileOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div
            aria-hidden="true"
            className="absolute inset-0 animate-fade-in bg-black/50 motion-reduce:animate-none"
            onClick={onCloseMobile}
          />
          <aside
            role="dialog"
            aria-modal="true"
            aria-label="Menu navigasi"
            className="absolute inset-y-0 left-0 w-72 max-w-[85vw] animate-drawer-in shadow-xl motion-reduce:animate-none"
          >
            {renderSidebar(false, true)}
          </aside>
        </div>
      )}
    </>
  );
}
