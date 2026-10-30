# ArgoCD Deployment

This example is intended to show how you can use Helmfile with ArgoCD in a private way. The example deployment demonstrates how to deploy ArgoCD with the helmfile plugin, and how to use helmfile within ArgoCD to connect to various private external systems.

If you intend to follow this example, please pay close attention to the caveats. This chart will not work if you deploy it as-is, you will need to customise many parts of it to match the infrastructure of your organisation.

## What is the point of this chart?

There are many organisations out there which would like to widen Kubernetes adoption but struggle with the requirement to keep all data confidential and follow best practices. For example, one best practice is to to implement a CI/CD process for all deployments including Helm charts. As part of this process we may want to keep secrets in private Vault secret storage, use Helm charts from a private chart museum, customised Helm values from a private Git repository and docker images from a private docker repository. ArgoCD alone cannot do all of these things at once, but we can achieve this if we configure it with Helmfile.

This example is intended to show how you COULD use ArgoCD together with Helmfile to implement a fully private CI/CD system. Your deployment may be a litte bit different, or very different to this example. It's intended only as an example.

The helm chart in this repo can be deployed with Helmfile, if you follow the prerequisites. In order for the deployment to work correctly, you will need to follow the prerequisites and update all of the `values.yaml` file entries with a `# TODO` comment.

# Deployment planning

* Getting started: https://argo-cd.readthedocs.io/en/stable/getting_started/
* Helm Chart: https://github.com/argoproj/argo-helm/tree/master/charts/argo-cd
* SSO Setup: https://argo-cd.readthedocs.io/en/stable/operator-manual/user-management/microsoft/#entra-id-app-registration-auth-using-oidc
* RBAC: https://medium.com/dzerolabs/configuring-sso-with-azure-active-directory-on-argocd-d20be4ba753b
* Vault
* Private Chartmuseum (Harbor)

What are the interactions between the different systems?

* ArgoCD will run on Kubernetes. We must use a custom build of ArgoCD as the official build doesn't ship with the runtime binaries which we will need to execute Helmfile and interact with Vault.
* ArgoCD interacts with AzureAD for SSO. ArgoCD has to be configured as an enterprise app in Azure AD for authorization and authentication by an administrator. We put the client ID and client secrets into Vault for safe keeping.
* ArgoCD interacts with Vault in order to pull secrets for customising Helm deployments. Sometimes we need to inject secrets into the values.yaml file of a Helm deployment. ArgoCD will use an AppRole for authentication into Vault, and this is done once during the deployment. The ArgoCD Vault credentials are saved in Vault for safe keeping.
* ArgoCD may need to interact with a private Git server. This is so that ArgoCD can access the `helmfile.yaml` files before executing Helmfile. Additional configuration files (like `conf.d` and other `values.yaml`) files can be kept in here.
* ArgoCD will authenticate to a private chart museum. This is so that we can keep the private helm charts private.

# Prerequisites

## Docker image

* Build the docker image from the `argocd.dockerfile` file in this folder.
* Upload the image to your private docker registry and make a note of the registry URL and tag.
* Customise the `values.yaml` file in this folder.
  Replace `argo-cd.global.image.repository` for the docker registry URL of your private registry.
  Replace `argo-cd.global.image.tag` for the tag which you used to upload your private docker image.

## Configure Single Sign-On for ArgoCD

* Log into AzureAD as an administrator.
* Create a new App Registration in your tenant for ArgoCD.
* Create a new Client ID and client secret.
* Save the client ID and client secret into your vault server.
  - Save the client ID and client secret into Vault. Suggested keys: `internal/argocd/auth#azure_oidc_client_id` and `internal/argocd/auth#azure_oidc_client_secret`.
* Update `values.yaml` for your deployment
  - Update `argo-cd.server.config.oidc.config` - replace `SOME_AZURE_AD_TENANT` for your actual plaintext AzureAD tenant ID.
  - Update `argo-cd.server.config.oidc.config` - replace `SOME_AZURE_AD_UUID` for your actual plaintext AzureAD client app ID.
  - Verify that the Vault path specified in `argo-cd.config.secret.extra.oidc.azure.clientSecret` is the correct path in Vault.
* Customise the ingress objects in the source to match what your expected external URL will be. Update `values.yaml` to have the correct values for the hostname you plan to use.
  - `argo-cd.config.server.(url|hostname)`
    ```yaml
    url: 'https://my.argocd.deployment.org' # TODO
    hostname: my.argocd.deployment.org      # TODO
    ```

## Configure authentication from ArgoCD into Vault (or read next section on how to disable it)

