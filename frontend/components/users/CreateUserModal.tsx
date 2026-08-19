"use client";

import { useState } from "react";
import { X, Search, Loader2, UserPlus } from "lucide-react";
import axios from "axios";
import { searchPegawaiByNIP, createUser } from "@/lib/api";
import { Button } from "@/components/ui/Button";

interface CreateUserModalProps {
    onClose: () => void;
    onCreated: () => void;
}

function getField(profile: Record<string, unknown>, keys: string[]): string {
    for (const k of keys) {
        const v = profile[k];
        if (typeof v === "string" && v) return v;
    }
    return "";
}

export function CreateUserModal({ onClose, onCreated }: CreateUserModalProps) {
    const [mode, setMode] = useState<"hris2" | "manual">("hris2");

    const [nip, setNip] = useState("");
    const [isSearching, setIsSearching] = useState(false);
    const [searchError, setSearchError] = useState<string | null>(null);
    const [hrisProfile, setHrisProfile] = useState<Record<string, unknown> | null>(null);

    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [email, setEmail] = useState("");
    const [fullName, setFullName] = useState("");
    const [role, setRole] = useState<"user" | "admin">("user");

    const [isSubmitting, setIsSubmitting] = useState(false);
    const [submitError, setSubmitError] = useState<string | null>(null);

    const handleSearchNIP = async () => {
        if (!nip.trim()) return;
        setIsSearching(true);
        setSearchError(null);
        setHrisProfile(null);
        try {
            const res = await searchPegawaiByNIP(nip.trim());
            const profile = res.data;
            setHrisProfile(profile);

            const nama = getField(profile, ["nama"]);
            const mail = getField(profile, ["email"]);
            setFullName(nama);
            setEmail(mail);
            if (!username) {
                setUsername(nip.trim());
            }
        } catch (err) {
            if (axios.isAxiosError(err) && err.response) {
                setSearchError(err.response.data?.message || "NIP tidak ditemukan");
            } else {
                setSearchError("Gagal terhubung ke server");
            }
        } finally {
            setIsSearching(false);
        }
    };

    const handleSubmit = async () => {
        setSubmitError(null);

        if (!username || !password) {
            setSubmitError("Username dan password wajib diisi");
            return;
        }
        if (password.length < 8) {
            setSubmitError("Password minimal 8 karakter");
            return;
        }
        if (mode === "hris2" && !nip.trim()) {
            setSubmitError("Cari NIP terlebih dahulu");
            return;
        }
        if (mode === "manual" && (!email || !fullName)) {
            setSubmitError("Email dan nama lengkap wajib diisi");
            return;
        }

        setIsSubmitting(true);
        try {
            await createUser({
                source: mode,
                nip: mode === "hris2" ? nip.trim() : undefined,
                username,
                password,
                email: email || undefined,
                full_name: fullName || undefined,
                role,
            });
            onCreated();
            onClose();
        } catch (err) {
            if (axios.isAxiosError(err) && err.response) {
                setSubmitError(err.response.data?.message || "Gagal membuat user");
            } else {
                setSubmitError("Gagal terhubung ke server");
            }
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
            <div className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-xl bg-white shadow-xl">
                <div className="flex items-center justify-between border-b border-slate-200 px-6 py-4">
                    <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900">
                        <UserPlus className="h-5 w-5 text-blue-600" />
                        Tambah Pengguna
                    </h2>
                    <button onClick={onClose} className="rounded-md p-1 text-slate-400 hover:bg-slate-100">
                        <X className="h-5 w-5" />
                    </button>
                </div>

                <div className="space-y-5 px-6 py-5">
                    {/* Mode Switch */}
                    <div className="flex rounded-lg border border-slate-200 p-1">
                        <button
                            onClick={() => setMode("hris2")}
                            className={`flex-1 rounded-md py-2 text-sm font-medium transition-colors ${mode === "hris2" ? "bg-blue-600 text-white" : "text-slate-600 hover:bg-slate-50"
                                }`}
                        >
                            Cari dari HRIS2 (NIP)
                        </button>
                        <button
                            onClick={() => setMode("manual")}
                            className={`flex-1 rounded-md py-2 text-sm font-medium transition-colors ${mode === "manual" ? "bg-blue-600 text-white" : "text-slate-600 hover:bg-slate-50"
                                }`}
                        >
                            Input Manual
                        </button>
                    </div>

                    {mode === "hris2" && (
                        <div className="space-y-3">
                            <div className="flex gap-2">
                                <input
                                    type="text"
                                    value={nip}
                                    onChange={(e) => setNip(e.target.value)}
                                    onKeyDown={(e) => e.key === "Enter" && handleSearchNIP()}
                                    placeholder="Masukkan NIP pegawai"
                                    className="flex-1 rounded-lg border border-slate-300 px-3.5 py-2.5 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
                                />
                                <button
                                    onClick={handleSearchNIP}
                                    disabled={isSearching}
                                    className="flex items-center gap-1.5 rounded-lg bg-slate-800 px-4 py-2.5 text-sm font-medium text-white hover:bg-slate-900 disabled:opacity-60"
                                >
                                    {isSearching ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
                                    Cari
                                </button>
                            </div>

                            {searchError && (
                                <p className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
                                    {searchError}
                                </p>
                            )}

                            {hrisProfile && (
                                <div className="rounded-lg border border-green-200 bg-green-50 px-4 py-3 text-sm">
                                    <p className="font-semibold text-green-800">{fullName || "(nama tidak ditemukan)"}</p>
                                    <p className="text-green-700">{email || "(email tidak ditemukan)"}</p>
                                    <p className="mt-1 text-xs text-green-600">
                                        Data akan digunakan untuk pendaftaran. Anda tetap bisa menyesuaikan email/nama di bawah bila perlu.
                                    </p>

                                    {(!fullName || !email) && (
                                        <details className="mt-2">
                                            <summary className="cursor-pointer text-xs font-medium text-amber-700">
                                                Field otomatis tidak lengkap — lihat data mentah dari HRIS2
                                            </summary>
                                            <pre className="mt-2 max-h-40 overflow-auto rounded bg-white p-2 text-[10px] text-slate-600">
                                                {JSON.stringify(hrisProfile, null, 2)}
                                            </pre>
                                        </details>
                                    )}
                                </div>
                            )}
                        </div>
                    )}

                    <div className="space-y-3">
                        {mode === "manual" && (
                            <>
                                <FormField label="Nama Lengkap" value={fullName} onChange={setFullName} placeholder="Nama lengkap pegawai" />
                                <FormField label="Email" value={email} onChange={setEmail} placeholder="nama@kemenkeu.go.id" type="email" />
                            </>
                        )}

                        {mode === "hris2" && hrisProfile && (
                            <>
                                <FormField label="Nama Lengkap (bisa diedit)" value={fullName} onChange={setFullName} />
                                <FormField label="Email (bisa diedit)" value={email} onChange={setEmail} type="email" />
                            </>
                        )}

                        <FormField label="Username" value={username} onChange={setUsername} placeholder="Username untuk login" />
                        <FormField label="Password" value={password} onChange={setPassword} placeholder="Minimal 8 karakter" type="password" />

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
                    </div>

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
                        Buat Pengguna
                    </Button>
                </div>
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