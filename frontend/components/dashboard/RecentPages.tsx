"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { History } from "lucide-react";
import { flattenNav, FlatNavItem } from "@/lib/navigation";
import { readRecent } from "@/components/layout/CommandPalette";

// Halaman yang baru dibuka pengguna ini (disimpan di browser, tidak dikirim ke server): jalan pintas satu klik.
export function RecentPages({ role }: { role: string }) {
  const [items, setItems] = useState<FlatNavItem[]>([]);

  useEffect(() => {
    const all = flattenNav(role);
    setItems(
      readRecent()
        .filter((h) => h !== "/dashboard") // halaman ini sendiri
        .map((h) => all.find((i) => i.href === h))
        .filter((x): x is FlatNavItem => Boolean(x))
    );
  }, [role]);

  if (items.length === 0) return null;

  return (
    <section aria-label="Baru dibuka" className="animate-fade-up">
      <div className="mb-2.5 flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-slate-500">
        <History className="h-3.5 w-3.5" aria-hidden="true" />
        Baru dibuka
      </div>
      <div className="no-scrollbar -mx-1 flex gap-2 overflow-x-auto px-1 pb-1">
        {items.map(({ href, label, icon: Icon }) => (
          <Link
            key={href}
            href={href}
            className="flex shrink-0 items-center gap-2 rounded-xl bg-white px-3.5 py-2.5 text-sm font-medium text-slate-700 shadow-card transition hover:-translate-y-0.5 hover:text-blue-700 hover:shadow-md"
          >
            <Icon className="h-4 w-4 text-slate-400" aria-hidden="true" />
            {label}
          </Link>
        ))}
      </div>
    </section>
  );
}
