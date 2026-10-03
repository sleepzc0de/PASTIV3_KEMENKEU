import { AlertCircle, CheckCircle2, Info, TriangleAlert } from "lucide-react";

export type AlertTone = "error" | "success" | "warning" | "info";

const TONES: Record<AlertTone, { cls: string; Icon: typeof AlertCircle }> = {
  error: { cls: "border-red-200 bg-red-50 text-red-800", Icon: AlertCircle },
  success: { cls: "border-emerald-200 bg-emerald-50 text-emerald-800", Icon: CheckCircle2 },
  warning: { cls: "border-amber-200 bg-amber-50 text-amber-900", Icon: TriangleAlert },
  info: { cls: "border-blue-200 bg-blue-50 text-blue-800", Icon: Info },
};

// Bawaan tetap galat (merah), seperti pemakaian lama; nada lain untuk keberhasilan, peringatan, dan info.
export function Alert({ message, tone = "error" }: { message: string; tone?: AlertTone }) {
  const { cls, Icon } = TONES[tone];
  return (
    <div role={tone === "error" || tone === "warning" ? "alert" : "status"} className={`flex animate-fade-in items-start gap-2.5 rounded-xl border px-3.5 py-3 text-sm ${cls}`}>
      <Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <span className="min-w-0 break-words">{message}</span>
    </div>
  );
}
