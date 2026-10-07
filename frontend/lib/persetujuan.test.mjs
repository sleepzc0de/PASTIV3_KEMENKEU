import test from "node:test";
import assert from "node:assert/strict";

// Jalankan dari folder frontend: node --test lib/persetujuan.test.mjs
const { FRASA_SETUJU, frasaSah, rapikanProfil, emailSah, validasiProfil, bolehKirimPersetujuan } = await import(new URL("./persetujuan.ts", import.meta.url).href);

test("frasa persetujuan: tanpa membedakan huruf besar/kecil dan spasi berlebih", () => {
  assert.equal(FRASA_SETUJU, "SAYA SETUJU");
  for (const s of ["SAYA SETUJU", "saya setuju", "  Saya   Setuju  ", "SAYA\tSETUJU"]) assert.equal(frasaSah(s), true, s);
  for (const s of ["", "saya", "SAYA TIDAK SETUJU", "SAYASETUJU", "SAYA SETUJU.", "setuju saya"]) assert.equal(frasaSah(s), false, s);
});

test("rapikan profil", () => {
  assert.deepEqual(rapikanProfil({ nama: "  Budi   Santoso ", nip: " 1996 0910 2018 0110 05 ", email: "  Budi@Kemenkeu.GO.ID " }), {
    nama: "Budi Santoso",
    nip: "199609102018011005",
    email: "budi@kemenkeu.go.id",
  });
});

test("email sah: alamat biasa dengan domain bertitik, maksimal 100 karakter", () => {
  for (const e of ["a@b.co", "budi.santoso@kemenkeu.go.id", "x+y@mail.example.com"]) assert.equal(emailSah(e), true, e);
  for (const e of ["", "budi", "budi@", "@kemenkeu.go.id", "budi@localhost", "Budi <budi@kemenkeu.go.id>", "a b@c.co", "a@b..co", "a@.co", "a@b.", "a".repeat(96) + "@b.co"]) assert.equal(emailSah(e), false, e);
});

test("validasi profil: galat per isian dengan pesan yang sama seperti backend", () => {
  const ok = rapikanProfil({ nama: "Budi Santoso", nip: "199609102018011005", email: "budi@kemenkeu.go.id" });
  assert.deepEqual(validasiProfil(ok), {});
  assert.deepEqual(validasiProfil({ ...ok, nip: "123456789" }), {}); // NIP lama 9 digit
  assert.match(validasiProfil({ ...ok, nama: "Bu" }).nama, /minimal 3 karakter/);
  assert.match(validasiProfil({ ...ok, nama: "a".repeat(101) }).nama, /maksimal 100/);
  assert.match(validasiProfil({ ...ok, nip: "19960910201801100X" }).nip, /18 digit/);
  assert.match(validasiProfil({ ...ok, nip: "1996" }).nip, /18 digit/);
  assert.match(validasiProfil({ ...ok, email: "bukan-email" }).email, /email yang sah/);
  assert.deepEqual(Object.keys(validasiProfil({ nama: "", nip: "", email: "" })).sort(), ["email", "nama", "nip"]);
});

test("tombol setuju aktif hanya bila frasa benar, dicentang, dan profil non-SSO sah", () => {
  const profil = { nama: "Budi Santoso", nip: "199609102018011005", email: "budi@kemenkeu.go.id" };
  const kosong = { nama: "", nip: "", email: "" };
  assert.equal(bolehKirimPersetujuan({ frasa: "saya setuju", dicentang: true, isiProfil: false, profil: kosong }), true); // SSO: profil tidak diperiksa
  assert.equal(bolehKirimPersetujuan({ frasa: "saya setuju", dicentang: false, isiProfil: false, profil: kosong }), false);
  assert.equal(bolehKirimPersetujuan({ frasa: "setuju", dicentang: true, isiProfil: false, profil: kosong }), false);
  assert.equal(bolehKirimPersetujuan({ frasa: "SAYA SETUJU", dicentang: true, isiProfil: true, profil: kosong }), false);
  assert.equal(bolehKirimPersetujuan({ frasa: "SAYA SETUJU", dicentang: true, isiProfil: true, profil }), true);
  assert.equal(bolehKirimPersetujuan({ frasa: "SAYA SETUJU", dicentang: true, isiProfil: true, profil: { ...profil, nip: " 1996 0910 2018 0110 05 " } }), true); // dirapikan sebelum diperiksa
});
