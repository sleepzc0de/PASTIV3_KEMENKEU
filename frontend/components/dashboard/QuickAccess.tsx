import Link from "next/link";
import { ArrowUpRight, DatabaseZap, ShieldCheck } from "lucide-react";
import { NAV_ENTRIES, NavItem } from "@/lib/navigation";

interface Section {
  title: string;
  icon: React.ElementType;
  items: NavItem[];
}

// Menyusun bagian "Akses Cepat" dari daftar menu sidebar, sehingga menu baru
// otomatis muncul di sini. Menu tunggal tanpa pembatasan role dianggap layanan
// data, sedangkan yang dibatasi role dikelompokkan sebagai administrasi.
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
    <div className="space-y-6">
      {sections.map(({ title, icon: SectionIcon, items }) => (
        <section key={title} className="rounded-xl bg-white p-5 shadow-sm">
          <div className="mb-4 flex items-center gap-2">
            <SectionIcon className="h-4 w-4 text-blue-600" />
            <h2 className="text-sm font-semibold text-slate-900">{title}</h2>
            <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-500">
              {items.length}
            </span>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {items.map(({ href, label, description, icon: Icon }) => (
              <Link
                key={href}
                href={href}
                className="group flex items-center gap-3 rounded-xl border border-transparent bg-slate-50 p-3.5 transition hover:border-blue-200 hover:bg-white hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              >
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-100 text-blue-600 transition-colors group-hover:bg-blue-600 group-hover:text-white">
                  <Icon className="h-5 w-5" />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-semibold leading-snug text-slate-900">{label}</p>
                  {description && (
                    <p className="mt-0.5 line-clamp-2 text-xs text-slate-500">{description}</p>
                  )}
                </div>
                <ArrowUpRight className="h-4 w-4 shrink-0 text-slate-300 transition group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-blue-600" />
              </Link>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
