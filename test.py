import subprocess
from kubernetes import client, config
import time

# ANSI color codes for colored logging
GREEN = '\033[92m'
RED = '\033[91m'
RESET = '\033[0m'
WAIT_TIME = 30

def log_info(message):
    print(f"[INFO] {message}")

def log_success(message):
    print(f"{GREEN}[SUCCESS] {message}{RESET}")

def log_error(message):
    print(f"{RED}[ERROR] {message}{RESET}")

def delete_namespace(core_api, namespace, cluster):
    """
    Delete a Kubernetes namespace.
    :param core_api: CoreV1Api instance for Kubernetes API interaction.
    :param namespace: The name of the namespace to be deleted.
    :param cluster: The name of the cluster that will be added in the log.
    """
    try:
        core_api.delete_namespace(namespace)
        log_info(f"Namespace '{namespace}' deleted in {cluster} cluster.")
    except client.exceptions.ApiException as e:
        if e.status != 404:  # Ignore not found errors
            log_error(f"Error deleting namespace '{namespace}': {e} in {cluster} cluster")

def create_empty_namespace(core_api, namespace, cluster):
    """
    Create a Kubernetes namespace.
    :param core_api: CoreV1Api instance for Kubernetes API interaction.
    :param namespace: The name of the namespace to be created.
    :param cluster: The name of the cluster that will be added in the log.
    """
    try:
        core_api.read_namespace(name=namespace)
        log_info(f"Namespace '{namespace}' already exists in {cluster} cluster.")
    except client.exceptions.ApiException as e:
        if e.status == 404:
            core_api.create_namespace(client.V1Namespace(metadata=client.V1ObjectMeta(name=namespace)))
            log_info(f"Namespace '{namespace}' created in {cluster} cluster.")
            time.sleep(2)  # Give it some time for the namespace to be fully set up


def get_k8s_client_for_cluster(context_name):
    """
    Create a Kubernetes client configured for a specific cluster using a context name.
    :param context_name: The name of the context for the target cluster in your kubeconfig.
    :return: A CoreV1Api instance connected to the target cluster.
    """
    # Load the kubeconfig file and set the context
    config.load_kube_config(context=context_name)

    # Create and return the CoreV1Api client instance
    return client.CoreV1Api()

def cleanup_resources(source_core_api, target_core_api, source_namespace, destination_namespaces):
    """
    Clean up the source namespace, destination namespaces, and specific resources before and after tests.
    :param source_core_api: CoreV1Api instance for source cluster Kubernetes API interaction.
    :param target_core_api: CoreV1Api instance for target cluster Kubernetes API interaction.
    :param source_namespace: The source namespace from which to delete the secret and configmap.
    :param secret_name: The name of the secret to delete.
    :param configmap_name: The name of the configmap to delete.
    :param destination_namespaces: A list of destination namespaces to clean up.
    """
    # Attempt to delete the source namespace in source cluster
    delete_namespace(source_core_api,source_namespace,'source')

    # Attempt to delete the source namespace in target cluster
    delete_namespace(target_core_api,source_namespace,'target')

    # Clean up destination namespaces in source cluster
    for ns in destination_namespaces:
        delete_namespace(source_core_api,ns,'source')
    time.sleep(2)

