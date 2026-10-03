"use client";

import { Fragment, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronDown, ChevronRight, Home, Landmark, LogOut, Menu, Search, ShieldCheck } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { useDashboard } from "@/lib/dashboard-context";
import { initialsOf } from "@/lib/initials";
import { breadcrumbsFor } from "@/lib/navigation";
import { ROLE_LABEL } from "@/lib/roles";

interface NavbarProps {
  onOpenMobileSidebar: () => void;
  onOpenPalette: () => void;
}

export function Navbar({ onOpenMobileSidebar, onOpenPalette }: NavbarProps) {
  const pathname = usePathname();
  const { profile, isLoadingProfile } = useDashboard();
  const crumbs = breadcrumbsFor(pathname);
  const current = crumbs[crumbs.length - 1];

  return (
    <header className="sticky top-0 z-20 flex h-16 shrink-0 items-center gap-2 border-b border-slate-200/70 bg-white/80 px-3 backdrop-blur-xl sm:gap-4 sm:px-6">
      <button
        onClick={onOpenMobileSidebar}
        aria-label="Buka menu navigasi"
        className="-ml-1 rounded-xl p-2 text-slate-600 hover:bg-slate-100 md:hidden"
      >
        <Menu className="h-6 w-6" />
      </button>

      {/* Mobile: sidebar tersembunyi, jadi tampilkan identitas dan judul halaman. */}
      <div className="flex min-w-0 items-center gap-2 md:hidden">
        <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-blue-500 to-blue-700 text-white">
          <ShieldCheck className="h-[18px] w-[18px]" />
        </span>
        <span className="truncate text-sm font-semibold text-slate-900">
          {current?.label === "Beranda" ? "PASTI V3" : current?.label === "Detail" && crumbs.length > 1 ? `${crumbs[crumbs.length - 2].label} · Detail` : current?.label}
        </span>
      </div>

      {/* Desktop: remah roti. */}
      <nav aria-label="Posisi halaman" className="hidden min-w-0 items-center gap-1.5 text-sm md:flex">
        {crumbs.map((c, i) => {
          const last = i === crumbs.length - 1;
          return (
            <Fragment key={i}>
              {i > 0 && <ChevronRight className="h-3.5 w-3.5 shrink-0 text-slate-300" aria-hidden="true" />}
              {c.href ? (
                <Link href={c.href} className="flex items-center gap-1 truncate rounded-md px-1 text-slate-500 hover:text-blue-700">
                  {i === 0 && <Home className="h-3.5 w-3.5" aria-hidden="true" />}
                  {c.label}
                </Link>
              ) : (
                <span aria-current={last ? "page" : undefined} className={`truncate px-1 ${last ? "font-semibold text-slate-900" : "text-slate-500"}`}>
                  {c.label}
                </span>
              )}
            </Fragment>
          );
        })}
      </nav>

      <div className="flex-1" />

      <button
        onClick={onOpenPalette}
        aria-label="Cari menu (Ctrl+K)"
        className="hidden items-center gap-2.5 rounded-xl border border-slate-200 bg-slate-50/80 py-2 pl-3 pr-2 text-sm text-slate-400 hover:border-slate-300 hover:bg-white hover:text-slate-600 sm:flex"
      >
        <Search className="h-4 w-4" aria-hidden="true" />
        <span className="w-32 text-left lg:w-44">Cari menu…</span>
        <kbd className="rounded-md border border-slate-200 bg-white px-1.5 py-0.5 text-[11px] font-medium text-slate-500">Ctrl K</kbd>
      </button>
      <button onClick={onOpenPalette} aria-label="Cari menu" className="rounded-xl p-2 text-slate-500 hover:bg-slate-100 sm:hidden">
        <Search className="h-5 w-5" />
      </button>

      {isLoadingProfile ? <div className="skeleton h-9 w-9 rounded-full sm:w-40 sm:rounded-xl" /> : profile && <UserMenu />}
    </header>
  );
}

function UserMenu() {
  const { logout } = useAuth();
  const { profile } = useDashboard();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const wrap = useRef<HTMLDivElement>(null);

  useEffect(() => setOpen(false), [pathname]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (wrap.current && !wrap.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  if (!profile) return null;
  const role = ROLE_LABEL[profile.role] || profile.role;

  return (
    <div ref={wrap} className="relative">
      <button
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="menu"
        aria-expanded={open}
        className="flex items-center gap-2.5 rounded-xl p-1 pr-2 hover:bg-slate-100 sm:pr-3"
      >
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-blue-500 to-blue-700 text-sm font-bold text-white shadow-sm">
          {initialsOf(profile.full_name)}
        </span>
        <span className="hidden text-left sm:block">
          <span className="block max-w-[10rem] truncate text-sm font-semibold leading-tight text-slate-900">{profile.full_name}</span>
          <span className="block text-xs leading-tight text-slate-500">{role}</span>
        </span>
        <ChevronDown className={`hidden h-4 w-4 text-slate-400 transition-transform sm:block ${open ? "rotate-180" : ""}`} aria-hidden="true" />
      </button>

      {open && (
        <div role="menu" className="absolute right-0 top-full z-30 mt-2 w-72 origin-top-right animate-scale-in overflow-hidden rounded-2xl bg-white shadow-xl ring-1 ring-slate-900/5">
          <div className="border-b border-slate-100 bg-gradient-to-br from-blue-50 to-white p-4">
            <div className="flex items-center gap-3">
              <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-blue-500 to-blue-700 text-base font-bold text-white">
                {initialsOf(profile.full_name)}
              </span>
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-slate-900">{profile.full_name}</p>
                <p className="truncate text-xs text-slate-500">{profile.email || `@${profile.username}`}</p>
              </div>
            </div>
            <div className="mt-3 flex flex-wrap gap-1.5">
              <span className="rounded-full bg-white px-2.5 py-0.5 text-xs font-medium text-slate-700 ring-1 ring-inset ring-slate-200">{role}</span>
              {profile.auth_provider === "sso" && (
                <span className="inline-flex items-center gap-1 rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-medium text-blue-700">
                  <Landmark className="h-3 w-3" aria-hidden="true" />
                  SSO Kemenkeu
                </span>
              )}
            </div>
          </div>
          <div className="p-1.5">
            <Link role="menuitem" href="/dashboard" className="flex items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm text-slate-700 hover:bg-slate-50">
              <Home className="h-4 w-4 text-slate-400" aria-hidden="true" />
              Beranda
            </Link>
            <button
              role="menuitem"
              onClick={() => logout("manual")}
              className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm font-medium text-red-600 hover:bg-red-50"
            >
              <LogOut className="h-4 w-4" aria-hidden="true" />
              Keluar
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
