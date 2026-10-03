import { Briefcase, Building2, IdCard, KeyRound, Landmark, Mail } from "lucide-react";
import type { Profile } from "@/lib/dashboard-context";
import { initialsOf } from "@/lib/initials";

interface DetailRow {
  icon: React.ElementType;
  label: string;
  value?: string;
}

export function ProfileCard({ profile }: { profile: Profile }) {
  const rows: DetailRow[] = [
    { icon: Mail, label: "Email", value: profile.email },
    { icon: IdCard, label: "NIP", value: profile.nip },
    { icon: Briefcase, label: "Jabatan", value: profile.jabatan },
    { icon: Building2, label: "Satuan Kerja", value: profile.satker },
    { icon: Landmark, label: "Organisasi", value: profile.organisasi },
    {
      icon: KeyRound,
      label: "Metode Login",
      value: profile.auth_provider === "sso" ? "SSO Kemenkeu" : "Akun lokal",
    },
  ];

  return (
    <section className="card overflow-hidden">
      <div className="h-16 bg-gradient-to-r from-blue-600 to-indigo-600" aria-hidden="true" />
      <div className="px-5 pb-5">
        <div className="-mt-8 flex h-16 w-16 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-blue-500 to-blue-700 text-xl font-bold text-white shadow-lg ring-4 ring-white">
          {initialsOf(profile.full_name)}
        </div>
        <div className="mt-3 min-w-0">
          <h2 className="break-words text-base font-semibold leading-tight text-slate-900">{profile.full_name}</h2>
          <p className="truncate text-xs text-slate-500">@{profile.username}</p>
        </div>

        <dl className="mt-4 space-y-1">
          {rows
            .filter((r) => r.value)
            .map(({ icon: Icon, label, value }) => (
              <div key={label} className="flex items-start gap-3 rounded-xl p-2.5 transition-colors hover:bg-slate-50">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-500">
                  <Icon className="h-4 w-4" aria-hidden="true" />
                </div>
                <div className="min-w-0">
                  <dt className="text-[11px] font-medium uppercase tracking-wide text-slate-400">{label}</dt>
                  <dd className="break-words text-sm font-medium text-slate-800">{value}</dd>
                </div>
              </div>
            ))}
        </dl>
      </div>
    </section>
  );
}
