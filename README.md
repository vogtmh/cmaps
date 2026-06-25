# CompanyMaps 9

CompanyMaps is an interactive office floor-plan and desk-finder application. This
is the **version 9** rewrite: the original PHP application has been ported to a
single, statically-linked **Go** binary while keeping every REST API, the admin
panel, the AD/LDAP import and the SAML single sign-on fully compatible with the
existing front-end and identity-provider configuration.

The browser front-end (JavaScript, CSS, images) is reused **unchanged** from the
PHP version; only the server side was rewritten.

## Highlights

- **Single binary, no runtime dependencies.** All templates, JS, CSS and the
  demo dataset are embedded with `go:embed`. The server stores everything in an
  embedded BoltDB file inside the data directory - no MySQL, no PHP, no
  SimpleSAMLphp.
- **Drop-in compatible APIs.** The `/rest/...` endpoints, the `/admin/` panel and
  the legacy SAML ACS paths keep the same URLs and payloads, so existing clients,
  bookmarks and the Entra app registration keep working.
- **Native SAML SSO** mounted at the original SimpleSAMLphp paths.
- **AD/LDAP sync** runs on a schedule and on demand from the admin panel.
- **Robin meeting-room** integration for live room availability.

## Configuration

`config.json` lives next to the executable. Only a few bootstrap settings live
here; everything else (maps, desks, users, teams, LDAP sources, the LDAP bind
password, the Robin token, app title and feature flags) is stored in the BoltDB
and edited from the admin panel.

```json
{
  "listen_addr": ":8096",
  "admin_password": "CHANGE-ME",
  "data_dir": "data",
  "saml": {
    "enabled": false,
    "allow_local_password_fallback": true
  }
}
```

- `listen_addr` - address the app binds to (proxied by nginx, see below).
- `admin_password` - break-glass local admin password (user `admin`).
- `data_dir` - directory holding `cmaps.db`, `maps/` and `avatarcache/`.
- `saml` - SAML SP/IdP settings (see `config.go` for the full field list).

## First run

On first start, with an empty database, open the app in a browser and you will be
guided through the setup wizard: load the demo dataset, import from an existing
CompanyMaps 8 MySQL dump, configure LDAP and Robin, then finish.

## Building

The module is fully vendored, so builds need no network access for dependencies
(a Go toolchain is still required to compile).

```sh
go build -mod=vendor -o cmaps .
```

## Deployment (Linux server)

`update.sh` cross-compiles for Linux, installs the binary under `/opt/cmaps`,
installs the systemd unit from `template.service` and (re)starts the service.

```sh
./update.sh        # run ON THE LINUX SERVER
```

> Do **not** run `update.sh` on macOS - it is a Linux deployment script.

Put `nginx.conf.example` in place (adjust `server_name` and the TLS certificate
paths) to terminate TLS and reverse-proxy all traffic - including the legacy
`/simplesaml/...` SAML endpoints - to the Go app on `127.0.0.1:8096`.

## Layout

| Path | Purpose |
|------|---------|
| `*.go` | Application source (handlers, db, SAML, LDAP, admin, REST). |
| `templates/` | Server-rendered HTML (embedded). |
| `static/` | Front-end JS/CSS/images reused from the PHP app (embedded). |
| `sample/` | Demo dataset used by the setup wizard (embedded). |
| `data/` | Runtime data: `cmaps.db`, `maps/`, `avatarcache/`. |

## Security notes

- Change `admin_password` from the default before exposing the app.
- The LDAP bind password and Robin token are stored in the database; if you
  imported them from an old `tvmaps.sql` dump that was committed to source
  control, rotate those credentials.
- Always run behind TLS (see `nginx.conf.example`).