* Read the documentation from Hashicorp on AppRoles. Provision a new AppRole for ArgoCD with policies.
  - In the example the authentication path in Vault is assumed to be `auth/approle/login`. If that's not the case then update line 123 in `values.yaml`.
* Store the AppRole credentials for ArgoCD in Vault. We will use Helmfile to pull them from here when we launch the chart.
  - Suggested paths: `internal/vault/argocd#role_id` and `internal/vault/argocd#secret_id`
* Verify that the Vault paths in `values.yaml` are correct
  - `argo-cd.config.secret.extra.vault_role_id`
  - `argo-cd.config.secret.extra.vault_secret_id`
* Replace `ROLE_ID` and `SECRET_ID` in `values.yaml` (line 124) with the actual plaintext values (values key `argocd.server.config.configManagementPlugins`). This is not 100% secure but was the only way that I could manage to get the rest of the deployment working. I advise you add `values.yaml` to `.gitignore` after doing this step.

## Disable Vault (optional)

If you are not going to use Vault, then you need to update the values to get your deployment to use Helmfile.
* If you are not going to use Vault, then remove the line in `values.yaml` that starts with `export VAULT_TOKEN=...`.

## Configure authentication from ArgoCD into private Git repository

* Create a new API user in your Git system (username + password auth). ArgoCD will use these credentials to pull `values.yaml` and `helmfile.yaml` for your project.
* Save the username and password into Vault
  - Recommended keys: `internal/git/users/argocd#username` and `internal/git/users/argocd#password`
* Verify that the Vault paths in `values.yaml` are correct
  - `argo-cd.config.secret.extra.git_username`
  - `argo-cd.config.secret.extra.git_password`
* Update the settings in `values.yaml` for repositories - remove the example Git URLS and replace with actual Git URLs for your project.
  - `https://my.git.server.org/my-team/my-repo-one.git`
  - `https://my.git.server.org/my-team/my-repo-two.git`

## Configure authentication from ArgoCD to your private Docker registry and Chartmuseum

* This guide is written with Harbor in mind, which is both a docker registry and chart museum. If you have separate systems in place you might need to configure these things separately.
* Create a new set of credentials for Harbor and save them into Vault.
  - Suggested paths: `internal/harbor/users/argocd#username`, `internal/harbor/users/argocd#username`.
* Verify that the Vault paths in `values.yaml` are correct
  - `argo-cd.config.secret.extra.harbor_username`
  - `argo-cd.config.secret.extra.harbor_password`
* Change the Helm URL on line 95 in `values.yaml` for the actual URL of your Helm Chartmuseum deployment.
  - `https://my.harbor.deployment.org/chartrepo/my-project`

# Deploy ArgoCD for the first time

Once you've completed the prerequisites you are ready to deploy ArgoCD for the first time. There is a helmfile that you can use to do this.

Steps:

* Auth to Vault and Kubernetes
* Add ArgoCD Helm repo
  - ```bash
    helm repo add argo https://argoproj.github.io/argo-helm
    helm install --name argocd --namespace argocd argo/argo-cd
    ```
* Download dependencies (`helmfile dep update`)
* Deploy chart (`helmfile apply`)
  - If everything worked correctly, then ArgoCD should have deployed using your customised docker image into your cluster.

* If the ingress deployed correctly then we should be able to access the UI: https://my.argocd.deployment.org
* Grab the admin password from `argocd-initial-admin-secret secret` and use it to log into the UI.
```bash
    echo "<password>" | base64 -d
```
* Verify that SSO works by signing on with AzureAD.
* Verify that Helmfile appears in the list of configured plugins in the UI.

# Configure ArgoCD apps for deployment with Helmfile

* Log into ArgoCD in the UI and configure a new app.
* Make sure that it's using the Helmfile plugin.
* For the source, select one of the Git repos which you configured previously.
* Make sure that your Helm repo (including `helmfile.yaml` is present in the Git repository).
* ArgoCD will pull the project from your private Git server and read the `helmfile.yaml` file.
* ArgoCD will also execute helmfile directly in the container using the files checked out from your project.
* If you configured ArgoCD with logins for Vault and private Chartmuseums, these can be used in ArgoCD by Helmfile.
* You can use the diff feature and resource views in ArgoCD when using Helmfile. Unfortunately you can no longer use the `values.yaml` editor in the UI. You can change `values.yaml` for deployments by creating `values.yaml` files and committing them to the folders in your project Git repo. You can also customise them using `!vault` tags.

If you would like an example, this ArgoCD + helmfile deployment can itself be deployed by ArgoCD + helmfile (you may need to drop the pods after you do deploy this way). Any other projects - just structure them in the same way as this example.

Be careful when reconfiguring a project which was previously deploy by ArgoCD with Helmfile. There are some differences around the way in which annotations are used.
