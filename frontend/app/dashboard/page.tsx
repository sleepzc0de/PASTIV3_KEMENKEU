"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

export default function DashboardPage() {
  const { logout } = useAuth();
  const [profile, setProfile] = useState<any>(null);

  useEffect(() => {
    api.get("/auth/me").then((res) => setProfile(res.data.data));
  }, []);

  return (
    <div className="min-h-screen bg-slate-50 p-8">
      <div className="mx-auto max-w-4xl rounded-xl bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-slate-900">Dashboard PASTI V3</h1>
          <button
            onClick={logout}
            className="rounded-lg bg-red-50 px-4 py-2 text-sm font-medium text-red-600 hover:bg-red-100"
          >
            Keluar
          </button>
        </div>
        {profile && (
          <div className="mt-6 text-sm text-slate-600">
            Selamat datang, <b>{profile.full_name}</b> ({profile.role})
          </div>
        )}
      </div>
    </div>
  );
}