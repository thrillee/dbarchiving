package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	Host       string
	Port       int
	User       string
	Password   string
	Database   string
	Table      string
	DaysToKeep int
	DryRun     bool
	ExportSQL  bool
	ExportCSV  bool
	ExportPath string
}

func main() {
	config := parseFlags()
	logger := NewLogger()

	logger.Info("Starting database archive process")
	logger.Info("Table: %s, Days to keep: %d, Dry run: %v", config.Table, config.DaysToKeep, config.DryRun)

	db, err := connectDB(config, logger)
	if err != nil {
		logger.Error("Failed to connect to database: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := archiveTable(db, config, logger); err != nil {
		logger.Error("Archive failed: %v", err)
		os.Exit(1)
	}

	logger.Info("Archive process completed successfully")
}

func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.Host, "host", "localhost", "Database host")
	flag.IntVar(&config.Port, "port", 3306, "Database port")
	flag.StringVar(&config.User, "user", "root", "Database user")
	flag.StringVar(&config.Password, "password", "", "Database password")
	flag.StringVar(&config.Database, "database", "", "Database name")
	flag.StringVar(&config.Table, "table", "", "Table name to archive")
	flag.IntVar(&config.DaysToKeep, "days", 90, "Number of days to keep in the original table")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Run without making changes")
	flag.BoolVar(&config.ExportSQL, "export-sql", false, "Export archived table to SQL file")
	flag.BoolVar(&config.ExportCSV, "export-csv", false, "Export archived table to CSV file")
	flag.StringVar(&config.ExportPath, "export-path", "./archives", "Path to save exported SQL files")

	flag.Parse()

	if config.Database == "" || config.Table == "" {
		fmt.Println("Error: database and table flags are required")
		flag.Usage()
		os.Exit(1)
	}

	return config
}

func connectDB(config *Config, logger *Logger) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		config.User, config.Password, config.Host, config.Port, config.Database)

	logger.Info("Connecting to database %s@%s:%d/%s", config.User, config.Host, config.Port, config.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	logger.Info("Database connection established")
	return db, nil
}

