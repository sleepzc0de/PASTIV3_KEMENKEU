"use client";

import { Menu, LogOut, ShieldCheck, User as UserIcon, Landmark } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { useDashboard } from "@/lib/dashboard-context";
import { ROLE_LABEL } from "@/lib/roles";

interface NavbarProps {
  onOpenMobileSidebar: () => void;
}

export function Navbar({ onOpenMobileSidebar }: NavbarProps) {
  const { logout } = useAuth();
  const { profile, isLoadingProfile } = useDashboard();

  return (
    <header className="sticky top-0 z-20 flex h-16 shrink-0 items-center gap-3 border-b border-slate-200 bg-white px-4 sm:gap-4 sm:px-6">
      <button
        onClick={onOpenMobileSidebar}
        aria-label="Buka menu navigasi"
        className="-ml-2 rounded-lg p-2 text-slate-600 hover:bg-slate-100 md:hidden"
      >
        <Menu className="h-6 w-6" />
      </button>

      {/* Sidebar tersembunyi di mobile, jadi identitas aplikasi ditampilkan di sini. */}
      <div className="flex items-center gap-2 md:hidden">
        <ShieldCheck className="h-6 w-6 shrink-0 text-blue-600" />
        <span className="text-sm font-bold text-slate-900">PASTI V3</span>
      </div>

      <div className="flex-1" />

      {!isLoadingProfile && profile && (
        <div className="flex items-center gap-2 sm:gap-3">
          {profile.auth_provider === "sso" && (
            <span className="hidden items-center gap-1 rounded-full bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700 sm:flex">
              <Landmark className="h-3 w-3" />
              SSO Kemenkeu
            </span>
          )}

          <div className="hidden text-right sm:block">
            <p className="text-sm font-semibold text-slate-900">{profile.full_name}</p>
            <p className="text-xs text-slate-500">{ROLE_LABEL[profile.role] || profile.role}</p>
          </div>

          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-blue-100 text-blue-700">
            <UserIcon className="h-5 w-5" />
          </div>

          <button
            onClick={() => logout("manual")}
            title="Keluar"
            aria-label="Keluar"
            className="flex items-center gap-1.5 rounded-lg border border-slate-200 p-2.5 text-sm font-medium text-slate-600 hover:bg-red-50 hover:text-red-600 sm:px-3 sm:py-2"
          >
            <LogOut className="h-4 w-4" />
            <span className="hidden sm:inline">Keluar</span>
          </button>
        </div>
      )}
    </header>
  );
}
