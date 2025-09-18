With `python scripts/python/stress_test.py --base-url http://localhost:8080         --users 10 --tables-per-user 3 --rows-per-table 500 --search-requests 500`

Machine:
- Laptop running postgresql (no modifications base setup) in WSL2, running go server with rate limiting disabled

Results:

(base) rpatt@GB-PF50E3GZ:~/dev/go_cmms_v2$ python scripts/python/stress_test.py --base-url http://localhost:8080         --users 10 --tables-per-user 3 --rows-per-table 500 --search-requests 500

=== Stress Test Summary ===

Stage: signup
  requests: 10
  success: 10
  failure: 0
  status_counts: {201: 10}
  avg_ms: 340.09
  min_ms: 274.51
  max_ms: 399.52
  p50_ms: 336.53
  p95_ms: 392.81

Stage: create_table
  requests: 30
  success: 30
  failure: 0
  status_counts: {201: 30}
  avg_ms: 462.76
  min_ms: 6.24
  max_ms: 1677.34
  p50_ms: 372.58
  p95_ms: 1452.18

Stage: add_column
  requests: 150
  success: 150
  failure: 0
  status_counts: {201: 150}
  avg_ms: 333.52
  min_ms: 5.26
  max_ms: 2216.29
  p50_ms: 96.79
  p95_ms: 1127.37

Stage: add_row
  requests: 15043
  success: 15000
  failure: 43
  status_counts: {201: 15000, 400: 43}
  avg_ms: 151.64
  min_ms: 36.98
  max_ms: 4036.68
  p50_ms: 107.74
  p95_ms: 273.86

Stage: search
  requests: 15000
  success: 15000
  failure: 0
  status_counts: {200: 15000}
  avg_ms: 1896.04
  min_ms: 72.70
  max_ms: 14368.21
  p50_ms: 1841.81
  p95_ms: 2829.83