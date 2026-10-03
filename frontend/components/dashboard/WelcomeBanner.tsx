import { CalendarDays, Landmark, Search, ShieldCheck, UserCog } from "lucide-react";
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
  const firstName = profile.full_name.trim().split(/\s+/)[0] || profile.full_name;

  return (
    <section className="relative overflow-hidden rounded-3xl bg-slate-950 p-6 text-white shadow-lg sm:p-8">
      <div
        aria-hidden="true"
        className="absolute inset-0 bg-[radial-gradient(50rem_26rem_at_0%_0%,rgba(79,115,242,0.6),transparent),radial-gradient(34rem_22rem_at_100%_100%,rgba(51,88,224,0.5),transparent)]"
      />
      <div
        aria-hidden="true"
        className="absolute inset-0 opacity-[0.06] [background-image:linear-gradient(white_1px,transparent_1px),linear-gradient(90deg,white_1px,transparent_1px)] [background-size:40px_40px] [mask-image:linear-gradient(to_left,black,transparent_70%)]"
      />
      <ShieldCheck aria-hidden className="pointer-events-none absolute -right-6 -top-6 h-52 w-52 text-white/[0.06] sm:h-64 sm:w-64" />

      <div className="relative">
        <p className="flex items-center gap-2 text-sm text-blue-100/90">
          <CalendarDays className="h-4 w-4" aria-hidden="true" />
          {today}
        </p>

        <h1 className="mt-4 break-words text-2xl font-bold tracking-tight sm:text-3xl">
          {getGreeting(now.getHours())}, <span className="bg-gradient-to-r from-white to-blue-200 bg-clip-text text-transparent">{firstName}</span>
        </h1>
        <p className="mt-2 max-w-xl text-sm leading-relaxed text-blue-100/80">
          Selamat datang di PASTI V3. Pilih modul di bawah untuk mulai bekerja, atau tekan{" "}
          <kbd className="rounded-md border border-white/20 bg-white/10 px-1.5 py-0.5 text-[11px] font-medium">Ctrl K</kbd> untuk mencari halaman dengan cepat.
        </p>

        <div className="mt-5 flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1.5 rounded-full bg-white/10 px-3 py-1 text-xs font-medium text-white ring-1 ring-inset ring-white/20 backdrop-blur">
            <UserCog className="h-3.5 w-3.5" aria-hidden="true" />
            {ROLE_LABEL[profile.role] || profile.role}
          </span>
          {profile.auth_provider === "sso" && (
            <span className="inline-flex items-center gap-1.5 rounded-full bg-blue-400/20 px-3 py-1 text-xs font-medium text-blue-50 ring-1 ring-inset ring-blue-200/30 backdrop-blur">
              <Landmark className="h-3.5 w-3.5" aria-hidden="true" />
              SSO Kemenkeu
            </span>
          )}
          <span className="hidden items-center gap-1.5 text-xs text-blue-100/70 sm:inline-flex">
            <Search className="h-3.5 w-3.5" aria-hidden="true" />
            Cari menu: Ctrl K
          </span>
        </div>
      </div>
    </section>
  );
}
