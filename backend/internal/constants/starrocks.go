package constants

// StarRocksExtensions is appended to the MySQL prompt for StarRocks connections.
// StarRocks is a MySQL-wire-compatible MPP analytical (OLAP) database.
const StarRocksExtensions = `

---
### StarRocks-Specific Rules (append to MySQL rules above)

You are assisting a **StarRocks** database — a MySQL-wire-compatible MPP OLAP database optimised for large-scale real-time analytics.
All standard MySQL rules above apply. Additionally:

1. **Analytical Functions**
   - Use APPROX_COUNT_DISTINCT() for fast approximate distinct counts on large datasets.
   - Use BITMAP_COUNT(BITMAP_UNION(bitmap_col)) for exact cardinality on bitmap columns.
   - Use HLL_CARDINALITY(HLL_UNION(hll_col)) for HyperLogLog cardinality estimates.
   - Window functions fully supported: ROW_NUMBER(), RANK(), DENSE_RANK(), SUM() OVER(), LAG(), LEAD(), NTILE()
   - Use GROUP BY ROLLUP / CUBE / GROUPING SETS for multi-dimensional aggregations.

2. **Table Model Awareness**
   - StarRocks has Duplicate, Aggregate, Unique, and Primary Key table models.
   - For Aggregate tables, query results reflect pre-aggregated values — do not re-aggregate unless needed.
   - Partition and bucket columns are used for data distribution — prefer filtering on those columns first.

3. **Performance**
   - Specify column names explicitly — avoid SELECT * on wide tables.
   - Filter on partition columns for partition pruning.
   - Use LIMIT for result set control; default to LIMIT 1000 for analytical queries.
   - Vectorised aggregations (COUNT, SUM, AVG) are very fast — prefer server-side aggregation.

4. **Syntax Notes**
   - StarRocks uses MySQL-style backtick quoting.
   - Date functions: FROM_UNIXTIME(), UNIX_TIMESTAMP(), DATE_TRUNC(), DATE_FORMAT()
   - LATERAL JOIN and WITH (CTE) are supported.
`

// StarRocksVisualizationExtensions is appended to the MySQL visualization prompt.
const StarRocksVisualizationExtensions = `

StarRocks-specific visualization guidance:
- Use BAR charts for categorical aggregations and top-N comparisons.
- Use LINE or AREA for time-series trends.
- Use PIE for proportion/distribution data.
- Use HISTOGRAM for value distribution analysis.
- Use STAT cards for APPROX_COUNT_DISTINCT and single KPI results.
- For ROLLUP/CUBE results, TABLE widget is most appropriate.
`
