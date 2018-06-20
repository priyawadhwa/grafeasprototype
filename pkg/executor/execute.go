package executor

import (
	"context"
	"encoding/json"
	"fmt"
	grafeas "github.com/grafeas/client-go/v1alpha1"
	grafeasprototypev1beta1 "github.com/priyawadhwa/grafeasprototype/pkg/apis/grafeasprototype/v1beta1"
	clientset "github.com/priyawadhwa/grafeasprototype/pkg/client/clientset/versioned"
	"github.com/sirupsen/logrus"
	googleAuth "golang.org/x/oauth2/google"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/url"
	"strings"
)

var (
	master     string
	kubeconfig string
	authScope  = "https://www.googleapis.com/auth/cloud-platform"
)

func Execute(m, k string) error {
	master = m
	kubeconfig = k
	iprs, err := getImageSecurityPolicies()
	if err != nil {
		return fmt.Errorf("Error getting image policy requirements: %v", err)
	}
	for _, ipr := range iprs {
		logrus.Infof("Checking images in namespace %s with maximum vulnz severity %s", ipr.Namespace, ipr.Spec.PackageVulernerabilityRequirements.MaximumSeverity)
		podsToImages, err := getImagesInNamespace(ipr.Namespace)
		if err != nil {
			return err
		}
		for pod, images := range podsToImages {
			for _, image := range images {
				logrus.Infof("Checking for vulnz in %s", image)
				occs, err := getOccurrences(image)
				if err != nil {
					fmt.Printf("couldn't get occurrences: %v", err)
					continue
				}
				filtered := filterOccurrences(occs, ipr.Spec.PackageVulernerabilityRequirements.MaximumSeverity)
				if len(filtered) > 0 {
					logrus.Errorf("Found vulnz in %s with severity greater than %s", image, ipr.Spec.PackageVulernerabilityRequirements.MaximumSeverity)
					for _, f := range filtered {
						logrus.Errorf("%s with severity %s", f.NoteName, f.VulnerabilityDetails.Severity)
					}
					if err := addAnnotation(pod, ipr.Namespace, "Some error in this pod."); err != nil {
						logrus.Errorf("Error adding annotation to pod %s", pod)
					}

				} else {
					logrus.Infof("No vulnz exceeding severity %s found in image %s", ipr.Spec.PackageVulernerabilityRequirements.MaximumSeverity, image)
				}
			}
		}
	}
	return nil
}

func addAnnotation(name, namespace, annotation string) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	pod, err := clientset.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	annotations := map[string]string{}
	if pod.Annotations != nil {
		annotations = pod.Annotations
	}
	annotations["invalidImageSecPolicy"] = annotation
	pod.SetAnnotations(annotations)
	logrus.Infof("Set annotation for %s to %s", pod.Name, annotations)
	return nil
}

func getImageSecurityPolicies() ([]grafeasprototypev1beta1.ImageSecurityPolicy, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Error building kubeconfig: %v", err)
	}

	exampleClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building example clientset: %v", err)
	}
	list, err := exampleClient.GrafeasprototypeV1beta1().ImageSecurityPolicies("").List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error listing all image policy requirements: %v", err)
	}
	return list.Items, nil
}

func getImagesInNamespace(namespace string) (map[string][]string, error) {
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
	podsToImages := map[string][]string{}
	for _, pod := range pods.Items {
		var images []string
		for _, ic := range pod.Spec.InitContainers {
			images = append(images, ic.Image)
		}
		for _, c := range pod.Spec.Containers {
			images = append(images, c.Image)
		}
		podsToImages[pod.Name] = images
	}
	return podsToImages, nil
}

func getOccurrences(image string) ([]grafeas.Occurrence, error) {
	httpsImage := fmt.Sprintf("https://%s", image)
	filter := fmt.Sprintf("resourceUrl=\"%s\"", httpsImage)
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
	q.Set("filter", filter)
	u.RawQuery = q.Encode()
	ctx := context.Background()
	c, err := googleAuth.DefaultClient(ctx, authScope)
	if err != nil {
		return nil, err
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
