//go:build windows

package handlers

// Mengaktifkan autentikasi Windows (SSPI) untuk tes integrasi SQL Server (PASTI_UJI_MSSQL_DSN dengan authenticator=winsspi).
import _ "github.com/microsoft/go-mssqldb/integratedauth/winsspi"
