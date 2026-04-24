package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ServiceParams は Service 作成に必要なパラメータです
type ServiceParams struct {
	Namespace  string
	Name       string
	Port       int32
	TargetPort int32
}

// ApplyService は Service を作成または更新します
func ApplyService(ctx context.Context, params ServiceParams) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: params.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": params.Name},
			Ports: []corev1.ServicePort{
				{
					Port:       params.Port,
					TargetPort: intstr.FromInt(int(params.TargetPort)),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	servicesClient := Clientset.CoreV1().Services(params.Namespace)
	_, err := servicesClient.Get(ctx, params.Name, metav1.GetOptions{})
	if err != nil {
		_, err = servicesClient.Create(ctx, svc, metav1.CreateOptions{})
	} else {
		_, err = servicesClient.Update(ctx, svc, metav1.UpdateOptions{})
	}
	return err
}

// DeleteService は Service を削除します
func DeleteService(ctx context.Context, namespace, name string) error {
	return Clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
