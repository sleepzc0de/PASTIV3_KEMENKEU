-- Mess Rumah Negara (kd_brg 4010202016). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
SET NOCOUNT ON;

/* 1) Satu kali baca M_ASET (semua snapshot untuk kd_brg ini) */
DROP TABLE IF EXISTS #AsetAll, #AsetTarget;

SELECT A.id_aset, A.kd_satker, A.kd_brg, A.ur_sskel, A.no_aset, A.alamat, A.kd_rtrw,
       A.ur_kel, A.ur_kec, A.ur_kab, A.ur_prov, A.luas, A.kd_kondisi, A.rph_aset,
       A.kd_status_hukum, A.tgl_tarik,
       A.gps_latitude, A.gps_longitude   -- [PASTI] koordinat untuk peta
INTO #AsetAll
FROM DJKN.SIMAN2_M_ASET AS A
WHERE A.kd_brg = N'4010202016'
  AND A.status_bmn_yn = N'Y'
  AND A.kd_jns_bmn = 3
  AND A.kd_satker LIKE N'015%'
OPTION (RECOMPILE);

/* 2) Ambil snapshot terbaru + satker aktif, lalu index */
SELECT A.id_aset, A.kd_satker, S.ur_satker,
       S.alamat_satker, S.nm_kel, S.nm_kec, S.nm_kab_kota, S.nm_prov,
       A.kd_brg, A.ur_sskel, A.no_aset, A.alamat, A.kd_rtrw,
       A.ur_kel, A.ur_kec, A.ur_kab, A.ur_prov,
       A.luas, A.kd_kondisi, A.rph_aset, A.kd_status_hukum,
       A.gps_latitude, A.gps_longitude   -- [PASTI] koordinat untuk peta
INTO #AsetTarget
FROM #AsetAll AS A
INNER JOIN DJKN.SIMAN2_R_SATKER AS S
        ON S.kd_satker = A.kd_satker
       AND S.status_uakpb = N'AKTIF'
