"use client";

import { useEffect, useState } from "react";
import { X, Loader2, Pencil } from "lucide-react";
import axios from "axios";
import { getUserDetail, updateUser, UserListItem } from "@/lib/api";
import { Button } from "@/components/ui/Button";

interface EditUserModalProps {
  userId: string;
  onClose: () => void;
  onUpdated: () => void;
}

export function EditUserModal({ userId, onClose, onUpdated }: EditUserModalProps) {
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [user, setUser] = useState<UserListItem | null>(null);

  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<"user" | "admin">("user");
  const [isActive, setIsActive] = useState(true);
  const [newPassword, setNewPassword] = useState("");

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    getUserDetail(userId)
      .then((res) => {
        if (cancelled) return;
        const u = res.data;
        setUser(u);
        setFullName(u.full_name);
        setEmail(u.email);
        setRole(u.role === "admin" ? "admin" : "user");
        setIsActive(u.is_active);
      })
      .catch((err) => {
        if (cancelled) return;
        if (axios.isAxiosError(err) && err.response) {
          setLoadError(err.response.data?.message || "Gagal memuat data user");
        } else {
          setLoadError("Gagal terhubung ke server");
        }
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [userId]);

  const handleSubmit = async () => {
    setSubmitError(null);

    if (!fullName.trim() || !email.trim()) {
      setSubmitError("Nama lengkap dan email wajib diisi");
      return;
    }
    if (newPassword && newPassword.length < 8) {
      setSubmitError("Password baru minimal 8 karakter");
      return;
    }

    setIsSubmitting(true);
    try {
      await updateUser(userId, {
        full_name: fullName,
        email,
        role,
        is_active: isActive,
        password: newPassword || undefined,
      });
      onUpdated();
      onClose();
    } catch (err) {
      if (axios.isAxiosError(err) && err.response) {
        setSubmitError(err.response.data?.message || "Gagal memperbarui user");
      } else {
        setSubmitError("Gagal terhubung ke server");
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="max-h-[90vh] w-full max-w-md overflow-y-auto rounded-xl bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-slate-200 px-6 py-4">
          <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900">
            <Pencil className="h-5 w-5 text-blue-600" />
            Edit Pengguna
          </h2>
          <button onClick={onClose} className="rounded-md p-1 text-slate-400 hover:bg-slate-100">
            <X className="h-5 w-5" />
          </button>
        </div>

        {isLoading && (
          <div className="flex justify-center py-16">
            <Loader2 className="h-6 w-6 animate-spin text-blue-600" />
          </div>
        )}

        {loadError && (
          <div className="px-6 py-5">
            <p className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{loadError}</p>
          </div>
        )}

        {!isLoading && !loadError && user && (
          <>
            <div className="space-y-4 px-6 py-5">
              <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-500">
                <p>Username: <span className="font-medium text-slate-700">{user.username}</span></p>
                <p>Sumber akun: <span className="font-medium text-slate-700">{user.auth_provider === "sso" ? "SSO Kemenkeu" : "Lokal"}</span></p>
              </div>

              <FormField label="Nama Lengkap" value={fullName} onChange={setFullName} />
              <FormField label="Email" value={email} onChange={setEmail} type="email" />

              <div>
                <label className="mb-1.5 block text-sm font-medium text-slate-700">Role</label>
                <select
                  value={role}
                  onChange={(e) => setRole(e.target.value as "user" | "admin")}
                  className="w-full rounded-lg border border-slate-300 px-3.5 py-2.5 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
                >
                  <option value="user">User</option>
                  <option value="admin">Admin</option>
                </select>
              </div>

              <label className="flex items-center gap-2 text-sm text-slate-700">
                <input
                  type="checkbox"
                  checked={isActive}
                  onChange={(e) => setIsActive(e.target.checked)}
                  className="h-4 w-4 rounded border-slate-300 text-blue-600"
                />
                Akun aktif
              </label>

              {user.auth_provider !== "sso" && (
                <FormField
                  label="Password Baru (opsional)"
                  value={newPassword}
                  onChange={setNewPassword}
                  type="password"
                  placeholder="Kosongkan jika tidak ingin mengubah"
                />
              )}

              {submitError && (
                <p className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">{submitError}</p>
              )}
            </div>

            <div className="flex justify-end gap-3 border-t border-slate-200 px-6 py-4">
              <button
                onClick={onClose}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Batal
              </button>
              <Button onClick={handleSubmit} isLoading={isSubmitting} className="w-auto px-5">
                Simpan Perubahan
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function FormField({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  type?: string;
}) {
  return (
    <div>
      <label className="mb-1.5 block text-sm font-medium text-slate-700">{label}</label>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full rounded-lg border border-slate-300 px-3.5 py-2.5 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
      />
    </div>
  );
}