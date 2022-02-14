# `sget`

`sget` is command for safer, automatic verification of signatures and integration with our binary transparency log, Rekor.

To install `sget`, if you have Go 1.16+, you can directly run:

    $ go install github.com/sigstore/sget@latest

and the resulting binary will be placed at `$GOPATH/bin/sget` (or `$GOBIN/sget`, if set).

Just like `curl`, `sget` can be used to fetch artifacts by digest using the OCI URL.
Digest verification is automatic:

```shell
$ sget us.gcr.io/dlorenc-vmtest2/readme@sha256:4aa3054270f7a70b4528f2064ee90961788e1e1518703592ae4463de3b889dec > artifact
```

You can also use `sget` to fetch contents by tag.
Fetching contents without verifying them is dangerous, so we require the artifact be signed in this case:

```shell
$ sget gcr.io/dlorenc-vmtest2/artifact
error: public key must be specified when fetching by tag, you must fetch by digest or supply a public key

$ sget --key cosign.pub us.gcr.io/dlorenc-vmtest2/readme > foo

Verification for us.gcr.io/dlorenc-vmtest2/readme --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - Existence of the claims in the transparency log was verified offline
  - The signatures were verified against the specified public key
  - Any certificates were verified against the Fulcio roots.
```

The signature, claims and transparency log proofs are all verified automatically by sget as part of the download.

`curl | bash` isn't a great idea, but `sget | bash` is less-bad.

