# feedhorn

Posts new items of RSS, Atom and JSON feeds to a Mastodon account. It runs as
a daemon, checks every feed at a fixed interval and posts what is new, oldest
first. The text of a post is a template, by default the title followed by the
link.

## Quick start

1. In Mastodon, create an application under *Preferences → Development* with
   the scope `write:statuses` and copy its access token.
2. Copy `config.example.yaml` to `config.yaml` and fill in your server and
   feeds.
3. Build and run:

```bash
go build -o feedhorn .
MASTODON_TOKEN=... ./feedhorn
```

The first time feedhorn sees a feed it posts nothing and only remembers the
items the feed lists. From then on, every new item is posted once.

## Configuration

```yaml
server: https://mastodon.social
visibility: public # public, unlisted, private or direct
template: "{{.Title}}\n\n{{.Link}}" # optional, this is the default
feeds:
  - url: https://blog.golang.org/feed.atom
  - url: https://example.com/feed.xml
    template: "{{.Title}} #example\n\n{{.Link}}" # overrides the global one
```

Unknown fields are rejected, so a typo fails at startup instead of silently
doing nothing.

Every flag can also be set through the environment:

| Flag | Environment | Default | Meaning |
| --- | --- | --- | --- |
| `-config` | `CONFIG` | `config.yaml` | path to the config file |
| `-state` | `STATE` | `state.json` | path to the state file |
| `-state-retention` | `STATE_RETENTION` | `2160h` (90 days) | how long to remember items a feed stopped listing |
| `-interval` | `INTERVAL` | `15m` | time between two rounds |
| `-log-level` | `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| | `MASTODON_TOKEN` | | access token, required |

The token is only read from the environment, so it never shows up in the
process list.

### Template

The template is a Go [text/template](https://pkg.go.dev/text/template) and
gets the fields `{{.Title}}` and `{{.Link}}` of the item. Leading and trailing
whitespace of the result is removed, so an item without title or link does not
start or end with a blank line. A template that does not parse or names an
unknown field fails at startup.

A post longer than 500 characters, Mastodon's default limit, is cut off at the
end with `…` and logged as a warning. That can cut the link, so a warning means
the template should be shorter.

## State

`state.json` holds the IDs of the known items per feed (the GUID, or the link
if a feed has none). It is written after every post, so a restart never posts
an item twice. If posting fails, the remaining items of that feed are retried in
the next round.

An item that drops out of a feed is remembered for the retention period, so it
is not posted again if it reappears. A feed removed from the config is
forgotten; adding it back counts as a new feed.

## Kubernetes

`deploy/` holds a Deployment with a small PersistentVolumeClaim for the state
and a ConfigMap for the config. The Deployment uses the `Recreate` strategy and
a single replica, since two instances would post every item twice.

1. Edit the feeds in `deploy/configmap.yaml`.
2. Create the secret with the token:

```bash
kubectl create secret generic feedhorn --from-literal=token=...
```

3. Apply the manifests:

```bash
kubectl apply -k deploy/
```
