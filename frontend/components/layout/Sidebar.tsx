"use client";

import { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronsLeft, ChevronsRight, ChevronDown, LogOut, ShieldCheck, X } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { useDashboard } from "@/lib/dashboard-context";
import { NAV_ENTRIES, NavEntry, isNavActive } from "@/lib/navigation";
import { peranEfektif, peranTampil } from "@/lib/peran";
import { initialsOf } from "@/lib/initials";

// Sama dengan breakpoint `md` Tailwind: di bawah ini sidebar jadi drawer.
const DESKTOP_QUERY = "(min-width: 768px)";

interface SidebarProps {
  collapsed: boolean;
  onToggleCollapse: () => void;
  mobileOpen: boolean;
  onCloseMobile: () => void;
}

interface Section {
  title: string;
  entries: NavEntry[];
}

// Pengelompokan otomatis dari daftar menu: menu biasa, modul (grup), lalu administrasi (menu khusus peran).
function buildSections(entries: NavEntry[]): Section[] {
  const main = entries.filter((e) => e.type === "item" && !e.roles);
  const modules = entries.filter((e) => e.type === "group");
  const admin = entries.filter((e) => e.type === "item" && e.roles);
  return [
    { title: "Menu", entries: main },
    { title: "Modul", entries: modules },
    { title: "Administrasi", entries: admin },
  ];
}

