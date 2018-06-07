package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"

	"k8s.io/client-go/tools/clientcmd"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	clientset "github.com/priyawadhwa/grafeasprototype/pkg/client/clientset/versioned"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/typed/rbac/v1beta1"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	master     = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
)

func main() {
	flag.Parse()

	cfg, err := clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %v", err)
	}

	exampleClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building example clientset: %v", err)
	}
	list, err := exampleClient.GrafeasprototypeV1beta1().ImagePolicyRequirements("default").List(metav1.ListOptions{})
	if err != nil {
		glog.Fatalf("Error listing all image policy requirements: %v", err)
	}

	for _, ipr := range list.Items {
		fmt.Printf("Image policy requirement: %s with severity %q\n", ipr.Name, ipr.Spec.PackageVulernerabilityRequirements.MaximumSeverity)
	}
}

func createServiceAccount() error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(clientConfig)
	_, err = client.CoreV1().ServiceAccounts("").Create(&v1.ServiceAccount{
		metav1.ObjectMeta: {
			Name:      "prototype-service-account",
			Namespace: "default",
		},
	})
	return err
}

func createClusterRoleBinding() error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return err
	}
	rbacClient, err := v1beta1.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	_, err = rbacClient.ClusterRoleBindings().Create(&v1beta1.ClusterRoleBinding{})
	return err
}

// func listProjects() error {
// 	pool, err := x509.SystemCertPool()
// 	if err != nil {
// 		return err
// 	}
// 	// error handling omitted
// 	creds := credentials.NewClientTLSFromCert(pool, "")
// 	perRPC, err := oauth.NewServiceAccountFromFile("service-account.json")
// 	if err != nil {
// 		return err
// 	}
// 	conn, err := grpc.Dial(
// 		"containeranalysis.googleapis.com",
// 		grpc.WithTransportCredentials(creds),
// 		grpc.WithPerRPCCredentials(perRPC),
// 	)
// 	if err != nil {
// 		return err
// 	}
// 	// error handling omitted
// 	defer conn.Close()
// 	client := pb.NewGrafeasProjectsClient(conn)
// 	// List projects
// 	resp, err := client.ListProjects(context.Background(), &pb.ListProjectsRequest{})
// 	if err != nil {
// 		return err
// 	}
// 	fmt.Println("Projects:", resp)
// 	return nil
// }
