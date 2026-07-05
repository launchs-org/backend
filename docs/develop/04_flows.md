# PaaS 設計書 — フロー詳細

## apply フロー

### image_url デプロイ

```
Client                    API Server                   DB                    k8s
  |                           |                          |                     |
  |-- POST /deployments ----→ |                          |                     |
  |                           |-- INSERT deployment  --→ |                     |
  |                           |   (pending_image_url 等) |                     |
  |←-- 201 { id } ----------- |                          |                     |
  |                           |                          |                     |
  |-- PUT /service ----------→|                          |                     |
  |                           |-- UPDATE service ------→ |                     |
  |                           |   (pending_ports)        |                     |
  |←-- 200 OK --------------- |                          |                     |
  |                           |                          |                     |
  |-- POST /apply -----------→|                          |                     |
  |                           |-- SELECT FOR UPDATE ---→ |  (ロック取得)        |
  |                           |-- env key 重複チェック -→ |                     |
  |                           |-- manifest 生成          |                     |
  |                           |-- INSERT apply_history → |                     |
  |                           |-------------------------------- k8s apply ----→|
  |                           |-- pending_*** を空に ---→ |                     |
  |                           |-- app_status=deploying → |                     |
  |                           |-- ロック解放              |                     |
  |←-- 200 { apply_id } ----- |                          |                     |
  |                           |                          |   (Watcher が監視)   |
  |                           |←-------- Pod Ready 検知 ---|←- Watch event ---- |
  |                           |-- k8s_status 更新 ------→ |                     |
  |                           |-- app_status=running ---→ |                     |
```

---

### dockerfile / railpack ビルド → デプロイ

```
Client         API Server        Build Worker      GitHub        Registry        k8s
  |                |                  |               |              |             |
  |-- POST /deploy→|                  |               |              |             |
  |                |-- INSERT ------→ DB              |              |             |
  |                |   (pending_github_branch 等)     |              |             |
  |←-- 201 --------|                  |               |              |             |
  |                |                  |               |              |             |
  |-- POST /apply→ |                  |               |              |             |
  |                | [ビルド要否判定]  |               |              |             |
  |                | pending_commit_sha = "HEAD"       |              |             |
  |                |-------------------- GET /commits/HEAD -------→  |             |
  |                |←------------------- latest SHA ----------------  |             |
  |                | pending_commit_sha を実 SHA で上書き              |             |
  |                |                  |               |              |             |
  |                | current_build_id を空に           |              |             |
  |                |-- INSERT build → DB (pending)     |              |             |
  |                |-- app_status=building → DB        |              |             |
  |                |-- ビルドジョブをキューに投入 ----→ |              |             |
  |←-- 200 --------| (ビルド完了前に返る)               |              |             |
  |                |                  |               |              |             |
  |                |                  |-- git clone →  |              |             |
  |                |                  |←-- source ----  |              |             |
  |                |                  | dockerfile build / railpack build           |
  |                |                  |-------------------------------- push -----→  |
  |                |                  | ビルド完了                    |             |
  |                |←- 完了通知 ------|               |              |             |
  |                |-- UPDATE build: succeeded ------→ DB            |             |
  |                |-- UPDATE deployment:              |              |             |
  |                |   current_build_id = build.id --→ DB            |             |
  |                |   github_commit_sha = 実SHA ----→ DB            |             |
  |                |   app_status = deploying -------→ DB            |             |
  |                |                                                 |             |
  |                |-- manifest 生成 (built_image_url を使用)        |             |
  |                |-- INSERT apply_history --------→ DB             |             |
  |                |---------------------------------------------- k8s apply ---→  |
  |                |-- pending_*** を空に → DB                       |             |
  |                |                                                 |             |
  |                |←--- Pod Ready (Watcher) ----------------------------------------|
  |                |-- app_status=running → DB                       |             |
```

---

## apply 内部処理詳細