func archiveTable(db *sql.DB, config *Config, logger *Logger) error {
	suffix := time.Now().Format("20060102")
	newTableName := fmt.Sprintf("%s_%s", config.Table, suffix)
	archiveTableName := fmt.Sprintf("%s_archive_%s", config.Table, suffix)

	// Step 1: Get the CREATE TABLE statement
	logger.Info("Step 1: Retrieving CREATE TABLE statement for %s", config.Table)
	createStmt, err := getCreateTable(db, config.Table)
	if err != nil {
		return fmt.Errorf("failed to get CREATE TABLE: %v", err)
	}

	// Step 2: Count records to archive and keep
	logger.Info("Step 2: Counting records")
	archiveCount, keepCount, dateColumn, err := countRecords(db, config, logger)
	if err != nil {
		return fmt.Errorf("failed to count records: %v", err)
	}

	logger.Info("Records to archive: %d, Records to keep: %d", archiveCount, keepCount)

	if archiveCount == 0 {
		logger.Warning("No records to archive. Exiting.")
		return nil
	}

	if config.DryRun {
		logger.Info("DRY RUN MODE - No changes will be made")
		logger.Info("Would create archive table: %s", newTableName)
		logger.Info("Would move %d records to archive", archiveCount)
		logger.Info("Would rename original table to: %s", archiveTableName)
		return nil
	}

	// Step 3: Create new table with modified name
	logger.Info("Step 3: Creating new table %s", newTableName)
	newCreateStmt := modifyCreateStatement(createStmt, config.Table, newTableName, suffix)
	if err := executeSQL(db, newCreateStmt, logger); err != nil {
		return fmt.Errorf("failed to create new table: %v", err)
	}

	// Step 4: Copy old records to new table
	logger.Info("Step 4: Copying old records to %s", newTableName)
	cutoffDate := time.Now().AddDate(0, 0, -config.DaysToKeep)
	if err := copyOldRecords(db, config.Table, newTableName, dateColumn, cutoffDate, logger); err != nil {
		logger.Error("Failed to copy records, dropping new table")
		executeSQL(db, fmt.Sprintf("DROP TABLE IF EXISTS `%s`", newTableName), logger)
		return fmt.Errorf("failed to copy records: %v", err)
	}

	// Step 5: Verify the copy
	logger.Info("Step 5: Verifying copied records")
	copiedCount, err := getTableCount(db, newTableName)
	if err != nil {
		return fmt.Errorf("failed to verify copied records: %v", err)
	}

	if copiedCount != keepCount {
		logger.Error("Record count mismatch! Expected: %d, Got: %d", archiveCount, copiedCount)
		return fmt.Errorf("record count mismatch")
	}

	logger.Info("Verification successful: %d records copied", copiedCount)

	// Step 6: Delete old records from original table
	logger.Info("Step 6: Deleting archived records from %s", config.Table)
	if err := deleteOldRecords(db, config.Table, dateColumn, cutoffDate, logger); err != nil {
		return fmt.Errorf("failed to delete old records: %v", err)
	}

	// Step 7: Rename original table
	logger.Info("Step 7: Renaming original table to %s", archiveTableName)
	renameSQL := fmt.Sprintf("RENAME TABLE `%s` TO `%s`", config.Table, archiveTableName)
	if err := executeSQL(db, renameSQL, logger); err != nil {
		return fmt.Errorf("failed to rename table: %v", err)
	}

	// Step 8: Rename new table to original name
	logger.Info("Step 8: Renaming %s to %s", newTableName, config.Table)
	renameSQL = fmt.Sprintf("RENAME TABLE `%s` TO `%s`", newTableName, config.Table)
	if err := executeSQL(db, renameSQL, logger); err != nil {
		return fmt.Errorf("failed to rename new table: %v", err)
	}

	logger.Info("Archive complete! Old table renamed to %s, new table is now %s", archiveTableName, config.Table)

	// Step 9: Export archived table if requested
	if config.ExportSQL || config.ExportCSV {
		if config.ExportSQL {
			logger.Info("Step 9a: Exporting archived table to SQL file")
			if err := exportTableToSQL(db, archiveTableName, config, logger); err != nil {
				logger.Error("Failed to export SQL: %v", err)
				// Don't fail the entire process if export fails
			} else {
				logger.Info("SQL export completed successfully")
			}
		}

		if config.ExportCSV {
			logger.Info("Step 9b: Exporting archived table to CSV file")
			if err := exportTableToCSV(db, archiveTableName, config, logger); err != nil {
				logger.Error("Failed to export CSV: %v", err)
				// Don't fail the entire process if export fails
			} else {
				logger.Info("CSV export completed successfully")
			}
		}
	}

	return nil
}

func getCreateTable(db *sql.DB, tableName string) (string, error) {
	var table, createStmt string
	query := fmt.Sprintf("SHOW CREATE TABLE `%s`", tableName)
	err := db.QueryRow(query).Scan(&table, &createStmt)
	return createStmt, err
}

func countRecords(db *sql.DB, config *Config, logger *Logger) (archiveCount, keepCount int64, dateColumn string, err error) {
	// Detect the date column to use
	dateColumn, err = detectDateColumn(db, config.Table)
	if err != nil {
		return 0, 0, "", err
	}

	logger.Info("Using date column: %s", dateColumn)

	cutoffDate := time.Now().AddDate(0, 0, -config.DaysToKeep)
	logger.Info("Cutoff date: %s", cutoffDate.Format("2006-01-02"))

	// Count records to archive (older than cutoff)
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE `%s` < ?", config.Table, dateColumn)
	err = db.QueryRow(query, cutoffDate).Scan(&archiveCount)
	if err != nil {
		return 0, 0, "", err
	}

	// Count records to keep (newer than or equal to cutoff)
	query = fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE `%s` >= ?", config.Table, dateColumn)
	err = db.QueryRow(query, cutoffDate).Scan(&keepCount)
	if err != nil {
		return 0, 0, "", err
	}

	return archiveCount, keepCount, dateColumn, nil
}

