package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

type sldkColumn struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	IsSearchable bool   `json:"is_searchable"`
}

var (
	schemaCache      []sldkColumn
	schemaCacheAt    time.Time
	schemaCacheMutex sync.Mutex
	schemaCacheTTL   = 10 * time.Minute
)

func getAssetTableSchema() ([]sldkColumn, error) {
	schemaCacheMutex.Lock()
	defer schemaCacheMutex.Unlock()

	if schemaCache != nil && time.Since(schemaCacheAt) < schemaCacheTTL {
		return schemaCache, nil
	}

	tableRef := config.Cfg.SLDKAssetTable
	if tableRef == "" {
		return nil, fmt.Errorf("SLDK_ASSET_TABLE belum dikonfigurasi di .env")
	}

	schemaName, tableName := splitSchemaTable(tableRef)

	rows, err := database.SLDKDB.Query(`
		SELECT COLUMN_NAME, DATA_TYPE
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = @p1 AND TABLE_NAME = @p2
		ORDER BY ORDINAL_POSITION`,
		schemaName, tableName,
	)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca skema tabel: %w", err)
	}
	defer rows.Close()

	// Kolom yang boleh di-search dibatasi ke daftar konfigurasi
	// SLDK_ASSET_SEARCH_COLUMNS (kalau diisi), supaya query pencarian
	// tetap cepat meski tabel punya banyak kolom teks (~140 kolom).
	allowedSearchCols := make(map[string]bool)
	for _, c := range config.Cfg.SLDKAssetSearchCols {
		allowedSearchCols[strings.ToLower(c)] = true
	}
	useWhitelist := len(allowedSearchCols) > 0

	textTypes := map[string]bool{
		"nvarchar": true, "varchar": true, "nchar": true, "char": true, "text": true, "ntext": true,
	}

	var columns []sldkColumn
	for rows.Next() {
		var name, dataType string
		if err := rows.Scan(&name, &dataType); err != nil {
			continue
		}

		isText := textTypes[strings.ToLower(dataType)]
		isSearchable := isText
		if useWhitelist {
			isSearchable = isText && allowedSearchCols[strings.ToLower(name)]
		}

		columns = append(columns, sldkColumn{
			Name:         name,
			DataType:     dataType,
			IsSearchable: isSearchable,
		})
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("tabel '%s' tidak ditemukan atau tidak punya kolom (cek nama tabel di SLDK_ASSET_TABLE)", tableRef)
	}

	schemaCache = columns
	schemaCacheAt = time.Now()
	return columns, nil
}

func splitSchemaTable(ref string) (schema, table string) {
	parts := strings.SplitN(ref, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "dbo", parts[0]
}

func GetAssetColumns(c *gin.Context) {
	if database.SLDKDB == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Koneksi ke SLDK sedang tidak tersedia")
		return
	}

	columns, err := getAssetTableSchema()
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil skema kolom", gin.H{
		"columns": columns,
		"table":   config.Cfg.SLDKAssetTable,
	})
}

func SearchAssets(c *gin.Context) {
	if database.SLDKDB == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Koneksi ke SLDK sedang tidak tersedia")
		return
	}

	query := strings.TrimSpace(c.Query("q"))
	limitStr := c.DefaultQuery("limit", "50")

	columns, err := getAssetTableSchema()
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	var searchableCols []string
	var allColNames []string
	for _, col := range columns {
		allColNames = append(allColNames, quoteIdent(col.Name))
		if col.IsSearchable {
			searchableCols = append(searchableCols, col.Name)
		}
	}

	tableRef := quoteTableRef(config.Cfg.SLDKAssetTable)
	selectClause := strings.Join(allColNames, ", ")

	// Urutkan hasil dari yang terbaru direkam, supaya listing default
	// (tanpa kata kunci) tetap informatif dan konsisten.
	orderClause := ""
	if hasColumn(columns, "tgl_rekam") {
		orderClause = "ORDER BY tgl_rekam DESC"
	} else if hasColumn(columns, "id_aset") {
		orderClause = "ORDER BY id_aset DESC"
	}

	var sqlQuery string
	var args []interface{}

	if query == "" {
		sqlQuery = fmt.Sprintf(
			"SELECT TOP (%s) %s FROM %s %s",
			sanitizeLimit(limitStr), selectClause, tableRef, orderClause,
		)
	} else {
		if len(searchableCols) == 0 {
			utils.ErrorResponse(c, http.StatusBadRequest, "Belum ada kolom yang dikonfigurasi untuk pencarian (cek SLDK_ASSET_SEARCH_COLUMNS di .env)")
			return
		}

		var whereParts []string
		for i, colName := range searchableCols {
			whereParts = append(whereParts, fmt.Sprintf("%s LIKE @p%d", quoteIdent(colName), i+1))
			args = append(args, "%"+query+"%")
		}

		sqlQuery = fmt.Sprintf(
			"SELECT TOP (%s) %s FROM %s WHERE %s %s",
			sanitizeLimit(limitStr), selectClause, tableRef, strings.Join(whereParts, " OR "), orderClause,
		)
	}

	rows, err := database.SLDKDB.Query(sqlQuery, args...)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menjalankan pencarian: "+err.Error())
		return
	}
	defer rows.Close()

	results, err := rowsToMaps(rows)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memproses hasil: "+err.Error())
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Pencarian berhasil", gin.H{
		"columns":         columns,
		"searched_fields": searchableCols,
		"results":         results,
		"count":           len(results),
	})
}

func hasColumn(columns []sldkColumn, name string) bool {
	for _, c := range columns {
		if strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

func rowsToMaps(rows *sql.Rows) ([]map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			switch v := val.(type) {
			case []byte:
				rowMap[col] = string(v)
			default:
				rowMap[col] = v
			}
		}
		results = append(results, rowMap)
	}

	return results, nil
}

func quoteIdent(name string) string {
	return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
}

func quoteTableRef(ref string) string {
	schema, table := splitSchemaTable(ref)
	return quoteIdent(schema) + "." + quoteIdent(table)
}

func sanitizeLimit(limitStr string) string {
	limit := 50
	fmt.Sscanf(limitStr, "%d", &limit)
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	return fmt.Sprintf("%d", limit)
}
