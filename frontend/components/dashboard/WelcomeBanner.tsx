import { CalendarDays, Landmark, ShieldCheck, UserCog } from "lucide-react";
import type { Profile } from "@/lib/dashboard-context";
import { ROLE_LABEL } from "@/lib/roles";

function getGreeting(hour: number): string {
  if (hour < 11) return "Selamat pagi";
  if (hour < 15) return "Selamat siang";
  if (hour < 18) return "Selamat sore";
  return "Selamat malam";
}

export function WelcomeBanner({ profile }: { profile: Profile }) {
  const now = new Date();
  const today = new Intl.DateTimeFormat("id-ID", {
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
  }).format(now);

  return (
    <section className="relative overflow-hidden rounded-2xl bg-gradient-to-br from-slate-900 via-slate-900 to-blue-900 p-6 text-white shadow-sm sm:p-8">
      <ShieldCheck
        aria-hidden
        className="pointer-events-none absolute -right-8 -top-8 h-56 w-56 text-white/5"
      />

      <div className="relative">
        <p className="flex items-center gap-2 text-sm text-blue-200">
          <CalendarDays className="h-4 w-4" />
          {today}
        </p>

        <p className="mt-4 text-sm text-slate-300">{getGreeting(now.getHours())},</p>
        <h1 className="mt-0.5 break-words text-2xl font-bold sm:text-3xl">{profile.full_name}</h1>
        <p className="mt-2 max-w-xl text-sm text-slate-300">
          Selamat datang di PASTI V3, sistem pemantauan aset terintegrasi. Pilih menu di bawah untuk
          mulai bekerja.
        </p>

        <div className="mt-5 flex flex-wrap gap-2">
          <span className="inline-flex items-center gap-1.5 rounded-full bg-white/10 px-3 py-1 text-xs font-medium text-white ring-1 ring-inset ring-white/20">
            <UserCog className="h-3.5 w-3.5" />
            {ROLE_LABEL[profile.role] || profile.role}
          </span>
          {profile.auth_provider === "sso" && (
            <span className="inline-flex items-center gap-1.5 rounded-full bg-blue-500/20 px-3 py-1 text-xs font-medium text-blue-100 ring-1 ring-inset ring-blue-300/30">
              <Landmark className="h-3.5 w-3.5" />
              SSO Kemenkeu
            </span>
          )}
        </div>
      </div>
    </section>
  );
}
