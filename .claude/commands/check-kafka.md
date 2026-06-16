Verify that a Kafka broker and topic are reachable. The user will provide bootstrap server and topic name.

Run these checks:

1. **Connectivity** — Check if the broker is reachable:
   - `nc -zv <host> <port> 2>&1` (extract host/port from bootstrap_server)
   - If unreachable, suggest checking VPN, security groups, or firewall rules

2. **Topic exists** — Try to list/describe the topic:
   - With kcat: `kcat -b <bootstrap_server> -L -t <topic> 2>&1 | head -20`
   - Without kcat: `echo "Topic check requires kcat — install with: brew install kcat"`

3. **Topic metadata** (if kcat available) — Report:
   - Partition count
   - Replica count
   - Whether the topic has messages (try consuming 1 with timeout)

4. **Auth check** — If the user mentions SCRAM/SASL/TLS:
   - Test with kcat using provided auth: `kcat -b <server> -t <topic> -X security.protocol=SASL_SSL -X sasl.mechanisms=SCRAM-SHA-512 -X sasl.username=<user> -X sasl.password=<pass> -L 2>&1 | head -10`
   - If no kcat, suggest a minimal probe pipeline to test connectivity

5. **Pipeline implications** — Based on findings, suggest:
   - Whether `server_auth_type: tls` is needed
   - Whether `user_auth_type: scram` or `sasl` is needed
   - A sensible `group_id` based on the topic name
   - Whether `retry_limit` should be set (empty topic)

Report a clear summary. If connection fails, explain common causes (wrong port, TLS required, auth mismatch).
