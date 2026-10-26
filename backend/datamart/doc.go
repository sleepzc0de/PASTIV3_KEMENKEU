// Paket datamart memuat script datamart untuk Digitalisasi Aset dan tes yang menjaganya.
//
// Script buat_datamart_digitalisasi.sql TIDAK dijalankan oleh aplikasi: ia dijalankan administrator database (SSMS, sqlcmd, atau SQL Agent) di database datamart. Ia membaca SLDK dengan
// ketujuh query asli Digitalisasi Aset dan membentuk tabel DIGITALISASI_* jadi di datamart, dengan konversi yang sama seperti implementasi acuan (digitalisasi/acuan). Aplikasi lalu
// menyalin tabel-tabel itu ke PASTI (digitalisasi.Sync) dan tidak pernah mengakses SLDK untuk dataset ini. File script DIHASILKAN oleh digitalisasi.go dari backend/digitalisasi;
// jangan disunting manual (lihat docs/datamart-digitalisasi.md).
//
// Tes statis menjaga script tetap sejalan dengan aplikasi (kolom, tipe, query, konversi). Tes integrasi (bila PASTI_UJI_MSSQL_DSN diisi) menjalankan script terhadap SLDK palsu
// (testdata/sldk_palsu.sql dan sldk_palsu_tepi.sql) dan membuktikan: tabel hasil script identik dengan tabel acuan; tabel PASTI hasil sinkronisasi aplikasi yang sungguhan (membaca datamart,
// tanpa SLDK) identik dengan tabel acuan; serta jalur gagal tidak merusak datamart maupun PASTI.
package datamart