export function Sidebar({ collapsed, onToggleCollapse, mobileOpen, onCloseMobile }: SidebarProps) {
  const pathname = usePathname();
  const { logout } = useAuth();
  const { profile } = useDashboard();
  const closeButtonRef = useRef<HTMLButtonElement>(null);

  // Grup yang berisi halaman aktif saat ini (kalau ada).
  const activeGroupLabel = NAV_ENTRIES.find((e) => e.type === "group" && e.children.some((c) => isNavActive(pathname, c)))?.label;

  // Status buka/tutup disimpan per grup (kunci: label), supaya membuka "Pengadaan" tidak ikut membuka/menutup "Tender".
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>(activeGroupLabel ? { [activeGroupLabel]: true } : {});

  // Auto-expand grup yang berisi halaman aktif, supaya konteks navigasi tetap terlihat. Hanya membuka; grup lain tidak disentuh.
  useEffect(() => {
    if (activeGroupLabel) {
      setOpenGroups((prev) => ({ ...prev, [activeGroupLabel]: true }));
    }
  }, [activeGroupLabel]);

  // Selama drawer mobile terbuka: kunci scroll halaman di belakangnya, tutup dengan Escape, dan tutup otomatis kalau layar
  // dilebarkan ke ukuran desktop (kalau tidak, scroll halaman akan tetap terkunci di desktop).
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

  const peranSaya = peranEfektif(profile?.peran, profile?.role);
  const canSee = (roles?: string[]) => !roles || (profile && roles.includes(peranSaya));

  const handleGroupClick = (label: string, isCollapsed: boolean) => {
    // Kalau sidebar sedang diciutkan, buka dulu sidebar-nya supaya submenu bisa terlihat, baru buka grup yang diklik.
    if (isCollapsed) {
      onToggleCollapse();
      setOpenGroups((prev) => ({ ...prev, [label]: true }));
      return;
    }
    setOpenGroups((prev) => ({ ...prev, [label]: !prev[label] }));
  };

  // Isi sidebar dipakai dua kali: sidebar tetap di desktop (boleh diciutkan) dan drawer di mobile (selalu penuh, status
  // `collapsed` desktop diabaikan).
  const renderSidebar = (isCollapsed: boolean, isMobile: boolean) => (
    <div className="flex h-full flex-col bg-gradient-to-b from-slate-900 to-slate-950 text-slate-300">
      {/* Logo */}
      <div className={`flex items-center gap-3 px-4 py-5 ${isCollapsed ? "justify-center" : ""}`}>
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-blue-400 to-blue-600 text-white shadow-lg shadow-blue-600/30">
          <ShieldCheck className="h-6 w-6" />
        </span>
        {!isCollapsed && (
          <div className="min-w-0">
            <p className="truncate text-[15px] font-bold tracking-tight text-white">PASTI V3</p>
            <p className="truncate text-[11px] text-slate-400">Pemantauan Aset Terintegrasi</p>
          </div>
        )}
        {isMobile && (
          <button
            ref={closeButtonRef}
            onClick={onCloseMobile}
            aria-label="Tutup menu"
            className="-mr-1 ml-auto shrink-0 rounded-lg p-2 text-slate-400 hover:bg-white/10 hover:text-white"
          >
            <X className="h-5 w-5" />
          </button>
        )}
      </div>

      {/* Menu */}
      <nav aria-label="Menu utama" className="flex-1 space-y-5 overflow-y-auto px-3 pb-4 pt-1">
        {buildSections(NAV_ENTRIES).map((section) => {
          const visible = section.entries.filter(
            (e) => canSee(e.roles) && (e.type === "item" || e.children.some((c) => canSee(c.roles)))
          );
          if (visible.length === 0) return null;
          return (
            <div key={section.title}>
              {isCollapsed ? (
                <div className="mx-3 mb-2 h-px bg-white/10" aria-hidden="true" />
              ) : (
                <p className="mb-1.5 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">{section.title}</p>
              )}
              <div className="space-y-0.5">
                {visible.map((entry) => {
                  if (entry.type === "item") {
                    const isActive = isNavActive(pathname, entry);
                    const Icon = entry.icon;
                    return (
                      <Link
                        key={entry.href}
                        href={entry.href}
                        onClick={onCloseMobile}
                        title={isCollapsed ? entry.label : undefined}
                        aria-current={isActive ? "page" : undefined}
                        className={`group relative flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all duration-200 ${
                          isActive ? "bg-white/10 text-white shadow-inner shadow-white/5" : "text-slate-400 hover:bg-white/5 hover:text-white"
                        } ${isCollapsed ? "justify-center" : ""}`}
                      >
                        {isActive && <span aria-hidden="true" className="absolute left-0 top-1/2 h-5 w-1 -translate-y-1/2 rounded-r-full bg-blue-400" />}
                        <Icon className={`h-5 w-5 shrink-0 transition-colors ${isActive ? "text-blue-300" : "text-slate-500 group-hover:text-slate-300"}`} />
                        {!isCollapsed && <span className="truncate">{entry.label}</span>}
                      </Link>
                    );
                  }

                  // entry.type === "group"
                  const GroupIcon = entry.icon;
                  const children = entry.children.filter((c) => canSee(c.roles));
                  const hasActiveChild = children.some((c) => isNavActive(pathname, c));
                  const isOpen = Boolean(openGroups[entry.label]);

                  return (
                    <div key={entry.label}>
                      <button
                        onClick={() => handleGroupClick(entry.label, isCollapsed)}
                        title={isCollapsed ? entry.label : undefined}
                        aria-label={isCollapsed ? entry.label : undefined}
                        aria-expanded={isCollapsed ? undefined : isOpen}
                        className={`group relative flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all duration-200 ${
                          hasActiveChild && !isOpen ? "bg-white/10 text-white" : "text-slate-400 hover:bg-white/5 hover:text-white"
                        } ${isCollapsed ? "justify-center" : ""}`}
                      >
                        {hasActiveChild && !isOpen && (
                          <span aria-hidden="true" className="absolute left-0 top-1/2 h-5 w-1 -translate-y-1/2 rounded-r-full bg-blue-400" />
                        )}
                        <GroupIcon className={`h-5 w-5 shrink-0 ${hasActiveChild ? "text-blue-300" : "text-slate-500 group-hover:text-slate-300"}`} />
                        {!isCollapsed && (
                          <>
                            <span className="flex-1 truncate text-left">{entry.label}</span>
                            <ChevronDown className={`h-4 w-4 shrink-0 text-slate-500 transition-transform duration-300 ease-smooth ${isOpen ? "rotate-180" : ""}`} />
                          </>
                        )}
                      </button>

                      {/* Buka/tutup halus: tinggi baris beranimasi dari 0fr ke 1fr. */}
                      {!isCollapsed && (
                        <div className={`grid transition-[grid-template-rows,opacity] duration-300 ease-smooth ${isOpen ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0"}`}>
                          <div className="overflow-hidden">
                            <div className="ml-[22px] mt-1 space-y-0.5 border-l border-white/10 pl-3">
                              {children.map((child) => {
                                const isActive = isNavActive(pathname, child);
                                const ChildIcon = child.icon;
                                return (
                                  <Link
                                    key={child.href}
                                    href={child.href}
                                    onClick={onCloseMobile}
                                    tabIndex={isOpen ? 0 : -1}
                                    aria-current={isActive ? "page" : undefined}
                                    className={`flex items-center gap-2.5 rounded-lg px-3 py-2.5 text-[13px] font-medium transition-colors md:py-2 ${
                                      isActive ? "bg-blue-600 text-white shadow-md shadow-blue-600/20" : "text-slate-400 hover:bg-white/5 hover:text-white"
                                    }`}
                                  >
                                    <ChildIcon className="h-4 w-4 shrink-0" />
                                    <span className="truncate">{child.label}</span>
                                  </Link>
                                );
                              })}
                            </div>
                          </div>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          );
        })}
      </nav>

      {isMobile ? (
        /* Akun + Keluar (mobile): di layar kecil Navbar hanya memuat ikon. */
        <div className="border-t border-white/10 p-3">
          {profile && (
            <div className="mb-1 flex items-center gap-3 px-3 py-2">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-blue-400 to-blue-600 text-sm font-bold text-white">
                {initialsOf(profile.full_name)}
              </div>
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-white">{profile.full_name}</p>
                <p className="truncate text-xs text-slate-400">{peranTampil(profile.peran, profile.role)}</p>
              </div>
            </div>
          )}
          <button
            onClick={() => logout("manual")}
            className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-slate-300 transition-colors hover:bg-red-500/10 hover:text-red-300"
          >
            <LogOut className="h-5 w-5 shrink-0" />
            Keluar
          </button>
        </div>
      ) : (
        /* Collapse Toggle (desktop only) */
        <div className="border-t border-white/10 p-3">
          <button
            onClick={onToggleCollapse}
            title={isCollapsed ? "Perluas menu" : undefined}
            aria-label={isCollapsed ? "Perluas menu" : "Ciutkan menu"}
            className={`flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-slate-400 transition-colors hover:bg-white/5 hover:text-white ${
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
        className={`fixed inset-y-0 left-0 z-30 hidden shrink-0 border-r border-slate-900 transition-[width] duration-300 ease-smooth md:block ${
          collapsed ? "w-[76px]" : "w-64"
        }`}
      >
        {renderSidebar(collapsed, false)}
      </aside>

      {/* Mobile Sidebar (drawer) */}
      {mobileOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div aria-hidden="true" className="absolute inset-0 animate-fade-in bg-slate-950/60 backdrop-blur-sm motion-reduce:animate-none" onClick={onCloseMobile} />
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
