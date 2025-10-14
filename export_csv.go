package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"time"
)

func exportTableToCSV(db *sql.DB, tableName string, config *Config, logger *Logger) error {
	// Create export directory if it doesn't exist
	if err := os.MkdirAll(config.ExportPath, 0o755); err != nil {
		return fmt.Errorf("failed to create export directory: %v", err)
	}

	// Generate filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s/%s_%s.csv", config.ExportPath, tableName, timestamp)

	// Create the CSV file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %v", err)
	}
	defer file.Close()

	logger.Info("Exporting to CSV file: %s", filename)

	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Get column names
	columnQuery := fmt.Sprintf("SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '%s' ORDER BY ORDINAL_POSITION", tableName)
	rows, err := db.Query(columnQuery)
	if err != nil {
		return fmt.Errorf("failed to get column names: %v", err)
	}

	var columns []string
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan column name: %v", err)
		}
		columns = append(columns, columnName)
	}
	rows.Close()

	// Write header row
	if err := writer.Write(columns); err != nil {
		return fmt.Errorf("failed to write CSV header: %v", err)
	}

	// Export data in batches
	batchSize := 5000
	offset := 0
	totalRows := 0

	for {
		dataQuery := fmt.Sprintf("SELECT * FROM `%s` LIMIT %d OFFSET %d", tableName, batchSize, offset)
		dataRows, err := db.Query(dataQuery)
		if err != nil {
			return fmt.Errorf("failed to query data: %v", err)
		}

		// Prepare value containers
		columnCount := len(columns)
		values := make([]interface{}, columnCount)
		valuePtrs := make([]interface{}, columnCount)
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		rowCount := 0

		for dataRows.Next() {
			if err := dataRows.Scan(valuePtrs...); err != nil {
				dataRows.Close()
				return fmt.Errorf("failed to scan row: %v", err)
			}

			// Convert values to strings for CSV
			record := make([]string, columnCount)
			for i, val := range values {
				record[i] = formatCSVValue(val)
			}

			if err := writer.Write(record); err != nil {
				dataRows.Close()
				return fmt.Errorf("failed to write CSV row: %v", err)
			}

			rowCount++
			totalRows++

			// Flush periodically to avoid memory issues
			if totalRows%1000 == 0 {
				writer.Flush()
				if err := writer.Error(); err != nil {
					dataRows.Close()
					return fmt.Errorf("failed to flush CSV: %v", err)
				}
			}
		}

		dataRows.Close()

		// Log progress
		if rowCount > 0 {
			logger.Info("Exported %d rows to CSV...", totalRows)
		}

		// If we got fewer rows than batch size, we're done
		if rowCount < batchSize {
			break
		}

		offset += batchSize
	}

	// Final flush
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("failed to flush CSV: %v", err)
	}

	logger.Info("Successfully exported %d rows to %s", totalRows, filename)
	return nil
}

func formatCSVValue(val interface{}) string {
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case []byte:
		return string(v)
	case time.Time:
		if v.IsZero() {
			return ""
		}
		return v.Format("2006-01-02 15:04:05")
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
