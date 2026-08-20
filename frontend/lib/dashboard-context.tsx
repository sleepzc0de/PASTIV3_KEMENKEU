"use client";

import { createContext, useContext, useEffect, useState, ReactNode } from "react";
import { api } from "@/lib/api";

interface Profile {
  id: string;
  username: string;
  email: string;
  full_name: string;
  role: string;
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
  isLoadingProfile: boolean;
  refetchProfile: () => void;
}

const DashboardContext = createContext<DashboardContextType>({
  profile: null,
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
    <DashboardContext.Provider value={{ profile, isLoadingProfile, refetchProfile: fetchProfile }}>
      {children}
    </DashboardContext.Provider>
  );
}

export function useDashboard() {
  return useContext(DashboardContext);
}