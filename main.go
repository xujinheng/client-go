package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()
	// fmt.Println(*kubeconfig)

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	// fmt.Println(config)

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	// fmt.Println(clientset)

	prompt()

	namespace := "tkc-workload"
	podClient := clientset.CoreV1().Pods(namespace)

	fmt.Printf("Listing pods in namespace %q:\n", namespace)
	list, err := podClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	nginxExist := false
	for idx, pod := range list.Items {
		fmt.Printf("%d, %q, %q\n", idx, pod.Name, pod.Namespace)
		if pod.Name == "nginx" {
			fmt.Println("Nginx exist!")
			nginxExist = true
		}
	}
	if !nginxExist {
		fmt.Println("Nginx does not exist, create one")
		createPod(clientset, namespace)
	}

	prompt()

	updatePod(clientset, namespace)

	prompt()

	fmt.Println("Deleting pod...")
	deletePolicy := metav1.DeletePropagationForeground
	if err := podClient.Delete(context.TODO(), "nginx", metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		panic(err)
	}
	fmt.Println("Deleted pod.")
}

func createPod(clientset *kubernetes.Clientset, namespace string) {
	podClient := clientset.CoreV1().Pods(namespace)
	nginx_pod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Name:  "web",
					Image: "nginx:1.12",
					Ports: []apiv1.ContainerPort{
						{
							Name:          "http",
							Protocol:      apiv1.ProtocolTCP,
							ContainerPort: 80,
						},
					},
				},
			},
		},
	}
	// Create Pod
	fmt.Println("Creating pod...")
	result, err := podClient.Create(context.TODO(), nginx_pod, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created pod %q.\n", result.GetObjectMeta().GetName())
}

func updatePod(clientset *kubernetes.Clientset, namespace string) {
	podClient := clientset.CoreV1().Pods(namespace)
	fmt.Println("Updating pod...")
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of Deployment before attempting update
		// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
		result, getErr := podClient.Get(context.TODO(), "nginx", metav1.GetOptions{})
		if getErr != nil {
			panic(fmt.Errorf("failed to get latest version of Pod: %v", getErr))
		}

		result.Annotations["app"] = "gpu-scheduler"
		// result.Spec.Containers[0].Image = "nginx:1.14" // change nginx version
		_, updateErr := podClient.Update(context.TODO(), result, metav1.UpdateOptions{})
		return updateErr
	})
	if retryErr != nil {
		panic(fmt.Errorf("update failed: %v", retryErr))
	}
	fmt.Println("Updated pod...")
}

func prompt() {
	fmt.Printf("-> Press Return key to continue.")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		break
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	fmt.Println()
}
