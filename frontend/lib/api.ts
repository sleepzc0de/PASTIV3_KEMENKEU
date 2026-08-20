import axios from "axios";
import Cookies from "js-cookie";

export const api = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL,
  headers: { "Content-Type": "application/json" },
});

api.interceptors.request.use((config) => {
  const token = Cookies.get("pasti_access_token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      Cookies.remove("pasti_access_token");
      if (typeof window !== "undefined") {
        window.location.href = "/login";
      }
    }
    return Promise.reject(error);
  }
);

// ============ Auth ============

export interface LoginResponse {
  success: boolean;
  message: string;
  data: {
    access_token: string;
    expires_in: number;
    user: {
      id: string;
      username: string;
      email: string;
      full_name: string;
      role: string;
    };
  };
}

export interface CaptchaResponse {
  success: boolean;
  message: string;
  data: {
    captcha_id: string;
    captcha_image: string;
  };
}

export async function fetchCaptcha() {
  const res = await api.get<CaptchaResponse>("/auth/captcha");
  return res.data;
}

export async function loginUser(
  username: string,
  password: string,
  captchaId: string,
  captchaAnswer: string
) {
  const res = await api.post<LoginResponse>("/auth/login", {
    username,
    password,
    captcha_id: captchaId,
    captcha_answer: captchaAnswer,
  });
  return res.data;
}

// ============ SLDK Integration ============

export interface SLDKColumn {
  name: string;
  data_type: string;
  is_searchable: boolean;
}

export interface SLDKSearchResponse {
  success: boolean;
  message: string;
  data: {
    columns: SLDKColumn[];
    results: Record<string, unknown>[];
    count: number;
  };
}

export async function searchSLDKAssets(query: string, limit = 50) {
  const res = await api.get<SLDKSearchResponse>("/sldk/assets/search", {
    params: { q: query, limit },
  });
  return res.data;
}

// ============ HRIS2 Integration ============

export interface HRIS2SearchResponse {
  success: boolean;
  message: string;
  data: unknown; // struktur respons HRIS2 belum terdokumentasi, ditangani fleksibel di komponen
}

export async function searchHRIS2Pegawai(query: string) {
  const res = await api.get<HRIS2SearchResponse>("/hris2/pegawai/search", {
    params: { q: query },
  });
  return res.data;
}

// ============ User Management ============

export interface UserListItem {
  id: string;
  username: string;
  email: string;
  full_name: string;
  role: string;
  is_active: boolean;
  auth_provider: string;
  is_protected: boolean;
  nip: string | null;
  jabatan: string | null;
  satker: string | null;
  created_at: string;
}

export interface ListUsersResponse {
  success: boolean;
  message: string;
  data: UserListItem[];
}

export async function listUsers() {
  const res = await api.get<ListUsersResponse>("/users");
  return res.data;
}

export interface CreateUserPayload {
  source: "hris2" | "manual";
  nip?: string;
  username: string;
  password: string;
  email?: string;
  full_name?: string;
  role: "user" | "admin";
}

export async function createUser(payload: CreateUserPayload) {
  const res = await api.post("/users", payload);
  return res.data;
}

export async function searchPegawaiByNIP(nip: string) {
  const res = await api.get<{ success: boolean; message: string; data: Record<string, unknown> }>(
    `/hris2/pegawai/by-nip/${encodeURIComponent(nip)}`
  );
  return res.data;
}

export async function updateUserRole(userId: string, role: string) {
  const res = await api.put(`/users/${userId}/role`, { role });
  return res.data;
}

export async function deactivateUser(userId: string) {
  const res = await api.put(`/users/${userId}/deactivate`);
  return res.data;
}

export async function deleteUser(userId: string) {
  const res = await api.delete(`/users/${userId}`);
  return res.data;
}