package manifest

import (
	"app/shared/models"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

// newTestGenerator は RuntimeClass 以外を固定した Generator を返す
func newTestGenerator(runtimeClassName string) *Generator {
	return &Generator{
		InstanceSizes: map[string]models.InstanceSize{
			"small": {Size: "small", CPURequest: "100m", CPULimit: "500m", MemoryRequest: "128Mi", MemoryLimit: "256Mi"},
		},
		RuntimeClassName: runtimeClassName,
	}
}

func generateTestDeployment(generator *Generator) *models.Deployment {
	deploymentData := models.Deployment{ID: "deployment-id", Name: "app", InstanceSize: "small"}
	return &deploymentData
}

// TestGenerateDeploymentSetsRuntimeClassWhenConfigured は RuntimeClass を指定すると Pod に反映されることを検証する
func TestGenerateDeploymentSetsRuntimeClassWhenConfigured(t *testing.T) {
	generator := newTestGenerator("gvisor")
	deploymentData := generateTestDeployment(generator)

	deploymentManifest := generator.GenerateDeployment(*deploymentData, "project-ns", "harbor.example/app:1", nil, nil)

	runtimeClass := deploymentManifest.Spec.Template.Spec.RuntimeClassName
	if runtimeClass == nil || *runtimeClass != "gvisor" {
		t.Errorf("runtimeClassName が gvisor ではありません: %v", runtimeClass)
	}
}

// TestGenerateDeploymentLeavesRuntimeClassUnsetByDefault は RuntimeClass を指定しなければ従来どおり何も設定しないことを検証する
func TestGenerateDeploymentLeavesRuntimeClassUnsetByDefault(t *testing.T) {
	generator := newTestGenerator("")
	deploymentData := generateTestDeployment(generator)

	deploymentManifest := generator.GenerateDeployment(*deploymentData, "project-ns", "harbor.example/app:1", nil, nil)

	if runtimeClass := deploymentManifest.Spec.Template.Spec.RuntimeClassName; runtimeClass != nil {
		t.Errorf("RuntimeClass を設定していないのに runtimeClassName が指定されています: %q", *runtimeClass)
	}
}

// TestGenerateDeploymentAppliesSecurityHardening はユーザーのアプリに最小限の強化(seccomp、権限昇格の禁止、不要な capability の削除、トークン非マウント)が付くことを検証する
func TestGenerateDeploymentAppliesSecurityHardening(t *testing.T) {
	generator := newTestGenerator("")
	deploymentData := generateTestDeployment(generator)

	deploymentManifest := generator.GenerateDeployment(*deploymentData, "project-ns", "harbor.example/app:1", nil, nil)
	podSpec := deploymentManifest.Spec.Template.Spec

	if podSpec.AutomountServiceAccountToken == nil || *podSpec.AutomountServiceAccountToken {
		t.Errorf("ServiceAccount トークンをマウントしない設定(false)であることを期待しました: %v", podSpec.AutomountServiceAccountToken)
	}
	securityContext := podSpec.Containers[0].SecurityContext
	if securityContext == nil {
		t.Fatal("コンテナの securityContext が設定されていません")
	}
	if securityContext.AllowPrivilegeEscalation == nil || *securityContext.AllowPrivilegeEscalation {
		t.Errorf("allowPrivilegeEscalation が false ではありません: %v", securityContext.AllowPrivilegeEscalation)
	}
	if securityContext.SeccompProfile == nil || securityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("seccompProfile が RuntimeDefault ではありません: %v", securityContext.SeccompProfile)
	}
	dropped := map[corev1.Capability]bool{}
	for _, capability := range securityContext.Capabilities.Drop {
		dropped[capability] = true
	}
	for _, want := range []corev1.Capability{"NET_RAW", "MKNOD", "AUDIT_WRITE", "SETFCAP"} {
		if !dropped[want] {
			t.Errorf("capability %s が削除されていません", want)
		}
	}
}

// TestGenerateDeploymentKeepsCapabilitiesNeededByRootImages は root 起動の一般的なイメージ(nginx など)が使う capability を、削除しないことを検証する
func TestGenerateDeploymentKeepsCapabilitiesNeededByRootImages(t *testing.T) {
	generator := newTestGenerator("")
	deploymentData := generateTestDeployment(generator)

	deploymentManifest := generator.GenerateDeployment(*deploymentData, "project-ns", "harbor.example/app:1", nil, nil)
	securityContext := deploymentManifest.Spec.Template.Spec.Containers[0].SecurityContext

	for _, capability := range securityContext.Capabilities.Drop {
		switch capability {
		case "ALL", "CHOWN", "SETUID", "SETGID", "NET_BIND_SERVICE", "DAC_OVERRIDE", "FOWNER":
			t.Errorf("root 起動のイメージが使う capability %s を削除してはいけません", capability)
		}
	}
	if securityContext.RunAsNonRoot != nil {
		t.Errorf("runAsNonRoot は指定しない方針です(root 前提のイメージを壊すため): %v", *securityContext.RunAsNonRoot)
	}
}
