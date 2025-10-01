# Database Archive CLI Tool

A robust Go-based CLI tool for archiving old database records. This tool automates the process of creating archive tables, migrating old data, and maintaining clean production tables.

## Features

- ✅ Automatic table schema replication
- ✅ Smart date column detection
- ✅ Index name modification with timestamps
- ✅ Safe data migration with verification
- ✅ Comprehensive logging
- ✅ Dry-run mode for testing
- ✅ Foreign key constraint handling
- ✅ Rollback on errors

## Installation

### Prerequisites

- Go 1.16 or higher
- MySQL database access

### Setup

1. Create a new directory and initialize the project:

```bash
mkdir db-archive-tool
cd db-archive-tool
```

2. Create `go.mod` file:

```bash
go mod init db-archive-tool
```

3. Install dependencies:

```bash
go get github.com/go-sql-driver/mysql
```

4. Save the main.go file and build:

```bash
go build -o db-archive
```

## Usage

### Basic Command

```bash
./db-archive \
  -host=localhost \
  -port=3306 \
  -user=root \
  -password=yourpassword \
  -database=your_database \
  -table=smspush \
  -days=90
```

### Command Line Flags

| Flag | Description | Default | Required |
|------|-------------|---------|----------|
| `-host` | Database host | localhost | No |
| `-port` | Database port | 3306 | No |
| `-user` | Database user | root | No |
| `-password` | Database password | (empty) | No |
| `-database` | Database name | - | **Yes** |
| `-table` | Table to archive | - | **Yes** |
| `-days` | Days of data to keep | 90 | No |
| `-dry-run` | Run without making changes | false | No |

### Examples

#### Test run (dry-run mode)

```bash
./db-archive \
  -database=sms_db \
  -table=smspush \
  -days=90 \
  -dry-run=true
```

#### Archive smspush table (keep last 90 days)

```bash
./db-archive \
  -host=localhost \
  -user=dbuser \
  -password=secret123 \
  -database=sms_db \
  -table=smspush \
  -days=90
```

#### Archive submitresponse table (keep last 60 days)

```bash
./db-archive \
  -database=sms_db \
  -table=submitresponse \
  -days=60 \
  -password=$DB_PASSWORD
```

#### Archive smsdelivery table (keep last 30 days)

```bash
./db-archive \
  -database=sms_db \
  -table=smsdelivery \
  -days=30 \
  -password=$DB_PASSWORD
```

## How It Works

The tool performs the following steps:

1. **Retrieves CREATE TABLE statement** - Gets the exact schema of the source table
2. **Counts records** - Calculates how many records will be archived vs kept
3. **Creates new table** - Creates a new table with modified index names (appends date suffix)
4. **Copies old records** - Moves records older than the cutoff date to the new table
5. **Verifies copy** - Ensures all records were copied correctly
6. **Deletes old records** - Removes archived records from the original table
7. **Renames tables** - Renames original table with archive suffix, then renames the cleaned table to the original name

### Example Flow

If you run on `2025-10-01` with `-table=smspush -days=90`:

- Creates table: `smspush_20251001` (with records older than July 3, 2025)
- Original table `smspush` keeps records from July 3, 2025 onwards
- After completion, `smspush` contains only recent data
- Archive is stored in `smspush_archive_20251001`

## Date Column Detection

The tool automatically detects the appropriate date column in this priority order:

1. `smsdate`
2. `request_time`
3. `deli_date`
4. `created_at`
5. `updated_at`
6. Any other datetime column

## Logging

Each run creates a timestamped log file: `archive_YYYYMMDD_HHMMSS.log`

Logs include:
- Connection details
- Record counts
- SQL operations
- Errors and warnings
- Execution time

## Safety Features

- **Dry-run mode**: Test the operation without making changes
- **Record verification**: Verifies copied data before deleting from source
- **Comprehensive logging**: All operations are logged for audit
- **Error rollback**: If copying fails, the new table is dropped
- **Count validation**: Ensures archive count matches expected records

## Environment Variables

You can use environment variables for sensitive data:

```bash
export DB_PASSWORD="your_password"
./db-archive -database=sms_db -table=smspush -days=90 -password=$DB_PASSWORD
```

## Troubleshooting

### Foreign Key Constraints

If you encounter foreign key constraint errors, you may need to:

1. Disable foreign key checks temporarily:
```sql
SET FOREIGN_KEY_CHECKS=0;
-- Run archive
SET FOREIGN_KEY_CHECKS=1;
```

2. Or modify the tool to handle this automatically (add to the code)

### Large Tables

For very large tables (millions of records):

- Consider running during off-peak hours
- Monitor disk space (you'll temporarily need 2x table size)
- Increase MySQL timeouts if needed
- Use smaller batch sizes for deletion

### No Records to Archive

If the tool reports "No records to archive", check:

- The date column being used
- The cutoff date calculation
- Data in your table

## Best Practices

1. **Always run dry-run first**: Test with `-dry-run=true`
2. **Backup before archiving**: Take a database backup
3. **Monitor disk space**: Ensure adequate space for duplication
4. **Schedule during low traffic**: Run during maintenance windows
5. **Keep logs**: Archive log files for compliance
6. **Test restore procedures**: Verify you can restore from archives

## Cron Job Setup

To run automatically:

```bash
# Edit crontab
crontab -e

# Add line to run daily at 2 AM
0 2 * * * /path/to/db-archive -database=sms_db -table=smspush -days=90 -password=$DB_PASSWORD >> /var/log/db-archive.log 2>&1
```

## License

MIT License - feel free to modify and use as needed.
