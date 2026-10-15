package k8s

import (
	"slices"
	"testing"
)

// TestBuildEgressRules は Egress が同一 namespace・DNS・インターネット(プライベート帯を除く)だけに限定されることを検証する
func TestBuildEgressRules(t *testing.T) {
	rules := buildEgressRules()
	if len(rules) != 3 { // 同一 namespace・DNS・インターネットの 3 つだけ
		t.Fatalf("Egress ルールは 3 つのはずが %d 個ある", len(rules))
	}

	// 全許可（To が空のルール）が含まれてはならない
	for index, rule := range rules {
		if len(rule.To) == 0 {
			t.Errorf("rules[%d] は宛先が空で、全宛先を許可してしまう", index)
		}
	}

	// DNS は kube-system の kube-dns に限定され、UDP/TCP の 53 番だけ許可される
	dns := rules[1]
	if dns.To[0].NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"] != "kube-system" || dns.To[0].PodSelector.MatchLabels["k8s-app"] != "kube-dns" {
		t.Errorf("DNS の宛先が kube-system の kube-dns に限定されていない: %+v", dns.To[0])
	}
	if len(dns.Ports) != 2 || dns.Ports[0].Port.IntValue() != 53 || dns.Ports[1].Port.IntValue() != 53 {
		t.Errorf("DNS のポートが UDP/TCP の 53 番だけではない: %+v", dns.Ports)
	}

	// インターネット宛は全宛先から、LAN・ノード・Service/Pod CIDR・Tailscale・リンクローカルを除外する
	internet := rules[2].To[0].IPBlock
	if internet == nil || internet.CIDR != "0.0.0.0/0" {
		t.Fatalf("インターネット宛のルールが 0.0.0.0/0 の ipBlock ではない: %+v", rules[2])
	}
	for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10", "169.254.0.0/16"} {
		if !slices.Contains(internet.Except, cidr) {
			t.Errorf("%s が除外されていない", cidr)
		}
	}
}
