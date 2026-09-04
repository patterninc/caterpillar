Verify that an HTTP API endpoint is reachable and responding. The user will provide a URL and optionally auth details.

Run these checks:

1. **Endpoint reachable** — `curl -s -o /dev/null -w "%{http_code} %{time_total}s" --max-time 10 <url>`
   - Report: status code, response time, redirect chain (if any)

2. **Response preview** — `curl -s --max-time 10 <url> | head -c 2000`
   - If JSON: pretty-print and show structure
   - If HTML/XML: note the content type
   - If empty: flag it

3. **Auth test** — If the user provides auth details:
   - Bearer: `curl -s -H "Authorization: Bearer <token>" <url>`
   - API key: `curl -s -H "X-Api-Key: <key>" <url>`
   - Basic: `curl -s -u <user>:<pass> <url>`
   - Report whether auth succeeds (2xx) or fails (401/403)

4. **Pagination check** — If the response is JSON:
   - Look for common pagination fields: `next`, `next_page`, `next_url`, `links.next`, `cursor`, `offset`, `page`
   - Suggest the `next_page` JQ expression for the pipeline

5. **TLS check** — `curl -vI --max-time 5 <url> 2>&1 | grep -E "SSL|TLS|certificate"`
   - Report TLS version and certificate validity
   - Flag if using `http://` instead of `https://`

6. **Pipeline implications** — Based on findings:
   - Whether `method: GET` or `POST` is needed
   - Suggested `next_page` expression if paginated
   - Whether `max_retries` should be increased (slow response)
   - Whether `expected_statuses` needs adjusting

Report a clear summary. If connection fails, explain common causes (DNS, firewall, TLS, auth).
