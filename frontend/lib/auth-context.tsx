"use client";

import { createContext, useContext, useState, ReactNode, useCallback } from "react";
import Cookies from "js-cookie";
import { useRouter, usePathname } from "next/navigation";
import { loginUser } from "./api";
import { useIdleTimer } from "./useIdleTimer";
import { IdleWarningModal } from "@/components/ui/IdleWarningModal";

interface UserProfile {
  id: string;
  username: string;
  email: string;
  full_name: string;
  role: string;
}

interface AuthContextType {
  user: UserProfile | null;
  login: (username: string, password: string, captchaId: string, captchaAnswer: string) => Promise<void>;
  logout: (reason?: "manual" | "idle") => void;
  isLoading: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

const IDLE_TIMEOUT_MS = 20 * 60 * 1000; // 20 menit
const WARNING_BEFORE_MS = 60 * 1000; // peringatan 1 menit sebelum logout

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserProfile | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [showIdleWarning, setShowIdleWarning] = useState(false);
  const router = useRouter();
  const pathname = usePathname();

  const isAuthenticated = Boolean(Cookies.get("pasti_access_token"));
  const isOnProtectedRoute = pathname?.startsWith("/dashboard") ?? false;

  const login = async (
    username: string,
    password: string,
    captchaId: string,
    captchaAnswer: string
  ) => {
    setIsLoading(true);
    try {
      const res = await loginUser(username, password, captchaId, captchaAnswer);
      const { access_token, expires_in, user } = res.data;

      Cookies.set("pasti_access_token", access_token, {
        expires: expires_in / 86400,
        secure: process.env.NODE_ENV === "production",
        sameSite: "strict",
      });

      setUser(user);
      router.push("/dashboard");
    } finally {
      setIsLoading(false);
    }
  };

  const logout = useCallback((reason: "manual" | "idle" = "manual") => {
    Cookies.remove("pasti_access_token");
    setUser(null);
    setShowIdleWarning(false);

    // Pakai hard redirect (bukan router.push) supaya navigasi selalu terjadi
    // secara pasti, memicu middleware re-check, dan mereset seluruh state JS
    // termasuk timer-timer yang mungkin masih berjalan.
    const target =
      reason === "idle"
        ? "/login?error=session_expired&reason=" +
        encodeURIComponent("Sesi berakhir karena tidak ada aktivitas")
        : "/login";

    window.location.href = target;
  }, []);

  const { resetTimer } = useIdleTimer({
    idleTimeout: IDLE_TIMEOUT_MS,
    warningBeforeMs: WARNING_BEFORE_MS,
    onIdle: () => logout("idle"),
    onWarning: () => setShowIdleWarning(true),
    onActive: () => setShowIdleWarning(false),
    enabled: isAuthenticated && isOnProtectedRoute,
  });

  return (
    <AuthContext.Provider value={{ user, login, logout, isLoading }}>
      {children}
      <IdleWarningModal
        open={showIdleWarning}
        countdownSeconds={WARNING_BEFORE_MS / 1000}
        onStayLoggedIn={() => {
          setShowIdleWarning(false);
          resetTimer();
        }}
        onLogoutNow={() => logout("manual")}
      />
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth harus dipakai di dalam AuthProvider");
  return ctx;
}