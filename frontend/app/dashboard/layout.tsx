"use client";

import { useState, useEffect, useCallback } from "react";
import { usePathname } from "next/navigation";
import { DashboardProvider } from "@/lib/dashboard-context";
import { flattenNav, pageTitleFor } from "@/lib/navigation";
import { Sidebar } from "@/components/layout/Sidebar";
import { Navbar } from "@/components/layout/Navbar";
import { Footer } from "@/components/layout/Footer";
import { CommandPalette, rememberPage } from "@/components/layout/CommandPalette";

const SIDEBAR_COLLAPSE_KEY = "pasti_sidebar_collapsed";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [mounted, setMounted] = useState(false);
  const pathname = usePathname();

  useEffect(() => {
    const saved = localStorage.getItem(SIDEBAR_COLLAPSE_KEY);
    if (saved === "true") setCollapsed(true);
    setMounted(true);
  }, []);

  // Judul tab browser mengikuti halaman, dan halaman yang dibuka dicatat untuk "Baru dibuka" di pencarian menu.
  useEffect(() => {
    const t = pageTitleFor(pathname);
    document.title = t === "Beranda" ? "PASTI V3 - Pemantauan Aset Terintegrasi" : `${t} · PASTI V3`;
    if (flattenNav().some((i) => i.href === pathname) || flattenNav("admin").some((i) => i.href === pathname)) rememberPage(pathname);
  }, [pathname]);

  // Ctrl/Cmd + K membuka pencarian menu dari halaman mana pun.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen((o) => !o);
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  const toggleCollapse = () => {
    setCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem(SIDEBAR_COLLAPSE_KEY, String(next));
      return next;
    });
  };

  // Identitas fungsi harus stabil: dipakai sebagai dependency effect di Sidebar dan CommandPalette.
  const closeMobile = useCallback(() => setMobileOpen(false), []);
  const closePalette = useCallback(() => setPaletteOpen(false), []);

  return (
    <DashboardProvider>
      <div className="flex min-h-dvh bg-slate-50">
        <Sidebar collapsed={collapsed} onToggleCollapse={toggleCollapse} mobileOpen={mobileOpen} onCloseMobile={closeMobile} />

        {/* min-w-0: supaya konten lebar (mis. tabel) tidak melebarkan halaman di layar kecil. */}
        <div
          className={`flex min-h-dvh w-full min-w-0 flex-1 flex-col transition-[padding] duration-300 ease-smooth ${
            mounted ? (collapsed ? "md:pl-[76px]" : "md:pl-64") : ""
          }`}
        >
          <Navbar onOpenMobileSidebar={() => setMobileOpen(true)} onOpenPalette={() => setPaletteOpen(true)} />

          <main className="mx-auto w-full max-w-[1600px] flex-1 p-4 sm:p-6 lg:p-8">{children}</main>

          <Footer />
        </div>

        <CommandPalette open={paletteOpen} onClose={closePalette} />
      </div>
    </DashboardProvider>
  );
}
