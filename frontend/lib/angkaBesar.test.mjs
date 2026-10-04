import test from "node:test";
import assert from "node:assert/strict";

// Jalankan dari folder frontend (Node 22.18+ membaca .ts langsung):
//   node --test lib/angkaBesar.test.mjs
const { kutipNipPanjang } = await import(new URL("./angkaBesar.ts", import.meta.url).href);

test("NIP 18 digit berupa angka dibaca utuh sebagai teks", () => {
  const r = kutipNipPanjang('{"data":[{"nip_ppk":198505152010011001,"rsk_id":7000}],"meta":{"has_more":false}}');
  assert.equal(r.data[0].nip_ppk, "198505152010011001");
  // Tanpa perlakuan ini, JSON.parse membulatkannya: inilah galat yang dicegah.
  assert.notEqual(String(JSON.parse('{"n":198505152010011001}').n), "198505152010011001");
  assert.equal(r.data[0].rsk_id, 7000);
  assert.equal(r.meta.has_more, false);
});

test("angka pendek dan teks tidak diubah", () => {
  const r = kutipNipPanjang('{"data":[{"nip_ppk":12345},{"nip_ppk":"198505152010011001"},{"nip_ppk":null}]}');
  assert.equal(r.data[0].nip_ppk, 12345);
  assert.equal(r.data[1].nip_ppk, "198505152010011001");
  assert.equal(r.data[2].nip_ppk, null);
});

test("spasi di sekitar titik dua tetap dikenali", () => {
  const r = kutipNipPanjang('{"nip_ppk" :   198505152010011001}');
  assert.equal(r.nip_ppk, "198505152010011001");
});

test("badan galat, nilai bukan teks, dan teks bukan JSON lewat apa adanya", () => {
  assert.deepEqual(kutipNipPanjang('{"success":false,"error":{"code":"Too Many Requests"}}'), { success: false, error: { code: "Too Many Requests" } });
  const sudah = { a: 1 };
  assert.equal(kutipNipPanjang(sudah), sudah);
  assert.equal(kutipNipPanjang(undefined), undefined);
  assert.equal(kutipNipPanjang("<html>502</html>"), "<html>502</html>");
  assert.equal(kutipNipPanjang(""), "");
});
