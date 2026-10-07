"use client";

import { useEffect, useState, useCallback, useMemo } from "react";
import { UserPlus, ShieldCheck, Trash2, Ban, Pencil, Search, SearchX, X, Users } from "lucide-react";
import axios from "axios";
import { listUsers, UserListItem, deactivateUser, deleteUser } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";
import { bolehKelolaPengguna, namaPeranBerkode, peranEfektif } from "@/lib/peran";
import { CreateUserModal } from "@/components/users/CreateUserModal";
import { EditUserModal } from "@/components/users/EditUserModal";
import { PeranPenggunaModal } from "@/components/users/PeranPenggunaModal";
import { matchesUser, parseTerms } from "@/components/users/userSearch";
import { initialsOf } from "@/lib/initials";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { useToast } from "@/components/ui/Toast";

// Pesan dari backend (mis. "tidak dapat menonaktifkan akun Anda sendiri") lebih berguna daripada pesan umum.
function errorMessage(err: unknown, fallback: string): string {
  return axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : fallback;
}

const LENCANA = "rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset";

// Peran pengguna: Super Admin (dari .env), peran data yang dipegang, atau Tamu bila belum punya peran apa pun. "Belum setuju" menandai pengguna yang belum
// menyetujui pernyataan penggunaan aplikasi.
function PeranBadges({ u }: { u: UserListItem }) {
  return (
    <div className="flex flex-wrap items-center gap-1">
      {u.role === "superadmin" ? (
        <span className={`${LENCANA} bg-violet-50 text-violet-700 ring-violet-200`}>Super Admin</span>
      ) : u.peran_data.length === 0 ? (
        <span className={`${LENCANA} bg-amber-50 text-amber-800 ring-amber-200`} title="Belum punya peran: belum dapat membuka fitur apa pun">
          Tamu
        </span>
      ) : (
        u.peran_data.map((b) => (
          <span key={b.id} className={`${LENCANA} bg-blue-50 text-blue-700 ring-blue-200`}>
            {namaPeranBerkode(b.role, b.kode)}
          </span>
        ))
      )}
      {u.role !== "superadmin" && !u.setuju && (
        <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-medium text-slate-500" title="Belum menyetujui pernyataan penggunaan aplikasi">
          Belum setuju
        </span>
      )}
    </div>
  );
}

// Satker pengguna: nama pada data aset (dari kode 6 digit karakter ke-10 sampai ke-15 kode satker SSO) atau nama satker dari SSO, beserta kode lengkapnya.
function SatkerSel({ u }: { u: UserListItem }) {
  const nama = u.satker_aset || u.satker;
  if (!nama && !u.kode_satker) return <span className="text-slate-300">-</span>;
  return (
    <div className="min-w-0 max-w-[16rem]">
      {nama && <p className="truncate text-slate-700">{nama}</p>}
      {u.kode_satker && <p className="font-mono text-[11px] text-slate-400">{u.kode_satker}</p>}
    </div>
  );
}

function StatusBadge({ active }: { active: boolean }) {
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${active ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-500"}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${active ? "bg-emerald-500" : "bg-slate-400"}`} aria-hidden="true" />
      {active ? "Aktif" : "Nonaktif"}
    </span>
  );
}

type Pending = { kind: "deactivate" | "delete"; user: UserListItem } | null;

