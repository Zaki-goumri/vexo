# Users, keys and policies

vexo uses an AWS-style IAM model.

- A **user** has a password (for the console) and access keys (for the S3 API).
- An **access key** is an `accessKey` + `secret` pair. It belongs to one user.
- A **policy** is a JSON document that allows or denies actions.
- A **group** holds users. Policies on a group apply to all its users.

A user can do nothing until a policy allows it.

## The root user

On first start vexo creates a root user with the built-in `root` policy. That policy allows everything:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow", "Action": ["*"], "Resource": ["*"] }
  ]
}
```

Use root to set things up. Give other people their own users.

## Give someone read-only access to one bucket

You can do all of this in the web console. Here is the same thing with the console API, so you can script it.

```bash
C=http://localhost:9091

curl -c cookie.txt -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"YOUR_PASSWORD"}' $C/api/login
```

**1. Create a policy**

```bash
curl -b cookie.txt -H 'Content-Type: application/json' $C/api/policies -d '{
  "name": "photos-read",
  "statement": [{
    "Effect": "Allow",
    "Action": ["s3:GetObject", "s3:ListBucket"],
    "Resource": ["arn:aws:s3:::photos", "arn:aws:s3:::photos/*"]
  }]
}'
```

**2. Create a user**

```bash
curl -b cookie.txt -H 'Content-Type: application/json' $C/api/users \
  -d '{"username":"alice","password":"a-strong-password"}'
```

**3. Attach the policy**

```bash
curl -b cookie.txt -X PATCH -H 'Content-Type: application/json' \
  $C/api/users/alice/policies -d '{"policies":["photos-read"]}'
```

**4. Create an access key**

```bash
curl -b cookie.txt -X POST $C/api/users/alice/keys
```

```json
{"accessKey": "HoqEnoL6b-JcsMfLGdgU", "secret": "4Bbb..."}
```

!!! warning
    The secret is shown only once. Save it now.

Alice can now list and read `photos`, and nothing else.

## How policies work

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:*"],
      "Resource": ["arn:aws:s3:::photos/*"]
    },
    {
      "Effect": "Deny",
      "Action": ["s3:DeleteObject"],
      "Resource": ["arn:aws:s3:::photos/archive/*"]
    }
  ]
}
```

Rules:

1. vexo collects all policies of the user and of the user's groups.
2. If any statement **denies** the action, the request is denied.
3. Else, if any statement **allows** it, the request is allowed.
4. Else, it is denied.

`*` is a wildcard in `Action` and `Resource`. `s3:*` means every S3 action. `arn:aws:s3:::photos/*` means every object in `photos`.

!!! tip
    `s3:ListBucket` checks the bucket ARN (`arn:aws:s3:::photos`). Object actions check the object ARN (`arn:aws:s3:::photos/key`). Add both if you need both.

See the full list of actions in [Using the S3 API](s3-api.md#permissions-per-request).

## Remove a key

Delete a key from the console, or with `DELETE /api/keys/<accessKey>`. Requests signed with it will fail with `AccessDenied`.
