"use client";

import { useEffect, useState, useCallback, useMemo } from "react";
import { UserPlus, Loader2, ShieldCheck, Trash2, Ban, Pencil, Search, SearchX, X } from "lucide-react";
import axios from "axios";
import { listUsers, UserListItem, deactivateUser, deleteUser } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";
import { CreateUserModal } from "@/components/users/CreateUserModal";
import { EditUserModal } from "@/components/users/EditUserModal";
import { matchesUser, parseTerms } from "@/components/users/userSearch";

// Pesan dari backend (mis. "tidak dapat menonaktifkan akun Anda sendiri") lebih berguna daripada pesan umum.
function errorMessage(err: unknown, fallback: string): string {
  return axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : fallback;
}

export default function UsersPage() {
  const { profile } = useDashboard();
  const [users, setUsers] = useState<UserListItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editUserId, setEditUserId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const fetchUsers = useCallback(async () => {
    setIsLoading(true);
    setLoadError(null);
    try {
      const res = await listUsers();
      setUsers(res.data ?? []);
    } catch (err) {
      setLoadError(errorMessage(err, "Gagal memuat daftar pengguna"));
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const terms = useMemo(() => parseTerms(query), [query]);
  const filtered = useMemo(() => users.filter((u) => matchesUser(u, terms)), [users, terms]);

  const isOwnAccount = (u: UserListItem) => Boolean(profile?.id) && profile?.id.toLowerCase() === u.id.toLowerCase();

  const handleDeactivate = async (u: UserListItem) => {
    // Penonaktifan langsung berlaku: pengguna tidak bisa login dan sesi yang sedang berjalan ikut berakhir.
    if (
      !confirm(
        `Nonaktifkan ${u.full_name}?\n\nPengguna ini langsung tidak bisa login, dan sesi yang sedang berjalan ikut berakhir.`
      )
    )
      return;
    setActionError(null);
    try {
      await deactivateUser(u.id);
      fetchUsers();
    } catch (err) {
      setActionError(errorMessage(err, "Gagal menonaktifkan user"));
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
      <div className="flex flex-col gap-4 rounded-xl bg-white p-4 shadow-sm sm:flex-row sm:items-center sm:justify-between sm:p-6">
        <div>
          <h1 className="text-xl font-bold text-slate-900">Manajemen Pengguna</h1>
          <p className="mt-1 text-sm text-slate-500">Kelola akun pengguna PASTI V3</p>
        </div>
        <button
          onClick={() => setShowCreateModal(true)}
          className="flex items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700"
        >
          <UserPlus className="h-4 w-4" />
          Tambah Pengguna
        </button>
      </div>

      {actionError && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{actionError}</div>
      )}

      <div className="rounded-xl bg-white p-4 shadow-sm">
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            inputMode="search"
            autoComplete="off"
            aria-label="Cari pengguna"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === "Escape" && setQuery("")}
            placeholder="Cari nama, username, email, NIP, atau satker..."
            className="w-full rounded-lg border border-slate-300 bg-white py-2.5 pl-10 pr-9 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
          />
          {query && (
            <button
              type="button"
              onClick={() => setQuery("")}
              aria-label="Hapus pencarian"
              className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>
        {!isLoading && users.length > 0 && (
          <p aria-live="polite" className="mt-2 text-xs text-slate-400">
            {terms.length > 0 ? `Menampilkan ${filtered.length} dari ${users.length} pengguna` : `${users.length} pengguna`}
          </p>
        )}
      </div>

      <div className="overflow-x-auto rounded-xl bg-white shadow-sm">
        {isLoading ? (
          <div className="flex justify-center py-16">
            <Loader2 className="h-6 w-6 animate-spin text-blue-600" />
          </div>
        ) : loadError ? (
          <p className="px-4 py-12 text-center text-sm text-red-600">{loadError}</p>
        ) : (
          <table className="w-full text-sm">
            <thead className="border-b border-slate-200 bg-slate-50">
              <tr>
                <th className="px-4 py-3 text-left font-semibold text-slate-600">Nama</th>
                <th className="hidden px-4 py-3 text-left font-semibold text-slate-600 md:table-cell">Username</th>
                <th className="hidden px-4 py-3 text-left font-semibold text-slate-600 md:table-cell">Role</th>
                <th className="hidden px-4 py-3 text-left font-semibold text-slate-600 md:table-cell">Sumber</th>
                <th className="hidden px-4 py-3 text-left font-semibold text-slate-600 md:table-cell">Status</th>
                <th className="px-4 py-3 text-right font-semibold text-slate-600">Aksi</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {users.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-sm text-slate-500">
                    Belum ada pengguna
                  </td>
                </tr>
              )}
              {users.length > 0 && filtered.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center">
                    <SearchX className="mx-auto h-8 w-8 text-slate-400" />
                    <p className="mt-2 break-words text-sm font-medium text-slate-700">
                      Tidak ada pengguna yang cocok dengan &ldquo;{query.trim()}&rdquo;
                    </p>
                    <button
                      type="button"
                      onClick={() => setQuery("")}
                      className="mt-2 text-sm font-medium text-blue-600 hover:underline"
                    >
                      Hapus pencarian
                    </button>
                  </td>
                </tr>
              )}
              {filtered.map((u) => (
                <tr key={u.id} className="hover:bg-slate-50">
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
                      <span className="font-medium text-slate-900">{u.full_name}</span>
                      {u.is_protected && (
                        <span title="Superadmin permanen">
                          <ShieldCheck className="h-4 w-4 text-amber-500" />
                        </span>
                      )}
                      {isOwnAccount(u) && (
                        <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-500">Anda</span>
                      )}
                    </div>
                    <p className="break-all text-xs text-slate-400">{u.email}</p>
                    {/* Di mobile kolom Username/Role/Sumber/Status disembunyikan; ringkasannya dipindah ke sini. */}
                    <div className="mt-1.5 flex flex-wrap items-center gap-1.5 md:hidden">
                      <span className="rounded-full bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700">{u.role}</span>
                      <span
                        className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${
                          u.is_active ? "bg-green-50 text-green-700" : "bg-slate-100 text-slate-500"
                        }`}
                      >
                        {u.is_active ? "Aktif" : "Nonaktif"}
                      </span>
                      <span className="text-[11px] text-slate-400">
                        @{u.username} · {u.auth_provider === "sso" ? "SSO Kemenkeu" : "Lokal"}
                      </span>
                    </div>
                  </td>
                  <td className="hidden px-4 py-3 text-slate-600 md:table-cell">{u.username}</td>
                  <td className="hidden px-4 py-3 md:table-cell">
                    <span className="rounded-full bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700">{u.role}</span>
                  </td>
                  <td className="hidden px-4 py-3 text-xs text-slate-500 md:table-cell">
                    {u.auth_provider === "sso" ? "SSO Kemenkeu" : "Lokal"}
                  </td>
                  <td className="hidden px-4 py-3 md:table-cell">
                    <span
                      className={`rounded-full px-2.5 py-1 text-xs font-medium ${
                        u.is_active ? "bg-green-50 text-green-700" : "bg-slate-100 text-slate-500"
                      }`}
                    >
                      {u.is_active ? "Aktif" : "Nonaktif"}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex justify-end gap-2">
                      <button
                        onClick={() => setEditUserId(u.id)}
                        title="Edit"
                        className="rounded-md p-2 text-slate-400 hover:bg-slate-100 hover:text-blue-600 md:p-1.5"
                      >
                        <Pencil className="h-4 w-4" />
                      </button>
                      {/* Pengguna yang sudah nonaktif diaktifkan lagi lewat Edit; akun sendiri tidak boleh dinonaktifkan. */}
                      {!u.is_protected && u.is_active && !isOwnAccount(u) && (
                        <button
                          onClick={() => handleDeactivate(u)}
                          title="Nonaktifkan"
                          className="rounded-md p-2 text-slate-400 hover:bg-slate-100 hover:text-amber-600 md:p-1.5"
                        >
                          <Ban className="h-4 w-4" />
                        </button>
                      )}
                      {!u.is_protected && (
                        <button
                          onClick={() => handleDelete(u.id)}
                          title="Hapus"
                          className="rounded-md p-2 text-slate-400 hover:bg-slate-100 hover:text-red-600 md:p-1.5"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {showCreateModal && (
        <CreateUserModal onClose={() => setShowCreateModal(false)} onCreated={fetchUsers} />
      )}

      {editUserId && (
        <EditUserModal userId={editUserId} onClose={() => setEditUserId(null)} onUpdated={fetchUsers} />
      )}
    </div>
  );
}
