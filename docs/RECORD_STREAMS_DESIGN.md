# Record Streams Design Document

## Overview

This document outlines the design for integrating RecordStream-like functionality into gosh, leveraging the unique M28 Lisp integration to create a powerful data processing shell that surpasses traditional Unix tools.

## Vision

Transform gosh from a traditional shell into a data-oriented shell where:
- Every command can emit structured records (JSON objects)
- Lisp expressions seamlessly operate on record streams
- Complex data processing becomes as simple as piping commands
- The shell understands and manipulates structured data natively

## Core Concepts

### 1. Records

A record is a JSON object that flows through the pipeline:

```json
{
  "pid": 1234,
  "user": "alice",
  "cpu": 45.2,
  "memory": 1024000,
  "command": "firefox",
  "started": "2024-06-04T10:30:00Z"
}
```

### 2. Record Streams

A record stream is a sequence of records separated by newlines. This format is:
- Human readable
- Machine parseable  
- Compatible with existing Unix tools
- Streamable (no need to load entire datasets into memory)

### 3. Hybrid Shell/M28 Processing

The key innovation is seamless integration between shell commands and M28's Python-Lisp expressions:

```bash
# Shell command producing records
ps --records | 
# M28 transformation inline using Python-like syntax
(map (lambda (r) {"memory_mb": r["memory"] / 1024, **r}) records) |
# Back to shell command
sort-by memory_mb desc |
# Another M28 operation
[r for r in records if r["memory_mb"] > 100][:10]
```

## Architecture

### Record Protocol

Commands can implement the record protocol by:

1. **Accepting `--records` flag** to output structured data
2. **Auto-detection** based on output destination (pipe vs terminal)
3. **Format negotiation** via environment variables

```go
type RecordEmitter interface {
    EmitRecord(record map[string]interface{}) error
    EmitError(err error)
    Flush() error
}

type RecordConsumer interface {
    ConsumeRecord(record map[string]interface{}) error
    EndOfStream() error
}
```

### M28 Integration Points

1. **M28 as First-Class Pipeline Component**
   ```bash
   # M28 expressions in parentheses are evaluated as stream processors
   cat data.json | (filter (lambda (r) (> r["age"] 21)) records) | count
   
   # Using M28's Python-like list comprehensions
   cat data.json | [r for r in records if r["age"] > 21] | count
   ```

2. **M28 Stream Processing Functions**
   ```python
   # Define reusable stream processors
   (def young_users (records)
     [r for r in records if r["age"] < 30])
   
   # Define with generator for large streams
   (def filter_active (records)
     (for r in records
       (if r["active"]
         (yield r))))
   
   # Use in pipeline
   cat users.json | (young_users) | select name email
   ```

3. **M28-Defined Aggregations**
   ```python
   # Using M28's Python-like syntax
   (def average (field records)
     (= values [r[field] for r in records if field in r])
     (if values
       (/ (sum values) (len values))
       None))
   
   # More complex aggregation with grouping
   (def group_aggregate (key_field agg_field records)
     (= groups {})
     (for r in records
       (= key r[key_field])
       (if key not in groups
         (= groups[key] []))
       (groups[key].append r[agg_field]))
     (for (k, vals) in (items groups)
       (yield {"key": k, "avg": (sum vals) / (len vals)})))
   ```

## Command Categories

### 1. Input/Generation Commands

Commands that produce record streams from various sources:

```bash
# From files
from-json file.json
from-csv data.csv --headers
from-yaml config.yaml
from-log /var/log/syslog --pattern nginx

# From system
ps --records
ls --records
netstat --records
docker-ps --records

# From APIs
http GET api.example.com/users
```

### 2. Transformation Commands

Commands that transform record streams:

```bash
# Selection and projection
where {.age > 21}                    # Filter records
select name email age                 # Project fields
rename old-name:new-name             # Rename fields

# Computation
compute tax:{.income * 0.3}         # Add computed fields
transform {.price = .price * 1.1}   # Modify in place

# M28 transformations using Python-like syntax
(map (lambda (r) {**r, "name": r["name"].upper()}) records)  # Uppercase names
[r for r in records if "email" in r]                         # Only records with email
```

### 3. Aggregation Commands

Commands that aggregate record streams:

```bash
# Basic aggregations
count
sum sales
average response-time
min/max timestamp

# Grouping
group-by category | count
group-by user | sum amount
group-by endpoint status | count

# Window functions
window 5m | average cpu
window 100 | moving-average price
```

