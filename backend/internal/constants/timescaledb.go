package constants

// TimescaleDBExtensions is appended to the PostgreSQL prompt for TimescaleDB connections.
// TimescaleDB is a PostgreSQL extension optimised for time-series and analytical workloads.
const TimescaleDBExtensions = `

---
### TimescaleDB-Specific Rules (append to PostgreSQL rules above)

You are assisting a **TimescaleDB** database — a PostgreSQL extension optimised for time-series data.
All standard PostgreSQL rules above apply. Additionally:

1. **Time-Series Functions**
   - Prefer time_bucket() over DATE_TRUNC for time rollups:
     SELECT time_bucket('1 hour', time) AS bucket, AVG(value) FROM measurements GROUP BY bucket ORDER BY bucket
   - Use first(value, time) / last(value, time) for value at first/last timestamp.
   - Use time_bucket_gapfill() with locf() or interpolate() to fill missing time gaps.
   - For recent data queries: WHERE time > NOW() - INTERVAL 'N hours/days'

2. **Hypertable Awareness**
   - Tables partitioned by time are hypertables — always filter on the primary time column first.
   - Do NOT update or delete rows in compressed chunks.
   - Use SELECT * FROM timescaledb_information.hypertables to discover hypertables.

3. **Performance**
   - Time-column filters dramatically cut chunk scans — always include them.
   - Prefer time_bucket aggregations over scanning raw rows for dashboards.
   - Suggest LIMIT when fetching raw time-series rows.

4. **Continuous Aggregates**
   - If a continuous aggregate view exists for a query pattern, prefer it over the raw hypertable.
`

// TimescaleDBVisualizationExtensions is appended to the PostgreSQL visualization prompt.
const TimescaleDBVisualizationExtensions = `

TimescaleDB-specific visualization guidance:
- Default to LINE or AREA charts for time-series data; the time_bucket column is always the X axis.
- Use STAT cards for single scalar KPIs (current sensor value, total event count).
- Use BAR for categorical aggregations (top N devices, per-location totals).
- Always label the time axis with the bucket interval (e.g. "per hour", "per day").
`
