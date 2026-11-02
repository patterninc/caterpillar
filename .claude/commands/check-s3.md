Verify that an S3 bucket/path exists and is accessible. The user will provide a bucket name or full S3 path.

Run these checks:

1. **Bucket exists** — `aws s3api head-bucket --bucket <bucket>`

2. **Bucket region** — `aws s3api get-bucket-location --bucket <bucket>`
   - Report the actual region (important for pipeline `region` field)

3. **Path check** — If the user gave a full path (`s3://bucket/prefix/`):
   - List objects: `aws s3 ls <path> --max-items 5`
   - Report count and sample filenames

4. **Bucket properties** — Report:
   - Versioning: `aws s3api get-bucket-versioning --bucket <bucket>`
   - Encryption: `aws s3api get-bucket-encryption --bucket <bucket>` (may need KMS permissions)
   - Public access block: `aws s3api get-public-access-block --bucket <bucket>`

5. **Write test** (only if user asks) — Check if write is possible:
   - `aws s3api put-object --bucket <bucket> --key _caterpillar_write_test --body /dev/null`
   - Then delete it: `aws s3api delete-object --bucket <bucket> --key _caterpillar_write_test`

6. **Pipeline implications** — Based on findings, suggest:
   - The correct `region` value for the pipeline `file` task
   - Whether `{{ macro "uuid" }}` or `{{ macro "timestamp" }}` is needed in write paths
   - Whether `success_file: true` is appropriate

Report a clear summary. If access is denied, list the IAM permissions needed (`s3:GetObject`, `s3:PutObject`, `s3:ListBucket`).
