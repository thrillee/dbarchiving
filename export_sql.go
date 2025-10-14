package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
)

func exportTableToSQL(db *sql.DB, tableName string, config *Config, logger *Logger) error {
	// Create export directory if it doesn't exist
	if err := os.MkdirAll(config.ExportPath, 0o755); err != nil {
		return fmt.Errorf("failed to create export directory: %v", err)
	}

	// Generate filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s/%s_%s.sql", config.ExportPath, tableName, timestamp)

	// Create the SQL file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create SQL file: %v", err)
	}
	defer file.Close()

	logger.Info("Exporting to file: %s", filename)

	// Write SQL file header
	header := fmt.Sprintf(`-- MySQL dump of table %s
-- Host: %s    Database: %s
-- Generated: %s
-- ------------------------------------------------------

SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0;
SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0;
SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO';
SET @OLD_TIME_ZONE=@@TIME_ZONE, TIME_ZONE='+00:00';

--
-- Table structure for table %s
--

DROP TABLE IF EXISTS `+"`%s`"+`;

`,
		tableName,
		config.Host,
		config.Database,
		time.Now().Format("2006-01-02 15:04:05"),
		tableName,
		tableName,
	)

	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	// Get CREATE TABLE statement
	createStmt, err := getCreateTable(db, tableName)
	if err != nil {
		return fmt.Errorf("failed to get CREATE TABLE: %v", err)
	}

	if _, err := file.WriteString(createStmt + ";\n\n"); err != nil {
		return fmt.Errorf("failed to write CREATE TABLE: %v", err)
	}

	// Lock table for consistent read
	lockSQL := fmt.Sprintf("LOCK TABLES `%s` READ", tableName)
	if _, err := db.Exec(lockSQL); err != nil {
		return fmt.Errorf("failed to lock table: %v", err)
	}
	defer db.Exec("UNLOCK TABLES")

	// Get column names
	columns, err := getColumnNames(db, tableName)
	if err != nil {
		return fmt.Errorf("failed to get column names: %v", err)
	}

	// Write data header
	dataHeader := fmt.Sprintf("--\n-- Dumping data for table `%s`\n--\n\nLOCK TABLES `%s` WRITE;\n", tableName, tableName)
	if _, err := file.WriteString(dataHeader); err != nil {
		return fmt.Errorf("failed to write data header: %v", err)
	}

	// Export data in batches
	batchSize := 1000
	offset := 0
	totalRows := 0

	for {
		rows, err := db.Query(fmt.Sprintf("SELECT * FROM `%s` LIMIT %d OFFSET %d", tableName, batchSize, offset))
		if err != nil {
			return fmt.Errorf("failed to query data: %v", err)
		}

		// Get column types
		columnTypes, err := rows.ColumnTypes()
		if err != nil {
			rows.Close()
			return fmt.Errorf("failed to get column types: %v", err)
		}

		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		rowCount := 0
		var insertStatements []string

		for rows.Next() {
			if err := rows.Scan(valuePtrs...); err != nil {
				rows.Close()
				return fmt.Errorf("failed to scan row: %v", err)
			}

			// Build INSERT statement
			valueStrings := make([]string, len(values))
			for i, val := range values {
				valueStrings[i] = formatSQLValue(val, columnTypes[i])
			}

			insertStatements = append(insertStatements, fmt.Sprintf("(%s)", strings.Join(valueStrings, ",")))
			rowCount++
			totalRows++

			// Write in batches of 100 rows per INSERT statement
			if len(insertStatements) >= 100 {
				insertSQL := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES\n%s;\n",
					tableName,
					strings.Join(columns, ","),
					strings.Join(insertStatements, ",\n"))

				if _, err := file.WriteString(insertSQL); err != nil {
					rows.Close()
					return fmt.Errorf("failed to write INSERT: %v", err)
				}
				insertStatements = nil
			}
		}

		rows.Close()

		// Write remaining INSERT statements
		if len(insertStatements) > 0 {
			insertSQL := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES\n%s;\n",
				tableName,
				strings.Join(columns, ","),
				strings.Join(insertStatements, ",\n"))

			if _, err := file.WriteString(insertSQL); err != nil {
				return fmt.Errorf("failed to write INSERT: %v", err)
			}
		}

		// If we got fewer rows than batch size, we're done
		if rowCount < batchSize {
			break
		}

		offset += batchSize
		logger.Info("Exported %d rows...", totalRows)
	}

	// Write footer
	footer := fmt.Sprintf(`UNLOCK TABLES;

--
-- Dump completed on %s
-- Total rows exported: %d
--

SET SQL_MODE=@OLD_SQL_MODE;
SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS;
SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS;
SET TIME_ZONE=@OLD_TIME_ZONE;
`,
		time.Now().Format("2006-01-02 15:04:05"),
		totalRows,
	)

	if _, err := file.WriteString(footer); err != nil {
		return fmt.Errorf("failed to write footer: %v", err)
	}

	logger.Info("Successfully exported %d rows to %s", totalRows, filename)
	return nil
}

func getColumnNames(db *sql.DB, tableName string) ([]string, error) {
	query := fmt.Sprintf("SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '%s' ORDER BY ORDINAL_POSITION", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, err
		}
		columns = append(columns, fmt.Sprintf("`%s`", columnName))
	}

	return columns, nil
}

func formatSQLValue(val interface{}, colType *sql.ColumnType) string {
	if val == nil {
		return "NULL"
	}

	switch v := val.(type) {
	case []byte:
		// Handle binary data and strings
		str := string(v)
		// Escape special characters
		str = strings.ReplaceAll(str, "\\", "\\\\")
		str = strings.ReplaceAll(str, "'", "\\'")
		str = strings.ReplaceAll(str, "\n", "\\n")
		str = strings.ReplaceAll(str, "\r", "\\r")
		str = strings.ReplaceAll(str, "\x00", "\\0")
		str = strings.ReplaceAll(str, "\x1a", "\\Z")
		return fmt.Sprintf("'%s'", str)
	case time.Time:
		if v.IsZero() {
			return "NULL"
		}
		return fmt.Sprintf("'%s'", v.Format("2006-01-02 15:04:05"))
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", v)
	case float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case string:
		// Escape special characters
		str := v
		str = strings.ReplaceAll(str, "\\", "\\\\")
		str = strings.ReplaceAll(str, "'", "\\'")
		str = strings.ReplaceAll(str, "\n", "\\n")
		str = strings.ReplaceAll(str, "\r", "\\r")
		str = strings.ReplaceAll(str, "\x00", "\\0")
		str = strings.ReplaceAll(str, "\x1a", "\\Z")
		return fmt.Sprintf("'%s'", str)
	default:
		// For any other type, convert to string and escape
		str := fmt.Sprintf("%v", v)
		str = strings.ReplaceAll(str, "\\", "\\\\")
		str = strings.ReplaceAll(str, "'", "\\'")
		return fmt.Sprintf("'%s'", str)
	}
}
