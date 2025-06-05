# M28-Enhanced gosh Examples

These examples show how the proposed M28 enhancements would make gosh the most powerful shell for data processing.

## Example 1: Log Analysis Pipeline

### Current (using jq and traditional tools)
```bash
# Complex and hard to read
cat nginx.log | \
  grep -E '"status":[4-5][0-9]{2}' | \
  jq -r '. | select(.status >= 400) | 
    {endpoint: .endpoint, status: .status, time: .timestamp}' | \
  jq -s 'group_by(.endpoint) | 
    map({endpoint: .[0].endpoint, count: length, 
         errors: map(.status) | unique})' | \
  jq -r '.[] | [.endpoint, .count, (.errors | join(","))] | @csv'
```

### With M28-Enhanced gosh
```bash
# Clear, readable, and powerful
from-log nginx.log |
(def analyze-errors (records)
  # Use Python-like syntax with path access
  errors = [r for r in records if r.status >= 400]
  
  # Group and summarize
  by_endpoint = {}
  (for e in errors
    (if e.endpoint not in by_endpoint
      (= by_endpoint[e.endpoint] {"count": 0, "statuses": set()}))
    by_endpoint[e.endpoint]["count"] += 1
    by_endpoint[e.endpoint]["statuses"].add e.status)
  
  # Transform to records
  (for (endpoint, data) in (items by_endpoint)
    (yield {
      "endpoint": endpoint,
      "error_count": data["count"],
      "status_codes": (sorted data["statuses"]),
      "severity": (if (any [s >= 500 for s in data["statuses"]]) 
                    "critical" "warning")
    }))) |
(analyze-errors) |
sort-by error_count desc |
to-table
```

## Example 2: Multi-Source Data Enrichment

### Current (multiple tools and temp files)
```bash
# Fetch user data
curl -s api.example.com/users > users.json

# Fetch order data  
curl -s api.example.com/orders > orders.json

# Complex join with jq
jq -s '.[0] as $users | .[1] as $orders |
  $users | map(. as $user |
    {user: ., 
     orders: ($orders | map(select(.user_id == $user.id)))})' \
  users.json orders.json > enriched.json

# Calculate metrics
jq '.[] | {
  name: .user.name,
  total_spent: (.orders | map(.amount) | add),
  order_count: (.orders | length)
}' enriched.json
```

### With M28-Enhanced gosh
```bash
# Elegant parallel fetch and join
parallel {
  http GET api.example.com/users | tag-as users
  http GET api.example.com/orders | tag-as orders
} |
(def enrich-users (data)
  # Build order index for O(1) lookups
  orders_by_user = {}
  (for o in data.orders
    (if o.user_id not in orders_by_user
      (= orders_by_user[o.user_id] []))
    orders_by_user[o.user_id].append o)
  
  # Enrich each user
  (for u in data.users
    user_orders = orders_by_user.get(u.id, [])
    (yield {
      **u,  # Spread user fields
      "total_spent": (sum [o.amount for o in user_orders] or 0),
      "order_count": (len user_orders),
      "avg_order": (if user_orders 
                     (sum [o.amount for o in user_orders]) / (len user_orders)
                     0),
      "vip": (sum [o.amount for o in user_orders] or 0) > 1000
    }))) |
(enrich-users) |
[u for u in records if u.vip] |  # Filter VIP users
sort-by total_spent desc |
to-csv vip-users.csv
```

## Example 3: Real-time System Monitoring

### Current (complex script with multiple tools)
```bash
#!/bin/bash
while true; do
  # Get process info
  ps_data=$(ps aux | awk 'NR>1 {print $1","$2","$3","$4","$11}')
  
  # Get disk info
  disk_data=$(df -h | awk 'NR>1 {print $1","$6","$5}')
  
  # Parse and analyze (very messy)
  echo "$ps_data" | while IFS=',' read -r user pid cpu mem cmd; do
    if (( $(echo "$cpu > 80" | bc -l) )); then
      echo "High CPU: $user $cmd $cpu%"
    fi
  done
  
  sleep 5
done
```

### With M28-Enhanced gosh
```bash
# Real-time monitoring with record streams
stream-interval 5s {
  parallel {
    ps --records | tag-as processes
    df --records | tag-as disks
    netstat --records | tag-as connections
  }
} |
(def monitor (snapshot)
  # Analyze processes
  cpu_hogs = [p for p in snapshot.processes if p.cpu > 80]
  mem_hogs = [p for p in snapshot.processes if p.memory_mb > 1000]
  
  # Analyze disks
  full_disks = [d for d in snapshot.disks if d.use_percent > 90]
  
  # Analyze network
  connections_by_state = {}
  (for c in snapshot.connections
    (= state c.state)
    (if state not in connections_by_state
      (= connections_by_state[state] 0))
    connections_by_state[state] += 1)
  
  # Generate alert record
  (yield {
    "timestamp": (datetime.now),
    "alerts": {
      "high_cpu": [{proc: p.command, cpu: p.cpu, user: p.user} 
                   for p in cpu_hogs],
      "high_memory": [{proc: p.command, mb: p.memory_mb} 
                      for p in mem_hogs],
      "disk_full": [{mount: d.mount, used: d.use_percent} 
                    for d in full_disks],
      "connections": connections_by_state
    },
    "healthy": (not (cpu_hogs or mem_hogs or full_disks))
  })) |
(monitor) |
tee system-health.jsonl |  # Save history
(def alert-on-critical (records)
  (for r in records
    (if (not r.healthy)
      # Send alerts for critical issues
      (if r.alerts.high_cpu
        (notify-slack f"CPU Alert: {r.alerts.high_cpu}"))
      (if r.alerts.disk_full
        (notify-pager f"Disk Full: {r.alerts.disk_full}")))))
```

