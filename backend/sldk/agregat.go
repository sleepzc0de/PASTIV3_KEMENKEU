package sldk

import (
	"fmt"
	"strings"
)

// AggregateColumns: kolom M_ASET yang dibaca query agregat (termasuk kolom aturan pemantauan).
var AggregateColumns = func() []string {
	cols := []string{
		"status_data", "sts_his", "sts_ast", "tgl_hapus", "kd_jns_bmn", "kd_kondisi", "kd_status", "id_satker",
		"ur_prov", "tgl_perlh", "rph_aset", "rph_buku", "rph_susut", "_ingestion_date",
	}
	seen := map[string]bool{}
	for _, c := range cols {
		seen[c] = true
	}
	for _, r := range Rules {
		for _, c := range r.Columns {
			if !seen[c] {
				seen[c] = true
				cols = append(cols, c)
			}
		}
	}
	return cols
}()

// FlagKeyExpr membentuk "kunci penanda" per baris dari kolom yang kemungkinan menandai aset aktif/tidak:
// status_data|sts_his|sts_ast|A (tgl_hapus kosong) atau H (sudah ada tgl_hapus). Arti nilainya belum
// dikonfirmasi, jadi kunci ini disimpan sebagai dimensi; admin memilih kunci mana yang dihitung "aktif"
// setelah melihat sebarannya (lihat pengaturan di dashboard).
const FlagKeyExpr = "CONCAT(ISNULL(CAST([status_data] AS varchar(12)), '-'), '|', ISNULL(CAST([sts_his] AS varchar(12)), '-'), '|', " +
	"ISNULL(CAST([sts_ast] AS varchar(12)), '-'), '|', CASE WHEN [tgl_hapus] IS NULL THEN 'A' ELSE 'H' END)"

// AggregateResultColumns: urutan kolom hasil AggregateSQL (dibaca apa adanya oleh cmd/sldk-sync).
func AggregateResultColumns() []string {
	cols := []string{"dim", "k1", "k2", "flag_key", "jumlah", "nilai_perolehan", "nilai_buku", "nilai_susut"}
	for _, r := range Rules {
		cols = append(cols, r.CountColumn())
	}
	return append(cols, "data_per")
}

