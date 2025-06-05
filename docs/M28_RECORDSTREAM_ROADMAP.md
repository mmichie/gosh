# M28 Record Stream and JSON Enhancement Roadmap

This document outlines proposed enhancements to M28 to make it a best-in-class language for record stream processing and JSON manipulation, surpassing tools like jq while maintaining Pythonic simplicity.

## Core Philosophy

- **Simpler than jq**: Use Python-like syntax instead of jq's domain-specific language
- **More powerful than jq**: Full programming language with classes, generators, etc.
- **Stream-oriented**: First-class support for record streams (line-delimited JSON)
- **Path-based access**: Simple syntax for nested data access
- **Type-aware**: Smart handling of JSON types with Python semantics

## Phase 1: JSON Path and Query Syntax

### 1.1 Dot Notation for Nested Access

```python
# Current (verbose)
user_name = data["users"][0]["name"]

# Proposed (elegant)
user_name = data.users[0].name

# With null safety
user_name = data?.users?[0]?.name  # Returns None if any part is missing
```

### 1.2 Path-Based Operations

```python
# Get nested value with path
(get-in data ["users" 0 "profile" "email"])
(get-in data "users.0.profile.email")  # String path syntax

# Set nested value
(= data (assoc-in data ["users" 0 "active"] True))

# Update nested value
(= data (update-in data ["users" 0 "score"] + 10))

# Delete nested key
(= data (dissoc-in data ["users" 0 "temp_field"]))
```

### 1.3 Query DSL

```python
# Simple query syntax inspired by MongoDB/JsonPath
(query data {
  "users": {
    "$filter": (lambda (u) u["age"] > 21),
    "$map": (lambda (u) {"name": u["name"], "email": u["email"]})
  }
})

# Or using a more Pythonic approach
(select data 
  .users
  [u for u in users if u.age > 21]
  [{name: u.name, email: u.email} for u in users])
```

## Phase 2: Record Stream Support

### 2.1 Line-Delimited JSON (JSONL)

```python
# Read record stream
(with-records "data.jsonl" as records
  (for r in records
    (process r)))

# Write record stream
(def write-records (filename records)
  (with (open filename "w") as f
    (for r in records
      (f.write (json.dumps r))
      (f.write "\n"))))

# Built-in streaming functions
(import recordstream as rs)

# Read records lazily
(= records (rs.read "huge-file.jsonl"))  # Returns generator

# Process in chunks
(for chunk in (rs.chunks records 1000)
  (process-batch chunk))
```

### 2.2 Stream Processing Functions

```python
# Record-specific map that preserves record structure
(def rmap (fn records)
  (for r in records
    (yield {**r, **fn(r)})))

# Record filtering with multiple conditions
(def rfilter (conditions records)
  (for r in records
    (if (all [(cond r) for cond in conditions])
      (yield r))))

# Record transformation pipelines
(def pipeline (*transforms)
  (lambda (records)
    (reduce (lambda (rs t) (t rs)) transforms records)))

# Usage
(= process (pipeline
  (rfilter [(lambda (r) r["active"]),
            (lambda (r) r["age"] > 18)])
  (rmap (lambda (r) {"full_name": f"{r['first']} {r['last']}"}))
  (rs.sort-by "full_name")))
```

## Phase 3: Data Manipulation Utilities

### 3.1 Dictionary Enhancements

```python
# Select specific keys
(select-keys record ["id" "name" "email"])

# Rename keys
(rename-keys record {"old_name": "new_name", "legacy_id": "id"})

# Deep merge with custom resolution
(deep-merge record1 record2 
  on-conflict=(lambda (k v1 v2) v2))  # Take second value on conflict

# Transform keys
(map-keys str.lower record)  # Lowercase all keys
(map-values float prices)    # Convert all values

# Nested operations
(def deep-map (fn data)
  "Recursively apply function to all values"
  (cond
    (isinstance data dict) 
      {k: (deep-map fn v) for (k, v) in (items data)}
    (isinstance data list)
      [(deep-map fn x) for x in data]
    True
      (fn data)))
```

### 3.2 Collection Operations

```python
# Group by with multiple keys
(group-by records ["category" "status"])

# Pivot operations
(pivot records 
  index="date"
  columns="category"
  values="amount"
  aggfunc=sum)

# Window functions
(def sliding-window (n records)
  (= window [])
  (for r in records
    (window.append r)
    (if (> (len window) n)
      (window.pop 0))
    (yield (list window))))

# Deduplicate by key
(unique-by records "email")
```

