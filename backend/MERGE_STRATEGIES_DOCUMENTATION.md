# Spreadsheet Merge Strategies Documentation

## Overview

This document explains how the backend handles various merge strategies and edge cases when uploading CSV/Excel files to existing tables in the spreadsheet database.

## Merge Strategies

### 1. Replace Strategy
- **Behavior**: Drops the existing table completely and creates a new one with the uploaded data
- **Use Case**: Complete data refresh, starting from scratch
- **Edge Cases Handled**: None needed - it's a complete replacement

### 2. Append Strategy
- **Behavior**: Adds new rows to the existing table
- **Column Requirements**: New data must have the same columns as existing table
- **Edge Cases Handled**:
  - Column count mismatch detection
  - Column order differences (uses column mapping)
  - Case sensitivity in column names
  - Whitespace in values

### 3. Simple Merge Strategy
- **Behavior**: Updates existing rows and inserts new ones
- **Key Matching**: Uses all columns as composite key
- **Edge Cases Handled**:
  - Row identification without explicit ID column
  - Duplicate detection
  - Null value handling

### 4. Smart Merge Strategy (Advanced)
- **Behavior**: Configurable merge with fine-grained control
- **Options Available**:
  - Column handling (add new, drop missing)
  - Row handling (update, insert, delete)
  - Data comparison rules

## Edge Cases and Solutions

### 1. Column Changes

#### New Columns Added
- **Detection**: Compares new columns against existing schema
- **Handling**: 
  - With `addNewColumns: true`: ALTER TABLE ADD COLUMN
  - With `addNewColumns: false`: Ignores new columns
- **Data Type**: All new columns are created as TEXT

#### Columns Removed
- **Detection**: Identifies columns present in table but not in new data
- **Handling**:
  - With `dropMissingColumns: true`: ALTER TABLE DROP COLUMN
  - With `dropMissingColumns: false`: Keeps existing columns (NULL for new rows)

#### Column Renamed
- **Detection**: Uses Levenshtein distance algorithm to find similar column names
- **Threshold**: 70% similarity for automatic matching
- **Handling**: 
  - >80% similarity: Suggests rename
  - 70-80% similarity: Maps columns but keeps original name
  - <70% similarity: Treats as new column

#### Column Order Changed
- **Detection**: Column position comparison
- **Handling**: Uses column name mapping, order doesn't affect data insertion

### 2. Data Changes

#### Row Updates
- **Key Detection**: 
  1. Looks for columns named: id, _id, *_id, key, code
  2. Falls back to using all columns as composite key
- **Comparison Rules**:
  - `ignoreCase: true`: Case-insensitive comparison
  - `trimWhitespace: true`: Trims before comparison
- **Update Logic**: Only updates if values actually changed

#### New Rows
- **Detection**: Key combination not found in existing data
- **Handling**: 
  - With `insertNew: true`: Inserts new rows
  - With `insertNew: false`: Skips new rows

#### Deleted Rows
- **Detection**: Existing keys not present in new data
- **Handling**:
  - With `deleteMissing: true`: Deletes missing rows
  - With `deleteMissing: false`: Keeps existing rows

### 3. Data Type Edge Cases

#### Empty vs NULL
- **Options**:
  - `handleNulls: "keep"`: Don't update if new value is empty
  - `handleNulls: "empty"`: Replace with empty string
  - `handleNulls: "null"`: Replace with database NULL

#### Date/Time Formats
- **Current**: All data stored as TEXT
- **Future**: Automatic date format detection and conversion

#### Numeric Formats
- **Current**: Numbers stored as TEXT
- **Comparison**: String comparison (may cause issues with numeric sorting)

### 4. Special Characters

#### In Column Names
- **Handling**: Sanitization process:
  1. Replace non-alphanumeric with underscore
  2. Remove consecutive underscores
  3. Trim leading/trailing underscores
  4. Prefix with "col_" if starts with number
  5. Convert to lowercase

#### In Data Values
- **SQL Injection Prevention**: Escapes single quotes
- **Unicode Support**: Full UTF-8 support
- **Line Breaks**: Preserved in data

### 5. Performance Considerations

#### Large Files
- **Batch Processing**: Inserts in batches of 1000 rows
- **Memory**: Processes data in chunks
- **Timeout**: 5-minute timeout for operations

#### Schema Changes
- **Transaction**: Each schema change in separate statement
- **Failure Handling**: Logs warnings but continues with data operations

## Implementation Details

### Column Matching Algorithm
```go
1. Normalize both column names (lowercase, remove special chars)
2. Check for exact match
3. Calculate Levenshtein distance for fuzzy matching
4. Apply similarity threshold (70%)
5. Mark as new/deleted if no match found
```

### Row Matching Process
```go
1. Build composite key from specified key columns
2. Apply comparison options (case, whitespace)
3. Create lookup map of existing data
4. Compare each new row against existing
5. Categorize as update/insert/delete
```

### Merge Execution Order
1. Schema changes (add/drop columns)
2. Updates to existing rows
3. Insertion of new rows
4. Deletion of missing rows

## Error Handling

### Recoverable Errors
- Column addition failure: Logged, continues
- Column drop failure: Logged, continues
- Individual row update failure: Logged, continues

### Fatal Errors
- Connection failure
- Table doesn't exist
- Permission denied
- Data type conversion errors (future)

## Best Practices

1. **Always Review Column Mapping**: Check that columns are correctly matched
2. **Test with Small Dataset**: Verify behavior before large uploads
3. **Use Appropriate Strategy**:
   - Replace: For complete refreshes
   - Append: For adding new data only
   - Merge: For incremental updates
   - Smart Merge: For complex scenarios

4. **Key Column Selection**: Ensure unique identifiers for accurate matching
5. **Backup Important Data**: Before using replace or delete options

## Future Enhancements

1. **Type Detection**: Automatic column type inference
2. **Custom Key Selection**: UI for selecting key columns
3. **Preview Mode**: Show changes before applying
4. **Rollback Support**: Undo merge operations
5. **Conflict Resolution**: Cell-level conflict handling
6. **Data Validation**: Rules for data quality checks