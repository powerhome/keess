from kubernetes import client, config
from deepdiff import DeepDiff
import time
import json

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

def delete_resource_type(core_api, namespace, resource_name, resource_type='secret'):
    # Delete resources in the source namespace
    try:
        if resource_type == "secret":
            core_api.delete_namespaced_secret(resource_name, namespace)
        elif resource_type == "configmap":
            core_api.delete_namespaced_config_map(resource_name, namespace)
        else:
            log_error(f"Resource type '{resource_type}' passed isn't valid.")
            return
        log_info(f"Deleted {resource_type} '{resource_name}' in namespace '{namespace}'.")
    except client.exceptions.ApiException as e:
        if e.status != 404:  # Ignore not found errors
            log_error(f"Error deleting {resource_type} '{resource_name}' in namespace '{namespace}': {e}")

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
        time.sleep(3)  # Give it some time for the namespace to be deleted
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

def get_all_resources_type(core_api, resource_type='secret'):
    """
    Get a list of all resoruces of specified kind.
    :param core_api: CoreV1Api instance for Kubernetes API interaction.
    :param resource_type: Type of resource that will be returned.
    :return List of all the resources of Kind specified in the cluster.
    """
    try:
        if resource_type == "secret":
            response = core_api.list_secret_for_all_namespaces()
        elif resource_type == "configmap":
            response = core_api.list_config_map_for_all_namespaces()
        else:
            log_error(f"Resource type '{resource_type}' passed isn't valid.")
            return
        result = {}
        # filter fields considered for diff
        for i in response.items:
            result[f"{i.metadata.namespace}/{i.metadata.name}"] = {
                'data': i.data,
                'metadata': {
                    'annotations': i.metadata.annotations,
                    'creation_timestamp': i.metadata.creation_timestamp,
                    'labels': i.metadata.labels,
                    'name': i.metadata.name,
                    'namespace': i.metadata.namespace,
                    'uid': i.metadata.uid,
                }
            }
        return result
    except client.exceptions.ApiException as e:
        log_error(f"Failed to get all resources of kind '{resource_type}': {e}")

def compare_json(json1, json2):
    return DeepDiff(json.loads(json1),json.loads(json2))

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

