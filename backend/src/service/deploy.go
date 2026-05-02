package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"backend/database"
	"backend/model"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func deployToKubernetes(containerID string, imageRef string) {
	ctx := context.Background()

	container, err := model.GetContainerByID(containerID)
	if err != nil {
		fmt.Printf("failed to get container %s for deploy: %v\n", containerID, err)
		return
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		fmt.Printf("failed to get project %s for deploy: %v\n", container.ProjectID, err)
		return
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)
	namespace := project.Namespace

	var envVars map[string]string
	if container.EnvVars != "" && container.EnvVars != "{}" {
		_ = json.Unmarshal([]byte(container.EnvVars), &envVars)
	}

	var k8sEnvVars []corev1.EnvVar
	for k, v := range envVars {
		k8sEnvVars = append(k8sEnvVars, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	replicas := int32(container.Replicas)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      container.Name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": container.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": container.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  container.Name,
							Image: imageRef,
							Env:   k8sEnvVars,
						},
					},
				},
			},
		},
	}

	deployClient := clientset.AppsV1().Deployments(namespace)

	existing, err := deployClient.Get(ctx, container.Name, metav1.GetOptions{})
	if err != nil {
		_, err = deployClient.Create(ctx, deployment, metav1.CreateOptions{})
		if err != nil {
			fmt.Printf("failed to create deployment: %v\n", err)
			return
		}
	} else {
		existing.Spec = deployment.Spec
		_, err = deployClient.Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			fmt.Printf("failed to update deployment: %v\n", err)
			return
		}
	}

	for i := 0; i < 60; i++ {
		time.Sleep(2 * time.Second)
		d, err := deployClient.Get(ctx, container.Name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		if d.Status.ReadyReplicas == *deployment.Spec.Replicas {
			model.UpdateContainerStatus(containerID, "Running")
			fmt.Printf("deployment %s is running\n", container.Name)
			return
		}
	}

	fmt.Printf("deployment %s timed out waiting for ready state\n", container.Name)
	model.UpdateContainerStatus(containerID, "Failed")
}
