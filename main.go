package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"

	//"k8s.io/client-go/pkg/api/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func main() {
	deploymentName := "demo-deployment"
	clientset := k8sClient()

	//deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

	// Create Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "demo",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "demo",
					},
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
			},
		},
	}

	result, err1 := createDeployment(clientset, apiv1.NamespaceDefault, deployment)
	if err1 != nil {
		fmt.Println(err1)
	}
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

	// Update Deployment
	//updateDeployment(deploymentsClient, deploymentName)

	// List Deployments
	deploymentList, err2 := listDeployments(clientset, apiv1.NamespaceDefault)
	if err2 != nil {
		fmt.Println(err2)
	}
	for _, deployment := range deploymentList.Items {
		fmt.Printf("%+v\n", deployment.Name)
	}

	// describe Deployment
	//describeDeployment(deploymentsClient, deploymentName)

	// Delete Deployment
	deleteDeployment(clientset, apiv1.NamespaceDefault, deploymentName)

	// watch deployments
	watchDeployment(clientset, apiv1.NamespaceDefault)
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

func int32Ptr(i int32) *int32 { return &i }

// 获取集群内部k8s客户端
func k8sClient() *kubernetes.Clientset {
	fmt.Printf("get k8s client ...")
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset
}

func createDeployment(clientSet *kubernetes.Clientset, namespaceName string, deployment *appsv1.Deployment) (result *appsv1.Deployment, err error) {
	fmt.Println("Creating deployment...")
	result, err = clientSet.AppsV1().Deployments(namespaceName).Create(context.TODO(), deployment, metav1.CreateOptions{})
	fmt.Printf("type %T", result)
	if err != nil {
		return nil, err
	}

	return result, err

}

func updateDeployment(clientSet *kubernetes.Clientset, namespaceName string, deploymentName string) {
	prompt()
	fmt.Println("Updating deployment...")

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of Deployment before attempting update
		// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
		result, getErr := clientSet.AppsV1().Deployments(namespaceName).Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if getErr != nil {
			panic(fmt.Errorf("Failed to get latest version of Deployment: %v", getErr))
		}

		result.Spec.Replicas = int32Ptr(1)                           // reduce replica count
		result.Spec.Template.Spec.Containers[0].Image = "nginx:1.13" // change nginx version
		_, updateErr := clientSet.AppsV1().Deployments(namespaceName).Update(context.TODO(), result, metav1.UpdateOptions{})
		return updateErr
	})
	if retryErr != nil {
		panic(fmt.Errorf("Update failed: %v", retryErr))
	}
	fmt.Println("Updated deployment...")
}

func listDeployments(clientSet *kubernetes.Clientset, namespaceName string) (deploymentList *appsv1.DeploymentList, err error) {
	prompt()
	fmt.Printf("Listing deployments in namespace %q:\n", namespaceName)
	deploymentList, err = clientSet.AppsV1().Deployments(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return deploymentList, err
}

func describeDeployment(clientSet *kubernetes.Clientset, namespaceName string, deploymentName string) (deploymentInfo *appsv1.Deployment, err error) {
	prompt()
	deploymentInfo, err = clientSet.AppsV1().Deployments(namespaceName).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	//fmt.Printf("the describsion of %s is %s", deploymentName, deploymentInfo)
	return deploymentInfo, err
}

func watchDeployment(clientSet *kubernetes.Clientset, namespaceName string) {
	prompt()
	fmt.Printf("Watching deployments in namespace %q:\n", apiv1.NamespaceDefault)
	watcher, err := clientSet.AppsV1().Deployments(namespaceName).Watch(context.TODO(), metav1.ListOptions{})

	if err != nil {
		panic(err)
	}

	for watchEvent := range watcher.ResultChan() {
		deploy := watchEvent.Object.(*appsv1.Deployment)

		switch watchEvent.Type {
		case watch.Added:
			fmt.Printf("Deployment %s/%s added, status %s", deploy.ObjectMeta.Namespace, deploy.ObjectMeta.Name)
			fmt.Println()
		case watch.Modified:
			fmt.Printf("Deployment %s/%s modified", deploy.ObjectMeta.Namespace, deploy.ObjectMeta.Name)
			fmt.Println()
		case watch.Deleted:
			fmt.Printf("Deployment %s/%s deleted", deploy.ObjectMeta.Namespace, deploy.ObjectMeta.Name)
			fmt.Println()
		}
	}

}

func deleteDeployment(clientSet *kubernetes.Clientset, namespaceName string, deploymentName string) {
	prompt()
	fmt.Println("Deleting deployment...")
	deletePolicy := metav1.DeletePropagationForeground
	fmt.Printf("Datatype of deletePolicy : %T \n", deletePolicy)
	if err := clientSet.AppsV1().Deployments(namespaceName).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		panic(err)
	}
	fmt.Println("Deleted deployment.")
}