```go
func (s *ApplyService) Apply(ctx context.Context, deploymentID string) (*ApplyResult, error) {
    // 1. SELECT FOR UPDATE でロック取得
    var deployment Deployment
    if err := s.db.WithContext(ctx).
        Set("gorm:query_option", "FOR UPDATE").
        First(&deployment, "id = ?", deploymentID).Error; err != nil {
        return nil, err
    }

    // 2. env_var 実効キーの重複チェック
    if err := s.checkEnvKeyDuplication(ctx, deploymentID); err != nil {
        return nil, err // 400 DUPLICATE_ENV_KEY
    }

    // 3. HEAD → 実 SHA に解決
    if deployment.PendingGithubCommitSHA == "HEAD" {
        sha, err := s.github.GetLatestCommitSHA(
            ctx,
            deployment.PendingGithubRepoURL,
            deployment.PendingGithubBranch,
        )
        if err != nil {
            return nil, err
        }
        deployment.PendingGithubCommitSHA = sha
        s.db.Save(&deployment)
    }

    // 4. ビルド要否判定
    needsBuild := s.needsBuild(deployment)
    if needsBuild {
        // current_build_id を空にしてビルド中を示す
        deployment.CurrentBuildID = ""
        s.db.Save(&deployment)

        build := &DeploymentBuild{
            DeploymentID:   deploymentID,
            Status:         BuildStatusPending,
            CommitSHA:      deployment.PendingGithubCommitSHA,
            Branch:         deployment.PendingGithubBranch,
            DockerfilePath: deployment.PendingDockerfilePath,
        }
        s.db.Create(build)

        // 非同期でビルド実行
        go s.buildWorker.Enqueue(build.ID)

        // pending_*** をクリアして current に昇格（build 完了後に image URL が更新される）
        s.promotePendingFields(&deployment)
        deployment.AppStatus = AppStatusBuilding
        s.db.Save(&deployment)

        return &ApplyResult{BuildID: build.ID, Status: "building"}, nil
    }

    // 5. manifest 生成
    manifests, err := s.manifestGenerator.Generate(ctx, deployment)
    if err != nil {
        return nil, err
    }

    // 6. apply_history INSERT
    history := &ApplyHistory{
        DeploymentID: deploymentID,
        Manifests:    manifests,
        Status:       ApplyStatusApplied,
        AppliedAt:    time.Now(),
    }
    s.db.Create(history)

    // 7. k8s apply
    if err := s.k8s.Apply(ctx, manifests); err != nil {
        history.Status = ApplyStatusFailed
        history.ErrorMessage = err.Error()
        s.db.Save(history)
        // pending_*** はそのまま（再 apply 可能）
        return nil, err
    }

    // 8. pending_*** を空に → current 値に昇格
    s.promotePendingFields(&deployment)
    deployment.AppStatus = AppStatusDeploying
    now := time.Now()
    deployment.AppliedAt = &now

    // env_var_mounts の status を applied に更新
    s.db.Model(&EnvVarMount{}).
        Where("deployment_id = ?", deploymentID).
        Update("status", EnvVarMountStatusApplied)

    s.db.Save(&deployment)

    return &ApplyResult{ApplyHistoryID: history.ID, Status: "applied"}, nil
}

// ビルド要否判定
func (s *ApplyService) needsBuild(d Deployment) bool {
    if d.Type == DeploymentTypeImageURL {
        return false
    }
    return d.PendingGithubRepoURL != d.GithubRepoURL ||
        d.PendingGithubBranch != d.GithubBranch ||
        d.PendingGithubCommitSHA != d.GithubCommitSHA ||
        d.PendingDockerfilePath != d.DockerfilePath
}
```

---

## Watcher プロセス

k8s の Watch API を使いリアルタイムで DB を更新する。

```go
func (w *Watcher) WatchDeployments(ctx context.Context) {
    watcher, _ := w.k8s.AppsV1().Deployments("").Watch(ctx, metav1.ListOptions{})
    for event := range watcher.ResultChan() {
        dep := event.Object.(*appsv1.Deployment)
        deploymentID := dep.Labels["launchs.org/deployment-id"]

        switch event.Type {
        case watch.Modified:
            // k8s_status を更新
            w.db.Model(&Deployment{}).
                Where("id = ?", deploymentID).
                Updates(map[string]interface{}{
                    "k8s_status": dep.Status,
                })

            // Pod が全て Ready になったら app_status = running
            if dep.Status.ReadyReplicas == *dep.Spec.Replicas {
                w.db.Model(&Deployment{}).
                    Where("id = ? AND app_status = ?", deploymentID, AppStatusDeploying).
                    Update("app_status", AppStatusRunning)
            }
        }
    }
}
```

**Watcher が監視するリソース:**

