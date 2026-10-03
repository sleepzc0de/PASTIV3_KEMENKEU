import { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

interface PageHeaderProps {
  title: string;
  description?: ReactNode;
  icon?: LucideIcon;
  actions?: ReactNode;
}

// Judul halaman: ikon berlatar gradien, judul besar, deskripsi, dan slot aksi di kanan.
export function PageHeader({ title, description, icon: Icon, actions }: PageHeaderProps) {
  return (
    <header className="flex flex-wrap items-start justify-between gap-4">
      <div className="flex min-w-0 items-start gap-3.5">
        {Icon && (
          <div className="mt-0.5 flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-blue-500 to-blue-700 text-white shadow-md shadow-blue-600/25">
            <Icon className="h-5 w-5" aria-hidden="true" />
          </div>
        )}
        <div className="min-w-0">
          <h1 className="text-balance text-xl font-bold tracking-tight text-slate-900 sm:text-2xl">{title}</h1>
          {description && <p className="mt-1 max-w-3xl text-sm leading-relaxed text-slate-500">{description}</p>}
        </div>
      </div>
      {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
    </header>
  );
}

// Kerangka halaman standar: header + satu kartu isi. Dipakai hampir semua halaman supaya tampilannya seragam.
export function PageShell({
  title,
  description,
  icon,
  actions,
  children,
  bare,
}: PageHeaderProps & { children: ReactNode; bare?: boolean }) {
  return (
    <div className="w-full space-y-5">
      <PageHeader title={title} description={description} icon={icon} actions={actions} />
      {bare ? children : <div className="card p-4 sm:p-6">{children}</div>}
    </div>
  );
}
