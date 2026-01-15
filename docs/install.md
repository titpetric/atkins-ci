# Installing Atkins

Depending on how you want to run Atkins, there are three methods of installation.

1. Build and install from source using Go,
2. Install a binary release from the [Releases page](https://github.com/titpetric/atkins/releases),
3. Install a binary into your Docker images (only `amd64` for now),

After installation, verify with:

- `atkins -v` - print help,
- `atkins -l` - list `atkins.yml` jobs,
- `atkins` - run `atkins.yml` jobs

On linux, your workflow can include a shebang line at the top of the
workflow that invokes atkins:

```bash
#!/usr/bin/env atkins
# Your pipeline goes here.
```

After making the workflow executable, you can run it as you would other
scripts in the system. This allows you to maintain a folder of workflows
which are directly executable. See the `/tests` folder for more examples.

## Building from source

To build from source, using the latest Go version is encouraged. The
`go.mod` will generally handle this requirement for you, all you need is
to run the following `go install` command:

```
go install github.com/titpetric/atkins@latest
```

## Installing a binary release

1. Navigate to the [Releases page](https://github.com/titpetric/atkins/releases),
2. Download an atkins binary compatible for your system,
3. Install under `/usr/local/bin` or available system PATH.

## Install a binary into Docker images

Atkins is available under `titpetric/atkins:latest`. Tagged versions are
provided for historical purposes and you're encouraged to use the latest
version. The way to do that is to add Atkins to your Dockerfile env:

- Atkins is built from `scratch` in [docker/Dockerfile](https://github.com/titpetric/atkins/blob/main/docker/Dockerfile),
- You create your Dockerfile like in [docker/Dockerfile.example](https://github.com/titpetric/atkins/blob/main/docker/Dockerfile.example),

If you already have a `Dockerfile`, you can install `atkins` with a one-liner:

```Dockerfile
# Copy the atkins binary from the latest release
COPY --from=titpetric/atkins:latest /usr/local/bin/atkins /usr/local/bin/atkins
```

This is tested in the repository with [ci/release.yml](https://github.com/titpetric/atkins/blob/main/ci/release.yml).

```yaml
test:
  desc: "Run tests required for release"
  passthru: true
  steps:
    - docker buildx build --platform linux/amd64 -t titpetric/atkins-test -f docker/Dockerfile.test .
    - docker run --rm titpetric/atkins-test -v $PWD/tests:/app -f nested.yml -l
```
