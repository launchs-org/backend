package kubernetes

import (
	"context"
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceConfig はリソース制限の設定です
type ResourceConfig struct {
	Requests struct {
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	} `json:"requests"`
	Limits struct {
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	} `json:"limits"`
}

// DeployParams はデプロイに必要なパラメータです
type DeployParams struct {
	Namespace     string
	Name          string
	Image         string
	Replicas      int32
	EnvVars       map[string]string
	ResourceJSON  string
}

// ApplyDeployment は Deployment を作成または更新します
func ApplyDeployment(ctx context.Context, params DeployParams) error {
	var resources ResourceConfig
	_ = json.Unmarshal([]byte(params.ResourceJSON), &resources)

	envVars := []corev1.EnvVar{}
	for k, v := range params.EnvVars {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: params.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &params.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": params.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": params.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  params.Name,
							Image: params.Image,
							Env:   envVars,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(resources.Requests.CPU),
									corev1.ResourceMemory: resource.MustParse(resources.Requests.Memory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(resources.Limits.CPU),
									corev1.ResourceMemory: resource.MustParse(resources.Limits.Memory),
								},
							},
						},
					},
				},
			},
		},
	}

	deploymentsClient := Clientset.AppsV1().Deployments(params.Namespace)
	_, err := deploymentsClient.Get(ctx, params.Name, metav1.GetOptions{})
	if err != nil {
		_, err = deploymentsClient.Create(ctx, deployment, metav1.CreateOptions{})
	} else {
		_, err = deploymentsClient.Update(ctx, deployment, metav1.UpdateOptions{})
	}
	return err
}

// DeleteDeployment は Deployment を削除します
func DeleteDeployment(ctx context.Context, namespace, name string) error {
	return Clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
