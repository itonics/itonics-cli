# itonics-cli

A fast Go CLI for the [ITONICS Innovation](https://www.itonics-innovation.com/) OData v2 API. Manage elements, element types, files, attachments, watches and likes from your terminal or from AI agents (Claude Code, Codex CLI, Cursor, …).

The companion [`itonics-skills`](https://github.com/itonics/agent-skills) repo provides the agent skill that documents this CLI for LLM-based assistants.

## Install

### Homebrew (macOS / Linux) — recommended

```bash
brew tap itonics/itonics https://github.com/itonics/itonics-cli
brew install itonics
```

Until the first `v*` tag is released, the tap installs the `--HEAD` formula and builds from `main`.

### Scoop (Windows)

```powershell
scoop bucket add itonics https://github.com/itonics/itonics-cli
scoop install itonics
```

`scoop update itonics` pulls the next release.

### Pre-built binary (any OS)

Grab a release archive for your OS/arch from the [latest release](https://github.com/itonics/itonics-cli/releases/latest):

- macOS / Linux — `itonics_X.Y.Z_<Os>_<arch>.tar.gz`, extract `itonics` into `~/.local/bin`
- Windows — `itonics_X.Y.Z_Windows_x86_64.zip`, extract `itonics.exe` into a directory on `PATH`

### From source

```bash
go install github.com/itonics/itonics-cli@latest
```

Or, working in a clone:

```bash
git clone https://github.com/itonics/itonics-cli.git
cd itonics-cli
go build -o itonics .
```

## Configure

```bash
itonics login
```

Interactively prompts for the tenant domain, space URI, and API key (hidden input), validates them against the API, and writes them to `~/.config/itonics/config.toml` with `chmod 0600`. Re-running `login` rotates the key — existing values are shown as defaults.

Non-interactive forms:

```bash
itonics login --domain https://acme.itonics.io --space SPACE_URI --api-key SECRET
echo "$KEY" | itonics login --domain ... --space ... --api-key -        # read key from stdin
itonics login ... --skip-check                                          # skip the validation round-trip
```

Other commands:

```bash
itonics whoami                                        # show which creds would be used, and from where
itonics logout                                        # delete ~/.config/itonics/config.toml
```

Credentials are resolved in this order (highest first):

1. `ITONICS_DOMAIN` / `ITONICS_API_KEY` / `ITONICS_SPACE` env vars — set by your shell, by `--env-file`, or by a `.env` in the current directory.
2. The persisted profile at `~/.config/itonics/config.toml`.

See [`.env.example`](.env.example) if you prefer the env-var path.

## Quick reference

| Task | Command |
|------|---------|
| Log in / rotate API key | `itonics login` |
| Show effective credentials | `itonics whoami` |
| Log out | `itonics logout` |
| List elements | `itonics elements list -n 20 --format table` |
| Filter by type | `itonics elements list -f "elementTypeUri eq 'XXX'"` |
| Full-text search | `itonics elements list -s "keyword"` |
| Get one element | `itonics elements get URI --raw` |
| Create | `itonics elements create --type URI --label LBL --created-by EMAIL` |
| Update | `itonics elements update URI --updated-by EMAIL --label "New"` |
| Delete | `itonics elements delete URI [URI…] --yes` |
| Upload file | `itonics files upload ./logo.png --created-by EMAIL` |
| Upload + attach | `itonics files upload ./logo.png --created-by EMAIL --attach-to ELEMENT_URI` |
| File details | `itonics files get FILE_URI` |
| List attachments | `itonics attachments list ELEMENT_URI` |
| Attach by URI | `itonics attachments attach ELEMENT_URI FILE_URI --attached-by EMAIL` |
| Detach | `itonics attachments detach ELEMENT_URI FILE_URI --detached-by EMAIL` |
| List watches | `itonics watches list ELEMENT_URI` |
| Add watch | `itonics watches add ELEMENT_URI USER_URI` |
| List likes | `itonics likes list ELEMENT_URI --orderby "likedOn desc"` |
| List element types | `itonics types list --format table` |

Run `itonics --help` or `itonics <group> --help` for the full surface.

## Programmatic usage

```go
import "github.com/itonics/itonics-cli/internal/api"

c := api.New("https://your-tenant.itonics.io", "your-api-key", "your-space-uri")
elements, err := c.ListElements(ctx, api.ListElementsParams{Filter: "elementTypeUri eq 'XXX'", RawFieldValues: true})
fileURI, err := c.UploadFile(ctx, "/path/to/logo.png", "user@example.com", "")
_, err = c.AttachFiles(ctx, "ELEMENT_URI", []string{fileURI}, "user@example.com")
```

> Note: the API client lives under `internal/` today — if you have a need for it as a public package, open an issue.

## API notes

- The OpenAPI spec lives at [apidocs.itonics-innovation.com](https://apidocs.itonics-innovation.com/) (`bundled-openapi.yaml`).
- `rawFieldValues=1` returns each property as `{uri, type, label, rawValue}`. `rawValue` is always an array.
- For `rich_text` properties, send plain HTML or `base64:<base64-of-HTML>`. The `tiptap:` storage encoding is read-only and rejected on writes.
- The pre-signed upload response advertises `x-amz-acl: private`, but the backing bucket has ACLs disabled. The CLI strips that header before PUTting the binary — replicate this when calling the API directly.
- Not every element type exposes a default Attachments field. When it doesn't, `POST /elements/{uri}/attachments` returns `400 BadRequestException: Element type 'X' does not expose the default Attachments field`. Use a `file` / `headerImage`-typed element property instead.

## Releases

Tagging `vX.Y.Z` triggers a [GoReleaser](https://goreleaser.com/) workflow that builds binaries for macOS / Linux / Windows (amd64 + arm64), creates a GitHub release, publishes checksums, and updates `Formula/itonics.rb` so the tap installs the new version.

## License

MIT — see [LICENSE](LICENSE).
