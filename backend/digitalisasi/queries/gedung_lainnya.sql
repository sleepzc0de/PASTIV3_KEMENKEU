-- Gedung dan bangunan lainnya (kd_brg 40101%, selain 4010101001 = gedung kantor). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
SELECT
    LEFT(A.kd_satker, 5)                    AS Kode_UE1,
    A.kd_satker                             AS Kode_Satker,
    S.ur_satker                             AS Nama_Satker,
    A.kode_register                         AS Kode_Register_bangunan,
    A.kd_brg                                AS Kode_bangunan,
    A.ur_sskel                              AS Uraian_bangunan,
    A.no_aset                               AS NUP_bangunan,
    ISNULL(A.alamat,  S.alamat_satker)      AS Alamat_bangunan,
    A.kd_rtrw                               AS RTRW_bangunan,
    ISNULL(A.ur_kel,  S.nm_kel)             AS KelurahanDesa_bangunan,
    ISNULL(A.ur_kec,  S.nm_kec)             AS Kecamatan_bangunan,
    ISNULL(A.ur_kab,  S.nm_kab_kota)        AS KabKota_bangunan,
    ISNULL(A.ur_prov, S.nm_prov)            AS Provinsi_bangunan,
    A.luas                                  AS Luas_Bangunan,
    F.folder                                AS Foto_bangunan,
    A.merk                                  AS Fungsi,
    SH.ur_status_hukum                      AS Status_Hukum,
    A.nm_unit_pengguna                      AS Nama_Pengguna,
    K.ur_kondisi                            AS Kondisi_Bangunan,
    A.rph_aset                              AS Nilai_Bangunan,
    AR.is_asuransi                          AS Status_Asuransi,
    A.gps_latitude                          AS GPS_Latitude,    -- [PASTI] koordinat untuk peta
    A.gps_longitude                         AS GPS_Longitude    -- [PASTI] koordinat untuk peta
FROM DJKN.SIMAN2_M_ASET AS A
INNER JOIN DJKN.SIMAN2_R_SATKER AS S
        ON S.kd_satker = A.kd_satker
       AND S.kd_satker LIKE N'015%'
       AND S.status_uakpb = N'AKTIF'
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K
       ON K.kd_kondisi = A.kd_kondisi
LEFT JOIN (
        SELECT
            SH.kd_status_hukum,
            STRING_AGG(
                CONVERT(nvarchar(max), SH.ur_status_hukum)
                + N' [' + CONVERT(nvarchar(max), SH.label) + N']', N' | '
            ) WITHIN GROUP (ORDER BY SH.id_status_hukum) AS ur_status_hukum
        FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW AS SH
        WHERE SH.TGL_TARIK = (SELECT MAX(TGL_TARIK) FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW)
        GROUP BY SH.kd_status_hukum
     ) AS SH
       ON SH.kd_status_hukum = A.kd_status_hukum
LEFT JOIN (
        SELECT id_aset, folder
        FROM (
            SELECT
                P.id_aset,
                COALESCE(NULLIF(P.folder, N''), NULLIF(P.filename, N''), NULLIF(P.nm_photo, N'')) AS folder,
                ROW_NUMBER() OVER (
                    PARTITION BY P.id_aset
                    ORDER BY P.photo_utama_yn DESC, P.tanggal DESC, P.id_photo DESC
                ) AS rn
            FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
            WHERE P.status_data = 1
        ) X
        WHERE rn = 1
     ) AS F
       ON F.id_aset = A.id_aset
LEFT JOIN (
        SELECT kode_register, is_asuransi
        FROM (
            SELECT
                P.kode_register,
                P.is_asuransi,
                ROW_NUMBER() OVER (PARTITION BY P.kode_register ORDER BY P.tgl_sk DESC) AS rn
            FROM DJKN.SIMAN2_PENGELOLAAN AS P
            WHERE P.kode_register IS NOT NULL
        ) X
        WHERE rn = 1
     ) AS AR
       ON AR.kode_register = A.kode_register
WHERE A.status_bmn_yn = N'Y'
  AND A.kd_brg LIKE N'40101%'
  AND A.kd_brg <> N'4010101001'
  AND A.kd_satker LIKE N'015%'
OPTION (MAXDOP 4);
