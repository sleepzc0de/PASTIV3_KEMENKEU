"use client";

import { useEffect, useState, useCallback } from "react";
import { UserPlus, Loader2, ShieldCheck, Trash2, Ban } from "lucide-react";
import { listUsers, UserListItem, deactivateUser, deleteUser } from "@/lib/api";
import { CreateUserModal } from "@/components/users/CreateUserModal";

export default function UsersPage() {
    const [users, setUsers] = useState<UserListItem[]>([]);
    const [isLoading, setIsLoading] = useState(true);
    const [showModal, setShowModal] = useState(false);
    const [actionError, setActionError] = useState<string | null>(null);

    const fetchUsers = useCallback(async () => {
        setIsLoading(true);
        try {
            const res = await listUsers();
            setUsers(res.data);
        } finally {
            setIsLoading(false);
        }
    }, []);

    useEffect(() => {
        fetchUsers();
    }, [fetchUsers]);

    const handleDeactivate = async (id: string) => {
        setActionError(null);
        try {
            await deactivateUser(id);
            fetchUsers();
        } catch {
            setActionError("Gagal menonaktifkan user");
        }
    };

    const handleDelete = async (id: string) => {
        if (!confirm("Yakin ingin menghapus user ini?")) return;
        setActionError(null);
        try {
            await deleteUser(id);
            fetchUsers();
        } catch {
            setActionError("Gagal menghapus user");
        }
    };

    return (
        <div className="w-full space-y-6">
            <div className="flex items-center justify-between rounded-xl bg-white p-6 shadow-sm">
                <div>
                    <h1 className="text-xl font-bold text-slate-900">Manajemen Pengguna</h1>
                    <p className="mt-1 text-sm text-slate-500">Kelola akun pengguna PASTI V3</p>
                </div>
                <button
                    onClick={() => setShowModal(true)}
                    className="flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700"
                >
                    <UserPlus className="h-4 w-4" />
                    Tambah Pengguna
                </button>
            </div>

            {actionError && (
                <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{actionError}</div>
            )}

            <div className="overflow-x-auto rounded-xl bg-white shadow-sm">
                {isLoading ? (
                    <div className="flex justify-center py-16">
                        <Loader2 className="h-6 w-6 animate-spin text-blue-600" />
                    </div>
                ) : (
                    <table className="w-full text-sm">
                        <thead className="border-b border-slate-200 bg-slate-50">
                            <tr>
                                <th className="px-4 py-3 text-left font-semibold text-slate-600">Nama</th>
                                <th className="px-4 py-3 text-left font-semibold text-slate-600">Username</th>
                                <th className="px-4 py-3 text-left font-semibold text-slate-600">Role</th>
                                <th className="px-4 py-3 text-left font-semibold text-slate-600">Sumber</th>
                                <th className="px-4 py-3 text-left font-semibold text-slate-600">Status</th>
                                <th className="px-4 py-3 text-right font-semibold text-slate-600">Aksi</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-100">
                            {users.map((u) => (
                                <tr key={u.id} className="hover:bg-slate-50">
                                    <td className="px-4 py-3">
                                        <div className="flex items-center gap-2">
                                            <span className="font-medium text-slate-900">{u.full_name}</span>
                                            {u.is_protected && (
                                                <span title="Superadmin permanen">
                                                    <ShieldCheck className="h-4 w-4 text-amber-500" />
                                                </span>
                                            )}
                                        </div>
                                        <p className="text-xs text-slate-400">{u.email}</p>
                                    </td>
                                    <td className="px-4 py-3 text-slate-600">{u.username}</td>
                                    <td className="px-4 py-3">
                                        <span className="rounded-full bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700">{u.role}</span>
                                    </td>
                                    <td className="px-4 py-3 text-xs text-slate-500">
                                        {u.auth_provider === "sso" ? "SSO Kemenkeu" : "Lokal"}
                                    </td>
                                    <td className="px-4 py-3">
                                        <span
                                            className={`rounded-full px-2.5 py-1 text-xs font-medium ${u.is_active ? "bg-green-50 text-green-700" : "bg-slate-100 text-slate-500"
                                                }`}
                                        >
                                            {u.is_active ? "Aktif" : "Nonaktif"}
                                        </span>
                                    </td>
                                    <td className="px-4 py-3 text-right">
                                        {!u.is_protected && (
                                            <div className="flex justify-end gap-2">
                                                <button
                                                    onClick={() => handleDeactivate(u.id)}
                                                    title="Nonaktifkan"
                                                    className="rounded-md p-1.5 text-slate-400 hover:bg-slate-100 hover:text-amber-600"
                                                >
                                                    <Ban className="h-4 w-4" />
                                                </button>
                                                <button
                                                    onClick={() => handleDelete(u.id)}
                                                    title="Hapus"
                                                    className="rounded-md p-1.5 text-slate-400 hover:bg-slate-100 hover:text-red-600"
                                                >
                                                    <Trash2 className="h-4 w-4" />
                                                </button>
                                            </div>
                                        )}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                )}
            </div>

            {showModal && (
                <CreateUserModal onClose={() => setShowModal(false)} onCreated={fetchUsers} />
            )}
        </div>
    );
}