def setup_resources_for_test(source_core_api, target_core_api, source_namespace, secret_name, configmap_name, destination_namespaces, label_selector=None):
    """
    Create a namespace (if it doesn't exist), a secret, and a configmap for testing, along with destination namespaces.
    :param source_core_api: CoreV1Api instance for source cluster Kubernetes API interaction.
    :param target_core_api: CoreV1Api instance for target cluster Kubernetes API interaction.
    :param source_namespace: The namespace in which to create the source secret and configmap.
    :param secret_name: The name of the secret to create.
    :param configmap_name: The name of the configmap to create.
    :param destination_namespaces: A list of destination namespaces to create for testing synchronization.
    """
    # Create source namespace if it doesn't exist in both clusters
    create_empty_namespace(source_core_api,source_namespace,'source')
    create_empty_namespace(target_core_api,source_namespace,'target')

    # Create destination namespaces
    labels = {}
    if label_selector:
        key, value = label_selector.split("=")
        labels[key] = value

    for dest_namespace in destination_namespaces:
        try:
            ns = source_core_api.read_namespace(name=dest_namespace)
            if label_selector and labels.items() <= ns.metadata.labels.items():
                log_info(f"Destination namespace '{dest_namespace}' with label '{label_selector}' already exists.")
            else:
                log_info(f"Updating labels for namespace '{dest_namespace}'.")
                # Update namespace labels if needed
        except client.exceptions.ApiException as e:
            if e.status == 404:
                # Create namespace with labels if it does not exist
                source_core_api.create_namespace(client.V1Namespace(
                    metadata=client.V1ObjectMeta(name=dest_namespace, labels=labels)))
                log_info(f"Created destination namespace '{dest_namespace}' with label '{label_selector}'.")
                time.sleep(2)  # Give it some time for the namespace to be fully set up
            else:
                raise

    # Create secret in source namespace
    secret_body = client.V1Secret(
        metadata=client.V1ObjectMeta(name=secret_name, namespace=source_namespace),
        data={"key": "c2VjcmV0VmFsdWU="}  # Example: base64 encoded "secretValue"
    )
    try:
        source_core_api.create_namespaced_secret(source_namespace, secret_body)
        log_info(f"Created secret '{secret_name}' in source namespace '{source_namespace}'.")
    except client.exceptions.ApiException as e:
        log_error(f"Failed to create secret '{secret_name}' in source namespace '{source_namespace}': {e}")

    # Create configmap in source namespace
    configmap_body = client.V1ConfigMap(
        metadata=client.V1ObjectMeta(name=configmap_name, namespace=source_namespace),
        data={"config": "configValue"}  # Example configuration data
    )
    try:
        source_core_api.create_namespaced_config_map(source_namespace, configmap_body)
        log_info(f"Created ConfigMap '{configmap_name}' in source namespace '{source_namespace}'.")
    except client.exceptions.ApiException as e:
        log_error(f"Failed to create ConfigMap '{configmap_name}' in source namespace '{source_namespace}': {e}")


def apply_labels_and_annotations(core_api, namespace, resource_name, labels, annotations, resource_type='secret'):
    """
    Apply labels and annotations to a resource.
    """
    metadata = client.V1ObjectMeta(labels=labels, annotations=annotations)
    if resource_type == 'secret':
        body = client.V1Secret(metadata=metadata)
        core_api.patch_namespaced_secret(resource_name, namespace, body)
    elif resource_type == 'configmap':
        body = client.V1ConfigMap(metadata=metadata)
        core_api.patch_namespaced_config_map(resource_name, namespace, body)
    log_info(f"Applied labels and annotations to {resource_type} '{resource_name}' in namespace '{namespace}'.")

