"use client";

import { ReactNode, useId } from "react";
import { AlertCircle, Info } from "lucide-react";
import type { ErrorInfo } from "./sapa";

// Kontrol formulir kecil yang dipakai berulang di halaman SAPA. Gaya mengikuti Input di components/ui.

export const inputCls =
  "w-full rounded-xl border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-900 shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 " +
  "focus:border-blue-500 focus:shadow-glow disabled:cursor-not-allowed disabled:bg-slate-50 disabled:text-slate-500 disabled:shadow-none";

export function Field({
  label,
  hint,
  required,
  htmlFor,
  className = "",
  children,
}: {
  label: string;
  hint?: string;
  required?: boolean;
  htmlFor: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={className}>
      <label htmlFor={htmlFor} className="mb-1.5 block text-xs font-medium text-slate-600">
        {label}
        {required && (
          <span aria-hidden="true" className="ml-0.5 text-red-500">
            *
          </span>
        )}
      </label>
      {children}
      {hint && <p className="mt-1 text-[11px] leading-snug text-slate-500">{hint}</p>}
    </div>
  );
}

interface TextFieldProps {
  label: string;
  value: string;
  onChange: (v: string) => void;
  hint?: string;
  required?: boolean;
  placeholder?: string;
  maxLength?: number;
  type?: "text" | "date";
  inputMode?: "text" | "numeric" | "decimal";
  disabled?: boolean;
  className?: string;
  list?: string;
  autoComplete?: string;
}

export function TextField({ label, value, onChange, hint, required, placeholder, maxLength, type = "text", inputMode, disabled, className, list, autoComplete }: TextFieldProps) {
  const id = useId();
  return (
    <Field label={label} hint={hint} required={required} htmlFor={id} className={className}>
      <input
        id={id}
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        maxLength={maxLength}
        inputMode={inputMode}
        disabled={disabled}
        list={list}
        autoComplete={autoComplete ?? "off"}
        className={inputCls}
      />
    </Field>
  );
}

export function TextAreaField({
  label,
  value,
  onChange,
  hint,
  required,
  placeholder,
  maxLength,
  rows = 3,
  disabled,
  className,
}: Omit<TextFieldProps, "type" | "inputMode" | "list" | "autoComplete"> & { rows?: number }) {
  const id = useId();
  return (
    <Field label={label} hint={hint} required={required} htmlFor={id} className={className}>
      <textarea
        id={id}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        maxLength={maxLength}
        rows={rows}
        disabled={disabled}
        className={inputCls + " resize-y"}
      />
    </Field>
  );
}

export function SelectInput({
  label,
  value,
  onChange,
  options,
  hint,
  required,
  disabled,
  placeholder = "Pilih…",
  className,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: string[];
  hint?: string;
  required?: boolean;
  disabled?: boolean;
  placeholder?: string;
  className?: string;
}) {
  const id = useId();
  return (
    <Field label={label} hint={hint} required={required} htmlFor={id} className={className}>
      <select id={id} value={value} onChange={(e) => onChange(e.target.value)} disabled={disabled} className={inputCls}>
        <option value="">{placeholder}</option>
        {options.map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </select>
    </Field>
  );
}

export function CheckField({
  label,
  checked,
  onChange,
  disabled,
  hint,
}: {
  label: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  hint?: string;
}) {
  const id = useId();
  return (
    <div>
      <label htmlFor={id} className="flex cursor-pointer items-start gap-2.5 text-sm text-slate-700">
        <input
          id={id}
          type="checkbox"
          checked={checked}
          onChange={(e) => onChange(e.target.checked)}
          disabled={disabled}
          className="mt-0.5 h-4 w-4 shrink-0 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
        />
        <span>{label}</span>
      </label>
      {hint && <p className="ml-[26px] mt-1 text-[11px] text-slate-500">{hint}</p>}
    </div>
  );
}

export function Section({ title, description, children, action }: { title: string; description?: string; children: ReactNode; action?: ReactNode }) {
  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-2 border-b border-slate-100 pb-2">
        <div className="min-w-0">
          <h4 className="text-sm font-semibold text-slate-800">{title}</h4>
          {description && <p className="mt-0.5 text-xs text-slate-500">{description}</p>}
        </div>
        {action}
      </div>
      {children}
    </section>
  );
}

// Kotak galat: pesan utama, ditambah rincian validasi per bidang bila ada.
export function ErrorBox({ error }: { error: ErrorInfo | null }) {
  if (!error) return null;
  return (
    <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
      <div className="flex items-start gap-2">
        <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{error.message}</span>
      </div>
      {error.errors.length > 0 && (
        <ul className="ml-6 mt-1.5 list-disc space-y-0.5 text-xs">
          {error.errors.map((e, i) => (
            <li key={i}>{e}</li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function NoticeBox({ children, tone = "info" }: { children: ReactNode; tone?: "info" | "warn" | "ok" }) {
  const cls = {
    info: "border-blue-200 bg-blue-50 text-blue-800",
    warn: "border-amber-200 bg-amber-50 text-amber-800",
    ok: "border-emerald-200 bg-emerald-50 text-emerald-800",
  }[tone];
  return (
    <div role={tone === "warn" ? "alert" : "status"} className={`flex items-start gap-2 rounded-lg border px-3.5 py-2.5 text-sm ${cls}`}>
      <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <div className="min-w-0">{children}</div>
    </div>
  );
}

export function PrimaryButton({
  children,
  onClick,
  disabled,
  busy,
  type = "button",
  className = "",
}: {
  children: ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  busy?: boolean;
  type?: "button" | "submit";
  className?: string;
}) {
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled || busy}
      className={`inline-flex items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm shadow-blue-600/20 transition hover:bg-blue-700 hover:shadow-md active:scale-[0.97] disabled:cursor-not-allowed disabled:opacity-55 ${className}`}
    >
      {busy && <span className="h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white" aria-hidden="true" />}
      {children}
    </button>
  );
}

export function SecondaryButton({
  children,
  onClick,
  disabled,
  busy,
  className = "",
}: {
  children: ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  busy?: boolean;
  className?: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled || busy}
      className={`inline-flex items-center justify-center gap-2 rounded-lg border border-slate-300 bg-white px-4 py-2.5 text-sm font-medium text-slate-700 shadow-sm transition hover:border-slate-400 hover:bg-slate-50 active:scale-[0.97] disabled:cursor-not-allowed disabled:opacity-55 ${className}`}
    >
      {busy && <span className="h-4 w-4 animate-spin rounded-full border-2 border-slate-300 border-t-slate-600" aria-hidden="true" />}
      {children}
    </button>
  );
}