### 4. Output Commands

Commands that format record streams for consumption:

```bash
# Formatting
to-table
to-csv
to-json --pretty
to-yaml
to-chart --x date --y value

# Storage
to-file output.json
to-sqlite db.sqlite table_name
to-http POST api.example.com/bulk
```

## Key Specifications

### Nested Field Access

Support RecordStream-style key specifications:

```bash
# Nested hash access with /
select user/name user/email

# Array access with #
select items/#0/price items/#1/price

# Wildcard selection
select user/*

# Fuzzy matching with @
select @nam  # matches "name" or "full_name"
```

### Pipeline Variables

Records can be referenced in M28 expressions:

```bash
# Direct dictionary access in M28
where (> $["age"] 21)

# Previous record for delta operations  
compute delta:{$["value"] - $$["value"]}

# Window operations with M28 generators
(def sliding_window (n records)
  (= window [])
  (for r in records
    (window.append r)
    (if (> (len window) n)
      (= window window[1:]))
    (if (== (len window) n)
      (yield window))))

# Use in pipeline for trend analysis
| (sliding_window 3) | (map (lambda (w) {"trend": w[-1]["value"] - w[0]["value"]}) windows)
```

## Implementation Phases

### Phase 1: Core Infrastructure
- [ ] Record type definition and serialization
- [ ] Basic record I/O commands (from-json, to-json)
- [ ] Pipeline protocol for record streams
- [ ] Integration with existing commands via --records flag

### Phase 2: M28 Stream Processing
- [ ] M28 stream processing with Python-like syntax
- [ ] List comprehensions for record filtering
- [ ] Generator support for large streams
- [ ] Custom aggregation functions using M28's def syntax
- [ ] Integration with M28's built-in map, filter, reduce

### Phase 3: Advanced Features
- [ ] Automatic format detection
- [ ] Nested field access and fuzzy matching
- [ ] Window operations
- [ ] Join operations between streams
- [ ] Parallel processing for large datasets

### Phase 4: Ecosystem
- [ ] Plugin system for custom formats
- [ ] Library of common transformations
- [ ] Integration with external data sources
- [ ] Performance optimizations

## Example Use Cases

### Log Analysis
```bash
# Find the top 10 error-producing endpoints using M28
from-log nginx.log |
[r for r in records if r["status"] >= 500] |
group-by endpoint |
count |
sort-by count desc |
(lambda (records) records[:10]) |
to-table
```

### System Monitoring
```bash
# Alert on high memory processes with M28 transformations
ps --records |
(map (lambda (r) {**r, "memory_gb": r["memory"] / 1024 / 1024}) records) |
[r for r in records if r["memory_gb"] > 2] |
select pid user command memory_gb |
to-alert "High memory usage"
```

### Data Processing
```bash
# Process CSV sales data with M28's Python-like syntax
from-csv sales.csv |
[r for r in records if r["amount"] > 0] |
(def process_sale (r)
  (= tax r["amount"] * 0.08)
  {**r, 
   "tax": tax,
   "total": r["amount"] + tax,
   "quarter": (datetime.strptime r["date"] "%Y-%m-%d").quarter}) |
(map process_sale records) |
group-by quarter |
aggregate sum:total average:total count |
to-chart --x quarter --y sum
```

### API Integration with M28 Classes
```bash
# Define a record processor class in M28
(class UserMerger
  (def __init__ (self)
    (= self.seen_emails set()))
  
  (def process (self records)
    (for r in records
      (if r["email"] not in self.seen_emails
        (self.seen_emails.add r["email"])
        (yield r)))))

# Use in pipeline
parallel {
  http GET api1.example.com/users | tag source:api1
  http GET api2.example.com/users | tag source:api2
} |
(= merger (UserMerger)) |
(merger.process) |
[r for r in records if r["active"]] |
to-json > combined-users.json
```

## Benefits

1. **Power**: Complex data processing without leaving the shell
2. **Composability**: Small, focused commands that work together
3. **Flexibility**: Use shell commands or Lisp as needed
4. **Performance**: Stream processing without loading everything into memory
5. **Discoverability**: Structured data enables better autocomplete and help

## Conclusion

By combining RecordStream concepts with M28 Lisp integration, gosh can become the most powerful shell for data processing and system administration. It maintains the Unix philosophy of composable tools while adding modern data processing capabilities that rival specialized tools like jq, awk, and even SQL databases.