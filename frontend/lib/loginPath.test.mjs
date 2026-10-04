import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

// Tes aturan alamat halaman login (bukan /login). Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/loginPath.test.mjs
const L = await import(new URL("./loginPath.ts", import.meta.url).href);

const JALUR = "/3f9a1c07d2b84e65a0c1f7e93b2d4a68";
const B64 = "/Q29udG9oUGF0aEJhc2U2NA==";

const masuk = (pathname, o = {}) => ({ pathname, search: "", adaToken: false, jalurMasuk: JALUR, ...o });

test("jalurMasukDari: kosong atau /login = tetap di /login, bentuk salah = galat", () => {
  assert.equal(L.jalurMasukDari(undefined), "/login");
  assert.equal(L.jalurMasukDari(""), "/login");
  assert.equal(L.jalurMasukDari("   "), "/login");
  assert.equal(L.jalurMasukDari("/login"), "/login"); // eksplisit: halaman login tetap di /login
  assert.equal(L.jalurMasukDari(JALUR), JALUR);
  assert.equal(L.jalurMasukDari(" " + JALUR + " "), JALUR);
  for (const salah of [
    "login", "/Login", "/login/", JALUR.slice(1), "/dashboard", "/abc", "/3f9a1c07d2b84e6", // pendek (15)
    "/3f9a1c07d2b84e65a0c1f7e93b2d4a68/x", // lebih dari satu segmen
    "/c2FtcGxlLnBhdGg.contohpanjang", // titik: dianggap berkas statis
    "/ada spasi di sini 12345678", "/aneh!?karakter1234567890", "/" + "a".repeat(129),
    "/halaman-tidak-ada", "/HALAMAN-TIDAK-ADA", // nama rute aplikasi
  ]) {
    assert.throws(() => L.jalurMasukDari(salah), /LOGIN_PATH/, salah);
  }
  // Bentuk yang diterima: heksadesimal acak, atau nilai buatan sendiri (mis. base64) dengan huruf besar dan "=".
  for (const sah of ["/" + "a".repeat(64), "/" + "a".repeat(128), "/3f9a1c07d2b84e65", "/ABCDEF0123456789ABCDEF0123456789", "/c2FtcGxlLXBhdGgtY29udG9o", B64, "/contoh-jalur_masuk-2026"]) {
    assert.equal(L.jalurMasukDari(sah), sah, sah);
  }
});

test("nilai hasil next.config dibaca kembali middleware dengan hasil yang sama", () => {
  // next.config menyisipkan alamat yang sudah dihitung; middleware membacanya lagi dengan jalurMasukDari (termasuk "/login" eksplisit).
  for (const env of [undefined, "", "/login", JALUR, B64]) {
    const jalur = L.jalurMasukDari(env);
    assert.equal(L.jalurMasukDari(jalur), jalur);
  }
});

test("frontend/login-path.txt berisi alamat yang sah dan bukan /login", () => {
  // Berkas ini dibaca next.config.ts dan deploy.sh; isi yang salah harus ketahuan di sini, bukan saat deploy.
  const isi = readFileSync(new URL("../login-path.txt", import.meta.url), "utf8").trim();
  assert.notEqual(isi, "/login", "bawaan repo tidak boleh /login");
  assert.equal(L.jalurMasukDari(isi), isi);
  assert.equal(isi.split("\n").length, 1);
});

test("samakanJalur: = yang disandikan menjadi %3D tetap dikenali sebagai jalur masuk", () => {
  assert.equal(L.samakanJalur(B64, B64), B64);
  assert.equal(L.samakanJalur("/Q29udG9oUGF0aEJhc2U2NA%3D%3D", B64), B64);
  assert.equal(L.samakanJalur("/Q29udG9oUGF0aEJhc2U2NA%3d%3d", B64), B64);
  // Pathname lain tidak disentuh, termasuk yang memuat sandi atau sandi rusak.
  assert.equal(L.samakanJalur("/dashboard/a%20b", B64), "/dashboard/a%20b");
  assert.equal(L.samakanJalur("/%E0%A4%A", B64), "/%E0%A4%A");
  assert.equal(L.samakanJalur("/login", B64), "/login");
  assert.equal(L.samakanJalur("/Q29udG9oUGF0aEJhc2U2NA%3D", B64), "/Q29udG9oUGF0aEJhc2U2NA%3D");
});

test("alamat utama (/) langsung mengarahkan ke halaman login, tanpa perlu mengetik alamatnya", () => {
  // Kunjungan pertama: tanpa cookie apa pun.
  assert.deepEqual(L.tentukanAksi(masuk("/")), { jenis: "alihkan", ke: JALUR });
  assert.deepEqual(L.tentukanAksi(masuk("/", { jalurMasuk: B64 })), { jenis: "alihkan", ke: B64 });
  // Yang sudah punya sesi langsung ke dashboard.
  assert.deepEqual(L.tentukanAksi(masuk("/", { adaToken: true })), { jenis: "alihkan", ke: "/dashboard" });
});

