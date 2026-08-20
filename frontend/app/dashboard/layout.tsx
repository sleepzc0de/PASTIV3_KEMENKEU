"use client";

import { useState, useEffect } from "react";
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

  return (
    <DashboardProvider>
      <div className="flex min-h-screen bg-slate-50">
        <Sidebar
          collapsed={collapsed}
          onToggleCollapse={toggleCollapse}
          mobileOpen={mobileOpen}
          onCloseMobile={() => setMobileOpen(false)}
        />

        <div
          className={`flex min-h-screen w-full flex-1 flex-col transition-all duration-200 ${
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