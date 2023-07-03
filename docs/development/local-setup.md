### admission-openstack

`admission-openstack` is an admission webhook server which is responsible for the validation of the cloud provider (OpenStack in this case) specific fields and resources. The Gardener API server is cloud provider agnostic and it wouldn't be able to perform similar validation.

Follow the steps below to run the admission webhook server locally.

1. Start the Gardener API server.

    For details, check the Gardener [local setup](https://github.com/gardener/gardener/blob/master/docs/development/local_setup.md).

1. Start the webhook server

    Make sure that the `KUBECONFIG` environment variable is pointing to the local garden cluster.

    ```bash
    make start-admission
    ```

1. Setup the `ValidatingWebhookConfiguration`.

    `hack/dev-setup-admission-openstack.sh` will configure the webhook Service which will allow the kube-apiserver of your local cluster to reach the webhook server. It will also apply the `ValidatingWebhookConfiguration` manifest.

    ```bash
    ./hack/dev-setup-admission-openstack.sh
    ```

You are now ready to experiment with the `admission-openstack` webhook server locally.
