import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

// Tes versi aplikasi. Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/versi.test.mjs
const V = await import(new URL("./versi.ts", import.meta.url).href);
const pkg = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8"));
const lock = JSON.parse(readFileSync(new URL("../package-lock.json", import.meta.url), "utf8"));

test("versiSah: tepat tiga angka", () => {
  for (const v of ["3.0.0", "3.0.1", "3.1.0", "3.12.34"]) assert.equal(V.versiSah(v), true, v);
  for (const v of ["", "3", "3.0", "3.0.0.1", "v3.0.0", "3.0.x", "3.0.0-beta", " 3.0.0", "3.0.0\n"]) assert.equal(V.versiSah(v), false, JSON.stringify(v));
});

test("teksVersi: menampilkan versi yang sah, kosong bila tidak sah", () => {
  assert.equal(V.teksVersi("3.0.0"), "Versi 3.0.0");
  assert.equal(V.teksVersi("3.1.0"), "Versi 3.1.0");
  assert.equal(V.teksVersi(""), "");
  assert.equal(V.teksVersi("3.0"), "");
});

test("package.json: versi aplikasi sah, tetap generasi 3 (V3), dan package-lock.json sama", () => {
  assert.equal(V.versiSah(pkg.version), true, `versi package.json "${pkg.version}" harus berbentuk X.Y.Z`);
  assert.equal(pkg.version.split(".")[0], "3", "angka pertama adalah generasi aplikasi (PASTI V3)");
  assert.equal(lock.version, pkg.version, "package-lock.json harus sama (pakai npm version, jangan ubah manual)");
  assert.equal(lock.packages[""].version, pkg.version);
});
