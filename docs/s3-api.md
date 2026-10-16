# Using the S3 API

The S3 API runs on port `9090`. Every request must be signed with AWS SigV4.

- Use **path-style** URLs: `http://host:9090/<bucket>/<key>`.
- Region: any value works, `us-east-1` is fine.
- Service name: `s3`.

## Set up your keys

Take the keys from `.vexo.root.key`, or create a new key for a user (see [Users, keys and policies](iam.md)).

```bash
export AK=your-access-key
export SK=your-secret
export VEXO=http://localhost:9090
```

## With curl

curl 7.75+ can sign requests with `--aws-sigv4`.

```bash
alias s3c='curl -s --aws-sigv4 "aws:amz:us-east-1:s3" --user "$AK:$SK"'
```

### Buckets

```bash
s3c -X PUT    $VEXO/photos        # create
s3c -I        $VEXO/photos        # check it exists
s3c           $VEXO/              # list all buckets
s3c -X DELETE $VEXO/photos        # delete
```

### Objects

```bash
s3c -X PUT --data-binary @cat.jpg -H "Content-Type: image/jpeg" \
  $VEXO/photos/2026/cat.jpg                    # upload

s3c -o cat.jpg $VEXO/photos/2026/cat.jpg       # download
s3c -I         $VEXO/photos/2026/cat.jpg       # metadata only
s3c "$VEXO/photos?prefix=2026/"                # list with a prefix
s3c -X DELETE  $VEXO/photos/2026/cat.jpg       # delete
```

!!! warning "Use `--data-binary`, not `-T`"
    `curl -T` sends `UNSIGNED-PAYLOAD`. vexo does not support that yet, so the request fails with `invalid signature`. Use `-X PUT --data-binary @file`.

## With other S3 clients

Any client that signs the full payload with SigV4 should work. Set:

- endpoint: `http://localhost:9090`
- path-style addressing: on
- region: `us-east-1`

=== "boto3"

    ```python
    import boto3
    from botocore.config import Config

    s3 = boto3.client(
        "s3",
        endpoint_url="http://localhost:9090",
        aws_access_key_id="AK",
        aws_secret_access_key="SK",
        region_name="us-east-1",
        config=Config(
            s3={"addressing_style": "path"},
            request_checksum_calculation="when_required",
            response_checksum_validation="when_required",
        ),
    )

    s3.create_bucket(Bucket="photos")
    s3.put_object(Bucket="photos", Key="hello.txt", Body=b"hi")
    print(s3.get_object(Bucket="photos", Key="hello.txt")["Body"].read())
    ```

=== "aws-cli"

    ```bash
    export AWS_ACCESS_KEY_ID=$AK
    export AWS_SECRET_ACCESS_KEY=$SK
    export AWS_DEFAULT_REGION=us-east-1
    export AWS_REQUEST_CHECKSUM_CALCULATION=when_required
    aws configure set default.s3.addressing_style path

    aws --endpoint-url http://localhost:9090 s3api create-bucket --bucket photos
    aws --endpoint-url http://localhost:9090 s3api list-buckets
    ```

!!! note
    These clients are not tested in CI yet. Some versions send streaming or unsigned uploads, which vexo does not support. If uploads fail, try curl first.

## Permissions per request

Each request maps to one IAM action. Your policies must allow it.

| Request | Action | Resource |
| --- | --- | --- |
| `GET /` | `s3:ListAllMyBuckets` | `*` |
| `PUT /<bucket>` | `s3:CreateBucket` | `arn:aws:s3:::<bucket>` |
| `DELETE /<bucket>` | `s3:DeleteBucket` | `arn:aws:s3:::<bucket>` |
| `GET` or `HEAD /<bucket>` | `s3:ListBucket` | `arn:aws:s3:::<bucket>` |
| `PUT /<bucket>/<key>` | `s3:PutObject` | `arn:aws:s3:::<bucket>/<key>` |
| `GET` or `HEAD /<bucket>/<key>` | `s3:GetObject` | `arn:aws:s3:::<bucket>/<key>` |
| `DELETE /<bucket>/<key>` | `s3:DeleteObject` | `arn:aws:s3:::<bucket>/<key>` |

## Errors

Errors come back as XML, like AWS:

```xml
<Error>
  <Code>NoSuchKey</Code>
  <Message>object not found</Message>
  <Resource>/photos/missing.txt</Resource>
</Error>
```

| Code | HTTP | Meaning |
| --- | --- | --- |
| `AccessDenied` | 403 | Missing auth, bad signature, disabled key, or policy denies it. |
| `NoSuchBucket` | 404 | The bucket does not exist. |
| `NoSuchKey` | 404 | The object does not exist. |
| `InternalError` | 500 | Something broke on the server. |

The `ListObjects` result shows the tier of each object in `StorageClass` (`hot`, `infrequent` or `cold`).
