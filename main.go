package main

import (
	"flag"
	"github.com/priyawadhwa/grafeasprototype/pkg/executor"

	"github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"os"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	master     = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
)

func main() {
	if err := executor.Execute(*master, *kubeconfig); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}
