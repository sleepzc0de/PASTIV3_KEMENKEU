import clsx from "clsx";

// Kerangka pemuatan dengan kilau halus. Dipakai menggantikan putaran indikator supaya tata letak tidak melompat
// ketika data tiba.
export function Skeleton({ className }: { className?: string }) {
  return <div aria-hidden="true" className={clsx("skeleton", className)} />;
}

export function SkeletonText({ lines = 3, className }: { lines?: number; className?: string }) {
  return (
    <div aria-hidden="true" className={clsx("space-y-2.5", className)}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton key={i} className={clsx("h-3.5", i === lines - 1 ? "w-2/3" : "w-full")} />
      ))}
    </div>
  );
}

// Kerangka tabel: satu baris judul dan beberapa baris isi.
export function SkeletonTable({ rows = 6, cols = 5, label = "Memuat data" }: { rows?: number; cols?: number; label?: string }) {
  return (
    <div role="status" aria-label={label} className="overflow-hidden rounded-xl border border-slate-200 bg-white">
      <div className="grid gap-4 border-b border-slate-100 bg-slate-50 px-4 py-3" style={{ gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))` }}>
        {Array.from({ length: cols }).map((_, i) => (
          <Skeleton key={i} className="h-3 w-3/4" />
        ))}
      </div>
      {Array.from({ length: rows }).map((_, r) => (
        <div
          key={r}
          className="grid gap-4 border-b border-slate-50 px-4 py-3.5 last:border-0"
          style={{ gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))` }}
        >
          {Array.from({ length: cols }).map((_, c) => (
            <Skeleton key={c} className={clsx("h-3.5", c === 0 ? "w-5/6" : c % 2 ? "w-2/3" : "w-full")} />
          ))}
        </div>
      ))}
    </div>
  );
}

export function SkeletonCards({ count = 3, className }: { count?: number; className?: string }) {
  return (
    <div role="status" aria-label="Memuat" className={clsx("grid gap-4 sm:grid-cols-2 xl:grid-cols-3", className)}>
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="rounded-2xl bg-white p-5 shadow-card">
          <Skeleton className="h-10 w-10 rounded-xl" />
          <Skeleton className="mt-4 h-4 w-1/2" />
          <Skeleton className="mt-3 h-3 w-full" />
          <Skeleton className="mt-2 h-3 w-4/5" />
        </div>
      ))}
    </div>
  );
}