## Phase 4: Type System and Validation

### 4.1 Schema Definition

```python
# Define schemas using Python-like syntax
(defschema User {
  "id": int,
  "name": str,
  "email": str,
  "age": (int, (lambda (x) (>= x 0))),  # Type with validation
  "tags": [str],
  "profile": {
    "bio": (str, optional=True),
    "avatar": (str, optional=True)
  }
})

# Validate records
(validate User record)  # Raises on invalid
(valid? User record)    # Returns True/False

# Coerce types
(coerce User raw-data)  # Attempts type conversion
```

### 4.2 Type Conversion Utilities

```python
# Smart type conversion
(def parse-value (s)
  "Parse string to appropriate type"
  (try
    (if (re.match r"^\d+$" s) (int s)
    (elif (re.match r"^\d+\.\d+$" s) (float s)
    (elif (s.lower in ["true" "false"]) (bool s)
    (else s)))
    (except ValueError
      s)))

# Batch conversion
(def parse-records (records)
  (for r in records
    (yield {k: (parse-value v) for (k, v) in (items r)})))
```

## Phase 5: Advanced Features

### 5.1 JSON Diff and Patch

```python
# Compare JSON structures
(= diff (json-diff old-data new-data))
# Returns: {"users": {"0": {"name": {"old": "John", "new": "Johnny"}}}}

# Apply patches
(= patched (json-patch data [
  {"op": "replace", "path": "/users/0/name", "value": "Johnny"},
  {"op": "add", "path": "/users/1", "value": new-user}
])

# Generate patch from diff
(= patch (diff-to-patch diff))
```

### 5.2 Template and Transform

```python
# JSON templates with expressions
(def template {
  "user_id": $.id,
  "full_name": f"{$.first_name} {$.last_name}",
  "adult": (>= $.age 18),
  "tags": [t.lower() for t in $.tags]
})

(= transformed (apply-template template record))

# Recursive template application
(deftransform flatten-addresses
  {"addresses.*": lambda (addr) f"{addr.street}, {addr.city}"})
```

### 5.3 Performance Optimizations

```python
# Compiled path expressions
(= getter (compile-path "users.*.profile.email"))
(= emails (getter data))  # Fast repeated access

# Lazy evaluation
(defn lazy-map (fn records)
  "Returns lazy-evaluated mapped records"
  (make-lazy (map fn records)))

# Parallel processing
(import concurrent)
(= results (concurrent.map process-record records num-workers=4))
```

## Phase 6: Integration Features

### 6.1 Direct Shell Integration

```python
# Execute and parse JSON output
(= containers (sh-json "docker ps --format json"))

# Stream from command
(with-process "tail -f log.json" as proc
  (for line in proc.stdout
    (= record (json.loads line))
    (process record)))
```

### 6.2 Format Converters

```python
# CSV to records
(= records (csv-to-records "data.csv" headers=True))

# Records to various formats
(records-to-csv records "output.csv")
(records-to-yaml records "output.yaml")
(records-to-table records)  # Pretty ASCII table
```

## Benefits Over jq

1. **Simpler Syntax**: 
   - jq: `.users[] | select(.age > 21) | {name, email}`
   - M28: `[{name: u.name, email: u.email} for u in data.users if u.age > 21]`

2. **Full Programming Language**:
   - Define functions, classes, use loops
   - Import libraries, handle errors properly
   - Reuse code across projects

3. **Better Debugging**:
   - Print statements, breakpoints
   - Step through transformations
   - Type checking and validation

4. **Performance**:
   - Lazy evaluation for large datasets
   - Parallel processing support
   - Compiled path expressions

5. **Extensibility**:
   - Easy to add custom functions
   - Integrate with Python libraries
   - Build domain-specific tools

## Implementation Priority

1. **High Priority** (Enables core record stream functionality):
   - Path-based access (`get-in`, `assoc-in`)
   - JSONL streaming support
   - Basic record operations (`select-keys`, `rename-keys`)

2. **Medium Priority** (Improves developer experience):
   - Dot notation for nested access
   - Schema validation
   - Type conversion utilities

3. **Low Priority** (Nice to have):
   - JSON diff/patch
   - Template system
   - Performance optimizations

This roadmap would make M28 the most powerful and user-friendly language for processing JSON and record streams, combining the best of jq, Python, and functional programming.