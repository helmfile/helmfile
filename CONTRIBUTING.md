# DCO + License

By contributing to `helmfile`, you accept and agree to the following DCO and license terms and
conditions for your present and future Contributions submitted to the `helmfile` project.

[DCO](https://developercertificate.org/)
[License](https://github.com/helmfile/helmfile/blob/master/LICENSE)

# Developing helmfile

Locate your `GOPATH`, usually `~/go`, and run:

```console
$ go get github.com/helmfile/helmfile

$ cd $GOPATH/src/github.com/helmfile/helmfile

$ git checkout -b your-shiny-new-feature origin/master

...

$ git commit -m 'feat: do whatever for whatever purpose

This adds ... by:

- Adding ...
- Changing ...
- Removing...

Resolves #ISSUE_NUMBER
'

$ hub fork
$ git push YOUR_GITHUB_USER your-shiny-new-feature
$ hub pull-request
```

Note that the above tutorial uses [hub](https://github.com/github/hub) just for ease of explanation.
Please use whatever tool or way to author your pull request!
