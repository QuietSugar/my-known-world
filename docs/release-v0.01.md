# MyKnownWorld v0.01

This is the first tester build: a read-only personal graph viewer with a Go API, React web UI, PostgreSQL snapshot storage and Docker Compose packaging.

## Included

- Read node list/detail, Self connectivity paths, and impact paths.
- Display node provenance labels and certainty.
- Run as a single app image beside PostgreSQL; database is internal to Compose and persists in a named volume.
- Run a dependency-free API smoke test with `python3 scripts/smoke_test.py`.

## Not included yet

- Create, edit, retire, or delete objects and relationships.
- Import/sync preview, provisional review, field-level evidence history, authentication, or multi-user access.
- This version is for source/deployment/usability testing with synthetic sample data, not for storing real personal information.

## Tester paths

### Source

Download the source archive for tag `v0.01`, then run:

```sh
go test -race ./...
cd web && npm ci && npm run build
cd ..
python3 scripts/smoke_test.py --base-url http://127.0.0.1:8081
```

For the API smoke, start `ADDR=:8081 go run ./cmd/myknownworld` in one terminal and run the script in another. For database-backed source testing, follow the PostgreSQL test setup in `README.md` and start the API with `DATABASE_URL`.

### Container

Download `my-known-world-image.tar` from the Assets section of this release. Optionally verify the adjacent `.sha256` checksum, then load it and select its `ci` tag in Compose:

```sh
docker load -i my-known-world-image.tar
# In .env, set IMAGE_TAG=ci, set a long POSTGRES_PASSWORD, then:
docker compose up -d --no-build
python3 scripts/smoke_test.py --base-url http://127.0.0.1:8080
```


The Compose default seeds only synthetic sample graph data. Do not add private information in this read-only test release.
