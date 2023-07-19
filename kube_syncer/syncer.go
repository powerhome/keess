package kube_syncer

import (
	"context"
	"flag"
	"path/filepath"
	"time"

	abstractions "keess/kube_syncer/abstractions"

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
func (s *Syncer) Start(kubeConfigPath string, sourceContext string, destinationContexts []string) error {
	loggerConfig := zap.NewProductionConfig()
	//loggerConfig := zap.NewDevelopmentConfig()
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

	// Now list all ConfigMaps that should be present in every namespace.
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
		namespaceLabelAnnotation := configMap.Annotations[abstractions.NamespaceNameAnnotation]
		if strings.IsEmpty(&namespaceLabelAnnotation) {
			abstractions.EntitiesToAllNamespaces["ConfigMaps"][configMap.Name] = configMap.DeepCopyObject()
		}
	}

	// Now list all Secrets that should be present in every namespace.
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
		if strings.IsEmpty(&namespaceLabelAnnotation) {
			abstractions.EntitiesToAllNamespaces["Secrets"][secret.Name] = secret.DeepCopyObject()
		}
	}

	// // Now list all ConfigMaps that are manageg by Keess.
	// managedConfigMapList, err := &kubeClient.CoreV1().ConfigMaps(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
	// 	LabelSelector: abstractions.ManagegLabelSelector,
	// })
	// if err != nil {
	// 	return err
	// }

	// for _, configMap := range managedConfigMapList.Items {
	// 	// Get the source namespace.
	// 	sourceNamespace := configMap.Annotations[abstractions.SourceNamespaceAnnotation]
	// 	sourceConfigMap, err := &kubeClient.CoreV1().ConfigMaps(sourceNamespace).Get(context.TODO(), configMap.Name, metav1.GetOptions{})
	// 	if err != nil && !errorsTypes.IsNotFound(err) {
	// 		return err
	// 	}

	// 	// Check if source configmap was deleted.
	// 	if errorsTypes.IsNotFound(err) {
	// 		err := &kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Delete(context.TODO(), configMap.Name, metav1.DeleteOptions{})
	// 		if err != nil && !errorsTypes.IsNotFound(err) {
	// 			return err
	// 		}
	// 		if errorsTypes.IsNotFound(err) {
	// 			s.logger.Debugf("The ConfigMap '%s' was already deleted from namespace '%s'.", configMap.Name, configMap.Namespace)
	// 		}
	// 	}

	// 	// Check if source configmap was changed.
	// 	if sourceConfigMap.ResourceVersion != configMap.Annotations[abstractions.SourceResourceVersionAnnotation] {

	// 		err := &kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Update(context.TODO())
	// 	}
	// }

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
