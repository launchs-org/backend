package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"deployyy/railpack"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	// uploadServerAddr はこのプロセスが受け取りサーバーとして待ち受けるアドレスです。
	// Job Pod からこのホストの 8080 番ポートに到達できる必要があります。
	uploadServerAddr = ":8080"

	// uploadServerHost は Job Pod からこのサーバーへ接続するホスト名または IP です。
	uploadServerHost = "10.10.11.8"

	// tarSaveDir は受け取った tar を保存するディレクトリです。
	tarSaveDir = "./received"
)

func main() {
	BuildImage()
}

func PushImage(tarPath,imageId,buildTag string) error {
	// tar をイメージとして読み込み
	image,err := crane.Load(tarPath)

	// エラー処理
	if err != nil {
		log.Printf("[server] tarball読み込み失敗: %v", err)
		return err
	}

	// 1. 証明書の検証をスキップするカスタムトランスポートを作成
    customTransport := &http.Transport{
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: true, // これが「強制的に信用する」設定
        },
    }

	// 認証情報の設定を行う
	opts := []crane.Option{
		crane.WithAuth(&authn.Basic{
			Username: "robot$buildkit+buildkit",
			Password: "",
		}),
		crane.WithTransport(customTransport), // カスタムトランスポートを適用
	}

	// プッシュを行う
	err = crane.Push(image,fmt.Sprintf("172.33.0.1/buildkit/%s:%s",imageId,buildTag),opts...)

	// エラー処理
	if err != nil {
		log.Printf("[server] プッシュに失敗しました %v",err)
	}

	return nil
}

func BuildImage() {
	uploadToken := os.Getenv("TAR_UPLOAD_TOKEN")

	// ── 受け取りサーバーをバックグラウンドで起動 ─────────────
	go startUploadServer(uploadToken)

	// ── Kubernetes クライアントの初期化 ──────────────────────
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("kubeconfig の読み込みに失敗: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("Kubernetes クライアントの作成に失敗: %v", err)
	}

	// ── railpack クライアントの初期化 ────────────────────────
	client, err := railpack.New(clientset, railpack.BuildConfig{
		// Git ソース
		GitRepo:   "https://github.com/launchs-org/sample-go-app",
		GitBranch: "main",
		Subdir:    ".",

		// 成果物
		ImageName: "sample-go-app",
		ImageTag:  uuid.New().String(),

		// tar 送信先 — このプロセス自身が受け取る
		UploadEndpoint: fmt.Sprintf("http://%s%s/upload", uploadServerHost, uploadServerAddr),
		UploadToken:    uploadToken,

		// Kubernetes
		Namespace: "buildkit",

		// リソース (省略時は DefaultResourceConfig() が使われる)
		Resources: railpack.ResourceConfig{
			BuildCPU:    "2",
			BuildMemory: "2Gi",
			BuildDisk:   "3Gi",
			InitCPU:     "500m",
			InitMemory:  "512Mi",
			PushCPU:     "500m",
			PushMemory:  "512Mi",
		},
		Timeout: 10 * time.Minute,
	})
	if err != nil {
		log.Fatalf("railpack.New に失敗: %v", err)
	}

	ctx := context.Background()

	// ── ビルド開始 ───────────────────────────────────────────
	jobID, err := client.Build(ctx)
	if err != nil {
		log.Fatalf("ビルドの開始に失敗: %v", err)
	}
	log.Printf("[railpack] ジョブを開始しました: %s", jobID)

	// ── ログをチャンネルで受け取って出力 ────────────────────
	logCh, errCh := client.StreamLogs(ctx, jobID)
	go func() {
		for line := range logCh {
			log.Println("[build]", line)
		}
		if err := <-errCh; err != nil {
			log.Printf("[railpack] ログストリームエラー: %v", err)
		}
	}()

	// ── 完了まで待機 ─────────────────────────────────────────
	status, err := client.Wait(ctx, jobID)
	if err != nil {
		log.Fatalf("ビルド待機中にエラー: %v", err)
	}

	if status == railpack.StatusComplete {
		log.Printf("✓ ビルド成功")
	} else {
		log.Fatalf("✗ ビルド失敗 (status=%s)", status)
	}
}

// ── 受け取りサーバー ────────────────────────────────────────

// startUploadServer は tar を受け取って ./received/ に保存する HTTP サーバーを起動します。
// この関数はブロックするため、goroutine で呼び出してください。
func startUploadServer(uploadToken string) {
	if err := os.MkdirAll(tarSaveDir, 0o755); err != nil {
		log.Fatalf("保存ディレクトリの作成に失敗しました: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/upload", makeUploadHandler(uploadToken))

	log.Printf("[server] 受け取りサーバーを起動しました: http://0.0.0.0%s", uploadServerAddr)
	if err := http.ListenAndServe(uploadServerAddr, mux); err != nil {
		log.Fatalf("[server] サーバーの起動に失敗しました: %v", err)
	}
}

// makeUploadHandler は /upload エンドポイントのハンドラーを返します。
func makeUploadHandler(uploadToken string) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(writer, "POST のみ受け付けます", http.StatusMethodNotAllowed)
			return
		}

		// Bearer トークン認証 (トークンが設定されている場合のみ検証)
		if uploadToken != "" && req.Header.Get("Authorization") != "Bearer "+uploadToken {
			http.Error(writer, "認証に失敗しました", http.StatusUnauthorized)
			return
		}

		jobID     := req.Header.Get("X-Job-Id")
		imageName := req.Header.Get("X-Image-Name")
		imageTag  := req.Header.Get("X-Image-Tag")
		if jobID == "" || imageName == "" || imageTag == "" {
			http.Error(writer, "X-Job-Id / X-Image-Name / X-Image-Tag ヘッダーが必要です", http.StatusBadRequest)
			return
		}

		log.Printf("[server] 受信開始 job=%s image=%s:%s", jobID, imageName, imageTag)

		savePath := filepath.Join(tarSaveDir, fmt.Sprintf("%s.tar", jobID))
		if err := saveTar(req.Body, savePath); err != nil {
			log.Printf("[server] 保存失敗 job=%s: %v", jobID, err)
			http.Error(writer, "保存に失敗しました", http.StatusInternalServerError)
			return
		}

		// プッシュを実行する
		err := PushImage(savePath,"famlink-front","latest")

		// エラー処理
		if err != nil {
			log.Printf("イメージのプッシュに失敗しました: %v",err)
			return 
		}

		log.Printf("[server] 保存完了 job=%s path=%s", jobID, savePath)
		writer.WriteHeader(http.StatusAccepted)
		fmt.Fprintln(writer, "accepted")
	}
}

// saveTar は body の内容を dest ファイルに書き込みます。
func saveTar(body io.Reader, dest string) error {
	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("ファイルの作成に失敗しました: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, body); err != nil {
		return fmt.Errorf("書き込みに失敗しました: %w", err)
	}
	return nil
}