func detectDateColumn(db *sql.DB, tableName string) (string, error) {
	// Priority order for date columns
	dateColumns := []string{"smsdate", "request_time", "deli_date", "created_at", "updated_at", "req_date", "res_date", "date_created", "created"}

	query := fmt.Sprintf("SHOW COLUMNS FROM `%s`", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	availableColumns := make(map[string]bool)
	for rows.Next() {
		var field, colType string
		var null, key, def, extra sql.NullString
		if err := rows.Scan(&field, &colType, &null, &key, &def, &extra); err != nil {
			return "", err
		}
		if strings.Contains(strings.ToLower(colType), "date") || strings.Contains(strings.ToLower(colType), "time") {
			availableColumns[field] = true
		}
	}

	// Return the first matching column from priority list
	for _, col := range dateColumns {
		if availableColumns[col] {
			return col, nil
		}
	}

	// If no priority column found, return the first datetime column
	for col := range availableColumns {
		return col, nil
	}

	return "", fmt.Errorf("no suitable date column found in table %s", tableName)
}

func modifyCreateStatement(createStmt, oldName, newName, suffix string) string {
	// Replace table name
	createStmt = strings.Replace(createStmt, fmt.Sprintf("CREATE TABLE `%s`", oldName), fmt.Sprintf("CREATE TABLE `%s`", newName), 1)

	// Update index names with suffix (KEY, UNIQUE KEY, INDEX)
	// re := regexp.MustCompile(`(KEY|INDEX|UNIQUE KEY)\s+` + "`" + `([^`]+)` + "`")
	re := regexp.MustCompile("(KEY|INDEX|UNIQUE KEY)\\s+`([^`]+)`")
	createStmt = re.ReplaceAllStringFunc(createStmt, func(match string) string {
		// Extract the index name
		parts := re.FindStringSubmatch(match)
		if len(parts) >= 3 {
			indexType := parts[1]
			indexName := parts[2]

			// Don't modify PRIMARY KEY
			if strings.ToUpper(indexType) == "PRIMARY KEY" {
				return match
			}

			newIndexName := fmt.Sprintf("%s_%s", indexName, suffix)
			return fmt.Sprintf("%s `%s`", indexType, newIndexName)
		}
		return match
	})

	// Update foreign key constraint names with suffix
	// Match: CONSTRAINT `constraint_name` FOREIGN KEY
	fkRe := regexp.MustCompile(`CONSTRAINT\s+` + "`" + `([^` + "`" + `]+)` + "`" + `\s+FOREIGN KEY`)
	createStmt = fkRe.ReplaceAllStringFunc(createStmt, func(match string) string {
		parts := fkRe.FindStringSubmatch(match)
		if len(parts) >= 2 {
			constraintName := parts[1]
			newConstraintName := fmt.Sprintf("%s_%s", constraintName, suffix)
			return fmt.Sprintf("CONSTRAINT `%s` FOREIGN KEY", newConstraintName)
		}
		return match
	})

	return createStmt
}

func copyOldRecords(db *sql.DB, sourceTable, destTable, dateColumn string, cutoffDate time.Time, logger *Logger) error {
	query := fmt.Sprintf("INSERT INTO `%s` SELECT * FROM `%s` WHERE `%s` < ?", destTable, sourceTable, dateColumn)
	logger.Info("Executing: %s with cutoff %s", query, cutoffDate.Format("2006-01-02"))

	result, err := db.Exec(query, cutoffDate)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	logger.Info("Copied %d rows", rowsAffected)

	return nil
}

func deleteOldRecords(db *sql.DB, table, dateColumn string, cutoffDate time.Time, logger *Logger) error {
	query := fmt.Sprintf("DELETE FROM `%s` WHERE `%s` < ?", table, dateColumn)
	logger.Info("Executing: %s with cutoff %s", query, cutoffDate.Format("2006-01-02"))

	result, err := db.Exec(query, cutoffDate)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	logger.Info("Deleted %d rows", rowsAffected)

	return nil
}

func getTableCount(db *sql.DB, tableName string) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)
	err := db.QueryRow(query).Scan(&count)
	return count, err
}

func executeSQL(db *sql.DB, query string, logger *Logger) error {
	logger.Info("Executing SQL: %s", truncateSQL(query, 200))
	_, err := db.Exec(query)
	return err
}

func truncateSQL(sql string, maxLen int) string {
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "..."
}
