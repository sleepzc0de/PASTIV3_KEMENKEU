"use client";

import { createContext, useContext, useEffect, useState, ReactNode } from "react";
import { api, type PeranInfo } from "@/lib/api";
import { bolehSemuaData } from "@/lib/peran";

export interface Profile {
  id: string;
  username: string;
  email: string;
  full_name: string;
  role: string; // hak administrasi saat ini (turun menjadi "user" selama bertindak sebagai peran data)
  akun_role?: string; // role akun yang sebenarnya
  peran?: PeranInfo; // peran data yang berlaku dan yang tersedia
  tamu?: boolean; // belum punya peran apa pun: hanya melihat pernyataan penggunaan aplikasi
  persetujuan?: { sudah: boolean; versi: string; isi_profil: boolean };
  auth_provider?: string;
  is_protected?: boolean;
  jabatan?: string;
  satker?: string;
  organisasi?: string;
  nip?: string;
  picture?: string;
}

interface DashboardContextType {
  profile: Profile | null;
  // Peran yang berlaku boleh melihat seluruh data. Bila false, menu dan tab yang butuh data lengkap (Pengadaan) disembunyikan.
  semuaData: boolean;
  isLoadingProfile: boolean;
  refetchProfile: () => void;
}

const DashboardContext = createContext<DashboardContextType>({
  profile: null,
  semuaData: true,
  isLoadingProfile: true,
  refetchProfile: () => {},
});

export function DashboardProvider({ children }: { children: ReactNode }) {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [isLoadingProfile, setIsLoadingProfile] = useState(true);

  const fetchProfile = () => {
    setIsLoadingProfile(true);
    api
      .get("/auth/me")
      .then((res) => setProfile(res.data.data))
      .catch(() => setProfile(null))
      .finally(() => setIsLoadingProfile(false));
  };

  useEffect(() => {
    fetchProfile();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <DashboardContext.Provider value={{ profile, semuaData: bolehSemuaData(profile?.peran), isLoadingProfile, refetchProfile: fetchProfile }}>
      {children}
    </DashboardContext.Provider>
  );
}

export function useDashboard() {
  return useContext(DashboardContext);
}