export default function UsersPage() {
  const { profile } = useDashboard();
  // Superadmin dan Pengguna Barang mengelola pengguna; UE1, Kanwil, dan Satker hanya melihat pengguna dalam cakupan kode satkernya.
  const kelola = bolehKelolaPengguna(peranEfektif(profile?.peran, profile?.role));
  const toast = useToast();
  const [users, setUsers] = useState<UserListItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editUserId, setEditUserId] = useState<string | null>(null);
  const [peranUser, setPeranUser] = useState<{ id: string; nama: string } | null>(null);
  const [pending, setPending] = useState<Pending>(null);
  const [busy, setBusy] = useState(false);

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
  const activeCount = useMemo(() => users.filter((u) => u.is_active).length, [users]);

  const isOwnAccount = (u: UserListItem) => Boolean(profile?.id) && profile?.id.toLowerCase() === u.id.toLowerCase();

  const runPending = async () => {
    if (!pending) return;
    const { kind, user } = pending;
    setBusy(true);
    try {
      if (kind === "deactivate") {
        await deactivateUser(user.id);
        toast.success(`${user.full_name} dinonaktifkan.`);
      } else {
        await deleteUser(user.id);
        toast.success(`${user.full_name} dihapus.`);
      }
      setPending(null);
      fetchUsers();
    } catch (err) {
      toast.error(errorMessage(err, kind === "deactivate" ? "Gagal menonaktifkan user" : "Gagal menghapus user"));
      setPending(null);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="w-full space-y-5">
      <PageHeader
        title="Manajemen Pengguna"
        icon={Users}
        description={
          kelola
            ? "Kelola akun pengguna PASTI V3: tambah, beri peran, nonaktifkan, atau hapus. Pengguna baru berstatus tamu sampai diberi peran."
            : "Pengguna yang kode satker SSO-nya berada dalam cakupan peran Anda. Anda hanya dapat melihat; peran ditetapkan oleh superadmin atau Pengguna Barang."
        }
        actions={
          kelola ? (
            <Button fullWidth={false} onClick={() => setShowCreateModal(true)} icon={<UserPlus className="h-4 w-4" />}>
              Tambah Pengguna
            </Button>
          ) : undefined
        }
      />

      <div className="card p-4 sm:p-5">
        <div className="flex flex-wrap items-center gap-3">
          <div className="relative min-w-[16rem] flex-1">
            <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              inputMode="search"
              autoComplete="off"
              aria-label="Cari pengguna"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === "Escape" && setQuery("")}
              placeholder="Cari nama, username, email, NIP, kode atau nama satker…"
              className="w-full rounded-xl border border-slate-300 bg-white py-2.5 pl-10 pr-9 text-sm shadow-sm outline-none hover:border-slate-400 focus:border-blue-500 focus:shadow-glow"
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
            <p aria-live="polite" className="text-xs text-slate-500">
              {terms.length > 0 ? (
                `Menampilkan ${filtered.length} dari ${users.length} pengguna`
              ) : (
                <>
                  <span className="font-semibold text-slate-700">{users.length}</span> pengguna · <span className="font-semibold text-emerald-700">{activeCount}</span> aktif
                </>
              )}
            </p>
          )}
        </div>
      </div>

      {loadError && <Alert message={loadError} />}

      {isLoading ? (
        <SkeletonTable rows={6} cols={5} label="Memuat pengguna" />
      ) : !loadError && users.length === 0 ? (
        <EmptyState icon={Users} title="Belum ada pengguna" description={kelola ? "Tambahkan pengguna pertama dengan tombol Tambah Pengguna." : "Tidak ada pengguna dengan kode satker SSO dalam cakupan peran Anda."} />
      ) : !loadError && filtered.length === 0 ? (
        <EmptyState
          icon={SearchX}
          title={`Tidak ada pengguna yang cocok dengan “${query.trim()}”`}
          description="Periksa ejaan atau coba kata kunci lain."
          action={
            <button type="button" onClick={() => setQuery("")} className="text-sm font-medium text-blue-600 hover:underline">
              Hapus pencarian
            </button>
          }
        />
      ) : (
        !loadError && (
          <div className="card overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b border-slate-100 bg-slate-50/80">
                <tr>
                  <th className="px-4 py-3 text-left font-semibold text-slate-500">Pengguna</th>
                  <th className="hidden px-4 py-3 text-left font-semibold text-slate-500 md:table-cell">Username</th>
                  <th className="hidden px-4 py-3 text-left font-semibold text-slate-500 md:table-cell">Peran</th>
                  <th className="hidden px-4 py-3 text-left font-semibold text-slate-500 lg:table-cell">Satker</th>
                  <th className="hidden px-4 py-3 text-left font-semibold text-slate-500 md:table-cell">Sumber</th>
                  <th className="hidden px-4 py-3 text-left font-semibold text-slate-500 md:table-cell">Status</th>
                  <th className="px-4 py-3 text-right font-semibold text-slate-500">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {filtered.map((u) => (
                  <tr key={u.id} className="hover:bg-blue-50/40">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-3">
                        <span
                          className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-xs font-bold ${
                            u.is_active ? "bg-gradient-to-br from-blue-100 to-blue-200 text-blue-700" : "bg-slate-100 text-slate-400"
                          }`}
                          aria-hidden="true"
                        >
                          {initialsOf(u.full_name)}
                        </span>
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
                            <span className="font-medium text-slate-900">{u.full_name}</span>
                            {u.is_protected && (
                              <span title="Superadmin permanen">
                                <ShieldCheck className="h-4 w-4 text-amber-500" />
                              </span>
                            )}
                            {isOwnAccount(u) && <span className="rounded bg-blue-50 px-1.5 py-0.5 text-[10px] font-semibold text-blue-600">Anda</span>}
                          </div>
                          <p className="break-all text-xs text-slate-400">{u.email}</p>
                          {/* Di mobile kolom Username/Role/Sumber/Status disembunyikan; ringkasannya dipindah ke sini. */}
                          <div className="mt-1.5 flex flex-wrap items-center gap-1.5 md:hidden">
                            <PeranBadges u={u} />
                            <StatusBadge active={u.is_active} />
                            <span className="text-[11px] text-slate-400">
                              @{u.username} · {u.auth_provider === "sso" ? "SSO Kemenkeu" : "Lokal"}
                            </span>
                          </div>
                        </div>
                      </div>
                    </td>
                    <td className="hidden px-4 py-3 text-slate-600 md:table-cell">{u.username}</td>
                    <td className="hidden px-4 py-3 md:table-cell">
                      <PeranBadges u={u} />
                    </td>
                    <td className="hidden px-4 py-3 text-xs lg:table-cell">
                      <SatkerSel u={u} />
                    </td>
                    <td className="hidden px-4 py-3 text-xs text-slate-500 md:table-cell">{u.auth_provider === "sso" ? "SSO Kemenkeu" : "Lokal"}</td>
                    <td className="hidden px-4 py-3 md:table-cell">
                      <StatusBadge active={u.is_active} />
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex justify-end gap-1">
                        <button
                          onClick={() => setPeranUser({ id: u.id, nama: u.full_name })}
                          title="Peran data"
                          aria-label={`Peran data ${u.full_name}`}
                          className="rounded-lg p-2 text-slate-400 hover:bg-violet-50 hover:text-violet-600"
                        >
                          <ShieldCheck className="h-4 w-4" />
                        </button>
                        {kelola && (
                        <button
                          onClick={() => setEditUserId(u.id)}
                          title="Edit"
                          aria-label={`Edit ${u.full_name}`}
                          className="rounded-lg p-2 text-slate-400 hover:bg-blue-50 hover:text-blue-600"
                        >
                          <Pencil className="h-4 w-4" />
                        </button>
                        )}
                        {/* Pengguna yang sudah nonaktif diaktifkan lagi lewat Edit; akun sendiri tidak boleh dinonaktifkan. */}
                        {kelola && !u.is_protected && u.is_active && !isOwnAccount(u) && (
                          <button
                            onClick={() => setPending({ kind: "deactivate", user: u })}
                            title="Nonaktifkan"
                            aria-label={`Nonaktifkan ${u.full_name}`}
                            className="rounded-lg p-2 text-slate-400 hover:bg-amber-50 hover:text-amber-600"
                          >
                            <Ban className="h-4 w-4" />
                          </button>
                        )}
                        {kelola && !u.is_protected && (
                          <button
                            onClick={() => setPending({ kind: "delete", user: u })}
                            title="Hapus"
                            aria-label={`Hapus ${u.full_name}`}
                            className="rounded-lg p-2 text-slate-400 hover:bg-red-50 hover:text-red-600"
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
          </div>
        )
      )}

      {kelola && showCreateModal && <CreateUserModal onClose={() => setShowCreateModal(false)} onCreated={fetchUsers} />}

      {kelola && editUserId && <EditUserModal userId={editUserId} onClose={() => setEditUserId(null)} onUpdated={fetchUsers} />}
      {peranUser && <PeranPenggunaModal userId={peranUser.id} nama={peranUser.nama} onClose={() => setPeranUser(null)} bacaSaja={!kelola} />}

      {pending && (
        <ConfirmDialog
          tone={pending.kind === "delete" ? "danger" : "warning"}
          title={pending.kind === "deactivate" ? `Nonaktifkan ${pending.user.full_name}?` : `Hapus ${pending.user.full_name}?`}
          message={
            pending.kind === "deactivate"
              ? "Pengguna ini langsung tidak bisa login, dan sesi yang sedang berjalan ikut berakhir. Akun bisa diaktifkan lagi lewat Edit."
              : "Akun ini akan dihapus permanen dan tidak bisa dikembalikan."
          }
          confirmLabel={pending.kind === "deactivate" ? "Nonaktifkan" : "Hapus permanen"}
          busy={busy}
          onConfirm={runPending}
          onCancel={() => !busy && setPending(null)}
        />
      )}
    </div>
  );
}
