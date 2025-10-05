package utils

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ColumnDataType represents the inferred data type for a column
type ColumnDataType struct {
	PostgreSQLType string // PostgreSQL specific type
	SQLType        string // Generic SQL type
	IsNullable     bool
	HasMixedTypes  bool   // If column has mixed types that couldn't be unified
	ErrorCount     int    // Number of values that couldn't be converted to the inferred type
	SampleSize     int    // Number of non-null values analyzed
}

// DataTypeInferrer provides methods for inferring data types from sample data
type DataTypeInferrer struct {
	minSampleSize int // Minimum number of rows to sample (default: 10)
	maxSampleSize int // Maximum number of rows to sample (default: 100)
}

// NewDataTypeInferrer creates a new DataTypeInferrer with default settings
func NewDataTypeInferrer() *DataTypeInferrer {
	return &DataTypeInferrer{
		minSampleSize: 10,
		maxSampleSize: 100,
	}
}

// InferColumnTypes analyzes the provided data and infers the most appropriate data type for each column
func (d *DataTypeInferrer) InferColumnTypes(headers []string, data [][]interface{}) (map[string]ColumnDataType, error) {
	if len(headers) == 0 {
		return nil, fmt.Errorf("no headers provided")
	}

	result := make(map[string]ColumnDataType)
	
	// Initialize result with default TEXT type
	for _, header := range headers {
		result[header] = ColumnDataType{
			PostgreSQLType: "TEXT",
			SQLType:        "TEXT",
			IsNullable:     true,
			HasMixedTypes:  false,
			ErrorCount:     0,
			SampleSize:     0,
		}
	}

	if len(data) == 0 {
		return result, nil
	}

	// Determine sample size
	sampleSize := len(data)
	if sampleSize > d.maxSampleSize {
		sampleSize = d.maxSampleSize
	}

	log.Printf("DataTypeInferrer -> Analyzing %d rows for %d columns", sampleSize, len(headers))

	// Analyze each column
	for colIdx, header := range headers {
		if colIdx >= len(headers) {
			continue
		}
		
		columnData := make([]interface{}, 0, sampleSize)
		
		// Collect sample data for this column
		for rowIdx := 0; rowIdx < sampleSize && rowIdx < len(data); rowIdx++ {
			if colIdx < len(data[rowIdx]) && data[rowIdx][colIdx] != nil {
				// Convert to string for analysis
				cellValue := fmt.Sprintf("%v", data[rowIdx][colIdx])
				if strings.TrimSpace(cellValue) != "" {
					columnData = append(columnData, cellValue)
				}
			}
		}

		// Infer type for this column
		inferredType := d.inferSingleColumnType(columnData)
		result[header] = inferredType

		log.Printf("DataTypeInferrer -> Column '%s': %s (sample: %d, errors: %d, mixed: %t)",
			header, inferredType.PostgreSQLType, inferredType.SampleSize, 
			inferredType.ErrorCount, inferredType.HasMixedTypes)
	}

	return result, nil
}

