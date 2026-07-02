package manifest

import (
	"handler/models"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// テスト用の Generator を生成するヘルパー関数
func newTestGenerator() *Generator {
	return &Generator{
		InstanceSizes: map[string]models.InstanceSize{
			"small": {
				Size:          "small",
				CPURequest:    "100m",
				CPULimit:      "200m",
				MemoryRequest: "128Mi",
				MemoryLimit:   "256Mi",
			},
			"medium": {
				Size:          "medium",
				CPURequest:    "500m",
				CPULimit:      "1000m",
				MemoryRequest: "512Mi",
				MemoryLimit:   "1Gi",
			},
		},
	}
}

// TestGenerateDeployment_正しいmanifestが返る は GenerateDeployment が正しい manifest を返すことを確認する
func TestGenerateDeployment_正しいmanifestが返る(t *testing.T) {
	generator := newTestGenerator() // テスト用 Generator を生成する
	deploymentData := models.Deployment{
		ID:           "test-id-001",
		Name:         "my-app",
		InstanceSize: "small",
		Replicas:     2,
	}

	result := generator.GenerateDeployment(deploymentData, "test-namespace", "nginx:latest", nil, nil) // manifest を生成する

	if result.Name != "my-app" { // Deployment 名を確認する
		t.Errorf("期待する Name: my-app, 実際: %s", result.Name)
	}
	if result.Namespace != "test-namespace" { // Namespace を確認する
		t.Errorf("期待する Namespace: test-namespace, 実際: %s", result.Namespace)
	}
	if result.Labels["launchs.org/deployment-id"] != "test-id-001" { // deployment-id ラベルを確認する
		t.Errorf("期待する deployment-id ラベル: test-id-001, 実際: %s", result.Labels["launchs.org/deployment-id"])
	}
	if result.Labels["app"] != "my-app" { // app ラベルを確認する
		t.Errorf("期待する app ラベル: my-app, 実際: %s", result.Labels["app"])
	}
	if *result.Spec.Replicas != 2 { // レプリカ数を確認する
		t.Errorf("期待する Replicas: 2, 実際: %d", *result.Spec.Replicas)
	}
	if result.Spec.Selector.MatchLabels["app"] != "my-app" { // セレクターを確認する
		t.Errorf("期待する Selector app: my-app, 実際: %s", result.Spec.Selector.MatchLabels["app"])
	}
	if len(result.Spec.Template.Spec.Containers) != 1 { // コンテナ数を確認する
		t.Fatalf("期待する Container 数: 1, 実際: %d", len(result.Spec.Template.Spec.Containers))
	}
	container := result.Spec.Template.Spec.Containers[0] // コンテナを取得する
	if container.Image != "nginx:latest" {               // イメージを確認する
		t.Errorf("期待する Image: nginx:latest, 実際: %s", container.Image)
	}
}

// TestGenerateDeployment_smallサイズでCPUとメモリが正しく設定される は small インスタンスサイズで Resources が正しく設定されることを確認する
func TestGenerateDeployment_smallサイズでCPUとメモリが正しく設定される(t *testing.T) {
	generator := newTestGenerator() // テスト用 Generator を生成する
	deploymentData := models.Deployment{
		ID:           "test-id-002",
		Name:         "small-app",
		InstanceSize: "small",
		Replicas:     1,
	}

	result := generator.GenerateDeployment(deploymentData, "default", "alpine:3", nil, nil) // manifest を生成する

	if len(result.Spec.Template.Spec.Containers) != 1 { // コンテナが存在することを確認する
		t.Fatalf("コンテナが存在しません")
	}
	container := result.Spec.Template.Spec.Containers[0] // コンテナを取得する

	expectedCPURequest := resource.MustParse("100m")    // 期待する CPU リクエスト
	expectedCPULimit := resource.MustParse("200m")      // 期待する CPU リミット
	expectedMemoryRequest := resource.MustParse("128Mi") // 期待するメモリリクエスト
	expectedMemoryLimit := resource.MustParse("256Mi")   // 期待するメモリリミット

	actualCPURequest := container.Resources.Requests[corev1.ResourceCPU]       // 実際の CPU リクエストを取得する
	actualCPULimit := container.Resources.Limits[corev1.ResourceCPU]           // 実際の CPU リミットを取得する
	actualMemoryRequest := container.Resources.Requests[corev1.ResourceMemory] // 実際のメモリリクエストを取得する
	actualMemoryLimit := container.Resources.Limits[corev1.ResourceMemory]     // 実際のメモリリミットを取得する

	if actualCPURequest.Cmp(expectedCPURequest) != 0 { // CPU リクエストを比較する
		t.Errorf("期待する CPURequest: %s, 実際: %s", expectedCPURequest.String(), actualCPURequest.String())
	}
	if actualCPULimit.Cmp(expectedCPULimit) != 0 { // CPU リミットを比較する
		t.Errorf("期待する CPULimit: %s, 実際: %s", expectedCPULimit.String(), actualCPULimit.String())
	}
	if actualMemoryRequest.Cmp(expectedMemoryRequest) != 0 { // メモリリクエストを比較する
		t.Errorf("期待する MemoryRequest: %s, 実際: %s", expectedMemoryRequest.String(), actualMemoryRequest.String())
	}
	if actualMemoryLimit.Cmp(expectedMemoryLimit) != 0 { // メモリリミットを比較する
		t.Errorf("期待する MemoryLimit: %s, 実際: %s", expectedMemoryLimit.String(), actualMemoryLimit.String())
	}
}

// TestGenerateDeployment_commandとargsが空の場合はmanifestに含まれない は Command/Args が空の場合にフィールドが nil であることを確認する
func TestGenerateDeployment_commandとargsが空の場合はmanifestに含まれない(t *testing.T) {
	generator := newTestGenerator() // テスト用 Generator を生成する
	deploymentData := models.Deployment{
		ID:           "test-id-003",
		Name:         "no-cmd-app",
		InstanceSize: "small",
		Replicas:     1,
		Command:      nil, // command を空に設定する
		Args:         nil, // args を空に設定する
	}

	result := generator.GenerateDeployment(deploymentData, "default", "alpine:3", nil, nil) // manifest を生成する

	if len(result.Spec.Template.Spec.Containers) != 1 { // コンテナが存在することを確認する
		t.Fatalf("コンテナが存在しません")
	}
	container := result.Spec.Template.Spec.Containers[0] // コンテナを取得する

	if container.Command != nil { // Command が nil であることを確認する
		t.Errorf("Command は nil であるべきですが、実際: %v", container.Command)
	}
	if container.Args != nil { // Args が nil であることを確認する
		t.Errorf("Args は nil であるべきですが、実際: %v", container.Args)
	}
}

// TestGenerateDeployment_commandとargsが設定される は Command/Args が設定された場合にフィールドに含まれることを確認する
func TestGenerateDeployment_commandとargsが設定される(t *testing.T) {
	generator := newTestGenerator() // テスト用 Generator を生成する
	deploymentData := models.Deployment{
		ID:           "test-id-004",
		Name:         "cmd-app",
		InstanceSize: "small",
		Replicas:     1,
		Command:      []string{"/bin/sh", "-c"},  // command を設定する
		Args:         []string{"echo hello"},     // args を設定する
	}

	result := generator.GenerateDeployment(deploymentData, "default", "alpine:3", nil, nil) // manifest を生成する

	container := result.Spec.Template.Spec.Containers[0] // コンテナを取得する

	if len(container.Command) != 2 || container.Command[0] != "/bin/sh" { // Command を確認する
		t.Errorf("期待する Command: [/bin/sh -c], 実際: %v", container.Command)
	}
	if len(container.Args) != 1 || container.Args[0] != "echo hello" { // Args を確認する
		t.Errorf("期待する Args: [echo hello], 実際: %v", container.Args)
	}
}
