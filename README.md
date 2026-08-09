# ISU-GeoGuessr (ReggieGuessr)

ReggieGuessr is a GeoGuessr-like game for the Illinois State University campus.
It was created as a class project for IT 391 to help incoming and transfer students familiarize themselves with the campus.

## Dependencies

The backend and frontend servers are written in [Go](https://go.dev/), which has to be installed to build the project.

A [PostgreSQL](https://www.postgresql.org/) database is required for storing user accounts and image locations.

`libwebp` is required by the backend for compressing images as they are uploaded.

```bash
# Debian/Ubuntu
sudo apt install libwebp-dev libwebp7
```

## Building

Use the `backend_build.sh` and `frontend_build.sh` scripts to build the backend and frontend.
Alternatively, run `go build` in each folder to get a binary for each.

## Running

The frontend listens on `:8080` and does not require any configuration, since it only serves static files.
It does not handle TLS termination and needs a reverse proxy to do so, like [nginx](https://nginx.org/).

The backend listens on `:3000`.
It will not start without these required environment variables:
* `FRONTEND_DOMAIN`: The allowed origin requests to the backend can come from (formatted like `https://example.org`).
  It can be set to the special value `*` where no origin checks are performed (only advisable for testing).
* `DATABASE_URL`: A PostgreSQL connection string, which has to start with `postgres://`.
  Refer to the [PostgreSQL documentation](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING-URIS) for example connection strings.

It is recommended to set the environment variable `GIN_MODE=release` in production.
