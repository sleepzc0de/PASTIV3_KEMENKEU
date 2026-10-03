"use client";

import { KeyboardEvent, ReactNode, useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { LucideIcon } from "lucide-react";

export interface TabItem<K extends string> {
  key: K;
  label: string;
  icon?: LucideIcon;
  badge?: ReactNode;
}

interface TabsProps<K extends string> {
  tabs: TabItem<K>[];
  value: K;
  onChange: (key: K) => void;
  label: string;
  // Awalan id: tombol berid `${idPrefix}-tab-${key}`; panel pemakai memakai `${idPrefix}-panel-${key}` dan aria-labelledby ke tombol.
  idPrefix: string;
}

// Tab berbentuk pil dengan penanda aktif yang meluncur halus. Mengikuti pola WAI-ARIA: panah kiri/kanan, Home, End.
export function Tabs<K extends string>({ tabs, value, onChange, label, idPrefix }: TabsProps<K>) {
  const refs = useRef<Record<string, HTMLButtonElement | null>>({});
  const wrap = useRef<HTMLDivElement>(null);
  const [indicator, setIndicator] = useState<{ left: number; width: number } | null>(null);

  const measure = useCallback(() => {
    const el = refs.current[value];
    if (!el) return;
    setIndicator({ left: el.offsetLeft, width: el.offsetWidth });
  }, [value]);

  useLayoutEffect(measure, [measure, tabs.length]);

  // Lebar tombol berubah saat ukuran layar atau font berubah; ukur ulang agar penanda tetap pas.
  useEffect(() => {
    const ro = typeof ResizeObserver !== "undefined" && wrap.current ? new ResizeObserver(measure) : null;
    if (ro && wrap.current) ro.observe(wrap.current);
    window.addEventListener("resize", measure);
    return () => {
      ro?.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, [measure]);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const i = tabs.findIndex((t) => t.key === value);
    let next = -1;
    if (e.key === "ArrowRight") next = (i + 1) % tabs.length;
    else if (e.key === "ArrowLeft") next = (i - 1 + tabs.length) % tabs.length;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = tabs.length - 1;
    if (next < 0) return;
    e.preventDefault();
    onChange(tabs[next].key);
    refs.current[tabs[next].key]?.focus();
  };

  return (
    <div className="no-scrollbar -mx-1 overflow-x-auto px-1 py-1">
      <div
        ref={wrap}
        role="tablist"
        aria-label={label}
        onKeyDown={onKeyDown}
        className="relative inline-flex min-w-full gap-1 rounded-2xl bg-slate-100 p-1 sm:min-w-0"
      >
        {indicator && (
          <span
            aria-hidden="true"
            className="absolute bottom-1 top-1 rounded-xl bg-white shadow-sm ring-1 ring-slate-900/5 transition-[left,width] duration-300 ease-smooth"
            style={{ left: indicator.left, width: indicator.width }}
          />
        )}
        {tabs.map(({ key, label: text, icon: Icon, badge }) => {
          const selected = value === key;
          return (
            <button
              key={key}
              ref={(el) => {
                refs.current[key] = el;
              }}
              type="button"
              role="tab"
              id={`${idPrefix}-tab-${key}`}
              aria-selected={selected}
              aria-controls={`${idPrefix}-panel-${key}`}
              tabIndex={selected ? 0 : -1}
              onClick={() => onChange(key)}
              className={`relative z-10 inline-flex flex-1 shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-xl px-3 py-2 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 sm:flex-none sm:px-4 ${
                selected ? "text-blue-700" : "text-slate-500 hover:text-slate-800"
              }`}
            >
              {/* Ikon disembunyikan di layar sempit supaya semua tab muat tanpa menggulir. */}
              {Icon && <Icon className="hidden h-4 w-4 sm:block" aria-hidden="true" />}
              {text}
              {badge}
            </button>
          );
        })}
      </div>
    </div>
  );
}