def remove_labels_and_anonotations(core_api, namespace, resource_name, labels, annotations, resource_type='secret'):
    """
    Remove labels and annotations from a resource.
    """
    metadata = client.V1ObjectMeta(labels=labels, annotations=annotations)
    if resource_type == 'secret':
        body = client.V1Secret(metadata=metadata)
        core_api.patch_namespaced_secret(resource_name, namespace, body)
    elif resource_type == 'configmap':
        body = client.V1ConfigMap(metadata=metadata)
        core_api.patch_namespaced_config_map(resource_name, namespace, body)
    log_info(f"Labels and annotations removed from {resource_type} '{resource_name}' in namespace '{namespace}'.")

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
    Test secret and configmap synchronization to two specific namespaces in the same cluster.
    :param core_api: CoreV1Api instance for Kubernetes API interaction.
    :param namespace: The source namespace where the secret and configmap are initially created.
    :param secret_name: The name of the secret to synchronize.
    :param configmap_name: The name of the configmap to synchronize.
    :param target_namespaces: A list of namespaces to which the resources will be synchronized.
    """
    log_info("Test scenario 1 running...")
    # Setup labels and annotations for synchronization
    labels = {"keess.powerhrg.com/sync": "namespace"}
    annotations = {"keess.powerhrg.com/namespaces-names": ",".join(target_namespaces)}

    # Get all secrets and configmaps before test
    initial_all_secrets = get_all_resources_type(core_api, "secret")
    initial_all_configmaps = get_all_resources_type(core_api, "configmap")

    # Apply labels and annotations to the secret and configmap
    apply_labels_and_annotations(core_api, namespace, secret_name, labels, annotations, 'secret')
    apply_labels_and_annotations(core_api, namespace, configmap_name, labels, annotations, 'configmap')

    log_info("Waiting for synchronization to complete...")
    time.sleep(WAIT_TIME)  # Adjust this delay as necessary for your environment

    # Verify the presence of the secret and configmap in the target namespaces
    for target_namespace in target_namespaces:
        verify_resource_in_namespace(core_api, core_api, target_namespace, secret_name, 'secret', source_cluster_name, namespace)
        verify_resource_in_namespace(core_api, core_api, target_namespace, configmap_name, 'configmap', source_cluster_name, namespace)

    # Compare all secrets and configmaps after test
    all_secrets = get_all_resources_type(core_api, "secret")
    all_configmaps = get_all_resources_type(core_api, "configmap")
    ddiff_secrets = DeepDiff(initial_all_secrets, all_secrets).to_json()
    ddiff_configmaps = DeepDiff(initial_all_configmaps, all_configmaps).to_json()

    expected_secrets_result = json.dumps({
        "type_changes": {
            "root['test-namespace/new-test-secret']['metadata']['annotations']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {
                    "keess.powerhrg.com/namespaces-names": "test-namespace-dest-1,test-namespace-dest-2"
                },
            },
            "root['test-namespace/new-test-secret']['metadata']['labels']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {"keess.powerhrg.com/sync": "namespace"},
            },
        },
        "dictionary_item_added": [
            "root['test-namespace-dest-1/new-test-secret']",
            "root['test-namespace-dest-2/new-test-secret']",
        ],
    })

    expected_configmaps_result = json.dumps({
        "type_changes": {
            "root['test-namespace/new-test-configmap']['metadata']['annotations']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {
                    "keess.powerhrg.com/namespaces-names": "test-namespace-dest-1,test-namespace-dest-2"
                },
            },
            "root['test-namespace/new-test-configmap']['metadata']['labels']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {"keess.powerhrg.com/sync": "namespace"},
            },
        },
        "dictionary_item_added": [
            "root['test-namespace-dest-1/new-test-configmap']",
            "root['test-namespace-dest-2/new-test-configmap']",
        ],
    })

    if expected_secrets_result != ddiff_secrets:
        log_info(f"ddiff_configmaps: \n{ddiff_secrets}")
        log_error(f"There were unexpected changes in secrets: \n{compare_json(expected_secrets_result,ddiff_secrets)}")

    if expected_configmaps_result != ddiff_configmaps:
        log_info(f"ddiff_configmaps: \n{ddiff_configmaps}")
        log_error(f"There were unexpected changes in configmaps: \n{compare_json(expected_configmaps_result,ddiff_configmaps)}")

    log_info("Test scenario 1 completed.")

def test_scenario_2(core_api, namespace, secret_name, configmap_name):
    """
    Delete origin secret and configmap, it expects replicas to be deleted.
    :param core_api: CoreV1Api instance for Kubernetes API interaction.
    :param namespace: The source namespace where the secret and configmap are initially created.
    :param secret_name: The name of the secret to synchronize.
    :param configmap_name: The name of the configmap to synchronize.
    """
    log_info("Test scenario 2 running...")

    # Get all secrets and configmaps before test
    initial_all_secrets = get_all_resources_type(core_api, "secret")
    initial_all_configmaps = get_all_resources_type(core_api, "configmap")

    # Delete origin secret and configmap
    delete_resource_type(core_api, namespace, secret_name, 'secret')
    delete_resource_type(core_api, namespace, configmap_name, 'configmap')

    log_info("Waiting for synchronization to complete...")
    time.sleep(WAIT_TIME)  # Adjust this delay as necessary for your environment

    # Compare all secrets and configmaps after test
    all_secrets = get_all_resources_type(core_api, "secret")
    all_configmaps = get_all_resources_type(core_api, "configmap")
    ddiff_secrets = DeepDiff(initial_all_secrets, all_secrets).to_json()
    ddiff_configmaps = DeepDiff(initial_all_configmaps, all_configmaps).to_json()

    expected_secrets_result = json.dumps({
        "dictionary_item_removed": [
            "root['test-namespace-dest-1/new-test-secret']",
            "root['test-namespace-dest-2/new-test-secret']",
            "root['test-namespace/new-test-secret']",
        ]
    })

    expected_configmaps_result = json.dumps({
        "dictionary_item_removed": [
            "root['test-namespace-dest-1/new-test-configmap']",
            "root['test-namespace-dest-2/new-test-configmap']",
            "root['test-namespace/new-test-configmap']",
        ]
    })

    if expected_secrets_result != ddiff_secrets:
        log_info(f"ddiff_configmaps: \n{ddiff_secrets}")
        log_error(f"There were unexpected changes in secrets: \n{compare_json(expected_secrets_result,ddiff_secrets)}")
    else:
        log_success(f"The deletion of the origin secret '{secret_name}' in namespace '{namespace}' triggered a deletion of its copies.")
    if expected_configmaps_result != ddiff_configmaps:
        log_info(f"ddiff_configmaps: \n{ddiff_configmaps}")
        log_error(f"There were unexpected changes in configmaps: \n{compare_json(expected_configmaps_result,ddiff_configmaps)}")
    else:
        log_success(f"The deletion of the origin configmap '{configmap_name}' in namespace '{namespace}' triggered a deletion of its copies.")

    log_info("Test scenario 2 completed.")

def test_scenario_3(core_api, source_cluster_name, source_namespace, secret_name, configmap_name, label_selector):
    """
    Test synchronization to namespaces matching a specific label, including checks for source cluster and namespace.
    :param core_api: CoreV1Api instance for Kubernetes API interaction.
    :param source_namespace: The source namespace where resources are created.
    :param secret_name: Name of the secret to synchronize.
    :param configmap_name: Name of the configmap to synchronize.
    :param source_cluster_name: Name of the source cluster for annotation verification.
    """
    log_info("Test scenario 3 running...")

    namespaces = core_api.list_namespace(label_selector=label_selector)
    target_namespaces = [ns.metadata.name for ns in namespaces.items if ns.metadata.name != source_namespace]

    # Get all secrets and configmaps before test
    initial_all_secrets = get_all_resources_type(core_api, "secret")
    initial_all_configmaps = get_all_resources_type(core_api, "configmap")

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

    # Compare all secrets and configmaps after test
    all_secrets = get_all_resources_type(core_api, "secret")
    all_configmaps = get_all_resources_type(core_api, "configmap")
    ddiff_secrets = DeepDiff(initial_all_secrets, all_secrets).to_json()
    ddiff_configmaps = DeepDiff(initial_all_configmaps, all_configmaps).to_json()

    expected_secrets_result = json.dumps({
        "type_changes": {
            "root['test-namespace/new-test-secret']['metadata']['annotations']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {
                    "keess.powerhrg.com/namespace-label": "keess.powerhrg.com/testing=yes"
                },
            },
            "root['test-namespace/new-test-secret']['metadata']['labels']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {"keess.powerhrg.com/sync": "namespace"},
            },
        },
        "dictionary_item_added": [
            "root['test-namespace-dest-1/new-test-secret']",
            "root['test-namespace-dest-2/new-test-secret']",
        ],
    })

    expected_configmaps_result = json.dumps({
        "type_changes": {
            "root['test-namespace/new-test-configmap']['metadata']['annotations']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {
                    "keess.powerhrg.com/namespace-label": "keess.powerhrg.com/testing=yes"
                },
            },
            "root['test-namespace/new-test-configmap']['metadata']['labels']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {"keess.powerhrg.com/sync": "namespace"},
            },
        },
        "dictionary_item_added": [
            "root['test-namespace-dest-1/new-test-configmap']",
            "root['test-namespace-dest-2/new-test-configmap']",
        ],
    })

    if expected_secrets_result != ddiff_secrets:
        log_info(f"ddiff_secrets: \n{ddiff_secrets}")
        log_error(f"There were unexpected changes in secrets: \n{compare_json(expected_secrets_result,ddiff_secrets)}")

    if expected_configmaps_result != ddiff_configmaps:
        log_info(f"ddiff_configmaps: \n{ddiff_configmaps}")
        log_error(f"There were unexpected changes in configmaps: \n{compare_json(expected_configmaps_result,ddiff_configmaps)}")

    log_info("Test scenario 3 completed.")

def test_scenario_4(core_api, namespace, secret_name, configmap_name):
    """
    Remove annotation and labels from origin secret and configmap, it expects replicas to be deleted.
    :param core_api: CoreV1Api instance for Kubernetes API interaction.
    :param namespace: The source namespace where the secret and configmap are initially created.
    :param secret_name: The name of the secret to synchronize.
    :param configmap_name: The name of the configmap to synchronize.
    """
    log_info("Test scenario 4 running...")

    # Setup labels and annotations for synchronization
    labels = {"keess.powerhrg.com/sync": None}
    annotations = {"keess.powerhrg.com/namespaces-names": None}

    # Get all secrets and configmaps before test
    initial_all_secrets = get_all_resources_type(core_api, "secret")
    initial_all_configmaps = get_all_resources_type(core_api, "configmap")

    # Remove labels and annotations to the secret and configmap
    apply_labels_and_annotations(core_api, namespace, secret_name, labels, annotations, 'secret')
    apply_labels_and_annotations(core_api, namespace, configmap_name, labels, annotations, 'configmap')

    log_info("Waiting for synchronization to complete...")
    time.sleep(WAIT_TIME)  # Adjust this delay as necessary for your environment

    # Compare all secrets and configmaps after test
    all_secrets = get_all_resources_type(core_api, "secret")
    all_configmaps = get_all_resources_type(core_api, "configmap")
    ddiff_secrets = DeepDiff(initial_all_secrets, all_secrets).to_json()
    ddiff_configmaps = DeepDiff(initial_all_configmaps, all_configmaps).to_json()

    expected_secrets_result = json.dumps({
        "dictionary_item_removed": [
            "root['test-namespace-dest-1/new-test-secret']",
            "root['test-namespace-dest-2/new-test-secret']",
            "root['test-namespace/new-test-secret']",
        ],
        "values_changed": {
            "root['test-namespace/new-test-secret']['metadata']['labels']['keess.powerhrg.com/sync']": {
                "new_value": None,
                "old_value": "namespace",
            },
            "root['test-namespace/new-test-secret']['metadata']['annotations']['keess.powerhrg.com/namespaces-names']": {
                "new_value": None,
                "old_value": "keess.powerhrg.com/testing=yes",
            }
        },
    })

    expected_configmaps_result = json.dumps({
        "dictionary_item_removed": [
            "root['test-namespace-dest-1/new-test-configmap']",
            "root['test-namespace-dest-2/new-test-configmap']",
        ],
        "values_changed": {
            "root['test-namespace/new-test-configmap']['metadata']['labels']['keess.powerhrg.com/sync']": {
                "new_value": None,
                "old_value": "namespace",
            },
            "root['test-namespace/new-test-configmap']['metadata']['annotations']['keess.powerhrg.com/namespaces-names']": {
                "new_value": None,
                "old_value": "keess.powerhrg.com/testing=yes",
            }
        },
    })

    if expected_secrets_result != ddiff_secrets:
        log_info(f"ddiff_secrets: \n{ddiff_secrets}")
        log_error(f"There were unexpected changes in secrets: \n{compare_json(expected_secrets_result,ddiff_secrets)}")
    else:
        log_success(f"The deletion of the origin secret '{secret_name}' in namespace '{namespace}' triggered a deletion of its copies.")
    if expected_configmaps_result != ddiff_configmaps:
        log_info(f"ddiff_configmaps: \n{ddiff_configmaps}")
        log_error(f"There were unexpected changes in configmaps: \n{compare_json(expected_configmaps_result,ddiff_configmaps)}")
    else:
        log_success(f"The deletion of the origin configmap '{configmap_name}' in namespace '{namespace}' triggered a deletion of its copies.")

    log_info("Test scenario 4 completed.")

def test_scenario_5(source_core_api, target_core_api, source_namespace, secret_name, configmap_name, source_cluster_name, target_cluster_name):
    """
    Test synchronization of a secret and configmap to a different cluster.
    :param core_api: CoreV1Api instance for Kubernetes API interaction in the source cluster.
    :param source_namespace: The namespace in the source cluster where the secret is created.
    :param secret_name: The name of the secret to be synchronized.
    :param source_cluster_name: Name of the source cluster (for verification purposes).
    :param target_cluster_name: Name of the target cluster where the secret should be synchronized.
    """
    log_info("Test scenario 5 running...")

    labels = {"keess.powerhrg.com/sync": "cluster"}
    annotations = {"keess.powerhrg.com/clusters": target_cluster_name}

    # Get all secrets and configmaps before test
    initial_source_all_secrets = get_all_resources_type(source_core_api, "secret")
    initial_source_all_configmaps = get_all_resources_type(source_core_api, "configmap")
    initial_target_all_secrets = get_all_resources_type(target_core_api, "secret")
    initial_target_all_configmaps = get_all_resources_type(target_core_api, "configmap")

    # Apply labels and annotations to the secret for cross-cluster synchronization
    apply_labels_and_annotations(source_core_api, source_namespace, secret_name, labels, annotations, 'secret')
    apply_labels_and_annotations(source_core_api, source_namespace, configmap_name, labels, annotations, 'configmap')

    log_info("Labels and annotations applied for cross-cluster synchronization. Waiting for synchronization to complete...")
    time.sleep(WAIT_TIME)  # Adjust based on expected synchronization time

    # Verify the presence of the secret in the target cluster
    verify_resource_in_namespace(source_core_api, target_core_api, source_namespace, secret_name, 'secret', source_cluster_name, source_namespace)
    verify_resource_in_namespace(source_core_api, target_core_api, source_namespace, configmap_name, 'configmap', source_cluster_name, source_namespace)

    # Compare all secrets and configmaps after test
    source_all_secrets = get_all_resources_type(source_core_api, "secret")
    source_all_configmaps = get_all_resources_type(source_core_api, "configmap")
    target_all_secrets = get_all_resources_type(target_core_api, "secret")
    target_all_configmaps = get_all_resources_type(target_core_api, "configmap")
    ddiff_source_secrets = DeepDiff(initial_source_all_secrets, source_all_secrets).to_json()
    ddiff_source_configmaps = DeepDiff(initial_source_all_configmaps, source_all_configmaps).to_json()
    ddiff_target_secrets = DeepDiff(initial_target_all_secrets, target_all_secrets).to_json()
    ddiff_target_configmaps = DeepDiff(initial_target_all_configmaps, target_all_configmaps).to_json()

    expected_source_secrets_result = json.dumps({
        "type_changes": {
            "root['test-namespace/new-test-secret']['metadata']['annotations']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {"keess.powerhrg.com/clusters": "kind-destination-cluster"},
            },
            "root['test-namespace/new-test-secret']['metadata']['labels']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {"keess.powerhrg.com/sync": "cluster"},
            },
        }
    })
    expected_source_configmaps_result = json.dumps({
        "type_changes": {
            "root['test-namespace/new-test-configmap']['metadata']['annotations']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {"keess.powerhrg.com/clusters": "kind-destination-cluster"},
            },
            "root['test-namespace/new-test-configmap']['metadata']['labels']": {
                "old_type": "NoneType",
                "new_type": "dict",
                "old_value": None,
                "new_value": {"keess.powerhrg.com/sync": "cluster"},
            },
        }
    })
    expected_target_secrets_result = json.dumps({"dictionary_item_added": ["root['test-namespace/new-test-secret']"]})
    expected_target_configmaps_result = json.dumps({"dictionary_item_added": ["root['test-namespace/new-test-configmap']"]})

    if expected_source_secrets_result != ddiff_source_secrets:
        log_info(f"ddiff_source_secrets: \n{ddiff_source_secrets}")
        log_error(f"There were unexpected changes in secrets in cluster '{source_cluster_name}': \n{compare_json(expected_source_secrets_result,ddiff_source_secrets)}")
    if expected_source_configmaps_result != ddiff_source_configmaps:
        log_info(f"ddiff_source_configmaps: \n{ddiff_source_configmaps}")
        log_error(f"There were unexpected changes in configmaps in cluster '{source_cluster_name}': \n{compare_json(expected_source_configmaps_result,ddiff_source_configmaps)}")
    if expected_target_secrets_result != ddiff_target_secrets:
        log_info(f"ddiff_target_secrets: \n{ddiff_target_secrets}")
        log_error(f"There were unexpected changes in secrets in cluster '{target_cluster_name}': \n{compare_json(expected_target_secrets_result,ddiff_target_secrets)}")
    if expected_target_configmaps_result != ddiff_target_configmaps:
        log_info(f"ddiff_target_configmaps: \n{ddiff_target_configmaps}")
        log_error(f"There were unexpected changes in configmaps in cluster '{target_cluster_name}': \n{compare_json(expected_target_configmaps_result,ddiff_target_configmaps)}")

    log_info("Test scenario 5 completed.")

def test_scenario_6(core_api):
    # Scenario 6: Synchronize to all namespaces
    pass

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


    # Execute test scenario 1
    test_scenario_1(source_core_api, source_cluster_name, source_namespace, secret_name, configmap_name, destination_namespaces)

    # Execute test scenario 2
    test_scenario_2(source_core_api, source_namespace, secret_name, configmap_name)

    # Setup resources again
    setup_resources_for_test(source_core_api, target_core_api, source_namespace, secret_name, configmap_name, destination_namespaces, label_selector)

    # Execute test scenario 3
    test_scenario_3(source_core_api, source_cluster_name, source_namespace, secret_name, configmap_name, label_selector)

    # Execute test scenario 4
    test_scenario_4(source_core_api, source_namespace, secret_name, configmap_name)

    # Cleanup
    cleanup_resources(source_core_api, target_core_api, source_namespace, destination_namespaces)
    # Setup resources for tests
    setup_resources_for_test(source_core_api, target_core_api, source_namespace, secret_name, configmap_name, destination_namespaces, label_selector)

    # Execute test scenario 5
    test_scenario_5(source_core_api, target_core_api, source_namespace, secret_name, configmap_name, source_cluster_name, target_cluster_name)


    # Final cleanup
    cleanup_resources(source_core_api, target_core_api, source_namespace, destination_namespaces)

if __name__ == "__main__":
    main()
