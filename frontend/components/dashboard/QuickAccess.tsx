import Link from "next/link";
import { ArrowUpRight, DatabaseZap, ShieldCheck } from "lucide-react";
import { NAV_ENTRIES, NavItem } from "@/lib/navigation";

interface Section {
  title: string;
  icon: React.ElementType;
  items: NavItem[];
}

// Warna aksen per bagian (kelas ditulis utuh supaya terbaca oleh Tailwind). Bagian yang tidak terdaftar memakai biru.
const ACCENT: Record<string, { tile: string; hover: string; badge: string }> = {
  "Data & Aset": { tile: "bg-blue-50 text-blue-600", hover: "group-hover:bg-blue-600 group-hover:text-white", badge: "bg-blue-50 text-blue-700" },
  SAPA: { tile: "bg-violet-50 text-violet-600", hover: "group-hover:bg-violet-600 group-hover:text-white", badge: "bg-violet-50 text-violet-700" },
  "Pengadaan (Inaproc)": { tile: "bg-emerald-50 text-emerald-600", hover: "group-hover:bg-emerald-600 group-hover:text-white", badge: "bg-emerald-50 text-emerald-700" },
  "Tender (Inaproc)": { tile: "bg-amber-50 text-amber-600", hover: "group-hover:bg-amber-500 group-hover:text-white", badge: "bg-amber-50 text-amber-700" },
  Administrasi: { tile: "bg-slate-100 text-slate-600", hover: "group-hover:bg-slate-800 group-hover:text-white", badge: "bg-slate-100 text-slate-600" },
};
const DEFAULT_ACCENT = ACCENT["Data & Aset"];

// Menyusun bagian "Akses Cepat" dari daftar menu sidebar, sehingga menu baru otomatis muncul di sini. Menu tunggal tanpa
// pembatasan role dianggap layanan data, sedangkan yang dibatasi role dikelompokkan sebagai administrasi.
function buildSections(role: string): Section[] {
  const canSee = (roles?: string[]) => !roles || roles.includes(role);

  const dataItems: NavItem[] = [];
  const adminItems: NavItem[] = [];
  const groups: Section[] = [];

  for (const entry of NAV_ENTRIES) {
    if (!canSee(entry.roles)) continue;

    if (entry.type === "group") {
      groups.push({ title: entry.label, icon: entry.icon, items: entry.children.filter((c) => canSee(c.roles)) });
    } else if (entry.href !== "/dashboard") {
      (entry.roles ? adminItems : dataItems).push(entry);
    }
  }

  return [
    { title: "Data & Aset", icon: DatabaseZap, items: dataItems },
    ...groups,
    { title: "Administrasi", icon: ShieldCheck, items: adminItems },
  ].filter((s) => s.items.length > 0);
}

export function QuickAccess({ role }: { role: string }) {
  const sections = buildSections(role);

  return (
    <div className="stagger space-y-7">
      {sections.map(({ title, icon: SectionIcon, items }) => {
        const accent = ACCENT[title] ?? DEFAULT_ACCENT;
        return (
          <section key={title}>
            <div className="mb-3 flex items-center gap-2.5">
              <span className={`flex h-7 w-7 items-center justify-center rounded-lg ${accent.tile}`}>
                <SectionIcon className="h-4 w-4" aria-hidden="true" />
              </span>
              <h2 className="text-sm font-semibold text-slate-900">{title}</h2>
              <span className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${accent.badge}`}>{items.length}</span>
            </div>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              {items.map(({ href, label, description, icon: Icon }) => (
                <Link
                  key={href}
                  href={href}
                  className="card-interactive group flex items-center gap-3.5 p-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                >
                  <div className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-xl transition-colors duration-200 ${accent.tile} ${accent.hover}`}>
                    <Icon className="h-5 w-5" aria-hidden="true" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-semibold leading-snug text-slate-900">{label}</p>
                    {description && <p className="mt-0.5 line-clamp-2 text-xs leading-relaxed text-slate-500">{description}</p>}
                  </div>
                  <ArrowUpRight className="h-4 w-4 shrink-0 text-slate-300 transition duration-200 group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-blue-600" aria-hidden="true" />
                </Link>
              ))}
            </div>
          </section>
        );
      })}
    </div>
  );
}
