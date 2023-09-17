# Helmfile practical examples and advanced usages

## Managing oneshot-jobs with Helmfile

In case you want to manage oneshot-jobs like a report generation or a database migration and so on, add a dedicated release spec for the job in `helmfile.yaml` like:

```yaml
repositories:
  - name: yourorg
    url: https://yourorg.example.com/charts

releases:
  - name: dbmigrator
    labels:
      job: dbmigrator
    chart: ./dbmigrator
    # DB host, port, and connection opts for the environment
    values:
    - "deploy/environments/{{ env "RAILS_ENV" }}/values.yaml"
    # DB username and password encrypted with helm-secrets (getsops/sops)
    secrets:
    - "deploy/environments/{{ env "RAILS_ENV" }}/secrets.yaml"
```

You would then start a database migration job by executing:

```console
# Start a database migration for the prod environment
$ RAILS_ENV=prod helmfile --selector job=dbmigrator sync

# Tail log until you are satisfied
$ kubectl logs -l job=dbmigrator
```

For more context, see [this issue](https://github.com/roboll/helmfile/issues/49).
