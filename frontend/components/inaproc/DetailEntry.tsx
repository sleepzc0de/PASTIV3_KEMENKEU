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
