-- Rumah Negara (kd_brg 4010201%, 4010202%, 4010209%; mess 4010202016 dikecualikan). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
WITH StatusHukumMap AS (
    SELECT
        SH.kd_status_hukum,
        STRING_AGG(
            CONVERT(nvarchar(max), SH.ur_status_hukum)
            + N' [' + CONVERT(nvarchar(max), SH.label) + N']',
            N' | '
        ) WITHIN GROUP (ORDER BY SH.id_status_hukum) AS ur_status_hukum
    FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW AS SH
    WHERE SH.TGL_TARIK = (SELECT MAX(TGL_TARIK) FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW)
    GROUP BY SH.kd_status_hukum
),
FotoRanked AS (
    SELECT
        P.id_aset,
        COALESCE(NULLIF(P.folder, N''), NULLIF(P.filename, N''), NULLIF(P.nm_photo, N'')) AS folder,
        ROW_NUMBER() OVER (
            PARTITION BY P.id_aset
            ORDER BY P.photo_utama_yn DESC, P.tanggal DESC, P.id_photo DESC
        ) AS rn
    FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
    WHERE P.status_data = 1
),
PemakaiRanked AS (
    SELECT
        P.id_aset, P.jns_pemakai, P.nm_pmk,
        ROW_NUMBER() OVER (
            PARTITION BY P.id_aset
            ORDER BY P.tgl_mulai DESC, P.id_aset_pemakai DESC
        ) AS rn
    FROM DJKN.SIMAN2_M_ASET_PEMAKAI AS P
)
SELECT
    LEFT(A.kd_satker, 5)                    AS Kode_UE1,
    A.kd_satker                             AS Kode_Satker,
    S.ur_satker                             AS Nama_Satker,
    A.kode_register                         AS Kode_Register_RN,
    A.kd_brg                                AS Kode_RN,
    A.ur_sskel                              AS Uraian_RN,
    A.no_aset                               AS NUP_RN,
    ISNULL(A.alamat,  S.alamat_satker)      AS Alamat_RN,
    A.kd_rtrw                               AS RTRW_RN,
    ISNULL(A.ur_kel,  S.nm_kel)             AS KelurahanDesa_RN,
    ISNULL(A.ur_kec,  S.nm_kec)             AS Kecamatan_RN,
    ISNULL(A.ur_kab,  S.nm_kab_kota)        AS KabKota_RN,
    ISNULL(A.ur_prov, S.nm_prov)            AS Provinsi_RN,
    A.luas                                  AS Luas_RN,
    F.folder                                AS Foto_RN,
    P.jns_pemakai                           AS Status_Penghuni,
    P.nm_pmk                                AS Nama_Penghuni,
    K.ur_kondisi                            AS Kondisi_RN,
    SH.ur_status_hukum                      AS Status_Hukum,
    A.gps_latitude                          AS GPS_Latitude,    -- [PASTI] koordinat untuk peta
    A.gps_longitude                         AS GPS_Longitude    -- [PASTI] koordinat untuk peta
FROM DJKN.SIMAN2_M_ASET AS A
INNER JOIN DJKN.SIMAN2_R_SATKER AS S
        ON S.kd_satker = A.kd_satker
       AND S.status_uakpb = N'AKTIF'
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K
       ON K.kd_kondisi = A.kd_kondisi
LEFT JOIN StatusHukumMap AS SH
       ON SH.kd_status_hukum = A.kd_status_hukum
LEFT JOIN FotoRanked AS F
       ON F.id_aset = A.id_aset AND F.rn = 1
LEFT JOIN PemakaiRanked AS P
       ON P.id_aset = A.id_aset AND P.rn = 1
WHERE A.status_bmn_yn = N'Y'
  AND A.kd_satker LIKE N'015%'          -- filter di M_ASET langsung, bukan hanya di satker
  AND (
       A.kd_brg LIKE N'4010201%'
    OR A.kd_brg LIKE N'4010202%'
    OR A.kd_brg LIKE N'4010209%'
  )
  AND A.kd_brg <> N'4010202016';
