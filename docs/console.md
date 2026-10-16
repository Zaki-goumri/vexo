# Web console

The web console runs on port `9091`. Open [http://localhost:9091](http://localhost:9091) and log in with your username and password.

The root password is in `.vexo.root.key`. See [Getting started](getting-started.md#get-your-root-credentials).

## What you can do

- Create and delete buckets.
- Upload, download, rename and delete files.
- Make a bucket public or private.
- Create share links that expire.
- Create users and access keys.
- Write policies and attach them to users.

## Share a file

Share links let anyone download one file, without logging in, until the link expires.

1. Open the bucket.
2. Pick a file and choose **Share**.
3. Pick how long the link lasts.
4. Copy the link.

Links look like this:

```text
http://localhost:9091/api/public/<bucket>/<key>?exp=<unix-time>&sig=<signature>
```

The signature uses the secret in `.vexo.share.key`. If you delete that file, all old links stop working.

## Public buckets

A public bucket lets anyone read its files with no link signature:

```text
http://localhost:9091/api/public/<bucket>/<key>
```

Only reads are public. Writes still need a login or a signed S3 request.

!!! warning
    Anyone who knows the bucket name and the file name can download the file. Keep private data in private buckets.

## Develop the console

The console is a Vue 3 + TypeScript app in `web/`.

```bash
make build && make run    # start the backend first
cd web
npm install
npm run dev               # http://localhost:5173
```

The dev server sends `/api` calls to `localhost:9091`.

To ship your changes, run `npm run build`. It writes to `internal/console/web/dist`, which is embedded in the Go binary.
