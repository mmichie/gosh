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

### 3. Hybrid Shell/Lisp Processing

The key innovation is seamless integration between shell commands and Lisp expressions:

```bash
# Shell command producing records
ps --records | 
# Lisp transformation inline
(map #(assoc % :memory-mb (/ (:memory %) 1024))) |
# Back to shell command
sort-by memory-mb desc |
# Another Lisp operation
(take 10)
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

1. **Lisp as First-Class Pipeline Component**
   ```bash
   # Lisp expressions in parentheses are evaluated as stream processors
   cat data.json | (filter #(> (:age %) 21)) | count
   ```

2. **Lisp Comprehensions for Records**
   ```lisp
   # Define reusable stream processors
   (defstream young-users []
     (filter #(< (:age %) 30)))
   
   # Use in pipeline
   cat users.json | (young-users) | select name email
   ```

3. **Lisp-Defined Aggregations**
   ```lisp
   (defagg average [field]
     (/ (sum field) (count)))
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

# Lisp transformations
(map #(update % :name str/upper))   # Uppercase names
(filter #(contains? % :email))      # Only records with email
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

Records can be referenced in Lisp expressions:

```bash
# $ refers to the current record
where (> (get $ :age) 21)

# $$ refers to the previous record (for delta operations)
compute delta:{.value - $$.value}

# $n refers to the nth record in a window
window 3 | compute trend:{$2.value - $0.value}
```

## Implementation Phases

### Phase 1: Core Infrastructure
- [ ] Record type definition and serialization
- [ ] Basic record I/O commands (from-json, to-json)
- [ ] Pipeline protocol for record streams
- [ ] Integration with existing commands via --records flag

### Phase 2: M28 Stream Processing
- [ ] Lisp stream processing operators (map, filter, reduce)
- [ ] Inline Lisp expressions in pipelines
- [ ] Lisp comprehension syntax
- [ ] Custom aggregation functions in Lisp

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
# Find the top 10 error-producing endpoints
from-log nginx.log |
where {.status >= 500} |
group-by endpoint |
count |
sort-by count desc |
(take 10) |
to-table
```

### System Monitoring
```bash
# Alert on high memory processes
ps --records |
(map #(assoc % :memory-gb (/ (:memory %) 1024 1024))) |
where {.memory-gb > 2} |
select pid user command memory-gb |
to-alert "High memory usage"
```

### Data Processing
```bash
# Process CSV sales data with complex transformations
from-csv sales.csv |
where {.amount > 0} |
(map #(assoc % 
  :tax (* (:amount %) 0.08)
  :total (+ (:amount %) (:tax %))
  :quarter (time/quarter (:date %)))) |
group-by quarter |
aggregate sum:total average:total count |
to-chart --x quarter --y sum
```

### API Integration
```bash
# Combine data from multiple APIs
parallel {
  http GET api1.example.com/users | tag source:api1
  http GET api2.example.com/users | tag source:api2
} |
(unique-by :email) |
where {.active == true} |
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