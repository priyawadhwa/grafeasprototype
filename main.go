package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	grafeas "github.com/grafeas/client-go/v1alpha1"
	grafeasprototypev1beta1 "github.com/priyawadhwa/grafeasprototype/pkg/apis/grafeasprototype/v1beta1"
	clientset "github.com/priyawadhwa/grafeasprototype/pkg/client/clientset/versioned"
	"github.com/sirupsen/logrus"
	googleAuth "golang.org/x/oauth2/google"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/url"
	"os"
	"strings"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	master     = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	authScope  = "https://www.googleapis.com/auth/cloud-platform"
)

func main() {
	if err := execute(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}

func execute() error {
	iprs, err := getImageSecurityPolicies()
	if err != nil {
		return fmt.Errorf("error getting image policy requirements: %v", err)
	}
	for _, ipr := range iprs {
		fmt.Println(ipr)
		images, err := getImagesInNamespace(ipr.Namespace)
		if err != nil {
			return err
		}
		for _, image := range images {
			logrus.Infof("checking for vulnz in %s", image)
			occs, err := getOccurrences(image)
			if err != nil {
				return fmt.Errorf("couldn't get occurrences: %v", err)
			}
			fmt.Println("occs:", occs)
			filtered := filterOccurrences(occs, ipr.Spec.PackageVulernerabilityRequirements.MaximumSeverity)
			if len(filtered) > 0 {
				fmt.Println("Found vulnz in", image)
				for _, f := range filtered {
					fmt.Println(f.VulnerabilityDetails.AffectedLocation)
				}
			} else {
				logrus.Infof("No vulnz exceeding severity %s found in image %s", ipr.Spec.PackageVulernerabilityRequirements.MaximumSeverity, image)
			}
		}
	}
	return nil
}

func getImageSecurityPolicies() ([]grafeasprototypev1beta1.ImageSecurityPolicy, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Error building kubeconfig: %v", err)
	}

	exampleClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building example clientset: %v", err)
	}
	list, err := exampleClient.GrafeasprototypeV1beta1().ImageSecurityPolicies("default").List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error listing all image policy requirements: %v", err)
	}
	return list.Items, nil
}

func getImagesInNamespace(namespace string) ([]string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var images []string
	for _, pod := range pods.Items {
		for _, ic := range pod.Spec.InitContainers {
			images = append(images, ic.Image)
		}
		for _, c := range pod.Spec.Containers {
			images = append(images, c.Image)
		}
	}
	return images, nil
}

func getOccurrences(image string) ([]grafeas.Occurrence, error) {
	filter := fmt.Sprintf("kind=\"PACKAGE_VULNERABILITY\" AND resourceUrl=\"%s\"", image)
	sp := strings.Split(strings.TrimPrefix(image, "https://"), "/")
	if len(sp) < 3 {
		return nil, fmt.Errorf("Malformed image %s should be gcr.io/<project>/<name>", image)
	}
	imgProject := sp[1]

	path := fmt.Sprintf("v1alpha1/projects/%s/occurrences", imgProject)

	u := &url.URL{
		Scheme: "https",
		Host:   "containeranalysis.googleapis.com",
		Path:   path,
	}
	q := &url.Values{}
	// q.Set("pageSize", "1000") // Just do one page
	q.Set("filter", filter)
	// if token != "" {
	// 	q.Set("pageToken", token)
	// }
	u.RawQuery = q.Encode()
	ctx := context.Background()
	c, err := googleAuth.DefaultClient(ctx, authScope)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := c.Get(u.String())
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("%v", string(data))
		return nil, fmt.Errorf("non 200 status code: %d", resp.StatusCode)
	}

	oResp := grafeas.ListOccurrencesResponse{}
	if err := json.Unmarshal(data, &oResp); err != nil {
		return nil, err
	}

	return oResp.Occurrences, nil
}

var sevOrder = [...]string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}

func sevGE(val, comp string) bool {
	if val == "" {
		return false
	}
	for _, sev := range sevOrder {
		if comp == sev {
			return true
		}
		if val == sev {
			return false
		}
	}
	return false
}

func filterOccurrences(occs []grafeas.Occurrence, maxSeverity string) []grafeas.Occurrence {
	new := make([]grafeas.Occurrence, 0)
	for _, o := range occs {
		if sevGE(o.VulnerabilityDetails.Severity, maxSeverity) {
			new = append(new, o)
		}
	}
	return new
}