// AggregateSQL menyusun query agregat untuk dashboard: SATU kali pemindaian yang menghitung semua dimensi
// sekaligus lewat GROUPING SETS. Setiap baris hasil adalah satu kelompok:
//
//	dim = total | jenis | kondisi | status | jenis_kondisi | satker | provinsi | tahun
//	k1, k2 = nilai dimensi (jenis_kondisi: k1 = jenis, k2 = kondisi)
//
// beserta jumlah baris, jumlah nilai, dan penghitung tiap aturan pemantauan. where boleh kosong (seluruh
// tabel) atau berisi predikat cakupan, mis. dari ScopePredicate. tableRef sudah ber-quote.
func AggregateSQL(tableRef, where string) string {
	var b strings.Builder

	b.WriteString("SELECT\n")
	b.WriteString("  CASE\n")
	b.WriteString("    WHEN GROUPING([g_jenis]) = 0 AND GROUPING([g_kondisi]) = 0 THEN 'jenis_kondisi'\n")
	b.WriteString("    WHEN GROUPING([g_jenis]) = 0 THEN 'jenis'\n")
	b.WriteString("    WHEN GROUPING([g_kondisi]) = 0 THEN 'kondisi'\n")
	b.WriteString("    WHEN GROUPING([g_status]) = 0 THEN 'status'\n")
	b.WriteString("    WHEN GROUPING([g_satker]) = 0 THEN 'satker'\n")
	b.WriteString("    WHEN GROUPING([g_prov]) = 0 THEN 'provinsi'\n")
	b.WriteString("    WHEN GROUPING([g_tahun]) = 0 THEN 'tahun'\n")
	b.WriteString("    ELSE 'total'\n")
	b.WriteString("  END AS [dim],\n")
	b.WriteString("  CASE\n")
	b.WriteString("    WHEN GROUPING([g_jenis]) = 0 THEN [g_jenis]\n")
	b.WriteString("    WHEN GROUPING([g_kondisi]) = 0 THEN [g_kondisi]\n")
	b.WriteString("    WHEN GROUPING([g_status]) = 0 THEN [g_status]\n")
	b.WriteString("    WHEN GROUPING([g_satker]) = 0 THEN [g_satker]\n")
	b.WriteString("    WHEN GROUPING([g_prov]) = 0 THEN [g_prov]\n")
	b.WriteString("    WHEN GROUPING([g_tahun]) = 0 THEN [g_tahun]\n")
	b.WriteString("    ELSE NULL\n")
	b.WriteString("  END AS [k1],\n")
	b.WriteString("  CASE WHEN GROUPING([g_jenis]) = 0 AND GROUPING([g_kondisi]) = 0 THEN [g_kondisi] ELSE NULL END AS [k2],\n")
	b.WriteString("  [g_flag] AS [flag_key],\n")
	b.WriteString("  COUNT_BIG(*) AS [jumlah],\n")
	b.WriteString("  SUM([v_aset]) AS [nilai_perolehan],\n")
	b.WriteString("  SUM([v_buku]) AS [nilai_buku],\n")
	b.WriteString("  SUM([v_susut]) AS [nilai_susut],\n")
	for _, r := range Rules {
		fmt.Fprintf(&b, "  SUM(CAST([a_%s] AS bigint)) AS %s,\n", r.Key, QuoteIdent(r.CountColumn()))
	}
	b.WriteString("  MAX([g_ing]) AS [data_per]\n")

	b.WriteString("FROM (\n")
	b.WriteString("  SELECT\n")
	fmt.Fprintf(&b, "    %s AS [g_flag],\n", FlagKeyExpr)
	b.WriteString("    CAST([kd_jns_bmn] AS varchar(20)) AS [g_jenis],\n")
	b.WriteString("    CAST([kd_kondisi] AS varchar(20)) AS [g_kondisi],\n")
	b.WriteString("    CAST([kd_status] AS varchar(20)) AS [g_status],\n")
	b.WriteString("    CAST([id_satker] AS varchar(20)) AS [g_satker],\n")
	b.WriteString("    CAST([ur_prov] AS nvarchar(100)) AS [g_prov],\n")
	b.WriteString("    CAST(YEAR([tgl_perlh]) AS varchar(4)) AS [g_tahun],\n")
	b.WriteString("    CAST(ISNULL([rph_aset], 0) AS decimal(28, 2)) AS [v_aset],\n")
	b.WriteString("    CAST(ISNULL([rph_buku], 0) AS decimal(28, 2)) AS [v_buku],\n")
	b.WriteString("    CAST(ISNULL([rph_susut], 0) AS decimal(28, 2)) AS [v_susut],\n")
	for _, r := range Rules {
		fmt.Fprintf(&b, "    CASE WHEN %s THEN 1 ELSE 0 END AS [a_%s],\n", r.Predicate, r.Key)
	}
	b.WriteString("    [_ingestion_date] AS [g_ing]\n")
	fmt.Fprintf(&b, "  FROM %s WITH (NOLOCK)\n", tableRef)
	if strings.TrimSpace(where) != "" {
		fmt.Fprintf(&b, "  WHERE %s\n", where)
	}
	b.WriteString(") x\n")

	b.WriteString("GROUP BY GROUPING SETS (\n")
	b.WriteString("  ([g_flag]),\n")
	b.WriteString("  ([g_flag], [g_jenis]),\n")
	b.WriteString("  ([g_flag], [g_kondisi]),\n")
	b.WriteString("  ([g_flag], [g_status]),\n")
	b.WriteString("  ([g_flag], [g_jenis], [g_kondisi]),\n")
	b.WriteString("  ([g_flag], [g_satker]),\n")
	b.WriteString("  ([g_flag], [g_prov]),\n")
	b.WriteString("  ([g_flag], [g_tahun])\n")
	b.WriteString(")\n")
	b.WriteString("OPTION (MAXDOP 4)")
	return b.String()
}
