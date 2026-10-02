import { Briefcase, Building2, IdCard, KeyRound, Landmark, Mail } from "lucide-react";
import type { Profile } from "@/lib/dashboard-context";

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  return words
    .slice(0, 2)
    .map((w) => w[0]!.toUpperCase())
    .join("");
}

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
    <section className="rounded-xl bg-white p-5 shadow-sm">
      <div className="flex items-center gap-3">
        <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-blue-500 to-blue-700 text-base font-bold text-white">
          {getInitials(profile.full_name) || "?"}
        </div>
        <div className="min-w-0">
          <h2 className="truncate text-sm font-semibold text-slate-900">{profile.full_name}</h2>
          <p className="truncate text-xs text-slate-500">@{profile.username}</p>
        </div>
      </div>

      <dl className="mt-4 grid grid-cols-1 gap-2 border-t border-slate-100 pt-4 sm:grid-cols-2 xl:grid-cols-1">
        {rows
          .filter((r) => r.value)
          .map(({ icon: Icon, label, value }) => (
            <div key={label} className="flex items-start gap-3 rounded-lg bg-slate-50 p-3">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-white text-slate-500 shadow-sm">
                <Icon className="h-4 w-4" />
              </div>
              <div className="min-w-0">
                <dt className="text-xs text-slate-500">{label}</dt>
                <dd className="break-words text-sm font-medium text-slate-900">{value}</dd>
              </div>
            </div>
          ))}
      </dl>
    </section>
  );
}