| k8s リソース | 更新先 DB カラム |
|-------------|----------------|
| Deployment | `deployments.k8s_status`, `deployments.app_status` |
| Service | `services.k8s_status`, `services.status` |
| IngressRoute (CRD) | `ingress_routes.k8s_status`, `ingress_routes.status` |
| PersistentVolumeClaim | `volumes.k8s_status`, `volumes.status` |
| Namespace | `projects.k8s_status`, `projects.status` |

---

## 削除フロー

### 単体リソース削除（例: volume）

```
1. API: volumes.status → deleting
2. API: k8s PVC 削除リクエスト
3. Watcher: PVC Deleted イベント検知
4. Watcher: volumes レコードを DB から削除
```

> volume 削除前に volume_mounts が存在する場合は 409 を返す。

### Project 削除

```
1. API: project.status → deleting
2. API: 配下の全リソースを deleting に一斉更新
   - deployments, services, ingress_routes, volumes, env_vars
3. Watcher: 各リソースの k8s 削除完了を個別に検知
   → 各 DB レコードを削除
4. 全リソースの DB レコードが消えたことを確認
5. API (非同期ワーカー): k8s namespace を削除
6. Watcher: namespace Deleted イベント検知
   → project レコードを DB から削除
```

---

## k8s manifest 生成ルール

### Deployment manifest

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {deployment.name}
  namespace: {project.namespace}
  labels:
    launchs.org/deployment-id: {deployment.id}
spec:
  replicas: {deployment.replicas}
  selector:
    matchLabels:
      app: {deployment.name}
  template:
    metadata:
      labels:
        app: {deployment.name}
    spec:
      containers:
        - name: app
          image: {built_image_url or image_url}
          command: {deployment.command or null}
          args: {deployment.args or null}
          resources:
            requests:
              cpu: {instance_size.cpu_request}
              memory: {instance_size.memory_request}
            limits:
              cpu: {instance_size.cpu_limit}
              memory: {instance_size.memory_limit}
          envFrom:
            - configMapRef:
                name: {deployment.name}-env  # is_secret=false の env_var
            - secretRef:
                name: {deployment.name}-secret  # is_secret=true の env_var
          volumeMounts:
            - name: {volume.name}
              mountPath: {volume_mount.mount_path}
      volumes:
        - name: {volume.name}
          persistentVolumeClaim:
            claimName: {volume.name}
```

### Service manifest

```yaml
apiVersion: v1
kind: Service
metadata:
  name: {deployment.name}
  namespace: {project.namespace}
spec:
  selector:
    app: {deployment.name}
  ports:
    - protocol: {port.protocol}
      port: {port.port}
      targetPort: {port.port}
```

### IngressRoute manifest (Traefik CRD)

```yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: {deployment.name}
  namespace: {project.namespace}
spec:
  entryPoints:
    - web
    - websecure
  routes:
    - match: Host(`{ingress_route.host}`) && PathPrefix(`{ingress_route.path_prefix}`)
      kind: Rule
      services:
        - name: {deployment.name}
          port: {service.ports[0].port}
```

### ConfigMap / Secret

```yaml
# ConfigMap (is_secret=false の env_var)
apiVersion: v1
kind: ConfigMap
metadata:
  name: {deployment.name}-env
  namespace: {project.namespace}
data:
  {effective_key}: {value}  # effective_key = override_key ?? key

# Secret (is_secret=true の env_var)
apiVersion: v1
kind: Secret
metadata:
  name: {deployment.name}-secret
  namespace: {project.namespace}
stringData:
  {effective_key}: {value}
```

### PersistentVolumeClaim

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {volume.name}
  namespace: {project.namespace}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {volume.size_mb}Mi
```

---

## バリデーション一覧

| 項目 | ルール |
|------|--------|
| deployment.name | lowercase, [a-z0-9-], 最大63文字 |
| service.ports[].port | 1–65535、同一 protocol の重複不可 |
| volume.size_mb | > 0、account 全体の合計が max_volume_mb 以下 |
| env_var.key | 英数字・アンダースコアのみ |
| env_var_mount 実効キー | deployment 内で一意（override_key ?? key） |
| replicas | 1 以上、account_quotas.max_replicas_per_deployment 以下 |
| ingress_route.host | UNIQUE（DB 制約） |