## Example 4: Data Transformation and Validation

### Current (with jq and multiple steps)
```bash
# Validate and transform data with jq is painful
cat input.json | jq '
  .users | map(
    select(.email | test("^[^@]+@[^@]+$")) |
    select(.age >= 18) |
    {
      id: .id,
      name: .name,
      email: (.email | ascii_downcase),
      adult: true,
      created: (now | strftime("%Y-%m-%d"))
    }
  )'
```

### With M28-Enhanced gosh
```bash
cat users.json |
(import re)
(import datetime)

# Define schema with validation
(defschema ValidUser {
  "id": int,
  "name": (str, (lambda (s) (len s) > 0)),
  "email": (str, (lambda (e) (re.match r"^[^@]+@[^@]+\.[^@]+$" e))),
  "age": (int, (lambda (a) a >= 0))
})

# Transform pipeline
(def transform-users (records)
  (for r in records
    (try
      # Validate
      (validate ValidUser r)
      
      # Transform
      (yield {
        **r,
        "email": r.email.lower(),
        "adult": r.age >= 18,
        "age_group": (cond
          (< r.age 18) "minor"
          (< r.age 30) "young_adult"
          (< r.age 50) "adult"
          True "senior"),
        "created": (datetime.now).strftime("%Y-%m-%d"),
        "username": (re.sub r"@.*" "" r.email)
      })
    (except ValidationError as e
      # Log invalid records
      (yield {"error": str(e), "original": r, "type": "validation_error"})
    )))) |
(transform-users) |
# Separate valid and invalid records
(def split-by-error (records)
  (= valid [])
  (= invalid [])
  (for r in records
    (if "error" in r
      invalid.append r
      valid.append r))
  {"valid": valid, "invalid": invalid}) |
(= result (split-by-error records)) |
# Save both
(do
  (write-records "valid_users.jsonl" result["valid"])
  (write-records "errors.jsonl" result["invalid"])
  (print f"Processed {len(result['valid'])} valid, {len(result['invalid'])} invalid"))
```

## Example 5: Complex Business Logic

### With M28-Enhanced gosh
```bash
# E-commerce order processing pipeline
from-kafka orders-topic |
(class OrderProcessor
  (def __init__ (self inventory_api pricing_api)
    (= self.inventory inventory_api)
    (= self.pricing pricing_api))
  
  (def process (self order)
    # Check inventory
    (for item in order.items
      (= stock (self.inventory.check item.sku))
      (if (< stock item.quantity)
        (return {"status": "failed", "reason": f"Insufficient stock for {item.sku}"})))
    
    # Calculate pricing
    (= subtotal 0)
    (= discounts [])
    (for item in order.items
      (= price (self.pricing.get_price item.sku))
      (= item_total (* price item.quantity))
      subtotal += item_total
      
      # Apply discounts
      (if (>= item.quantity 10)
        discounts.append {"type": "bulk", "amount": (* item_total 0.1)}))
    
    (= total (- subtotal (sum [d.amount for d in discounts])))
    
    # Apply tax
    (= tax (* total 0.08))
    (= final_total (+ total tax))
    
    (return {
      **order,
      "status": "approved",
      "pricing": {
        "subtotal": subtotal,
        "discounts": discounts,
        "tax": tax,
        "total": final_total
      },
      "processed_at": (datetime.now)
    }))) |

# Initialize processor
(= processor (OrderProcessor 
  inventory_api="http://inventory.internal/api"
  pricing_api="http://pricing.internal/api")) |

# Process orders
(map processor.process records) |

# Route based on status
(def route-orders (orders)
  (for o in orders
    (match o.status
      "approved" (yield {"queue": "fulfillment", "order": o})
      "failed" (yield {"queue": "manual-review", "order": o})
      _ (yield {"queue": "error", "order": o})))) |

(route-orders) |

# Send to appropriate queues
(def send-to-queues (routed)
  (= queues {})
  (for r in routed
    (if r.queue not in queues
      (= queues[r.queue] []))
    queues[r.queue].append r.order)
  
  # Send each queue
  (for (queue, orders) in (items queues)
    (to-kafka f"{queue}-topic" orders)
    (print f"Sent {len(orders)} orders to {queue}")))
```

## Key Advantages

1. **Readability**: Python-like syntax is familiar and clear
2. **Power**: Full programming language with classes, functions, error handling
3. **Streaming**: Generators and lazy evaluation for large datasets
4. **Integration**: Easy to call APIs and external services
5. **Debugging**: Can add print statements, logging, breakpoints
6. **Reusability**: Define functions and classes once, use everywhere
7. **Type Safety**: Optional schema validation
8. **Performance**: Lazy evaluation and parallel processing

These examples show how M28-enhanced gosh would be superior to traditional Unix tools, jq, and even specialized data processing frameworks.