def verify_resource_in_namespace(source_core_api, target_core_api, namespace, resource_name, resource_type, source_cluster_name, source_namespace):
    """
    Verify if a resource is present in a given namespace and check for specific annotations.
    :param core_api: CoreV1Api instance for Kubernetes API interaction.
    :param namespace: The namespace where the resource will be checked.
    :param resource_name: The name of the resource to verify.
    :param resource_type: The type of the resource ('secret' or 'configmap').
    :param source_cluster_name: The name of the source cluster where the resource originated.
    :param source_namespace: The name of the source namespace where the resource originated.
    """
    try:
        if resource_type == 'secret':
            resource = target_core_api.read_namespaced_secret(resource_name, namespace)
        elif resource_type == 'configmap':
            resource = target_core_api.read_namespaced_config_map(resource_name, namespace)

        annotations = resource.metadata.annotations
        expected_annotations = {
            "keess.powerhrg.com/source-cluster": source_cluster_name,
            "keess.powerhrg.com/source-namespace": source_namespace,
            # The source-resource-version will be checked separately for equality
        }

        for key, expected_value in expected_annotations.items():
            if annotations.get(key) != expected_value:
                log_error(f"{resource_type.capitalize()} '{resource_name}' in namespace '{namespace}' does not have the correct annotation '{key}': expected '{expected_value}', found '{annotations.get(key)}'")
                return False

        # Check source resource version
        target_resource_version = annotations.get("keess.powerhrg.com/source-resource-version")
        if resource_type == 'secret':
            source_resource = source_core_api.read_namespaced_secret(resource_name, source_namespace)
        elif resource_type == 'configmap':
            source_resource = source_core_api.read_namespaced_config_map(resource_name, source_namespace)

        if source_resource.metadata.resource_version != target_resource_version:
            log_error(f"{resource_type.capitalize()} '{resource_name}' in namespace '{namespace}' has mismatched source resource version: expected '{source_resource.metadata.resource_version}', found '{target_resource_version}'")
            return False

        log_success(f"{resource_type.capitalize()} '{resource_name}' in namespace '{namespace}' has been verified successfully with all annotations.")
        return True

    except client.exceptions.ApiException as e:
        log_error(f"Failed to find {resource_type} '{resource_name}' in namespace '{namespace}': {e}")
        return False


def test_scenario_1(core_api, source_cluster_name, namespace, secret_name, configmap_name, target_namespaces):
    """
    Test secret and configmap synchronization to two specific namespaces.
    :param core_api: CoreV1Api instance for Kubernetes API interaction.
    :param namespace: The source namespace where the secret and configmap are initially created.
    :param secret_name: The name of the secret to synchronize.
    :param configmap_name: The name of the configmap to synchronize.
    :param target_namespaces: A list of namespaces to which the resources will be synchronized.
    """
    # Setup labels and annotations for synchronization
    labels = {"keess.powerhrg.com/sync": "namespace"}
    annotations = {"keess.powerhrg.com/namespaces-names": ",".join(target_namespaces)}

    log_info("Waiting for resources to be created...")
    time.sleep(5)  # Adjust this delay as necessary for your environment

    # Apply labels and annotations to the secret and configmap
    apply_labels_and_annotations(core_api, namespace, secret_name, labels, annotations, 'secret')
    apply_labels_and_annotations(core_api, namespace, configmap_name, labels, annotations, 'configmap')

    log_info("Waiting for synchronization to complete...")
    time.sleep(WAIT_TIME)  # Adjust this delay as necessary for your environment

    # Verify the presence of the secret and configmap in the target namespaces
    for target_namespace in target_namespaces:
        verify_resource_in_namespace(core_api, core_api, target_namespace, secret_name, 'secret', source_cluster_name, namespace)
        verify_resource_in_namespace(core_api, core_api, target_namespace, configmap_name, 'configmap', source_cluster_name, namespace)


def test_scenario_2(core_api):
    # Scenario 2: Synchronize to all namespaces
    pass

