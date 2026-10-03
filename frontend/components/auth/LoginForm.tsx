"use client";

import { useState } from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { User, Lock, Landmark, Info } from "lucide-react";
import axios from "axios";

import { loginSchema, LoginFormData } from "@/lib/validation";
import { useAuth } from "@/lib/auth-context";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import { ModalShell } from "@/components/ui/ModalShell";
import { CaptchaField } from "@/components/auth/CaptchaField";

export function LoginForm() {
  const { login, isLoading } = useAuth();
  const [serverError, setServerError] = useState<string | null>(null);
  const [captchaId, setCaptchaId] = useState("");
  const [showForgotPasswordInfo, setShowForgotPasswordInfo] = useState(false);

  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
    defaultValues: { captcha_answer: "" },
  });

  const onSubmit = async (data: LoginFormData) => {
    setServerError(null);
    if (!captchaId) {
      setServerError("Captcha belum siap, silakan tunggu sebentar");
      return;
    }
    try {
      await login(data.username, data.password, captchaId, data.captcha_answer);
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
    <div className="w-full space-y-6">
      <a
        href={ssoLoginUrl}
        className="group flex w-full items-center justify-center gap-2.5 rounded-xl border border-slate-300 bg-white px-4 py-3 text-sm font-semibold text-slate-800 shadow-sm transition-all hover:-translate-y-0.5 hover:border-blue-300 hover:shadow-md active:translate-y-0 active:scale-[0.98]"
      >
        <span className="flex h-6 w-6 items-center justify-center rounded-md bg-blue-50 text-blue-700 transition-colors group-hover:bg-blue-600 group-hover:text-white">
          <Landmark className="h-3.5 w-3.5" />
        </span>
        Masuk dengan SSO Kemenkeu
      </a>

      <div className="flex items-center gap-3">
        <div className="h-px flex-1 bg-slate-200" />
        <span className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">atau dengan akun PASTI</span>
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

        <Controller
          name="captcha_answer"
          control={control}
          render={({ field }) => (
            <CaptchaField
              value={field.value}
              onChange={field.onChange}
              onCaptchaIdChange={setCaptchaId}
              error={errors.captcha_answer?.message}
            />
          )}
        />

        <div className="flex justify-end text-sm">
          <button
            type="button"
            onClick={() => setShowForgotPasswordInfo(true)}
            className="font-medium text-blue-600 hover:text-blue-700 hover:underline"
          >
            Lupa password?
          </button>
        </div>

        <Button type="submit" isLoading={isLoading} size="lg" className="rounded-xl">
          {isLoading ? "Memproses..." : "Masuk"}
        </Button>
      </form>

      {showForgotPasswordInfo && (
        <ModalShell title="Lupa Password" subtitle="Cara mengatur ulang akses akun" size="sm" onClose={() => setShowForgotPasswordInfo(false)}>
          <div className="p-5 sm:p-6">
            <div className="flex items-start gap-3">
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600">
                <Info className="h-5 w-5" />
              </span>
              <p className="text-sm leading-relaxed text-slate-600">
                Untuk reset password, silakan hubungi Administrator sistem PASTI. Jika akun Anda terdaftar melalui SSO Kemenkeu, gunakan tombol{" "}
                <span className="font-medium text-slate-800">&quot;Masuk dengan SSO Kemenkeu&quot;</span> di atas. Password Anda dikelola langsung oleh sistem SSO
                Kemenkeu, bukan oleh PASTI.
              </p>
            </div>
            <Button onClick={() => setShowForgotPasswordInfo(false)} className="mt-5">
              Mengerti
            </Button>
          </div>
        </ModalShell>
      )}
    </div>
  );
}