// inferSingleColumnType analyzes a single column's data and returns the most appropriate type
func (d *DataTypeInferrer) inferSingleColumnType(columnData []interface{}) ColumnDataType {
	if len(columnData) == 0 {
		return ColumnDataType{
			PostgreSQLType: "TEXT",
			SQLType:        "TEXT",
			IsNullable:     true,
			HasMixedTypes:  false,
			ErrorCount:     0,
			SampleSize:     0,
		}
	}

	sampleSize := len(columnData)
	
	// Type counters
	intCount := 0
	floatCount := 0
	boolCount := 0
	dateCount := 0
	timestampCount := 0
	uuidCount := 0
	emailCount := 0
	textCount := 0

	// Analyze each value
	for _, value := range columnData {
		valueStr := strings.TrimSpace(fmt.Sprintf("%v", value))
		
		if d.isInteger(valueStr) {
			intCount++
		} else if d.isFloat(valueStr) {
			floatCount++
		} else if d.isBoolean(valueStr) {
			boolCount++
		} else if d.isDate(valueStr) {
			dateCount++
		} else if d.isTimestamp(valueStr) {
			timestampCount++
		} else if d.isUUID(valueStr) {
			uuidCount++
		} else if d.isEmail(valueStr) {
			emailCount++
		} else {
			textCount++
		}
	}

	// Calculate percentages for decision making
	total := float64(sampleSize)
	intPct := float64(intCount) / total
	floatPct := float64(floatCount) / total
	boolPct := float64(boolCount) / total
	datePct := float64(dateCount) / total
	timestampPct := float64(timestampCount) / total
	uuidPct := float64(uuidCount) / total
	emailPct := float64(emailCount) / total

	// Decision thresholds - More aggressive for clear majorities
	const highConfidence = 0.85 // 85% of values match the type
	const mediumConfidence = 0.70 // 70% of values match the type  
	const majorityThreshold = 0.60 // 60% majority - should still infer type with errors

	// Determine the best type based on confidence levels
	errorCount := 0
	var postgresType, sqlType string
	hasMixedTypes := false

	// Timestamp has priority over date
	if timestampPct >= highConfidence {
		postgresType = "TIMESTAMP"
		sqlType = "TIMESTAMP"
		errorCount = sampleSize - timestampCount
	} else if datePct >= highConfidence {
		postgresType = "DATE"
		sqlType = "DATE"
		errorCount = sampleSize - dateCount
	} else if uuidPct >= highConfidence {
		postgresType = "UUID"
		sqlType = "CHAR(36)" // Standard UUID length
		errorCount = sampleSize - uuidCount
	} else if boolPct >= highConfidence {
		postgresType = "BOOLEAN"
		sqlType = "BOOLEAN"
		errorCount = sampleSize - boolCount
	} else if intPct >= highConfidence {
		postgresType = "INTEGER"
		sqlType = "INTEGER"
		errorCount = sampleSize - intCount
	} else if (intPct + floatPct) >= highConfidence {
		// Combined numeric types
		postgresType = "NUMERIC"
		sqlType = "DECIMAL"
		errorCount = sampleSize - (intCount + floatCount)
	} else if emailPct >= mediumConfidence {
		postgresType = "VARCHAR(255)"
		sqlType = "VARCHAR(255)"
		errorCount = sampleSize - emailCount
	} else if intPct >= mediumConfidence {
		postgresType = "INTEGER"
		sqlType = "INTEGER"
		errorCount = sampleSize - intCount
		hasMixedTypes = true // Lower confidence, likely mixed
	} else if (intPct + floatPct) >= mediumConfidence {
		postgresType = "NUMERIC"
		sqlType = "DECIMAL"
		errorCount = sampleSize - (intCount + floatCount)
		hasMixedTypes = true
	} else if intPct >= majorityThreshold {
		// Majority are integers, even with some conversion errors
		postgresType = "INTEGER"
		sqlType = "INTEGER"
		errorCount = sampleSize - intCount
		hasMixedTypes = true
	} else if (intPct + floatPct) >= majorityThreshold {
		// Majority are numeric, even with some conversion errors
		postgresType = "NUMERIC"
		sqlType = "DECIMAL"
		errorCount = sampleSize - (intCount + floatCount)
		hasMixedTypes = true
	} else {
		// Default to text for mixed or unrecognized data
		postgresType = "TEXT"
		sqlType = "TEXT"
		hasMixedTypes = textCount < sampleSize // Mixed if not all text
	}

	return ColumnDataType{
		PostgreSQLType: postgresType,
		SQLType:        sqlType,
		IsNullable:     true, // Always allow nulls for safety
		HasMixedTypes:  hasMixedTypes,
		ErrorCount:     errorCount,
		SampleSize:     sampleSize,
	}
}

// Type detection helper functions

func (d *DataTypeInferrer) isInteger(value string) bool {
	// Remove common number formatting (commas, spaces)
	cleanValue := strings.ReplaceAll(value, ",", "")
	cleanValue = strings.ReplaceAll(cleanValue, " ", "")
	
	// Handle negative numbers
	cleanValue = strings.TrimPrefix(cleanValue, "-")
	cleanValue = strings.TrimPrefix(cleanValue, "+")
	
	_, err := strconv.ParseInt(cleanValue, 10, 64)
	return err == nil
}

func (d *DataTypeInferrer) isFloat(value string) bool {
	// Remove common number formatting (commas, spaces) 
	cleanValue := strings.ReplaceAll(value, ",", "")
	cleanValue = strings.ReplaceAll(cleanValue, " ", "")
	
	// Handle negative numbers
	cleanValue = strings.TrimPrefix(cleanValue, "-")
	cleanValue = strings.TrimPrefix(cleanValue, "+")
	
	_, err := strconv.ParseFloat(cleanValue, 64)
	return err == nil
}

func (d *DataTypeInferrer) isBoolean(value string) bool {
	lower := strings.ToLower(value)
	return lower == "true" || lower == "false" || 
		   lower == "yes" || lower == "no" ||
		   lower == "1" || lower == "0" ||
		   lower == "y" || lower == "n"
}

func (d *DataTypeInferrer) isDate(value string) bool {
	// Common date formats
	dateFormats := []string{
		"2006-01-02",
		"01/02/2006",
		"01-02-2006",
		"2006/01/02",
		"02/01/2006",
		"02-01-2006",
		"Jan 02, 2006",
		"January 02, 2006",
		"02 Jan 2006",
		"02 January 2006",
	}

	for _, format := range dateFormats {
		if _, err := time.Parse(format, value); err == nil {
			return true
		}
	}
	return false
}

func (d *DataTypeInferrer) isTimestamp(value string) bool {
	// Common timestamp formats
	timestampFormats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.000",
		"2006-01-02T15:04:05.000Z",
		"01/02/2006 15:04:05",
		"01-02-2006 15:04:05",
	}

	for _, format := range timestampFormats {
		if _, err := time.Parse(format, value); err == nil {
			return true
		}
	}
	return false
}

func (d *DataTypeInferrer) isUUID(value string) bool {
	// UUID v4 pattern: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(strings.ToLower(value))
}

func (d *DataTypeInferrer) isEmail(value string) bool {
	// Basic email validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(value)
}