test("halaman login: ditulis ulang ke /login (alamat di peramban tetap)", () => {
  assert.deepEqual(L.tentukanAksi(masuk(JALUR)), { jenis: "tulis-ulang", ke: "/login" });
  // Pesan galat (query) ikut diteruskan ke halaman login.
  assert.deepEqual(L.tentukanAksi(masuk(JALUR, { search: "?error=session_expired&reason=x" })), { jenis: "tulis-ulang", ke: "/login?error=session_expired&reason=x" });
  assert.deepEqual(L.tentukanAksi(masuk(B64, { jalurMasuk: B64 })), { jenis: "tulis-ulang", ke: "/login" });
  // Yang sudah punya sesi tidak perlu melihat halaman login.
  assert.deepEqual(L.tentukanAksi(masuk(JALUR, { adaToken: true })), { jenis: "alihkan", ke: "/dashboard" });
});

test("rute internal /login tidak bisa dibuka langsung", () => {
  for (const p of ["/login", "/login/", "/login/apa-saja"]) {
    assert.deepEqual(L.tentukanAksi(masuk(p)), { jenis: "tidak-ada" }, p);
    assert.deepEqual(L.tentukanAksi(masuk(p, { adaToken: true })), { jenis: "tidak-ada" }, p);
  }
  // Awalan serupa bukan rute login.
  assert.deepEqual(L.tentukanAksi(masuk("/loginx")), { jenis: "lanjut" });
});

test("/dashboard tanpa sesi dan /kembali-masuk mengarahkan ke halaman login", () => {
  assert.deepEqual(L.tentukanAksi(masuk("/dashboard")), { jenis: "alihkan", ke: JALUR });
  assert.deepEqual(L.tentukanAksi(masuk("/dashboard/sapa/penjualan")), { jenis: "alihkan", ke: JALUR });
  // Logout, sesi habis, dan galat SSO: pesan galat dibawa serta, juga untuk pengunjung yang belum pernah membuka halaman login.
  assert.deepEqual(L.tentukanAksi(masuk("/kembali-masuk")), { jenis: "alihkan", ke: JALUR });
  assert.deepEqual(L.tentukanAksi(masuk("/kembali-masuk", { search: "?error=session_expired&reason=abc" })), { jenis: "alihkan", ke: JALUR + "?error=session_expired&reason=abc" });
  assert.deepEqual(L.tentukanAksi(masuk("/kembali-masuk", { search: "?error=sso_failed&reason=x", jalurMasuk: B64 })), { jenis: "alihkan", ke: B64 + "?error=sso_failed&reason=x" });
});

test("pemilik sesi: /dashboard berjalan seperti biasa", () => {
  assert.deepEqual(L.tentukanAksi(masuk("/dashboard", { adaToken: true })), { jenis: "lanjut" });
  assert.deepEqual(L.tentukanAksi(masuk("/dashboard/sapa/penjualan/x", { adaToken: true })), { jenis: "lanjut" });
});

test("rute lain tidak disentuh", () => {
  for (const p of ["/sso/callback", "/dashboardx", "/apa-saja", "/halaman-tidak-ada"]) {
    assert.deepEqual(L.tentukanAksi(masuk(p)), { jenis: "lanjut" }, p);
  }
});

test("LOGIN_PATH=/login (tidak diganti): perilaku /login semula", () => {
  const dev = (pathname, o = {}) => ({ pathname, search: "", adaToken: false, jalurMasuk: "/login", ...o });
  assert.deepEqual(L.tentukanAksi(dev("/login")), { jenis: "lanjut" });
  assert.deepEqual(L.tentukanAksi(dev("/login", { adaToken: true })), { jenis: "alihkan", ke: "/dashboard" });
  assert.deepEqual(L.tentukanAksi(dev("/")), { jenis: "alihkan", ke: "/login" });
  assert.deepEqual(L.tentukanAksi(dev("/", { adaToken: true })), { jenis: "alihkan", ke: "/dashboard" });
  assert.deepEqual(L.tentukanAksi(dev("/dashboard")), { jenis: "alihkan", ke: "/login" });
  assert.deepEqual(L.tentukanAksi(dev("/dashboard", { adaToken: true })), { jenis: "lanjut" });
  assert.deepEqual(L.tentukanAksi(dev("/kembali-masuk", { search: "?error=sso_failed" })), { jenis: "alihkan", ke: "/login?error=sso_failed" });
});
