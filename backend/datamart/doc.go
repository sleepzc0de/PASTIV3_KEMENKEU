// Paket datamart memuat script datamart SLDK untuk Digitalisasi Aset (buat_datamart_sldk.sql) dan tes yang menjaganya.
//
// Script ini TIDAK dijalankan oleh aplikasi: ia dijalankan administrator database (sqlcmd atau SQL Agent) di database datamart, dan membuat salinan data SLDK yang dibutuhkan
// ketujuh query Digitalisasi Aset (backend/digitalisasi/queries) dengan nama skema, tabel, dan kolom yang sama, sehingga query itu berjalan tanpa diubah bila SLDK_DB_* diarahkan
// ke datamart. Lihat docs/datamart-sldk.md.
//
// Tes statis memastikan script selalu mencakup kolom, tabel, dan kelompok kode barang yang dipakai query; tes integrasi (bila PASTI_UJI_MSSQL_DSN diisi) menjalankan script terhadap SLDK
// palsu (testdata/sldk_palsu.sql) dan membuktikan hasil ketujuh query dari datamart identik dengan dari SLDK.
package datamart
