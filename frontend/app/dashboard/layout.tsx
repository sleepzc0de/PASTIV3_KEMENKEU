"use client";

import { useState, useEffect, useCallback } from "react";
import { DashboardProvider } from "@/lib/dashboard-context";
import { Sidebar } from "@/components/layout/Sidebar";
import { Navbar } from "@/components/layout/Navbar";
import { Footer } from "@/components/layout/Footer";

const SIDEBAR_COLLAPSE_KEY = "pasti_sidebar_collapsed";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    const saved = localStorage.getItem(SIDEBAR_COLLAPSE_KEY);
    if (saved === "true") setCollapsed(true);
    setMounted(true);
  }, []);

  const toggleCollapse = () => {
    setCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem(SIDEBAR_COLLAPSE_KEY, String(next));
      return next;
    });
  };

  // Identitas fungsi harus stabil: dipakai sebagai dependency effect di Sidebar.
  const closeMobile = useCallback(() => setMobileOpen(false), []);

  return (
    <DashboardProvider>
      <div className="flex min-h-dvh bg-slate-50">
        <Sidebar
          collapsed={collapsed}
          onToggleCollapse={toggleCollapse}
          mobileOpen={mobileOpen}
          onCloseMobile={closeMobile}
        />

        {/* min-w-0: supaya konten lebar (mis. tabel) tidak melebarkan halaman di layar kecil. */}
        <div
          className={`flex min-h-dvh w-full min-w-0 flex-1 flex-col transition-all duration-200 ${
            mounted ? (collapsed ? "md:pl-[72px]" : "md:pl-64") : ""
          }`}
        >
          <Navbar onOpenMobileSidebar={() => setMobileOpen(true)} />

          <main className="flex-1 p-4 sm:p-6 lg:p-8">{children}</main>

          <Footer />
        </div>
      </div>
    </DashboardProvider>
  );
}
