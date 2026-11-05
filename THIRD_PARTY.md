# Third-party assets

Browser assets are embedded locally. There are no runtime CDN requests.

| Asset | Version | License | Source |
| --- | --- | --- | --- |
| HTMX | 2.0.11 | 0BSD | https://github.com/bigskysoftware/htmx/tree/v2.0.11 |
| Alpine CSP | 3.17.5 | MIT | https://github.com/alpinejs/alpine/tree/v3.17.5 |

The exact npm source archives and SHA-256 digests of the embedded JavaScript
are in `internal/folkweave/static/vendor.json`. Matching license files are
beside the scripts. Go dependencies and their checksums are pinned in `go.mod`
and `go.sum`. The mutation runner is reused from comfylib under AGPL-3.0-or-later.
