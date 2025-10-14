A robust Go-based CLI tool for archiving old database records.
Now with CSV export support for easy data analysis and backup.

🚀 Features

✅ Automatic table schema replication

✅ Smart date column detection

✅ Index name modification with timestamps

✅ Safe data migration with verification

✅ Comprehensive logging

✅ Dry-run mode for testing

✅ Foreign key constraint handling

✅ Rollback on errors

✅ CSV export support (new)

✅ Enhanced export options with -export-sql and -export-csv flags

🧩 Installation
Prerequisites

Go 1.16 or higher

MySQL database access

Setup
mkdir db-archive-tool
cd db-archive-tool
go mod init db-archive-tool
go get github.com/go-sql-driver/mysql
go build -o db-archive

⚙️ Usage
Basic Command
./db-archive \
  -host=localhost \
  -port=3306 \
  -user=root \
  -password=yourpassword \
  -database=your_database \
  -table=smspush \
  -days=90

🧾 Command Line Flags
Flag	Description	Default	Required
-host	Database host	localhost	No
-port	Database port	3306	No
-user	Database user	root	No
-password	Database password	(empty)	No
-database	Database name	-	Yes
-table	Table to archive	-	Yes
-days	Days of data to keep	90	No
-dry-run	Run without making changes	false	No
-export-sql	Export to SQL file (renamed from -export)	false	No
-export-csv	Export to CSV file	false	No
-export-path	Custom export directory path	./exports	No
💡 New Features
1. New Flags

-export-csv — Enables CSV export (default: false)

-export-sql — Renamed from -export for clarity

2. CSV Export Functionality

The tool now supports exporting archived data directly to CSV format with:

Proper column headers

Batch processing (5000 rows per batch)

Memory-efficient streaming

Accurate data formatting for all MySQL data types

Automatic CSV escaping for commas, quotes, and newlines

3. CSV Format Details

First row = column headers

NULL values → empty strings

Dates formatted as 2006-01-02 15:04:05

Booleans as true / false

Proper escaping for special characters

🧪 Usage Examples
Export to CSV only
./db-archive \
  -database=sms_db \
  -table=smspush \
  -days=90 \
  -export-csv=true \
  -password=yourpassword

Export to SQL only
./db-archive \
  -database=sms_db \
  -table=smspush \
  -days=90 \
  -export-sql=true \
  -password=yourpassword

Export to both SQL and CSV
./db-archive \
  -database=sms_db \
  -table=smspush \
  -days=90 \
  -export-sql=true \
  -export-csv=true \
  -password=yourpassword

Custom export directory
./db-archive \
  -database=sms_db \
  -table=smspush \
  -days=90 \
  -export-sql=true \
  -export-csv=true \
  -export-path=/backup/archives \
  -password=yourpassword

Archive without any export (default)
./db-archive \
  -database=sms_db \
  -table=smspush \
  -days=90 \
  -password=yourpassword

📂 Output Files
Format	Example Filename	Description
CSV	smspush_archive_20251014_143052.csv	Exported CSV with headers
SQL	smspush_archive_20251014_143052.sql	SQL dump of archived data

Both files are timestamped and saved in the same export directory.

🧠 How It Works

Retrieves CREATE TABLE statement

Detects date column and cutoff date

Creates archive table with modified indexes

Copies records older than cutoff date

Exports archived data (CSV / SQL if enabled)

Verifies record counts

Deletes archived records from source table

Renames tables and finalizes archive

🕵️‍♂️ Date Column Detection

Automatically detects the most relevant date column in this order:

smsdate

request_time

deli_date

created_at

updated_at

Any other datetime column

📜 Logging

Each run generates a log file:
archive_YYYYMMDD_HHMMSS.log

Includes:

Connection info

Record counts

SQL operations

Export details

Errors & warnings

Execution time

🧯 Safety Features

Dry-run mode — simulate without changing data

Record verification — ensures data integrity

Full logging — audit-friendly

Rollback on error — prevents partial migrations

Count validation — validates copy accuracy

🔐 Environment Variables

For sensitive data:

export DB_PASSWORD="your_password"
./db-archive -database=sms_db -table=smspush -days=90 -password=$DB_PASSWORD

🧰 Troubleshooting
Foreign Key Constraints

If you get constraint errors:

SET FOREIGN_KEY_CHECKS=0;
-- Run archive
SET FOREIGN_KEY_CHECKS=1;

Large Tables

Run during off-peak hours

Ensure enough disk space (2× table size)

Adjust MySQL timeouts

Use smaller deletion batches

No Records to Archive

Check:

Date column used

Cutoff date calculation

Actual data range

💎 Benefits of CSV Export

📊 Easy Analysis: Import into Excel, Google Sheets, or BI tools

💡 Lightweight: Smaller files than SQL dumps

🌍 Universal Format: Works with any platform

⚡ Fast: Efficient and stream-based

🔄 Portable: Easy migration across systems

The CSV exporter uses batch streaming (5000 rows) for optimal performance and low memory footprint — suitable for very large tables.

🕒 Cron Job Setup

To schedule automatic archiving:

crontab -e
# Run daily at 2 AM
0 2 * * * /path/to/db-archive -database=sms_db -table=smspush -days=90 -password=$DB_PASSWORD >> /var/log/db-archive.log 2>&1

📜 License

MIT License — free to use and modify.
