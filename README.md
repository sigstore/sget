# `sget`

### 🚨 `sget` is not ready for non-testing use at this time 🚨

`sget` is currently an experiment, and as such is not guaranteed to be safe or reliable.

Its config formats and flags may change in breaking ways at any time.
Its security properties may change, including in ways that make it _less_ secure than it previously was, or even less secure than _not_ using `sget`. 

Please see the [roadmap](./ROADMAP.md) to get an idea where we're heading.

Join `#sget` on the [Sigstore Slack](https://sigstore.slack.com) ([invite link](https://sigstore.slack.com/join/shared_invite/zt-mhs55zh0-XmY3bcfWn4XEyMqUUutbUQ#/shared-invite/email)) to get involved, ask questions, propose ideas, etc.
Also feel free to [file an issue](https://github.com/sigstore/sget/issuues/new) to ask a question or report a bug.

With that out of the way...

# What is `sget`?

`sget` is command for safer, automatic verification of signatures and integration with Sigstore's binary transparency log, [Rekor](https://github.com/sigstore/rekor).

It aims to provide a safer alternative to `curl` when fetching artifacts from the internet, and especially when piping those contents to a shell, for example the dreaded and often (correctly!) maligned `curl <url> | sh`.

`curl | sh` isn't a great idea, but `sget | sh` is less-bad.

### Installation

To install `sget`, if you have Go 1.16+, you can directly run:

```
go install github.com/sigstore/sget@main
```

and the resulting binary will be placed at `$GOPATH/bin/sget` (or `$GOBIN/sget`, if set).

### `sget <URL>`

This will fetch an HTTPS URL and look for any signatures for the contents' digest in the Rekor transparency log.

If any signatures are found matching your trusted identities (see below), the contents will be piped to stdout.

```
sget https://example.com/archive.zip > archive.zip
```

or

```
sget https://example.com/install.sh | sh
```

### `sget sign <URL>`

This will prompt you to go through the Fulcio OIDC flow to generate a signature for the URL, tied to your identity.
This record will be appended to the Rekor transparency log so that other users can see it.

If other users configure your identity as a trusted identity, you signature will be accepted when they attempt to fetch the URL using `sget`.

### `sget trust <IDENTITY>...`

This adds the specified identities to your list of trusted identities.

When fetching content using `sget <URL>`, if any of these identities have signatures for the URL in Rekor, the content will be considered to be trusted.

Trusted identities can be removed with the `--rm` flag:

```
sget trust --rm alice@example.com
```

Trusted identities can be scoped only to certain URL hostnames with the `--host` flag:

```
sget trust --host=example.com alice@example.com
```

In this case, signatures for `alice@example.com` will only be trusted if the URL is being fetched from `example.com`.

### `sget <OCI ref>`

`sget` began its life as a tool to fetch contents stored in an OCI registry, and this functionality remains today.

The main benefit of relying on an OCI registry in this case is that OCI provides a standard and broadly-supported API for fetching content by its digest, i.e., content-addressed storage.
This means that if someone shares a reference to an object in an OCI registry that includes the digests of its contents, the registry will verify this digest matches before accepting it, and clients (including `sget`) will verify it again before fetching it.

This makes `sget` resistant to attacks where the contents fetched today by one person may differ unexpectedly when fetched tomorrow by someone else.

`sget <OCI ref>` only supports references by digest at this time, and will not currently check for any signatures from trusted identities.