WHERE A.tgl_tarik = (SELECT MAX(tgl_tarik) FROM #AsetAll);

CREATE CLUSTERED INDEX CX_AT ON #AsetTarget (kd_satker, kd_brg, no_aset);
CREATE INDEX IX_AT_id ON #AsetTarget (id_aset);

/* 3) Query utama */
WITH InventarisRanked AS (
    SELECT I.id_aset, I.alamat, I.kd_rtrw, I.ur_kel, I.ur_kec, I.ur_kab, I.ur_prov, I.luas_bdg,
           ROW_NUMBER() OVER (PARTITION BY I.id_aset
                              ORDER BY I.TGL_TARIK DESC, I.updated_at DESC, I.id_inv_bangunan DESC) AS rn
    FROM DJKN.SIMAN2_T_INV_BANGUNAN AS I
    WHERE I.id_aset IN (SELECT id_aset FROM #AsetTarget)
), FotoRanked AS (
    SELECT P.id_aset,
           COALESCE(NULLIF(LTRIM(RTRIM(P.folder)), N''), NULLIF(LTRIM(RTRIM(P.filename)), N''),
                    NULLIF(LTRIM(RTRIM(P.nm_photo)), N'')) AS foto,
           ROW_NUMBER() OVER (PARTITION BY P.id_aset
                              ORDER BY CASE WHEN P.photo_utama_yn = N'Y' THEN 0 ELSE 1 END,
                                       P.TGL_TARIK DESC, P.tanggal DESC, P.id_photo DESC) AS rn
    FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
    WHERE P.status_data = 1
      AND P.id_aset IN (SELECT id_aset FROM #AsetTarget)
), StatusHukumMap AS (
    SELECT LTRIM(RTRIM(R.kd_status_hukum)) AS kd_status_hukum,
           STRING_AGG(CONVERT(nvarchar(max), R.ur_status_hukum)
                      + CASE WHEN NULLIF(LTRIM(RTRIM(R.label)), N'') IS NOT NULL
                             THEN N' [' + R.label + N']' ELSE N'' END, N' | ')
                      WITHIN GROUP (ORDER BY R.id_status_hukum) AS ur_status_hukum
    FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW AS R
    WHERE R.TGL_TARIK = (SELECT MAX(TGL_TARIK) FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW)
    GROUP BY LTRIM(RTRIM(R.kd_status_hukum))
), HukumRanked AS (
    SELECT H.id_aset, NULLIF(LTRIM(RTRIM(H.kd_status_hukum)), N'') AS kd_status_hukum,
           ROW_NUMBER() OVER (PARTITION BY H.id_aset
                              ORDER BY CASE WHEN H.terakhir_yn = N'Y' THEN 0 ELSE 1 END,
                                       H.TGL_TARIK DESC, H.updated_at DESC, H.id_aset_hukum DESC) AS rn
    FROM DJKN.SIMAN2_M_ASET_HUKUM AS H
    WHERE H.status_data = 1
      AND H.id_aset IN (SELECT id_aset FROM #AsetTarget)
), RuangAgg AS (
    SELECT R.KODE_SATKER, R.KODE_BARANG, R.NUP,
           SUM(CASE WHEN UPPER(LTRIM(RTRIM(R.tipe_ruang))) = N'RUANG TIDUR'
                     AND UPPER(LTRIM(RTRIM(R.kode_ruang))) LIKE N'E%' THEN 1 ELSE 0 END) AS kamar_tipe_e,
           SUM(CASE WHEN UPPER(LTRIM(RTRIM(R.tipe_ruang))) = N'RUANG TIDUR'
                     AND UPPER(LTRIM(RTRIM(R.kode_ruang))) LIKE N'D%' THEN 1 ELSE 0 END) AS kamar_tipe_d
    FROM DJKN.VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015 AS R
    INNER JOIN #AsetTarget AS A
            ON A.kd_satker = R.KODE_SATKER AND A.kd_brg = R.KODE_BARANG AND A.no_aset = R.NUP
    GROUP BY R.KODE_SATKER, R.KODE_BARANG, R.NUP
)
SELECT
    LEFT(A.kd_satker, 5) AS Kode_UE1,
    A.kd_satker          AS Kode_Satker,
    A.ur_satker          AS Nama_Satker,
    A.kd_brg             AS Kode_mess,
    A.ur_sskel           AS Uraian_mess,
    A.no_aset            AS NUP_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.alamat)), N''), NULLIF(LTRIM(RTRIM(A.alamat)), N''), A.alamat_satker) AS Alamat_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.kd_rtrw)), N''), NULLIF(LTRIM(RTRIM(A.kd_rtrw)), N'')) AS RTRW_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kel)), N''), NULLIF(LTRIM(RTRIM(A.ur_kel)), N''), A.nm_kel) AS KelurahanDesa_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kec)), N''), NULLIF(LTRIM(RTRIM(A.ur_kec)), N''), A.nm_kec) AS Kecamatan_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kab)), N''), NULLIF(LTRIM(RTRIM(A.ur_kab)), N''), A.nm_kab_kota) AS KabKota_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_prov)), N''), NULLIF(LTRIM(RTRIM(A.ur_prov)), N''), A.nm_prov) AS Provinsi_mess,
    COALESCE(NULLIF(I.luas_bdg, 0), NULLIF(A.luas, 0), A.luas) AS Luas_mess,
    F.foto               AS Foto_mess,
    K.ur_kondisi         AS Kondisi_mess,
    A.rph_aset           AS Nilai_mess,
    SH.ur_status_hukum   AS Status_Hukum,
    COALESCE(R.kamar_tipe_e, 0) AS Kamar_tipe_E,
    COALESCE(R.kamar_tipe_d, 0) AS Kamar_tipe_D,
    A.gps_latitude       AS GPS_Latitude,    -- [PASTI] koordinat untuk peta
    A.gps_longitude      AS GPS_Longitude    -- [PASTI] koordinat untuk peta
FROM #AsetTarget AS A
LEFT JOIN InventarisRanked AS I  ON I.id_aset = A.id_aset AND I.rn = 1
LEFT JOIN FotoRanked       AS F  ON F.id_aset = A.id_aset AND F.rn = 1
LEFT JOIN HukumRanked      AS H  ON H.id_aset = A.id_aset AND H.rn = 1
LEFT JOIN StatusHukumMap   AS SH ON SH.kd_status_hukum = COALESCE(H.kd_status_hukum, NULLIF(LTRIM(RTRIM(A.kd_status_hukum)), N''))
LEFT JOIN RuangAgg         AS R  ON R.KODE_SATKER = A.kd_satker AND R.KODE_BARANG = A.kd_brg AND R.NUP = A.no_aset
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K ON K.kd_kondisi = A.kd_kondisi
ORDER BY A.kd_satker, A.kd_brg, A.no_aset
OPTION (RECOMPILE);
