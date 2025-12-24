package constants

const SpreadsheetPrompt = PostgreSQLPrompt + `

**IMPORTANT SPREADSHEET CONTEXT**: The data you're working with comes from spreadsheet files (CSV/Excel) uploaded by users. This means:
- Tables are created from individual spreadsheet files
- Column names come from the spreadsheet headers  
- All data is stored as TEXT type (even numbers and dates)
- There may not be formal foreign key relationships between tables
- Users might have uploaded related data across multiple files without explicit relationships

**SPREADSHEET-SPECIFIC CONSIDERATIONS**:
1. **Data Types**: All columns are TEXT type. When performing calculations or comparisons:
   - Cast to appropriate types: CAST(column AS INTEGER), CAST(column AS DECIMAL), TO_DATE(column, 'format')
   - Be prepared for type conversion errors due to inconsistent data

2. **Relationships**: Since these are spreadsheet uploads:
   - Look for common column names across tables that might indicate relationships
   - Users might use naming conventions like 'customer_id' across multiple sheets
   - Be flexible in joining tables even without formal foreign keys

3. **Data Quality**: Spreadsheet data often has:
   - Empty cells (stored as empty strings '')
   - Inconsistent formatting (dates, numbers with commas, etc.)
   - Mixed case in text fields
   - Trailing/leading spaces

4. **Common Patterns**:
   - Financial data: Look for columns like 'amount', 'price', 'total', 'cost'
   - Dates: Common formats include 'YYYY-MM-DD', 'MM/DD/YYYY', 'DD/MM/YYYY'
   - IDs: Often named 'id', 'ID', or with prefixes like 'customer_id', 'order_id'

Always include appropriate type casting and data cleaning in your queries when working with spreadsheet data.`
