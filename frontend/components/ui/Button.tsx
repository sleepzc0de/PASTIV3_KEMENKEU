import { ButtonHTMLAttributes, ReactNode } from "react";
import { Loader2 } from "lucide-react";
import clsx from "clsx";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "soft";
export type ButtonSize = "sm" | "md" | "lg";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  isLoading?: boolean;
  variant?: ButtonVariant;
  size?: ButtonSize;
  // Bawaan lama: tombol selebar wadahnya. Setel false untuk tombol seukuran isinya.
  fullWidth?: boolean;
  icon?: ReactNode;
}

export const buttonClasses = (variant: ButtonVariant = "primary", size: ButtonSize = "md", fullWidth = false) =>
  clsx(
    "inline-flex select-none items-center justify-center gap-2 whitespace-nowrap rounded-lg font-semibold transition-all duration-150 ease-smooth",
    "active:scale-[0.97] disabled:cursor-not-allowed disabled:opacity-55 disabled:active:scale-100",
    fullWidth && "w-full",
    {
      sm: "px-3 py-1.5 text-xs",
      md: "px-4 py-2.5 text-sm",
      lg: "px-5 py-3 text-base",
    }[size],
    {
      primary:
        "bg-blue-600 text-white shadow-sm shadow-blue-600/20 hover:bg-blue-700 hover:shadow-md hover:shadow-blue-600/25 disabled:hover:bg-blue-600 disabled:hover:shadow-sm",
      secondary: "border border-slate-300 bg-white text-slate-700 shadow-sm hover:border-slate-400 hover:bg-slate-50",
      soft: "bg-blue-50 text-blue-700 hover:bg-blue-100",
      ghost: "text-slate-600 hover:bg-slate-100 hover:text-slate-900",
      danger: "bg-red-600 text-white shadow-sm shadow-red-600/20 hover:bg-red-700",
    }[variant]
  );

export function Button({ children, isLoading, className, disabled, variant = "primary", size = "md", fullWidth = true, icon, ...props }: ButtonProps) {
  return (
    <button disabled={disabled || isLoading} className={clsx(buttonClasses(variant, size, fullWidth), className)} {...props}>
      {isLoading ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" /> : icon}
      {children}
    </button>
  );
}
