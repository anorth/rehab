# Rehab

_Treatment for dependencies._

Rehab is a tool for managing module dependencies, automating some of the manual labour required
to propagate upgrades and stay up to date. 
Its goal is to bring some of the [advantages of monorepos](https://danluu.com/monorepo/) a little closer 
for multi-repo projects by making it easy for all projects in an ecosystem to be working "at head".

Rehab works across many modules at once, but only requires one repository to be checked out locally to provide
a build context. Rehab can then work across the dependency tree of that repository. Rehab proposes changes by
pushing them directly to GitHub branches (optionally opening pull requests). Rehab doesn't rely on local build
environments or standard configuration of, say, Make targets; it instead assumes CI will check the correctness
of pull requests.

At present, Rehab works only with Go projects, but could be extended to other build systems
with similar module architectures (Rust, Javascript).

## Usage examples

### Show stale requirements
Show all mismatches between declared requirement versions and the versions actually selected by MVS
across a dependency graph. Also shows requirements on modules that themselves declare stale requirements.

These mismatches represent gaps in testing, where some module is tested with its declared requirements, but
is built for production with a different version of those requirements.
In theory, the build requirements are compatible (behave identically) to the declared requirements, but only
if programmers don't make any mistakes. The existence of tests suggests we don't believe that.

```shell
$ rehab show --all <path to anorth/go-dar>
github.com/anorth/go-dar requires github.com/multiformats/go-multihash@v0.0.14, builds with v0.0.14 (has stale transitive requirements) (highest v0.1.0)
github.com/anorth/go-dar requires github.com/ipfs/go-cid@v0.0.7, builds with v0.0.7 (has stale transitive requirements) (highest v0.1.0)
github.com/anorth/go-dar requires github.com/ipld/go-ipld-prime@v0.7.1-0.20210125211748-8d37030e16e1, builds with v0.7.1-0.20210125211748-8d37030e16e1 (has stale transitive requirements) (highest v0.14.3)
github.com/ipfs/go-cid@v0.0.7 requires github.com/multiformats/go-multihash@v0.0.13, builds with v0.0.14 via github.com/anorth/go-dar (highest v0.1.0)
github.com/ipld/go-ipld-prime@v0.7.1-0.20210125211748-8d37030e16e1 requires github.com/ipfs/go-cid@v0.0.4, builds with v0.0.7 via github.com/anorth/go-dar (highest v0.1.0)
github.com/multiformats/go-multibase@v0.0.3 requires github.com/mr-tron/base58@v1.1.0, builds with v1.1.3 via github.com/ipld/go-ipld-prime@v0.7.1-0.20210125211748-8d37030e16e1 (highest v1.2.0)
github.com/multiformats/go-multihash@v0.0.14 requires github.com/minio/sha256-simd@v0.1.1-0.20190913151208-6de447530771, builds with v0.1.1 via github.com/ipld/go-ipld-prime@v0.7.1-0.20210125211748-8d37030e16e1 (highest v1.0.0)
github.com/multiformats/go-multihash@v0.0.14 requires golang.org/x/crypto@v0.0.0-20190611184440-5c40567a22f8, builds with v0.0.0-20200117160349-530e935923ad via github.com/anorth/go-dar (highest v0.0.0-20211215153901-e495a2d5b3d3)
```

### Upgrade module requirements
Push a branch upgrading all requirements for a project to their latest version.

```shell
$ rehab upgrade <path to workspace>
```

Upgrade instead only to the version selected by MVS, which may not be the latest.
```shell
$ rehab upgrade --minimum <path to workspace>
```

### Push a release downstream (coming soon)
Push branches upgrading all stale requirements of a specific module across a dependency graph to the latest version.

```shell
$ rehab upgrade --of <modulepath> <path to workspace>
```

### Upgrade a full dependency graph
Issue pull requests upgrading all stale requirements across a full dependency graph to the 
latest version of upstream modules.
```shell
$ rehab upgrade --all --pull <path to workspace>
```

Upgrading a full graph like this may result in new stale requirements as mid-stream modules are upgraded to the latest
version of far-upstream requirements. After releases are tagged or requirements declared on unreleased git SHAs, run 
upgrade again to propagate changes downstream.
