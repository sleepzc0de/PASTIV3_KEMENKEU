import { Skeleton } from "@/components/ui/Skeleton";

// Tampil seketika saat berpindah halaman sambil kode halaman tujuan dimuat.
export default function DashboardLoading() {
  return (
    <div role="status" aria-label="Memuat halaman" className="w-full space-y-5">
      <div className="flex items-start gap-3.5">
        <Skeleton className="h-11 w-11 rounded-2xl" />
        <div className="flex-1 space-y-2.5">
          <Skeleton className="h-6 w-64 max-w-full" />
          <Skeleton className="h-3.5 w-full max-w-xl" />
        </div>
      </div>
      <div className="card space-y-4 p-6">
        <Skeleton className="h-10 w-full max-w-md" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-11/12" />
        <Skeleton className="h-4 w-4/5" />
        <Skeleton className="h-40 w-full" />
      </div>
    </div>
  );
}
