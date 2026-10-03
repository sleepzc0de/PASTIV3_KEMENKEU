import type { Metadata, Viewport } from "next";
import { Suspense } from "react";
import { AuthProvider } from "@/lib/auth-context";
import { ToastProvider } from "@/components/ui/Toast";
import { TopLoader } from "@/components/ui/TopLoader";
import "./globals.css";

export const metadata: Metadata = {
  title: "PASTI V3 - Pemantauan Aset Terintegrasi",
  description: "Sistem Pemantauan Aset Terintegrasi",
};

export const viewport: Viewport = {
  themeColor: "#3358e0",
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="id" suppressHydrationWarning>
      <body className="font-sans antialiased" suppressHydrationWarning>
        <ToastProvider>
          <Suspense fallback={null}>
            <TopLoader />
          </Suspense>
          <AuthProvider>{children}</AuthProvider>
        </ToastProvider>
      </body>
    </html>
  );
}
