# `sget` Roadmap

### Today

`sget` fetches blobs from OCI registries, using Docker API registry auth, checking content digests to ensure integrity, and verifying any `cosign` signatures that are found in the registry.

`sget` currently has no functionality to fetch arbitrary URLs.

### Phase 0

Add basic URL-fetching functionality to `sget`, with content digests to ensure integrity.

This phase may also include a manual verification step where buffered contents are shown to the user, so they can manually inspect the contents before proceeding.

### Phase 1: "Social Proof"

In the absence of a maintainer-provided policy, `sget` users can gain assurance about the safety of an artifact by relying on crowd-sourced verificatioin and signatures.

Signatures will be stored in Rekor, using certificates created by Fulcio's "keyless" OIDC capabilities.

Before trusting an artifact, users should be able to:

- see the total number of unique identities that have signed indicating that the artifact is safe to fetch.
- see a random small subset of those identities.
- see any identities with email addresses that match the URL's domain.
- locally configure trusted identities, and trust those identities more than arbitrary public identities.
- locally configure policies, and automatically trust artifacts that satisfy those requirements.

The goal of this phase is to encourage adoption of `sget` by end users wishing to consume content more safely, without requiring any action by maintainers.

Our options in this phase are necessarily limited and incomplete without some help from maintainers.

### Phase 2: Maintainer Policy

In this phase we'll describe how policies and maintainer signatures can be made available alongside released artifacts, so that consumers of the artifacts can have maximum assurance that the artifact is safe to consume.

This phase will also include release tooling to make new releases of artifacts/policies/signatures as smooth as possible for maintainers and end users, without sacrificing availability or safety assurances.

The goal of this phase is to gain adoption among maintainers wishing to make consuming their content even safer for interested users.


