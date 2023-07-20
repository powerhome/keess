package kube_syncer

import (
	"context"
	"flag"
	"path/filepath"
	"time"

	abstractions "keess/kube_syncer/abstractions"

	errorsTypes "k8s.io/apimachinery/pkg/api/errors"

	"github.com/appscode/go/strings"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Represents a base structure for any syncer.
type Syncer struct {
	kubeClients map[string]*kubernetes.Clientset

	sourceContext string

	destinationContexts []string

	// The logger object.
	logger *zap.SugaredLogger
}

func init() {
	abstractions.EntitiesToAllNamespaces["ConfigMaps"] = make(map[string]runtime.Object)
	abstractions.EntitiesToAllNamespaces["Secrets"] = make(map[string]runtime.Object)
	abstractions.EntitiesToLabeledNamespaces["ConfigMaps"] = make(map[string]runtime.Object)
	abstractions.EntitiesToLabeledNamespaces["Secrets"] = make(map[string]runtime.Object)
}

// Load the kubeClient based in the given configuration.
func (s *Syncer) Start(kubeConfigPath string, developmentMode bool, sourceContext string, destinationContexts []string) error {
	var loggerConfig zap.Config

	if developmentMode {
		loggerConfig = zap.NewDevelopmentConfig()
	} else {
		loggerConfig = zap.NewProductionConfig()
	}

	loggerConfig.EncoderConfig.TimeKey = "timestamp"
	loggerConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)

	zapLogger, err := loggerConfig.Build()
	if err != nil {
		return err
	}
	abstractions.Logger = zapLogger.Sugar()
	s.logger = abstractions.Logger

	s.sourceContext = sourceContext
	s.destinationContexts = destinationContexts
	var kubeconfig *string

	if kubeConfigPath == "" {

		flagLookup := flag.Lookup("kubeconfig")
		if flagLookup == nil {
			if home := homedir.HomeDir(); home != "" {
				kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
			} else {
				kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
			}
			flag.Parse()
		} else {
			lookup := flagLookup.Value.String()
			kubeconfig = &lookup
		}

	} else {
		kubeconfig = &kubeConfigPath
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// use the source context if it's passed
	if len(s.sourceContext) > 0 {
		config, err = buildConfigWithContextFromFlags(s.sourceContext, *kubeconfig)
		if err != nil {
			panic(err)
		}
	}

	// create the clientset
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	s.kubeClients = map[string]*kubernetes.Clientset{}
	s.kubeClients[s.sourceContext] = client

	for _, context := range destinationContexts {
		config, err := buildConfigWithContextFromFlags(context, *kubeconfig)
		if err != nil {
			panic(err)
		}

		// create the clientset
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}

		s.kubeClients[context] = client
	}

	s.logger.Info("Starting Keess.")

	return nil
}

