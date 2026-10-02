import type { ElementType } from "react";

export type DetailField = [label: string, value: string | false | null | undefined];

// Menampilkan pasangan label/nilai; pasangan yang nilainya kosong dilewati
// (false muncul dari ekspresi `nilai && format(nilai)` di pemanggil).
export function DetailEntry({ fields, bordered = true }: { fields: DetailField[]; bordered?: boolean }) {
  const visible = fields.filter(([, value]) => value);
  if (visible.length === 0) return null;
  return (
    <dl
      className={`grid grid-cols-1 gap-x-4 gap-y-2.5 text-sm sm:grid-cols-2 ${
        bordered ? "rounded-lg border border-slate-200 p-3" : ""
      }`}
    >
      {visible.map(([label, value]) => (
        <div key={label} className="min-w-0">
          <dt className="text-xs text-slate-500">{label}</dt>
          <dd className="break-words font-medium text-slate-800">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

// Satu kelompok field dengan judul; seluruh kelompok disembunyikan kalau semua nilainya kosong.
export function DetailGroup({ icon: Icon, title, fields }: { icon: ElementType; title: string; fields: DetailField[] }) {
  if (!fields.some(([, value]) => value)) return null;
  return (
    <section>
      <h3 className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-slate-700">
        <Icon className="h-4 w-4" />
        {title}
      </h3>
      <DetailEntry fields={fields} />
    </section>
  );
}
