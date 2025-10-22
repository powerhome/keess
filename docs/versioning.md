# Keess Versioning

There are two versions to note here in this repo. The binary/image version, and the Helm Chart version.

These are set at:

- cmd/version.go
- chart/Chart.yaml

You MUST remember to update those files when making changes to Go code, the Chart, or
both. The next section explain the conventions we are following for the relation between
app and chart version.

Container images MUST always follow the version defined in cmd/version.go. If needed (a
new docker build), a suffix may be added to the image tag.

We might adopt other schemes or automations at some point, but for now this is what we have.

## Chart version caveat, or "What to do if we make a change only to the chart?"

First of all, we follow the Helm standard directions, as said in Chart.yaml.

```yaml
# This is the chart version. This version number should be incremented each time you make changes
# to the chart and its templates, including the app version.
# Versions are expected to follow Semantic Versioning (https://semver.org/)
version: 1.3.1

# This is the version number of the application being deployed. This version number should be
# incremented each time you make changes to the application. Versions are not expected to
# follow Semantic Versioning. They should reflect the version the application is using.
# It is recommended to use it with quotes.
appVersion: "1.3.0"
```

But note in the example above that, we are using equal or similar version numbers for
app and chart version, and sometimes we need to make a change only in the Chart.

So when the change is only to the Chart, we:

- Bump the Chart patch version, keeping major and minor in sync with appVersion.
- Next time we make a change to the app, we skip both to the next version (minor or patch)

So in the example above, after making a new change to Go code, we would use either
`1.3.2`, `1.4.0`, or `2.0.0` for **both** `version` and `appVersion`, depending if doing a
patch, minor or major code update.

And why not add a suffix like `version: 1.3.0-1` for a chart-only change instead?

Because Helm expects SemVer semantics on that version, and according to SemVer semantics
`1.3.0-1` is a pre-release that should come **before** `1.3.0`, not after. To dodge any
problems using helm commands to manage versions, let's avoid that.