func (s *Syncer) Run() error {
	kubeClient := *s.kubeClients[s.sourceContext]

	var namespaceWatcher = NamespaceWatcher{
		kubeClient: &kubeClient,
		logger:     s.logger,
	}

	var configMapWatcher = ConfigMapWatcher{
		kubeClient: &kubeClient,
		logger:     s.logger,
	}

	var secretWatcher = SecretWatcher{
		kubeClient: &kubeClient,
		logger:     s.logger,
	}

	s.logger.Info("Executing bootstrap process.")

	// First of all we need to load all namespaces.
	namespaceList, err := kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, namespace := range namespaceList.Items {
		abstractions.Namespaces[namespace.Name] = namespace.DeepCopy()
	}

	// Now list all ConfigMaps that must be synchronized.
	configMapList, err := kubeClient.CoreV1().ConfigMaps(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
		LabelSelector: abstractions.LabelSelector,
	})
	if err != nil {
		return err
	}

	for _, configMap := range configMapList.Items {
		if configMap.Annotations[abstractions.NamespaceNameAnnotation] == abstractions.All {
			abstractions.EntitiesToAllNamespaces["ConfigMaps"][configMap.Name] = configMap.DeepCopyObject()
		}
		namespaceLabelAnnotation := configMap.Annotations[abstractions.NamespaceLabelAnnotation]
		if !strings.IsEmpty(&namespaceLabelAnnotation) {
			abstractions.EntitiesToLabeledNamespaces["ConfigMaps"][configMap.Name] = configMap.DeepCopyObject()
		}
	}

	// Now list all Secrets that must be synchronized.
	secretList, err := kubeClient.CoreV1().Secrets(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
		LabelSelector: abstractions.LabelSelector,
	})
	if err != nil {
		return err
	}

	for _, secret := range secretList.Items {
		if secret.Annotations[abstractions.NamespaceNameAnnotation] == abstractions.All {
			abstractions.EntitiesToAllNamespaces["Secrets"][secret.Name] = secret.DeepCopyObject()
		}
		namespaceLabelAnnotation := secret.Annotations[abstractions.NamespaceLabelAnnotation]
		if !strings.IsEmpty(&namespaceLabelAnnotation) {
			abstractions.EntitiesToLabeledNamespaces["Secrets"][secret.Name] = secret.DeepCopyObject()
		}
	}

	for currentContext, kubeClient := range s.kubeClients {

		// Now list all ConfigMaps that are manageg by Keess.
		managedConfigMapList, err := kubeClient.CoreV1().ConfigMaps(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
			LabelSelector: abstractions.ManagegLabelSelector,
		})
		if err != nil {
			return err
		}

		for _, configMap := range managedConfigMapList.Items {
			var entity abstractions.KubernetesEntity

			// Get the source namespace name.
			sourceNamespace := configMap.Annotations[abstractions.SourceNamespaceAnnotation]
			sourceContext := configMap.Annotations[abstractions.SourceClusterAnnotation]

			sourceKubeClient := s.kubeClients[sourceContext]
			sourceConfigMap, err := sourceKubeClient.CoreV1().ConfigMaps(sourceNamespace).Get(context.TODO(), configMap.Name, metav1.GetOptions{})

			if err != nil && !errorsTypes.IsNotFound(err) {
				return err
			}

			// Check if source configmap was deleted.
			if errorsTypes.IsNotFound(err) {
				entity = abstractions.NewKubernetesEntity(s.kubeClients, &configMap, abstractions.ConfigMapEntity, sourceNamespace, configMap.Namespace, sourceContext, currentContext)

				err := entity.Delete()
				if err != nil && !errorsTypes.IsNotFound(err) {
					return err
				} else {
					s.logger.Infof("The ConfigMap '%s' was deleted in namespace '%s' on context '%s' because It was deleted in the source namespace '%s' on the source context '%s'.", configMap.Name, configMap.Namespace, currentContext, sourceNamespace, sourceContext)
				}
			}

			if err == nil {
				// Check if source configmap was changed.
				if sourceConfigMap.ResourceVersion != configMap.Annotations[abstractions.SourceResourceVersionAnnotation] {
					entity = abstractions.NewKubernetesEntity(s.kubeClients, sourceConfigMap, abstractions.ConfigMapEntity, sourceNamespace, configMap.Namespace, sourceContext, currentContext)
					err := entity.Update()
					if err != nil {
						return err
					} else {
						s.logger.Infof("The ConfigMap '%s' was updated in namespace '%s' on context '%s' because It was updated in the source namespace '%s' on the source context '%s'.", configMap.Name, configMap.Namespace, currentContext, sourceNamespace, sourceContext)
					}
				}
			}
		}

		// Now list all Secrets that are manageg by Keess.
		managedSecretList, err := kubeClient.CoreV1().Secrets(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
			LabelSelector: abstractions.ManagegLabelSelector,
		})
		if err != nil {
			return err
		}

		for _, secret := range managedSecretList.Items {
			var entity abstractions.KubernetesEntity

			// Get the source namespace name.
			sourceNamespace := secret.Annotations[abstractions.SourceNamespaceAnnotation]
			sourceContext := secret.Annotations[abstractions.SourceClusterAnnotation]

			sourceKubeClient := s.kubeClients[sourceContext]
			sourceSecret, err := sourceKubeClient.CoreV1().Secrets(sourceNamespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})

			if err != nil && !errorsTypes.IsNotFound(err) {
				return err
			}

			// Check if source secret was deleted.
			if errorsTypes.IsNotFound(err) {
				entity = abstractions.NewKubernetesEntity(s.kubeClients, &secret, abstractions.SecretEntity, sourceNamespace, secret.Namespace, sourceContext, currentContext)

				err := entity.Delete()
				if err != nil && !errorsTypes.IsNotFound(err) {
					return err
				} else {
					s.logger.Infof("The Secret '%s' was deleted in namespace '%s' on context '%s' because It was deleted in the source namespace '%s' on the source context '%s'.", secret.Name, secret.Namespace, currentContext, sourceNamespace, sourceContext)
				}
			}

			if err == nil {
				// Check if source secret was changed.
				if sourceSecret.ResourceVersion != secret.Annotations[abstractions.SourceResourceVersionAnnotation] {
					entity = abstractions.NewKubernetesEntity(s.kubeClients, sourceSecret, abstractions.SecretEntity, sourceNamespace, secret.Namespace, sourceContext, currentContext)
					err := entity.Update()
					if err != nil {
						return err
					} else {
						s.logger.Infof("The Secret '%s' was updated in namespace '%s' on context '%s' because It was updated in the source namespace '%s' on the source context '%s'.", secret.Name, secret.Namespace, currentContext, sourceNamespace, sourceContext)
					}
				}
			}
		}
	}

	s.logger.Info("The bootstrap process was finished.")

	// Than starting watching for changes on configmpas, secrets, and namespaces.
	configMapChan := configMapWatcher.Watch()
	secretChan := secretWatcher.Watch()
	namespaceChan := namespaceWatcher.Watch()

	eventsChan := multiplex(configMapChan, secretChan, namespaceChan)

	go func() {
		for {
			for event := range eventsChan {
				event.Sync(s.sourceContext, &s.kubeClients)
			}
		}
	}()

	return nil
}

func multiplex(configMapChan, secretChan, namespaceChan <-chan abstractions.ISynchronizable) <-chan abstractions.ISynchronizable {
	outputChan := make(chan abstractions.ISynchronizable)

	go func() {
		for {
			select {
			case event := <-configMapChan:
				outputChan <- event
			case event := <-secretChan:
				outputChan <- event
			case event := <-namespaceChan:
				outputChan <- event
			}
		}
	}()

	return outputChan
}

func buildConfigWithContextFromFlags(context string, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}
