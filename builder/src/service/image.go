package service

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
)

// DeleteFromRegistry は Harbor からイメージ（全タグ）を削除します
func DeleteFromRegistry(imageName string, tags []string) error {
	registryHost := registryHostEnv()
	registryProject := registryProjectEnv()
	username := os.Getenv("REGISTRY_USERNAME")
	if username == "" {
		username = "robot$buildkit+buildkit"
	}
	password := os.Getenv("REGISTRY_PASSWORD")
	insecure := os.Getenv("REGISTRY_INSECURE") == "true"

	opts := []crane.Option{
		crane.WithAuth(&authn.Basic{Username: username, Password: password}),
		crane.WithTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		}),
	}

	var lastErr error
	for _, tag := range tags {
		ref := fmt.Sprintf("%s/%s/%s:%s", registryHost, registryProject, imageName, tag)
		if err := crane.Delete(ref, opts...); err != nil {
			fmt.Printf("[builder] failed to delete image %s: %v\n", ref, err)
			lastErr = err
		}
	}
	return lastErr
}

func registryHostEnv() string {
	if v := os.Getenv("REGISTRY_HOST"); v != "" {
		return v
	}
	return "172.33.0.1"
}

func registryProjectEnv() string {
	if v := os.Getenv("REGISTRY_PROJECT"); v != "" {
		return v
	}
	return "launchs"
}
