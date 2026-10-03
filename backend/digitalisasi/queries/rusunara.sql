-- Rusunara (kd_brg 4010208%). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
WITH SatkerScope AS (
    SELECT S.kd_satker, S.ur_satker, S.alamat_satker, S.nm_kel, S.nm_kec, S.nm_kab_kota, S.nm_prov
    FROM DJKN.SIMAN2_R_SATKER AS S
    WHERE S.kd_satker LIKE N'015%'
      AND S.status_uakpb = N'AKTIF'
),
AsetFiltered AS (
    SELECT
        A.id_aset, A.kd_satker, A.kd_brg, A.ur_sskel, A.no_aset, A.alamat, A.kd_rtrw,
        A.ur_kel, A.ur_kec, A.ur_kab, A.ur_prov, A.luas, A.kd_kondisi, A.rph_aset,
        A.gps_latitude, A.gps_longitude,   -- [PASTI] koordinat untuk peta
        A.TGL_TARIK,
        MAX(A.TGL_TARIK) OVER () AS max_tgl_tarik
    FROM DJKN.SIMAN2_M_ASET AS A
    WHERE A.status_bmn_yn = N'Y'
      AND A.kd_brg LIKE N'4010208%'
      AND A.kd_satker LIKE N'015%'
),
AsetTarget AS (
    SELECT
        F.id_aset, F.kd_satker, S.ur_satker,
        S.alamat_satker, S.nm_kel, S.nm_kec, S.nm_kab_kota, S.nm_prov,
        F.kd_brg, F.ur_sskel, F.no_aset, F.alamat, F.kd_rtrw,
        F.ur_kel, F.ur_kec, F.ur_kab, F.ur_prov,
        F.luas, F.kd_kondisi, F.rph_aset,
        F.gps_latitude, F.gps_longitude    -- [PASTI] koordinat untuk peta
    FROM AsetFiltered AS F
    INNER JOIN SatkerScope AS S ON S.kd_satker = F.kd_satker
    WHERE F.TGL_TARIK = F.max_tgl_tarik
),
InventarisRanked AS (
    SELECT
           I.id_aset, I.alamat, I.kd_rtrw, I.ur_kel, I.ur_kec,
           I.ur_kab, I.ur_prov, I.luas_bdg,
           ROW_NUMBER() OVER (PARTITION BY I.id_aset ORDER BY I.TGL_TARIK DESC, I.updated_at DESC, I.id_inv_bangunan DESC) AS rn
    FROM DJKN.SIMAN2_T_INV_BANGUNAN AS I
    INNER JOIN AsetTarget AS A ON A.id_aset = I.id_aset
),
FotoRanked AS (
    SELECT
        P.id_aset,
        COALESCE(NULLIF(LTRIM(RTRIM(P.folder)), N''), NULLIF(LTRIM(RTRIM(P.filename)), N''), NULLIF(LTRIM(RTRIM(P.nm_photo)), N'')) AS foto,
        ROW_NUMBER() OVER (
            PARTITION BY P.id_aset
            ORDER BY CASE WHEN P.photo_utama_yn = N'Y' THEN 0 ELSE 1 END, P.TGL_TARIK DESC, P.tanggal DESC, P.id_photo DESC
        ) AS rn
    FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
    INNER JOIN AsetTarget AS A ON A.id_aset = P.id_aset
    WHERE P.status_data = 1
),
RuangAgg AS (
    SELECT
        R.KODE_SATKER, R.KODE_BARANG, R.NUP,
        SUM(CASE WHEN UPPER(LTRIM(RTRIM(R.tipe_ruang))) = N'RUANG TIDUR'
                      AND UPPER(LTRIM(RTRIM(R.kode_ruang))) LIKE N'E%' THEN 1 ELSE 0 END) AS kamar_tipe_e,
        SUM(CASE WHEN UPPER(LTRIM(RTRIM(R.tipe_ruang))) = N'RUANG TIDUR'
                      AND UPPER(LTRIM(RTRIM(R.kode_ruang))) LIKE N'D%' THEN 1 ELSE 0 END) AS kamar_tipe_d,
        SUM(CASE WHEN UPPER(LTRIM(RTRIM(R.tipe_ruang))) = N'RUANG TIDUR'
                      AND UPPER(LTRIM(RTRIM(R.kode_ruang))) LIKE N'C%' THEN 1 ELSE 0 END) AS kamar_tipe_c
    FROM DJKN.VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015 AS R
    INNER JOIN AsetTarget AS A
            ON A.kd_satker = R.KODE_SATKER
           AND A.kd_brg = R.KODE_BARANG
           AND A.no_aset = R.NUP
    GROUP BY R.KODE_SATKER, R.KODE_BARANG, R.NUP
)
SELECT
    LEFT(A.kd_satker, 5) AS Kode_UE1,
    A.kd_satker AS Kode_Satker,
    A.ur_satker AS Nama_Satker,
    A.kd_brg AS Kode_Rusun,
    A.ur_sskel AS Uraian_Rusun,
    A.no_aset AS NUP_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.alamat)), N''), NULLIF(LTRIM(RTRIM(A.alamat)), N''), A.alamat_satker) AS Alamat_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.kd_rtrw)), N''), NULLIF(LTRIM(RTRIM(A.kd_rtrw)), N'')) AS RTRW_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kel)), N''), NULLIF(LTRIM(RTRIM(A.ur_kel)), N''), A.nm_kel) AS KelurahanDesa_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kec)), N''), NULLIF(LTRIM(RTRIM(A.ur_kec)), N''), A.nm_kec) AS Kecamatan_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kab)), N''), NULLIF(LTRIM(RTRIM(A.ur_kab)), N''), A.nm_kab_kota) AS KabKota_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_prov)), N''), NULLIF(LTRIM(RTRIM(A.ur_prov)), N''), A.nm_prov) AS Provinsi_Rusun,
    COALESCE(NULLIF(I.luas_bdg, 0), NULLIF(A.luas, 0), A.luas) AS Luas_Rusun,
    F.foto AS Foto_Rusun,
    K.ur_kondisi AS Kondisi_Rusun,
    A.rph_aset AS Nilai_Rusun,
    COALESCE(R.kamar_tipe_e, 0) AS Kamar_tipe_E,
    COALESCE(R.kamar_tipe_d, 0) AS Kamar_tipe_D,
    COALESCE(R.kamar_tipe_c, 0) AS Kamar_tipe_C,
    A.gps_latitude AS GPS_Latitude,        -- [PASTI] koordinat untuk peta
    A.gps_longitude AS GPS_Longitude       -- [PASTI] koordinat untuk peta
FROM AsetTarget AS A
LEFT JOIN InventarisRanked AS I ON I.id_aset = A.id_aset AND I.rn = 1
LEFT JOIN FotoRanked AS F ON F.id_aset = A.id_aset AND F.rn = 1
LEFT JOIN RuangAgg AS R
       ON R.KODE_SATKER = A.kd_satker
      AND R.KODE_BARANG = A.kd_brg
      AND R.NUP = A.no_aset
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K ON K.kd_kondisi = A.kd_kondisi
ORDER BY A.kd_satker, A.no_aset;
