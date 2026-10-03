-- Tanah (kd_brg 2%). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
SET NOCOUNT ON;

-- 1) Hitung TGL_TARIK terbaru SEKALI SAJA, bukan sebagai correlated subquery
DECLARE @MaxTglTarik int =
    (SELECT MAX(TGL_TARIK) FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW);

-- 2) Materialisasi mapping status hukum (tabel kecil, cepat)
SELECT
    SH.kd_status_hukum,
    STRING_AGG(
        CONVERT(nvarchar(max), SH.ur_status_hukum)
        + N' [' + CONVERT(nvarchar(max), SH.label) + N']',
        N' | '
    ) WITHIN GROUP (ORDER BY SH.id_status_hukum) AS ur_status_hukum
INTO #StatusHukumMap
FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW AS SH
WHERE SH.TGL_TARIK = @MaxTglTarik
GROUP BY SH.kd_status_hukum;

CREATE UNIQUE CLUSTERED INDEX IX_temp_SH ON #StatusHukumMap (kd_status_hukum);

-- 3) Materialisasi AsetTarget SEKALI SAJA — ini kunci utamanya
SELECT
    A.id_aset,
    A.kode_register,
    A.kd_satker,
    S.ur_satker,
    A.kd_brg,
    A.ur_sskel,
    A.no_aset,
    ISNULL(A.alamat, S.alamat_satker) AS alamat,
    A.kd_rtrw,
    ISNULL(A.ur_kel, S.nm_kel) AS ur_kel,
    ISNULL(A.ur_kec, S.nm_kec) AS ur_kec,
    ISNULL(A.ur_kab, S.nm_kab_kota) AS ur_kab,
    ISNULL(A.ur_prov, S.nm_prov) AS ur_prov,
    A.luas,
    A.luas_tnhb,
    SH.ur_status_hukum,
    K.ur_kondisi,
    A.rph_aset,
    A.gps_latitude,    -- [PASTI] koordinat untuk peta
    A.gps_longitude    -- [PASTI] koordinat untuk peta
INTO #AsetTarget
FROM DJKN.SIMAN2_M_ASET AS A
INNER JOIN DJKN.SIMAN2_R_SATKER AS S
        ON S.kd_satker = A.kd_satker
LEFT JOIN #StatusHukumMap AS SH
       ON SH.kd_status_hukum = A.kd_status_hukum
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K
       ON K.kd_kondisi = A.kd_kondisi
WHERE A.status_bmn_yn = N'Y'
  AND A.kd_brg LIKE N'2%'
  AND A.kd_satker LIKE N'015%'
  AND S.status_uakpb = N'AKTIF';

-- Ini index buatan sendiri di tempdb — TIDAK butuh izin apa pun di tabel asli
CREATE UNIQUE CLUSTERED INDEX IX_temp_Aset ON #AsetTarget (id_aset);

-- 4) FotoRanked — sekarang join ke tabel kecil (#AsetTarget), bukan ke keseluruhan AsetTarget yang di-inline ulang
SELECT
    P.id_aset,
    COALESCE(
        NULLIF(P.folder, N''),
        NULLIF(P.filename, N''),
        NULLIF(P.nm_photo, N'')
    ) AS folder,
    ROW_NUMBER() OVER (
        PARTITION BY P.id_aset
        ORDER BY P.photo_utama_yn DESC, P.tanggal DESC, P.id_photo DESC
    ) AS rn
INTO #FotoRanked
FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
INNER JOIN #AsetTarget AS T
        ON T.id_aset = P.id_aset
WHERE P.status_data = 1;

CREATE CLUSTERED INDEX IX_temp_Foto ON #FotoRanked (id_aset, rn);

-- 5) BangunanAgg
SELECT
    TB.id_aset_tanah,
    COUNT(*) AS jumlah
INTO #BangunanAgg
FROM DJKN.SIMAN2_M_ASET_TANAH_BANGUNAN AS TB
INNER JOIN #AsetTarget AS T
        ON T.id_aset = TB.id_aset_tanah
INNER JOIN DJKN.SIMAN2_M_ASET AS B
        ON B.id_aset = TB.id_aset_bangunan
       AND B.status_bmn_yn = N'Y'
GROUP BY TB.id_aset_tanah;

CREATE UNIQUE CLUSTERED INDEX IX_temp_Bgn ON #BangunanAgg (id_aset_tanah);

SELECT
    T.id_aset                         AS id_aset_tanah,
    T.kode_register                   AS kode_register_tanah,
    LEFT(T.kd_satker, 5)              AS Kode_UE1,
    T.kd_satker                       AS Kode_Satker,
    T.ur_satker                       AS Nama_Satker,
    T.kd_brg                          AS Kode_tanah,
    T.ur_sskel                        AS Uraian_tanah,
    T.no_aset                         AS No_aset,
    T.alamat                          AS Alamat_tanah,
    T.kd_rtrw                         AS RTRW_tanah,
    T.ur_kel                          AS KelurahanDesa_tanah,
    T.ur_kec                          AS Kecamatan_tanah,
    T.ur_kab                          AS KabKota_tanah,
    T.ur_prov                         AS Provinsi_tanah,
    T.luas                            AS Luas_Tanah,
    T.luas_tnhb                       AS Luas_Bangunan,
    ISNULL(BG.jumlah, 0)              AS Jumlah_Bangunan,
    T.ur_status_hukum                 AS Status_hukum,
    F.folder                          AS Foto,
    T.ur_kondisi                      AS Kondisi_Tanah,
    T.rph_aset                        AS Nilai_Tanah,
    T.gps_latitude                    AS GPS_Latitude,    -- [PASTI] koordinat untuk peta
    T.gps_longitude                   AS GPS_Longitude    -- [PASTI] koordinat untuk peta
FROM #AsetTarget AS T
LEFT JOIN #FotoRanked AS F
       ON F.id_aset = T.id_aset
      AND F.rn = 1
LEFT JOIN #BangunanAgg AS BG
       ON BG.id_aset_tanah = T.id_aset;

DROP TABLE #StatusHukumMap, #AsetTarget, #FotoRanked, #BangunanAgg;
