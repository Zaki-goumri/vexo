# Getting started

You can run vexo with Docker or build it from source.

## Option 1: Docker Compose

This is the easiest way.

```bash
git clone https://github.com/Zaki-goumri/vexo.git
cd vexo
cp .env.example .env
docker compose up -d --build
```

Data is kept in the `vexo-data` Docker volume, mounted at `/data`.

## Option 2: Docker

```bash
docker build -t vexo .
docker run -d --name vexo \
  -p 9090:9090 -p 9091:9091 -p 3001:3001 \
  -e VEXO_ROOT_USER=admin \
  -v vexo-data:/data \
  vexo
```

## Option 3: From source

You need Go 1.26+. Node.js is only needed if you change the web console.

```bash
git clone https://github.com/Zaki-goumri/vexo.git
cd vexo
make build
make run
```

The web console is embedded in the Go binary from `internal/console/web/dist`. If you edit the frontend, rebuild it first:

```bash
cd web
npm install
npm run build
cd ..
make build
```

## Get your root credentials

On the first start, vexo creates a root user. It prints the credentials once and saves them to a file:

```bash
cat volume/.vexo.root.key
```

With Docker, the file is inside the volume:

```bash
docker compose exec vexo cat /data/.vexo.root.key
```

The file looks like this:

```text
username=admin
password=...
accessKey=...
secret=...
```

- `username` + `password` log you into the web console.
- `accessKey` + `secret` sign your S3 requests.

!!! note
    The root user is only created when there are no users yet. Changing `VEXO_ROOT_USER` later does nothing.

## Check it works

Open the console at [http://localhost:9091](http://localhost:9091) and log in.

Or create a bucket from the terminal:

```bash
AK=$(sed -n 's/^accessKey=//p' volume/.vexo.root.key)
SK=$(sed -n 's/^secret=//p' volume/.vexo.root.key)

curl --aws-sigv4 "aws:amz:us-east-1:s3" --user "$AK:$SK" \
  -X PUT http://localhost:9090/my-bucket
```

Next: [Using the S3 API](s3-api.md).

## Run the tests

```bash
make test
```