def test_scenario_3(core_api, source_cluster_name, source_namespace, secret_name, configmap_name, label_selector):
    """
    Test synchronization to namespaces matching a specific label, including checks for source cluster and namespace.
    :param core_api: CoreV1Api instance for Kubernetes API interaction.
    :param source_namespace: The source namespace where resources are created.
    :param secret_name: Name of the secret to synchronize.
    :param configmap_name: Name of the configmap to synchronize.
    :param source_cluster_name: Name of the source cluster for annotation verification.
    """
    namespaces = core_api.list_namespace(label_selector=label_selector)
    target_namespaces = [ns.metadata.name for ns in namespaces.items if ns.metadata.name != source_namespace]

    # Apply labels and annotations for synchronization
    labels = {"keess.powerhrg.com/sync": "namespace"}
    annotations = {
        "keess.powerhrg.com/namespace-label": label_selector,
        # Include source cluster and namespace annotations directly if needed for setup
    }
    apply_labels_and_annotations(core_api, source_namespace, secret_name, labels, annotations, 'secret')
    apply_labels_and_annotations(core_api, source_namespace, configmap_name, labels, annotations, 'configmap')

    log_info("Waiting for synchronization to matching namespaces...")
    time.sleep(WAIT_TIME)  # Adjust based on expected synchronization time

    # Verification
    for ns in target_namespaces:
        verify_resource_in_namespace(core_api, core_api, ns, secret_name, 'secret', source_cluster_name, source_namespace)
        verify_resource_in_namespace(core_api, core_api, ns, configmap_name, 'configmap', source_cluster_name, source_namespace)

    log_info("Test scenario 3 completed.")



def test_scenario_4(source_core_api, target_core_api, source_namespace, secret_name, configmap_name, source_cluster_name, target_cluster_name):
    """
    Test synchronization of a secret to a different cluster.
    :param core_api: CoreV1Api instance for Kubernetes API interaction in the source cluster.
    :param source_namespace: The namespace in the source cluster where the secret is created.
    :param secret_name: The name of the secret to be synchronized.
    :param source_cluster_name: Name of the source cluster (for verification purposes).
    :param target_cluster_name: Name of the target cluster where the secret should be synchronized.
    """
    labels = {"keess.powerhrg.com/sync": "cluster"}
    annotations = {"keess.powerhrg.com/clusters": target_cluster_name}

    # Apply labels and annotations to the secret for cross-cluster synchronization
    apply_labels_and_annotations(source_core_api, source_namespace, secret_name, labels, annotations, 'secret')
    apply_labels_and_annotations(source_core_api, source_namespace, configmap_name, labels, annotations, 'configmap')

    log_info("Labels and annotations applied for cross-cluster synchronization. Waiting for synchronization to complete...")
    time.sleep(WAIT_TIME)  # Adjust based on expected synchronization time

    # Verify the presence of the secret in the target cluster
    verify_resource_in_namespace(source_core_api, target_core_api, source_namespace, secret_name, 'secret', source_cluster_name, source_namespace)
    verify_resource_in_namespace(source_core_api, target_core_api, source_namespace, configmap_name, 'configmap', source_cluster_name, source_namespace)

    log_info("Test scenario 4 completed.")


def main():
    source_cluster_name = "kind-source-cluster"
    target_cluster_name = "kind-destination-cluster"
    source_namespace = "test-namespace"
    secret_name = "new-test-secret"
    configmap_name = "new-test-configmap"
    destination_namespaces = ["test-namespace-dest-1", "test-namespace-dest-2"]
    label_selector = "keess.powerhrg.com/testing=yes"

    source_core_api = get_k8s_client_for_cluster(source_cluster_name)
    target_core_api = get_k8s_client_for_cluster(target_cluster_name)

    # Initial cleanup
    cleanup_resources(source_core_api, target_core_api, source_namespace, destination_namespaces)

    # Setup resources for tests
    setup_resources_for_test(source_core_api, target_core_api, source_namespace, secret_name, configmap_name, destination_namespaces, label_selector)


    # Execute each test scenario
    test_scenario_1(source_core_api, source_cluster_name, source_namespace, secret_name, configmap_name, destination_namespaces)

    # Execute test scenario 3
    test_scenario_3(source_core_api, source_cluster_name, source_namespace, secret_name, configmap_name, label_selector)

    # Execute test scenario 4
    test_scenario_4(source_core_api, target_core_api, source_namespace, secret_name, configmap_name, source_cluster_name, target_cluster_name)


    # Final cleanup
    cleanup_resources(source_core_api, target_core_api, source_namespace, destination_namespaces)

if __name__ == "__main__":
    main()
