-- Gedung kantor utama (kd_brg 4010101001). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
SELECT
    LEFT(A.kd_satker, 5)                AS Kode_UE1,
    A.kd_satker                         AS Kode_Satker,
    S.ur_satker                         AS Nama_Satker,
    A.kode_register                     AS Kode_Register_bangunan,
    A.kd_brg                            AS Kode_bangunan,
    A.ur_sskel                          AS Uraian_bangunan,
    A.no_aset                           AS NUP_bangunan,
    ISNULL(A.alamat,  S.alamat_satker)  AS Alamat_bangunan,
    A.kd_rtrw                           AS RTRW_bangunan,
    ISNULL(A.ur_kel,  S.nm_kel)         AS KelurahanDesa_bangunan,
    ISNULL(A.ur_kec,  S.nm_kec)         AS Kecamatan_bangunan,
    ISNULL(A.ur_kab,  S.nm_kab_kota)    AS KabKota_bangunan,
    ISNULL(A.ur_prov, S.nm_prov)        AS Provinsi_bangunan,
    A.luas                              AS Luas_Bangunan,
    F.folder                            AS Foto_bangunan,
    K.ur_kondisi                        AS Kondisi_Bangunan,
    A.rph_aset                          AS Nilai_Bangunan,
    AR.is_asuransi                      AS Status_Asuransi,
    T.kd_brg                            AS Kode_tanah,
    T.ur_sskel                          AS Uraian_tanah,
    T.no_aset                           AS No_aset_tanah,
    A.gps_latitude                      AS GPS_Latitude,    -- [PASTI] koordinat untuk peta
    A.gps_longitude                     AS GPS_Longitude    -- [PASTI] koordinat untuk peta
FROM DJKN.SIMAN2_M_ASET AS A
INNER JOIN DJKN.SIMAN2_R_SATKER AS S
        ON S.kd_satker = A.kd_satker
       AND S.status_uakpb = N'AKTIF'
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K
       ON K.kd_kondisi = A.kd_kondisi
OUTER APPLY (
    SELECT TOP (1)
        COALESCE(NULLIF(P.folder, N''), NULLIF(P.filename, N''), NULLIF(P.nm_photo, N'')) AS folder
    FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
    WHERE P.id_aset = A.id_aset
      AND P.status_data = 1
    ORDER BY P.photo_utama_yn DESC, P.tanggal DESC, P.id_photo DESC
) AS F
OUTER APPLY (
    SELECT TOP (1) G.is_asuransi
    FROM DJKN.SIMAN2_PENGELOLAAN AS G
    WHERE G.kode_register = A.kode_register
    ORDER BY G.tgl_sk DESC
) AS AR
OUTER APPLY (
    SELECT TOP (1) AT.kd_brg, AT.ur_sskel, AT.no_aset
    FROM DJKN.SIMAN2_M_ASET_TANAH_BANGUNAN AS TB
    INNER JOIN DJKN.SIMAN2_M_ASET AS AT
            ON AT.id_aset = TB.id_aset_tanah
    WHERE TB.id_aset_bangunan = A.id_aset
      AND TB.id_aset_tanah IS NOT NULL
      AND TB.status_data = 1
      AND AT.status_bmn_yn = N'Y'
      AND AT.kd_brg LIKE N'2%'
    ORDER BY TB.id_aset_tanah
) AS T
WHERE A.kd_brg = N'4010101001'
  AND A.status_bmn_yn = N'Y'
  AND A.kd_satker LIKE N'015%'
  AND A.kode_register IS NOT NULL;   -- hapus baris ini jika ada bangunan tanpa kode_register
