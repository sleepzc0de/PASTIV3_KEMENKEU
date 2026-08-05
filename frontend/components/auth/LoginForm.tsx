"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { User, Lock, Landmark } from "lucide-react";
import axios from "axios";

import { loginSchema, LoginFormData } from "@/lib/validation";
import { useAuth } from "@/lib/auth-context";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";

export function LoginForm() {
  const { login, isLoading } = useAuth();
  const [serverError, setServerError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
  });

  const onSubmit = async (data: LoginFormData) => {
    setServerError(null);
    try {
      await login(data.username, data.password);
    } catch (err) {
      if (axios.isAxiosError(err) && err.response) {
        setServerError(err.response.data?.message || "Login gagal, silakan coba lagi");
      } else {
        setServerError("Tidak dapat terhubung ke server");
      }
    }
  };

  const ssoLoginUrl = process.env.NEXT_PUBLIC_API_ROOT_URL + "/sso/login";

  return (
    <div className="w-full space-y-5">
      <a href={ssoLoginUrl} className="flex w-full items-center justify-center gap-2 rounded-lg border border-slate-300 bg-white px-4 py-2.5 text-sm font-semibold text-slate-700 shadow-sm transition-all hover:bg-slate-50 active:scale-[0.98]">
        <Landmark className="h-4 w-4 text-blue-700" />
        Masuk dengan SSO Kemenkeu
      </a>

      <div className="flex items-center gap-3">
        <div className="h-px flex-1 bg-slate-200" />
        <span className="text-xs font-medium text-slate-400">ATAU</span>
        <div className="h-px flex-1 bg-slate-200" />
      </div>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        {serverError && <Alert message={serverError} />}

        <Input
          label="Username atau Email"
          icon={User}
          placeholder="Masukkan username Anda"
          error={errors.username?.message}
          {...register("username")}
        />

        <Input
          label="Password"
          icon={Lock}
          isPassword
          placeholder="Masukkan password Anda"
          error={errors.password?.message}
          {...register("password")}
        />

        <div className="flex items-center justify-between text-sm">
          <label className="flex items-center gap-2 text-slate-600">
            <input type="checkbox" className="h-4 w-4 rounded border-slate-300 text-blue-600" />
            Ingat saya
          </label>
          <a href="#" className="font-medium text-blue-600 hover:underline">
            Lupa password?
          </a>
        </div>

        <Button type="submit" isLoading={isLoading}>
          {isLoading ? "Memproses..." : "Masuk"}
        </Button>
      </form>
    </div>
  );
}