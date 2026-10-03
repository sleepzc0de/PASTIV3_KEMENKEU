"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { netCount } from "@/lib/netActivity";

const SHOW_DELAY_MS = 220; // permintaan cepat tidak memunculkan bilah (menghindari kedipan)
const NAV_TIMEOUT_MS = 10_000;

// Bilah kemajuan tipis di tepi atas: tampil saat berpindah halaman atau saat permintaan API berjalan lebih dari
// beberapa ratus milidetik. Memberi tahu pengguna bahwa aplikasi bekerja, tanpa menghalangi tampilan.
export function TopLoader() {
  const pathname = usePathname();
  const [progress, setProgress] = useState(0);
  const [visible, setVisible] = useState(false);
  const navigating = useRef(false);
  const netBusy = useRef(false);
  const showTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const navTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const tick = useRef<ReturnType<typeof setInterval> | null>(null);
  const hideTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const active = useRef(false);

  useEffect(() => {
    const stopTick = () => {
      if (tick.current) clearInterval(tick.current);
      tick.current = null;
    };

    const start = () => {
      if (active.current) return;
      active.current = true;
      if (hideTimer.current) clearTimeout(hideTimer.current);
      setVisible(true);
      setProgress(8);
      stopTick();
      // Maju cepat di awal lalu melambat mendekati 90%: kesan bergerak tanpa menjanjikan waktu selesai.
      tick.current = setInterval(() => setProgress((p) => p + (90 - p) * 0.07 + 0.2), 180);
    };

    const finishIfIdle = () => {
      if (navigating.current || netBusy.current || !active.current) return;
      active.current = false;
      stopTick();
      setProgress(100);
      hideTimer.current = setTimeout(() => {
        setVisible(false);
        setProgress(0);
      }, 260);
    };

    const onNet = (e: Event) => {
      const n = (e as CustomEvent<number>).detail;
      if (n > 0) {
        if (!showTimer.current && !netBusy.current) {
          showTimer.current = setTimeout(() => {
            showTimer.current = null;
            if (netCount() > 0) {
              netBusy.current = true;
              start();
            }
          }, SHOW_DELAY_MS);
        }
      } else {
        if (showTimer.current) {
          clearTimeout(showTimer.current);
          showTimer.current = null;
        }
        netBusy.current = false;
        finishIfIdle();
      }
    };

    // Klik tautan internal = mulai berpindah halaman; selesai ketika alamat berubah (efek di bawah).
    const onClick = (e: MouseEvent) => {
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      const a = (e.target as HTMLElement | null)?.closest?.("a");
      if (!a || a.target === "_blank" || a.hasAttribute("download")) return;
      const href = a.getAttribute("href");
      if (!href || !href.startsWith("/") || href.startsWith("//")) return;
      if (href.split("#")[0] === window.location.pathname + window.location.search) return;
      navigating.current = true;
      start();
      if (navTimer.current) clearTimeout(navTimer.current);
      navTimer.current = setTimeout(() => {
        navigating.current = false;
        finishIfIdle();
      }, NAV_TIMEOUT_MS);
    };

    const onNavDone = () => {
      navigating.current = false;
      if (navTimer.current) clearTimeout(navTimer.current);
      finishIfIdle();
    };

    window.addEventListener("pasti:net", onNet);
    window.addEventListener("pasti:nav-done", onNavDone);
    document.addEventListener("click", onClick, true);
    return () => {
      window.removeEventListener("pasti:net", onNet);
      window.removeEventListener("pasti:nav-done", onNavDone);
      document.removeEventListener("click", onClick, true);
      stopTick();
      [showTimer, navTimer, hideTimer].forEach((t) => t.current && clearTimeout(t.current));
    };
  }, []);

  // Alamat berubah = halaman tujuan sudah tampil.
  useEffect(() => {
    window.dispatchEvent(new Event("pasti:nav-done"));
  }, [pathname]);

  return (
    <div
      aria-hidden="true"
      className={`pointer-events-none fixed inset-x-0 top-0 z-[70] h-[3px] transition-opacity duration-300 ${visible ? "opacity-100" : "opacity-0"}`}
    >
      <div
        className="h-full rounded-r-full bg-gradient-to-r from-blue-400 via-blue-600 to-indigo-500 shadow-[0_0_10px_rgba(51,88,224,0.55)] transition-[width] duration-200 ease-out"
        style={{ width: `${Math.min(progress, 100)}%` }}
      />
    </div>
  );
}
