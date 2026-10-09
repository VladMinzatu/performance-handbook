-- wrk script: POST a ~200-byte JSON event to /events.
wrk.method = "POST"
wrk.path = "/events"
wrk.headers["Content-Type"] = "application/json"
wrk.body = '{"type":"order.created","order_id":"o-1842","customer_id":"c-77",'
  .. '"amount_cents":4599,"currency":"EUR","source":"checkout",'
  .. '"ts":"2026-10-09T12:00:00Z","note":"' .. string.rep("x", 40) .. '"}'
