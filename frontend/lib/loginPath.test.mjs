import test from "node:test";
import assert from "node:assert/strict";

// Tes aturan alamat login tersembunyi. Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/loginPath.test.mjs
const L = await import(new URL("./loginPath.ts", import.meta.url).href);

const JALUR = "/3f9a1c07d2b84e65a0c1f7e93b2d4a68";

const masuk = (pathname, o = {}) => ({ pathname, search: "", adaToken: false, adaPetunjuk: false, jalurMasuk: JALUR, ...o });

test("jalurMasukDari: kosong = bawaan, bentuk salah = galat", () => {
  assert.equal(L.jalurMasukDari(undefined), "/login");
  assert.equal(L.jalurMasukDari(""), "/login");
  assert.equal(L.jalurMasukDari("   "), "/login");
  assert.equal(L.jalurMasukDari(JALUR), JALUR);
  assert.equal(L.jalurMasukDari(" " + JALUR + " "), JALUR);
  for (const salah of ["/login", "login", JALUR.slice(1), "/dashboard", "/ABCDEF0123456789ABCDEF0123456789", "/3f9a1c07d2b84e65", "/3f9a1c07d2b84e65a0c1f7e93b2d4a68/x", "/zz9a1c07d2b84e65a0c1f7e93b2d4a68", "/" + "a".repeat(65)]) {
    assert.throws(() => L.jalurMasukDari(salah), /LOGIN_PATH/, salah);
  }
  assert.equal(L.jalurMasukDari("/" + "a".repeat(64)), "/" + "a".repeat(64));
});

test("nilai yang disisipkan ke build dibaca kembali middleware dengan hasil yang sama", () => {
  // Pengembangan lokal (tanpa LOGIN_PATH) dan produksi (alamat acak) harus bolak-balik tanpa galat.
  for (const env of [undefined, "", JALUR]) {
    const jalur = L.jalurMasukDari(env);
    assert.equal(L.jalurMasukDari(L.nilaiUntukBuild(jalur)), jalur);
  }
  assert.equal(L.nilaiUntukBuild("/login"), "");
  assert.equal(L.nilaiUntukBuild(JALUR), JALUR);
});

test("halaman login: ditulis ulang ke /login dan peramban diberi petunjuk", () => {
  assert.deepEqual(L.tentukanAksi(masuk(JALUR)), { jenis: "tulis-ulang", ke: "/login", setPetunjuk: true });
  // Pesan galat (query) ikut diteruskan ke halaman login.
  assert.deepEqual(L.tentukanAksi(masuk(JALUR, { search: "?error=session_expired&reason=x" })), { jenis: "tulis-ulang", ke: "/login?error=session_expired&reason=x", setPetunjuk: true });
  // Petunjuk yang sudah ada tidak dipasang ulang.
  assert.equal(L.tentukanAksi(masuk(JALUR, { adaPetunjuk: true })).setPetunjuk, false);
  // Yang sudah punya sesi tidak perlu melihat halaman login.
  assert.deepEqual(L.tentukanAksi(masuk(JALUR, { adaToken: true })), { jenis: "alihkan", ke: "/dashboard" });
});

test("rute internal /login tidak bisa dibuka langsung", () => {
  for (const p of ["/login", "/login/", "/login/apa-saja"]) {
    assert.deepEqual(L.tentukanAksi(masuk(p)), { jenis: "tidak-ada" }, p);
    assert.deepEqual(L.tentukanAksi(masuk(p, { adaPetunjuk: true })), { jenis: "tidak-ada" }, p);
    assert.deepEqual(L.tentukanAksi(masuk(p, { adaToken: true })), { jenis: "tidak-ada" }, p);
  }
  // Awalan serupa bukan rute login.
  assert.deepEqual(L.tentukanAksi(masuk("/loginx")), { jenis: "lanjut" });
});

test("pengunjung tanpa petunjuk tidak pernah diberi tahu alamat login", () => {
  const tanpa = { adaToken: false, adaPetunjuk: false };
  for (const p of ["/", "/dashboard", "/dashboard/sapa/penjualan", "/kembali-masuk"]) {
    const a = L.tentukanAksi(masuk(p, { ...tanpa, search: "?error=sso_failed" }));
    assert.deepEqual(a, { jenis: "tidak-ada" }, p);
    assert.ok(!JSON.stringify(a).includes(JALUR.slice(1)), p);
  }
  // Cookie sesi saja bukan petunjuk (namanya terbaca di JavaScript klien, jadi bisa dipalsukan): /kembali-masuk tetap 404.
  assert.deepEqual(L.tentukanAksi(masuk("/kembali-masuk", { adaToken: true })), { jenis: "tidak-ada" });
});

test("peramban yang pernah membuka halaman login diarahkan kembali ke sana", () => {
  const kenal = { adaPetunjuk: true };
  assert.deepEqual(L.tentukanAksi(masuk("/", kenal)), { jenis: "alihkan", ke: JALUR });
  assert.deepEqual(L.tentukanAksi(masuk("/dashboard", kenal)), { jenis: "alihkan", ke: JALUR });
  assert.deepEqual(L.tentukanAksi(masuk("/dashboard/sapa", kenal)), { jenis: "alihkan", ke: JALUR });
  // Logout dan sesi habis: pesan galat diteruskan.
  assert.deepEqual(L.tentukanAksi(masuk("/kembali-masuk", { ...kenal, search: "?error=session_expired&reason=abc" })), { jenis: "alihkan", ke: JALUR + "?error=session_expired&reason=abc" });
  assert.deepEqual(L.tentukanAksi(masuk("/kembali-masuk", kenal)), { jenis: "alihkan", ke: JALUR });
});

test("pemilik sesi: / dan /dashboard berjalan seperti biasa", () => {
  assert.deepEqual(L.tentukanAksi(masuk("/", { adaToken: true })), { jenis: "alihkan", ke: "/dashboard" });
  assert.deepEqual(L.tentukanAksi(masuk("/dashboard", { adaToken: true })), { jenis: "lanjut" });
  assert.deepEqual(L.tentukanAksi(masuk("/dashboard/sapa/penjualan/x", { adaToken: true })), { jenis: "lanjut" });
});

test("rute lain tidak disentuh", () => {
  for (const p of ["/sso/callback", "/dashboardx", "/apa-saja", "/halaman-tidak-ada"]) {
    assert.deepEqual(L.tentukanAksi(masuk(p)), { jenis: "lanjut" }, p);
  }
});

test("tanpa LOGIN_PATH (pengembangan): perilaku /login semula", () => {
  const dev = (pathname, o = {}) => ({ pathname, search: "", adaToken: false, adaPetunjuk: false, jalurMasuk: "/login", ...o });
  assert.deepEqual(L.tentukanAksi(dev("/login")), { jenis: "lanjut" });
  assert.deepEqual(L.tentukanAksi(dev("/login", { adaToken: true })), { jenis: "alihkan", ke: "/dashboard" });
  assert.deepEqual(L.tentukanAksi(dev("/")), { jenis: "alihkan", ke: "/login" });
  assert.deepEqual(L.tentukanAksi(dev("/", { adaToken: true })), { jenis: "alihkan", ke: "/dashboard" });
  assert.deepEqual(L.tentukanAksi(dev("/dashboard")), { jenis: "alihkan", ke: "/login" });
  assert.deepEqual(L.tentukanAksi(dev("/dashboard", { adaToken: true })), { jenis: "lanjut" });
  assert.deepEqual(L.tentukanAksi(dev("/kembali-masuk", { search: "?error=sso_failed" })), { jenis: "alihkan", ke: "/login?error=sso_failed" });
});
