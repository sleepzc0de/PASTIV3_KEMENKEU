import test from "node:test";
import assert from "node:assert/strict";

// Tes aturan jalan pintas callback SSO. Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/ssoCallback.test.mjs
const S = await import(new URL("./ssoCallback.ts", import.meta.url).href);

test("mati secara bawaan: tidak ada aturan bila variabel kosong, 0, atau tidak ada", () => {
  for (const nilai of [undefined, null, "", "   ", "0", " 0 "]) {
    assert.deepEqual(S.aturanCallbackSSO({ nyala: nilai }), [], JSON.stringify(nilai));
  }
  // Alamat backend tidak mengubah apa pun selama jalan pintas mati (bahkan yang salah bentuk tidak diperiksa).
  assert.deepEqual(S.aturanCallbackSSO({ nyala: "0", backend: "bukan-url" }), []);
});

test("nyala hanya dengan 1; nilai lain ditolak, bukan diam-diam mati atau nyala", () => {
  assert.equal(S.callbackViaFrontendNyala("1"), true);
  assert.equal(S.callbackViaFrontendNyala(" 1 "), true);
  for (const salah of ["true", "TRUE", "ya", "yes", "on", "2", "01", "1 1", "-1"]) {
    assert.throws(() => S.callbackViaFrontendNyala(salah), /SSO_CALLBACK_VIA_FRONTEND/, salah);
    assert.throws(() => S.aturanCallbackSSO({ nyala: salah }), /SSO_CALLBACK_VIA_FRONTEND/, salah);
  }
});

test("satu aturan: persis /sso/callback/login ke backend Docker bawaan", () => {
  const a = S.aturanCallbackSSO({ nyala: "1" });
  assert.deepEqual(a, [{ source: "/sso/callback/login", destination: "http://backend:8686/sso/callback/login" }]);
  // Tidak ada pola, awalan, atau parameter yang bisa menangkap rute lain (mis. /sso/callback yang adalah halaman frontend, atau /api).
  assert.equal(a.length, 1);
  assert.ok(!/[:*(]/.test(a[0].source), "source harus path persis tanpa parameter/wildcard: " + a[0].source);
  assert.notEqual(a[0].source, "/sso/callback");
});

test("alamat backend dapat diganti (mis. pengembangan lokal) dan dirapikan", () => {
  assert.equal(S.aturanCallbackSSO({ nyala: "1", backend: "http://localhost:8686" })[0].destination, "http://localhost:8686/sso/callback/login");
  assert.equal(S.aturanCallbackSSO({ nyala: "1", backend: " http://pasti-backend:8686/ " })[0].destination, "http://pasti-backend:8686/sso/callback/login");
  assert.equal(S.aturanCallbackSSO({ nyala: "1", backend: "https://api.contoh.go.id" })[0].destination, "https://api.contoh.go.id/sso/callback/login");
  assert.equal(S.aturanCallbackSSO({ nyala: "1", backend: "" })[0].destination, S.BACKEND_INTERNAL_BAWAAN + S.JALUR_CALLBACK_SSO);
  assert.equal(S.aturanCallbackSSO({ nyala: "1", backend: null })[0].destination, S.BACKEND_INTERNAL_BAWAAN + S.JALUR_CALLBACK_SSO);
});

test("alamat backend yang salah bentuk ditolak saat jalan pintas nyala", () => {
  for (const salah of ["backend:8686", "ftp://backend", "http://backend:8686/api", "http://backend:8686/?x=1", "http://", "http://back end", "http://backend:99999999", "//backend",
    "http://user:pw@backend", "javascript:alert(1)", "http://backend:8686#x"]) {
    assert.throws(() => S.aturanCallbackSSO({ nyala: "1", backend: salah }), /BACKEND_INTERNAL_URL/, salah);
  }
});

test("BACKEND_VIA_FRONTEND mati secara bawaan dan hanya menyala dengan 1", () => {
  for (const nilai of [undefined, null, "", " ", "0"]) {
    assert.deepEqual(S.aturanJalanPintas({ semua: nilai }), [], JSON.stringify(nilai));
  }
  for (const salah of ["true", "ya", "on", "2"]) {
    assert.throws(() => S.aturanJalanPintas({ semua: salah }), /BACKEND_VIA_FRONTEND/, salah);
  }
  // Salah tulis di SSO_CALLBACK_VIA_FRONTEND tetap menggagalkan walau BACKEND_VIA_FRONTEND menyala.
  assert.throws(() => S.aturanJalanPintas({ semua: "1", nyala: "true" }), /SSO_CALLBACK_VIA_FRONTEND/);
});

test("BACKEND_VIA_FRONTEND=1 meneruskan semua rute backend seperti Nginx, mencakup callback SSO", () => {
  const a = S.aturanJalanPintas({ semua: "1" });
  assert.deepEqual(a, [
    { source: "/api/:path+", destination: "http://backend:8686/api/:path+" },
    { source: "/health", destination: "http://backend:8686/health" },
    { source: "/sso/login", destination: "http://backend:8686/sso/login" },
    { source: "/sso/callback/login", destination: "http://backend:8686/sso/callback/login" },
  ]);
  // Sama saja bila jalan pintas callback juga menyala (tidak ada aturan ganda).
  assert.deepEqual(S.aturanJalanPintas({ semua: "1", nyala: "1" }), a);
  // Halaman frontend /sso/callback (tanpa /login) tidak ikut diteruskan.
  assert.ok(!a.some((r) => r.source === "/sso/callback"));
  assert.equal(S.aturanJalanPintas({ semua: "1", backend: "http://localhost:18686" })[0].destination, "http://localhost:18686/api/:path+");
});

test("tanpa BACKEND_VIA_FRONTEND, jalan pintas callback tetap bekerja seperti sebelumnya", () => {
  assert.deepEqual(S.aturanJalanPintas({ nyala: "1" }), S.aturanCallbackSSO({ nyala: "1" }));
  assert.deepEqual(S.aturanJalanPintas({}), []);
});

test("pendaftaran akun publik ditutup dan batas proxy di atas batas backend", () => {
  assert.equal(S.JALUR_DAFTAR_DITUTUP, "/api/v1/auth/register");
  assert.equal(S.BATAS_WAKTU_PROXY_MS, 900_000);
  assert.equal(S.BATAS_BADAN_PROXY, "16mb");
});
