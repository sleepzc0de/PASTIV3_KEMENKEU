"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { CornerDownLeft, History, LogOut, Search } from "lucide-react";
import { FlatNavItem, flattenNav } from "@/lib/navigation";
import { useAuth } from "@/lib/auth-context";
import { useDashboard } from "@/lib/dashboard-context";

export const RECENT_KEY = "pasti_recent_pages";
const MAX_RECENT = 4;

export function readRecent(): string[] {
  try {
    const v = JSON.parse(localStorage.getItem(RECENT_KEY) ?? "[]");
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === "string").slice(0, MAX_RECENT) : [];
  } catch {
    return [];
  }
}

export function rememberPage(href: string): void {
  try {
    const next = [href, ...readRecent().filter((h) => h !== href)].slice(0, MAX_RECENT);
    localStorage.setItem(RECENT_KEY, JSON.stringify(next));
  } catch {
    // penyimpanan tidak tersedia (mode privat): tidak masalah
  }
}

interface Entry {
  key: string;
  label: string;
  hint?: string;
  icon: React.ElementType;
  run: () => void;
  recent?: boolean;
}

const norm = (s: string) => s.toLowerCase();

// Pencarian menu cepat (Ctrl/Cmd + K): ketik sebagian nama halaman, tekan Enter. Halaman yang baru dibuka tampil lebih dulu.
export function CommandPalette({ open, onClose }: { open: boolean; onClose: () => void }) {
  const router = useRouter();
  const { logout } = useAuth();
  const { profile, semuaData } = useDashboard();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const [recent, setRecent] = useState<string[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  useEffect(() => {
    if (!open) return;
    setQuery("");
    setActive(0);
    setRecent(readRecent());
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    // Fokus setelah render supaya animasi masuk tidak menggeser kursor.
    const t = setTimeout(() => inputRef.current?.focus(), 10);
    return () => {
      clearTimeout(t);
      document.body.style.overflow = overflow;
    };
  }, [open]);

  const entries = useMemo<Entry[]>(() => {
    const items = flattenNav(profile?.role, semuaData);
    const go = (href: string) => () => {
      onClose();
      router.push(href);
    };
    const toEntry = (it: FlatNavItem, isRecent = false): Entry => ({
      key: it.href + (isRecent ? "#r" : ""),
      label: it.label,
      hint: it.group ? `${it.group}` : "Menu utama",
      icon: isRecent ? History : it.icon,
      run: go(it.href),
      recent: isRecent,
    });

    const q = norm(query.trim());
    if (q === "") {
      const rec = recent.map((h) => items.find((i) => i.href === h)).filter((x): x is FlatNavItem => Boolean(x));
      return [...rec.map((i) => toEntry(i, true)), ...items.filter((i) => !rec.includes(i)).map((i) => toEntry(i))];
    }
    const tokens = q.split(/\s+/);
    const hit = items.filter((i) => {
      const hay = norm(`${i.label} ${i.group ?? ""} ${i.description ?? ""}`);
      return tokens.every((t) => hay.includes(t));
    });
    // Cocok di judul lebih dulu daripada cocok di deskripsi saja.
    hit.sort((a, b) => Number(norm(b.label).includes(q)) - Number(norm(a.label).includes(q)));
    const list = hit.map((i) => toEntry(i));
    if ("keluar logout".includes(q) || q === "k") {
      list.push({ key: "logout", label: "Keluar dari aplikasi", hint: "Akun", icon: LogOut, run: () => logout("manual") });
    }
    return list;
  }, [query, profile?.role, semuaData, recent, router, onClose, logout]);

  useEffect(() => {
    setActive(0);
  }, [query]);

  useEffect(() => {
    listRef.current?.querySelector<HTMLElement>(`[data-index="${active}"]`)?.scrollIntoView({ block: "nearest" });
  }, [active]);

  if (!open) return null;

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((a) => (entries.length ? (a + 1) % entries.length : 0));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((a) => (entries.length ? (a - 1 + entries.length) % entries.length : 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      entries[active]?.run();
    } else if (e.key === "Escape") {
      e.preventDefault();
      onClose();
    }
  };

  return (
    <div
      className="fixed inset-0 z-[65] flex animate-fade-in items-start justify-center bg-slate-950/50 px-4 pt-[12vh] backdrop-blur-sm"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div role="dialog" aria-modal="true" aria-label="Cari menu" className="w-full max-w-xl animate-scale-in overflow-hidden rounded-2xl bg-white shadow-xl ring-1 ring-slate-900/5">
        <div className="flex items-center gap-3 border-b border-slate-100 px-4">
          <Search className="h-5 w-5 shrink-0 text-slate-400" aria-hidden="true" />
          <input
            ref={inputRef}
            role="combobox"
            aria-expanded="true"
            aria-controls="palette-list"
            aria-activedescendant={entries[active] ? `palette-opt-${active}` : undefined}
            aria-label="Cari halaman atau menu"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder="Cari halaman atau menu…"
            autoComplete="off"
            className="h-14 w-full bg-transparent text-sm text-slate-900 outline-none placeholder:text-slate-400"
          />
          <kbd className="hidden shrink-0 rounded-md border border-slate-200 bg-slate-50 px-1.5 py-0.5 text-[11px] font-medium text-slate-500 sm:block">Esc</kbd>
        </div>

        <ul id="palette-list" role="listbox" ref={listRef} className="max-h-[50vh] overflow-y-auto p-2">
          {entries.length === 0 && (
            <li className="px-3 py-10 text-center text-sm text-slate-500">
              Tidak ada menu yang cocok dengan &ldquo;{query}&rdquo;.
            </li>
          )}
          {entries.map((e, i) => {
            const Icon = e.icon;
            const sel = i === active;
            return (
              <li
                key={e.key}
                id={`palette-opt-${i}`}
                role="option"
                aria-selected={sel}
                data-index={i}
                onMouseMove={() => setActive(i)}
                onClick={e.run}
                className={`flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 ${sel ? "bg-blue-50" : ""}`}
              >
                <span className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${sel ? "bg-blue-600 text-white" : "bg-slate-100 text-slate-500"}`}>
                  <Icon className="h-[18px] w-[18px]" aria-hidden="true" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className={`block truncate text-sm font-medium ${sel ? "text-blue-900" : "text-slate-800"}`}>{e.label}</span>
                  <span className="block truncate text-xs text-slate-500">{e.recent ? `Baru dibuka · ${e.hint}` : e.hint}</span>
                </span>
                {sel && <CornerDownLeft className="h-4 w-4 shrink-0 text-blue-500" aria-hidden="true" />}
              </li>
            );
          })}
        </ul>

        <div className="hidden items-center gap-4 border-t border-slate-100 bg-slate-50/70 px-4 py-2 text-[11px] text-slate-500 sm:flex">
          <span>
            <kbd className="rounded border border-slate-200 bg-white px-1">↑</kbd> <kbd className="rounded border border-slate-200 bg-white px-1">↓</kbd> pilih
          </span>
          <span>
            <kbd className="rounded border border-slate-200 bg-white px-1">Enter</kbd> buka
          </span>
          <span className="ml-auto">Pintasan: Ctrl + K</span>
        </div>
      </div>
    </div>
  );
}
