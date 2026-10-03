-- ============================================================
-- REVISI 5: satker.sql (fix duplikat + kolom Jenis_Satker)
-- Tambahan:
--   Kolom Jenis_Satker berdasarkan SUBSTRING(kd_satker,16,3)
--   '000' = INDUK SATKER, selain itu = ANAK SATKER
-- ============================================================

WITH SatkerExclude AS (
    SELECT kd_satker FROM (VALUES
        (N'015010199409294002KP'),
        (N'015010199673102000KP'),
        (N'015010199409294001KP')
    ) AS X(kd_satker)
),
SatkerMergeRule AS (
    SELECT
        prefix,
        prefix + N'000KP' AS induk_kd_satker
    FROM (VALUES
        (N'015040199119091'),   -- UE1 DJP
        (N'015050199410640'),   -- UE1 DJBC
        (N'015080199527010'),   -- UE1 DJPB
        (N'015150199691273')    -- UE1 BATII
    ) AS X(prefix)
),
SatkerBase AS (
    -- Dedup baris mentah per kd_satker (jaga-jaga ada duplikat di sumber,
    -- karena tabel SIMAN2_R_SATKER adalah heap tanpa PK)
    SELECT *
    FROM (
        SELECT
            S.*,
            ROW_NUMBER() OVER (
                PARTITION BY S.kd_satker
                ORDER BY S.updated_at DESC, S.id_satker DESC
            ) AS rn
        FROM DJKN.SIMAN2_R_SATKER AS S
        WHERE S.status_uakpb = N'AKTIF'
          AND S.kd_satker LIKE N'015%'
    ) AS X
    WHERE X.rn = 1
),
SatkerAktif AS (
    -- DISTINCT wajib: tanpa ini, satu satker induk akan muncul
    -- berkali-kali (sebanyak jumlah anak satker yang di-merge)
    SELECT DISTINCT
        COALESCE(INDUK.kd_satker, S.kd_satker) AS kd_satker,
        COALESCE(INDUK.ur_satker, S.ur_satker) AS ur_satker,
        COALESCE(INDUK.alamat_satker, S.alamat_satker) AS alamat_satker,
        COALESCE(INDUK.nm_kel, S.nm_kel) AS nm_kel,
        COALESCE(INDUK.nm_kec, S.nm_kec) AS nm_kec,
        COALESCE(INDUK.nm_kab_kota, S.nm_kab_kota) AS nm_kab_kota,
        COALESCE(INDUK.nm_prov, S.nm_prov) AS nm_prov
    FROM SatkerBase AS S
    LEFT JOIN SatkerMergeRule AS M
           ON S.kd_satker LIKE M.prefix + N'%'
          AND S.kd_satker <> M.induk_kd_satker
    LEFT JOIN SatkerBase AS INDUK
           ON INDUK.kd_satker = M.induk_kd_satker
          AND M.prefix IS NOT NULL
    WHERE NOT EXISTS (
          SELECT 1 FROM SatkerExclude AS E
          WHERE E.kd_satker = S.kd_satker
      )
),
KdBrgKendaraan AS (
    SELECT kd_brg, is_sedan_only FROM (VALUES
        (N'3020101001', 1),  -- Sedan
        (N'3020101002', 0),  -- Jeep
        (N'3020101003', 0),  -- Station Wagon
        (N'3020101004', 1),  -- Sedan Listrik
        (N'3020101005', 0),  -- SUV Listrik
        (N'3020101006', 0),  -- MPV Listrik
        (N'3020101009', 0),  -- Mini Bus - Listrik (Penumpang 14 Orang Kebawah)
        (N'3020102003', 0),  -- Mini Bus (Penumpang 14 Orang Kebawah)
        (N'3020102007', 0),  -- Mini Bus - Listrik (Penumpang 14 Orang Kebawah)
        (N'3020105133', 0)   -- Mobil Listrik
    ) AS X(kd_brg, is_sedan_only)
),
KendaraanRaw AS (
    SELECT
        A.kd_satker,
        A.kd_status,
        ISNULL(K.kd_kondisi, '9') AS kd_kondisi,
        ISNULL(K.ur_kondisi, '-') AS ur_kondisi,
        COUNT(*) AS jml
    FROM DJKN.SIMAN2_M_ASET AS A
    INNER JOIN SatkerAktif AS S
            ON S.kd_satker = A.kd_satker
    INNER JOIN KdBrgKendaraan AS KB
            ON KB.kd_brg = A.kd_brg
    LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K
           ON K.kd_kondisi = A.kd_kondisi
    WHERE A.status_bmn_yn = N'Y'
      AND (
            A.kd_status = '01'
         OR (A.kd_status = '02' AND KB.is_sedan_only = 0)
          )
    GROUP BY
        A.kd_satker,
        A.kd_status,
        ISNULL(K.kd_kondisi, '9'),
        ISNULL(K.ur_kondisi, '-')
),
KendaraanStats AS (
    SELECT
        KR.kd_satker,
        SUM(CASE WHEN KR.kd_status = '01' THEN KR.jml ELSE 0 END) AS Jumlah_KDJ,
        STRING_AGG(
            CASE WHEN KR.kd_status = '01'
                 THEN KR.ur_kondisi + N' (' + CAST(KR.jml AS nvarchar(10)) + N')'
            END,
            N', '
        ) WITHIN GROUP (ORDER BY KR.kd_status, KR.kd_kondisi) AS Kondisi_KDJ,
        SUM(CASE WHEN KR.kd_status = '02' THEN KR.jml ELSE 0 END) AS Jumlah_KDO,
        STRING_AGG(
            CASE WHEN KR.kd_status = '02'
                 THEN KR.ur_kondisi + N' (' + CAST(KR.jml AS nvarchar(10)) + N')'
            END,
            N', '
        ) WITHIN GROUP (ORDER BY KR.kd_status, KR.kd_kondisi) AS Kondisi_KDO
    FROM KendaraanRaw AS KR
    GROUP BY KR.kd_satker
)
SELECT
    LEFT(S.kd_satker, 5)          AS Kode_UE1,
    S.kd_satker                   AS Kode_Satker,
    CASE
        WHEN SUBSTRING(S.kd_satker, 16, 3) = N'000' THEN N'INDUK SATKER'
        ELSE N'ANAK SATKER'
    END                            AS Jenis_Satker,
    S.ur_satker                   AS Nama_Satker,
    S.alamat_satker               AS Alamat_Satker,
    S.nm_kel                      AS KelurahanDesa_Satker,
    S.nm_kec                      AS Kecamatan_Satker,
    S.nm_kab_kota                 AS KabKota_Satker,
    S.nm_prov                     AS Provinsi_Satker,
    CAST(NULL AS varchar(200))    AS Status_Gedung_Kantor,
    CAST(NULL AS varchar(900))    AS Foto,
    ISNULL(KS.Jumlah_KDJ, 0)      AS Jumlah_KDJ,
    KS.Kondisi_KDJ                AS Kondisi_KDJ,
    ISNULL(KS.Jumlah_KDO, 0)      AS Jumlah_KDO,
    KS.Kondisi_KDO                AS Kondisi_KDO
FROM SatkerAktif AS S
LEFT JOIN KendaraanStats AS KS
       ON KS.kd_satker = S.kd_satker;
