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
