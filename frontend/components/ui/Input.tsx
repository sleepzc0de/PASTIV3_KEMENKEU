"use client";

import { forwardRef, InputHTMLAttributes, useId, useState } from "react";
import { Eye, EyeOff, LucideIcon } from "lucide-react";
import clsx from "clsx";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  error?: string;
  icon?: LucideIcon;
  isPassword?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ label, error, icon: Icon, isPassword, className, id, ...props }, ref) => {
    const [showPassword, setShowPassword] = useState(false);
    const autoId = useId();
    const inputId = id ?? autoId;

    return (
      <div className="w-full">
        <label htmlFor={inputId} className="mb-1.5 block text-sm font-medium text-slate-700">
          {label}
        </label>
        <div className="group relative">
          {Icon && (
            <Icon className="pointer-events-none absolute left-3.5 top-1/2 h-[18px] w-[18px] -translate-y-1/2 text-slate-400 transition-colors group-focus-within:text-blue-600" />
          )}
          <input
            ref={ref}
            id={inputId}
            aria-invalid={error ? true : undefined}
            type={isPassword ? (showPassword ? "text" : "password") : props.type}
            className={clsx(
              "w-full rounded-xl border bg-white px-3.5 py-3 text-sm text-slate-900 shadow-sm outline-none transition-all placeholder:text-slate-400",
              "hover:border-slate-400 focus:border-blue-500 focus:shadow-glow",
              Icon && "pl-11",
              isPassword && "pr-11",
              error ? "border-red-400 focus:border-red-500 focus:ring-red-500/10" : "border-slate-300",
              className
            )}
            {...props}
          />
          {isPassword && (
            <button
              type="button"
              onClick={() => setShowPassword((s) => !s)}
              aria-label={showPassword ? "Sembunyikan password" : "Tampilkan password"}
              className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
            >
              {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
            </button>
          )}
        </div>
        {error && <p className="mt-1.5 animate-fade-in text-xs font-medium text-red-600">{error}</p>}
      </div>
    );
  }
);

Input.displayName = "Input";
