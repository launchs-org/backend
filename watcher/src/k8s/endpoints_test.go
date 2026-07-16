package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// TestCheckEndpointsReady_Readyなアドレスが存在する場合はtrue は Endpoints に Ready なアドレスがある場合に true を返すことを確認する
func TestCheckEndpointsReady_Readyなアドレスが存在する場合はtrue(t *testing.T) {
	fakeK8sClient := k8sfake.NewSimpleClientset(&corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "test-ns"},
		Subsets: []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "10.0.0.1"}}},
		},
	})

	ready, err := CheckEndpointsReady(context.Background(), fakeK8sClient, "test-ns", "test-svc")
	if err != nil {
		t.Fatalf("CheckEndpointsReady がエラーを返しました: %v", err)
	}
	if !ready { // Ready なアドレスがあるため true が返ることを確認する
		t.Error("期待する結果: true, 実際の結果: false")
	}
}

// TestCheckEndpointsReady_Addressesが空の場合はfalse は Subsets はあるが Addresses が空の場合に false を返すことを確認する
func TestCheckEndpointsReady_Addressesが空の場合はfalse(t *testing.T) {
	fakeK8sClient := k8sfake.NewSimpleClientset(&corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "test-svc-empty", Namespace: "test-ns"},
		Subsets: []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{}},
		},
	})

	ready, err := CheckEndpointsReady(context.Background(), fakeK8sClient, "test-ns", "test-svc-empty")
	if err != nil {
		t.Fatalf("CheckEndpointsReady がエラーを返しました: %v", err)
	}
	if ready { // Ready なアドレスがないため false が返ることを確認する
		t.Error("期待する結果: false, 実際の結果: true")
	}
}

// TestCheckEndpointsReady_Endpointsが存在しない場合はfalseかつエラーなし は Endpoints が未作成の場合に false・エラーなしを返すことを確認する
func TestCheckEndpointsReady_Endpointsが存在しない場合はfalseかつエラーなし(t *testing.T) {
	fakeK8sClient := k8sfake.NewSimpleClientset() // Endpoints を何も登録しない

	ready, err := CheckEndpointsReady(context.Background(), fakeK8sClient, "test-ns", "not-exist-svc")
	if err != nil {
		t.Fatalf("CheckEndpointsReady がエラーを返しました: %v", err)
	}
	if ready { // Endpoints 未作成のため false が返ることを確認する
		t.Error("期待する結果: false, 実際の結果: true")
	}
}
