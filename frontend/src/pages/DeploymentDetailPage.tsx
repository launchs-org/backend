import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { Play, Trash2, GitBranch, Container, Package, ExternalLink, Clock, CheckCircle2, XCircle, AlertCircle, Ban, GitCommit, X, Hammer, Copy, Eye, EyeOff, Webhook as WebhookIcon } from 'lucide-react'
import { Layout } from '@/components/Layout'
import { StatusBadge } from '@/components/StatusBadge'
import { LogViewer } from '@/components/LogViewer'
import { MetricsCharts } from '@/components/MetricsCharts'
import { MetricsDownloadButton } from '@/components/MetricsDownloadButton'
import { downsampleMetrics } from '@/lib/metricsDownsample'
import { get, post, put, del, ApiError } from '@/lib/api'
import type {
  Deployment,
  Project,
  Build,
  K8sService,
  ApplyHistory,
  PodLogsResponse,
  PodLogEntry,
  Volume,
  VolumeMount,
  EnvVar,
  EnvVarMount,
  DeploymentTemplate,
  DeploymentMetrics,
  Webhook,
} from '@/lib/types'
import {
  POLL_INTERVAL_NORMAL,
  POLL_INTERVAL_FAST,
  POLL_INTERVAL_BUILDS,
  FAST_POLL_DURATION,
  REPLICAS_MIN,
  REPLICAS_MAX,
  INSTANCE_SIZES,
  GITHUB_BRANCHES_PER_PAGE,
  GITHUB_COMMITS_PER_PAGE,
  GITHUB_COMMIT_MESSAGE_MAX_LENGTH,
} from '@/lib/config'
import { useTutorialContext } from '@/tutorial/TutorialContext' // チュートリアル Context をインポートする
import { toast } from 'sonner' // トースト通知をインポートする
import { ConfirmDialog } from '@/components/ui/confirm-dialog' // 確認ダイアログをインポートする

type Tab = 'overview' | 'logs' | 'builds' | 'settings' | 'networking' | 'env-vars' | 'volumes' | 'history' | 'metrics' | 'webhook'

// pending 項目の種類と redo 操作を保持する型
type PendingItem = {
  label: string           // 表示ラベル
  onDiscard: () => Promise<void> // 取り消し操作
}

const TYPE_ICON = {
  image_url: Container,
  dockerfile: GitBranch,
  railpack: Package,
}

export function DeploymentDetailPage() {
  const { advance: tutorialAdvance, isActive: tutorialIsActive, actualStep: tutorialActualStep } = useTutorialContext() // チュートリアルの状態を取得する

  const { projectId, deploymentId } = useParams<{ projectId: string; deploymentId: string }>()
  const navigate = useNavigate()

  const [deployment, setDeployment] = useState<Deployment | null>(null) // デプロイメント情報を管理する
  const [activeTab, setActiveTab] = useState<Tab>('overview') // アクティブなタブを管理する
  const [loading, setLoading] = useState(true) // ローディング状態を管理する
  const [applying, setApplying] = useState(false) // Apply中フラグ
  const [deleting, setDeleting] = useState(false) // 削除中フラグ
  const [deleted, setDeleted] = useState(false) // 削除完了フラグ
  const [allPendingItems, setAllPendingItems] = useState<PendingItem[]>([]) // 全pending項目を管理する
  const [discardingLabel, setDiscardingLabel] = useState<string | null>(null) // 取り消し中の項目ラベル
  const [fastPolling, setFastPolling] = useState(false) // 高速ポーリングフラグ（操作後に有効化する）
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false) // デプロイメント削除確認ダイアログの開閉を管理する

  const fetchDeployment = useCallback(async () => {
    if (!deploymentId) return
    try {
      const data = await get<Deployment>(`/deployments/${deploymentId}`) // デプロイメント情報を取得する
      setDeployment(data)
    } catch (fetchError) {
      if (fetchError instanceof ApiError && fetchError.status === 404) {
        setDeleted(true) // 404 は物理削除完了とみなして完了画面を表示する
      } else {
        console.error(fetchError)
      }
    } finally {
      setLoading(false)
    }
  }, [deploymentId])

  useEffect(() => {
    void fetchDeployment() // 初回データ取得
    const pollInterval = fastPolling ? POLL_INTERVAL_FAST : POLL_INTERVAL_NORMAL // 操作後は高速、通常は通常間隔でポーリングする
    const intervalId = setInterval(() => { void fetchDeployment() }, pollInterval)
    return () => clearInterval(intervalId) // クリーンアップ
  }, [fetchDeployment, fastPolling])

  // deployment・service・env-var-mounts・volume-mounts を一括取得して pending 項目を生成する
  const fetchAllPending = useCallback(async () => {
    if (!deploymentId || !projectId) return
    const [deploymentData, serviceData, envVarMounts, volumeMounts, envVars, volumes] = await Promise.all([
      get<Deployment>(`/deployments/${deploymentId}`).catch(() => null),
      get<K8sService>(`/deployments/${deploymentId}/service`).catch(() => null),
      get<EnvVarMount[]>(`/deployments/${deploymentId}/env-var-mounts`).catch(() => [] as EnvVarMount[]),
      get<VolumeMount[]>(`/deployments/${deploymentId}/volume-mounts`).catch(() => [] as VolumeMount[]),
      get<EnvVar[]>(`/projects/${projectId}/env-vars`).catch(() => [] as EnvVar[]),
      get<Volume[]>(`/projects/${projectId}/volumes`).catch(() => [] as Volume[]),
    ])

    const items: PendingItem[] = []

    // deployment の pending フィールドを確認する
    if (deploymentData) {
      if (deploymentData.pending_image_url && deploymentData.pending_image_url !== deploymentData.image_url)
        items.push({ label: `イメージURL: ${deploymentData.pending_image_url}`, onDiscard: async () => { await post(`/deployments/${deploymentId}/discard-pending`) } })
      if (deploymentData.pending_github_repo_url && deploymentData.pending_github_repo_url !== deploymentData.github_repo_url)
        items.push({ label: `リポジトリURL: ${deploymentData.pending_github_repo_url}`, onDiscard: async () => { await post(`/deployments/${deploymentId}/discard-pending`) } })
      if (deploymentData.pending_github_branch && deploymentData.pending_github_branch !== deploymentData.github_branch)
        items.push({ label: `ブランチ: ${deploymentData.pending_github_branch}`, onDiscard: async () => { await post(`/deployments/${deploymentId}/discard-pending`) } })
      if (deploymentData.pending_instance_size && deploymentData.pending_instance_size !== deploymentData.instance_size)
        items.push({ label: `インスタンスサイズ: ${deploymentData.pending_instance_size}`, onDiscard: async () => { await post(`/deployments/${deploymentId}/discard-pending`) } })
      if (deploymentData.pending_replicas && deploymentData.pending_replicas !== deploymentData.replicas)
        items.push({ label: `レプリカ数: ${deploymentData.pending_replicas}`, onDiscard: async () => { await post(`/deployments/${deploymentId}/discard-pending`) } })
      if (deploymentData.pending_dockerfile_path && deploymentData.pending_dockerfile_path !== deploymentData.dockerfile_path)
        items.push({ label: `Dockerfileパス: ${deploymentData.pending_dockerfile_path}`, onDiscard: async () => { await post(`/deployments/${deploymentId}/discard-pending`) } })
      setDeployment(deploymentData) // deployment 情報も更新する
    }

    // service の pending 状態を確認する
    if (serviceData) {
      if (serviceData.status === 'pending' && serviceData.port === 0)
        items.push({ label: `Service 追加: ${serviceData.pending_port} → ${serviceData.pending_target_port}`, onDiscard: async () => { await del(`/deployments/${deploymentId}/service`) } })
      else if (serviceData.status === 'deleting')
        items.push({ label: `Service 無効化待ち (現在: ${serviceData.port} → ${serviceData.target_port})`, onDiscard: async () => { await put(`/deployments/${deploymentId}/service`, { port: serviceData.port, target_port: serviceData.target_port }) } })
      else if (serviceData.pending_port !== 0 && serviceData.pending_port !== serviceData.port)
        items.push({ label: `Serviceポート変更: ${serviceData.port} → ${serviceData.pending_port}`, onDiscard: async () => { await put(`/deployments/${deploymentId}/service`, { port: serviceData.port, target_port: serviceData.target_port }) } })
    }

    // env-var-mounts の pending 状態を確認する
    for (const mount of (envVarMounts ?? [])) {
      const envVar = (envVars ?? []).find(ev => ev.id === mount.env_var_id) // 対応する環境変数を取得する
      const keyLabel = mount.override_key || envVar?.key || mount.env_var_id.slice(0, 8) // 表示キーを決定する
      if (mount.status === 'pending')
        items.push({ label: `環境変数マウント追加: ${keyLabel}`, onDiscard: async () => { await del(`/env-var-mounts/${mount.id}`) } })
      else if (mount.status === 'deleting')
        items.push({ label: `環境変数マウント削除待ち: ${keyLabel}`, onDiscard: async () => { await post(`/deployments/${deploymentId}/env-var-mounts`, { env_var_id: mount.env_var_id, override_key: mount.override_key }) } })
    }

    // volume-mounts の pending 状態を確認する
    for (const mount of (volumeMounts ?? [])) {
      const volume = (volumes ?? []).find(vol => vol.id === mount.volume_id) // 対応するボリュームを取得する
      const volumeLabel = volume?.name ?? mount.volume_id.slice(0, 8) // 表示名を決定する
      if (mount.status === 'pending')
        items.push({ label: `ボリュームマウント追加: ${volumeLabel} → ${mount.mount_path}`, onDiscard: async () => { await del(`/volume-mounts/${mount.id}`) } })
      else if (mount.status === 'deleting')
        items.push({ label: `ボリュームマウント削除待ち: ${volumeLabel} → ${mount.mount_path}`, onDiscard: async () => { await post(`/deployments/${deploymentId}/volume-mounts`, { volume_id: mount.volume_id, mount_path: mount.mount_path }) } })
    }

    setAllPendingItems(items) // pending 項目一覧を更新する
  }, [deploymentId, projectId])

  useEffect(() => {
    void fetchAllPending() // 初回 pending 一括取得
    const pollInterval = fastPolling ? POLL_INTERVAL_FAST : POLL_INTERVAL_NORMAL // 操作後は高速、通常は通常間隔でポーリングする
    const intervalId = setInterval(() => { void fetchAllPending() }, pollInterval)
    return () => clearInterval(intervalId) // クリーンアップ
  }, [fetchAllPending, fastPolling])

  const hasPending = allPendingItems.length > 0 // pending項目が1件以上あればtrueにする

  const handleDiscard = async (item: PendingItem) => {
    setDiscardingLabel(item.label) // 取り消し中の項目を記録する
    try {
      await item.onDiscard() // 取り消し操作を実行する
      await Promise.all([fetchDeployment(), fetchAllPending()]) // 取り消し後に全データを再取得する
    } catch (discardError) {
      console.error(discardError)
      toast.error(discardError instanceof Error ? discardError.message : '取り消しに失敗しました') // エラーをトーストで表示する
    } finally {
      setDiscardingLabel(null) // 取り消し中フラグをリセットする
    }
  }

  const handleApply = async () => {
    if (!deploymentId) return
    setApplying(true)
    setFastPolling(true) // Apply後はポーリングを高速化する
    try {
      await post(`/deployments/${deploymentId}/apply`) // Applyを実行する
      await Promise.all([fetchDeployment(), fetchAllPending()]) // デプロイメント情報と pending 一覧を再取得する
      // チュートリアル中なら Apply 完了でステップを進める
      if (tutorialIsActive && (tutorialActualStep?.id === 'deployment-apply-button' || tutorialActualStep?.id === 'adv-storage-apply' || tutorialActualStep?.id === 'adv-envvar-apply')) {
        tutorialAdvance() // 次のステップへ進む
      }
    } catch (applyError) {
      console.error(applyError)
      toast.error(applyError instanceof Error ? applyError.message : 'Apply に失敗しました') // エラーをトーストで表示する
    } finally {
      setApplying(false)
      setTimeout(() => setFastPolling(false), FAST_POLL_DURATION) // 一定時間後に通常ポーリングへ戻す
    }
  }

  const handleDelete = () => {
    if (!deploymentId || !deployment) return
    setDeleteConfirmOpen(true) // 削除確認ダイアログを開く
  }

  // type によって表示するタブを決定する
  const availableTabs: Tab[] = deployment
    ? [
        'overview',
        'logs',
        ...(deployment.type !== 'image_url' ? (['builds'] as Tab[]) : []),
        'settings',
        'networking',
        'env-vars',
        'volumes',
        'history',
        'metrics',
        'webhook' as Tab,
      ]
    : ['overview']

  if (loading) {
    return (
      <Layout>
        <div className="h-48 flex items-center justify-center text-sm text-gray-400">読み込み中...</div>
      </Layout>
    )
  }

  if (deleted) {
    return (
      <Layout>
        <div className="h-96 flex flex-col items-center justify-center gap-4">
          <div className="w-12 h-12 rounded-full bg-green-100 flex items-center justify-center">
            <Trash2 className="w-6 h-6 text-green-600" />
          </div>
          <div className="text-center">
            <p className="text-lg font-semibold text-[#111827]">削除が完了しました</p>
            <p className="text-sm text-gray-400 mt-1">デプロイメントが正常に削除されました</p>
          </div>
          {window.self === window.top && ( // iframe 内では表示しない
            <button
              onClick={() => navigate(`/projects/${projectId}`)}
              className="text-sm px-4 py-2 rounded-md bg-[#111827] text-white hover:bg-gray-800 transition-colors"
            >
              プロジェクトへ戻る
            </button>
          )}
        </div>
      </Layout>
    )
  }

  if (deployment?.status === 'not_init') {
    return (
      <NotInitScreen
        deployment={deployment}
        deploymentId={deploymentId!}
        projectId={projectId!}
        onDelete={() => void handleDelete()}
        deleting={deleting}
      />
    )
  }

  if (deployment?.status === 'deleting') {
    return (
      <Layout
        breadcrumbs={[
          { label: 'Project', href: `/projects/${projectId}` },
          { label: deployment.name },
        ]}
      >
        <div className="h-96 flex flex-col items-center justify-center gap-4">
          <div className="w-12 h-12 rounded-full bg-red-50 flex items-center justify-center animate-pulse">
            <Trash2 className="w-6 h-6 text-red-400" />
          </div>
          <div className="text-center">
            <p className="text-lg font-semibold text-[#111827]">削除中...</p>
            {deployment.delete_progress && (
              <p className="text-sm text-gray-400 mt-1 font-mono">{deployment.delete_progress}</p>
            )}
          </div>
        </div>
      </Layout>
    )
  }

  if (!deployment) {
    return (
      <Layout>
        <div className="h-48 flex items-center justify-center text-sm text-gray-400">デプロイメントが見つかりません</div>
      </Layout>
    )
  }

  const TypeIcon = TYPE_ICON[deployment.type] ?? Container

  return (
    <Layout
      breadcrumbs={[
        { label: 'Project', href: `/projects/${projectId}` },
        { label: deployment.name },
      ]}
      actions={
        <div className="flex items-center gap-2">
          <button
            data-tutorial="tutorial-deployment-apply-btn"
            onClick={() => void handleApply()}
            disabled={applying}
            className={`flex items-center gap-1.5 text-sm px-3 py-1.5 rounded-md transition-colors ${
              hasPending
                ? 'bg-amber-500 hover:bg-amber-600 text-white'
                : 'bg-[#111827] hover:bg-gray-800 text-white'
            } disabled:opacity-50`}
          >
            <Play className="w-3.5 h-3.5" />
            {applying ? 'Apply中...' : 'Apply'}
          </button>
          <button
            onClick={() => void handleDelete()}
            disabled={deleting}
            className="flex items-center gap-1.5 text-red-500 text-sm px-3 py-1.5 rounded-md hover:bg-red-50 border border-red-200 transition-colors disabled:opacity-50"
          >
            <Trash2 className="w-3.5 h-3.5" />
            {deleting ? '削除中...' : '削除'}
          </button>
        </div>
      }
    >
      <div className="space-y-4">
        {/* ヘッダー */}
        <div className="flex items-center gap-3">
          <span className="p-2 rounded-lg bg-gray-100 text-gray-600">
            <TypeIcon className="w-5 h-5" />
          </span>
          <div>
            <h1 className="text-xl font-semibold text-[#111827]">{deployment.name}</h1>
            <div className="flex items-center gap-2 mt-0.5">
              <StatusBadge status={deployment.status} size="sm" />
              {deployment.app_status !== deployment.status && (
                <StatusBadge status={deployment.app_status} size="sm" />
              )}
            </div>
          </div>
        </div>

        {/* 保留中バナー */}
        {hasPending && (
          <div className="bg-amber-50 border border-amber-200 rounded-lg px-4 py-3 space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 text-sm text-amber-800">
                <span className="w-2 h-2 rounded-full bg-amber-500 shrink-0" />
                <span className="font-medium">Apply 待ちの変更が {allPendingItems.length} 件あります</span>
              </div>
              <button
                onClick={() => void handleApply()}
                disabled={applying}
                className="text-xs font-medium text-amber-700 hover:text-amber-900 underline"
              >
                今すぐ Apply
              </button>
            </div>
            <ul className="space-y-1 pl-4">
              {allPendingItems.map((item, itemIndex) => (
                <li key={itemIndex} className="flex items-center justify-between gap-2">
                  <span className="text-xs text-amber-700 font-mono truncate">{item.label}</span>
                  <button
                    onClick={() => void handleDiscard(item)}
                    disabled={discardingLabel !== null}
                    className="text-[11px] text-amber-600 hover:text-red-600 underline shrink-0 disabled:opacity-50"
                  >
                    {discardingLabel === item.label ? '取り消し中...' : '取り消す'}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* タブナビゲーション */}
        <div className="border-b border-gray-200">
          <nav className="flex gap-0">
            {availableTabs.map((tab) => (
              <button
                key={tab}
                data-tutorial={
                  tab === 'networking' ? 'tutorial-deployment-networking-tab'
                  : tab === 'env-vars' ? 'tutorial-deployment-envvars-tab'
                  : tab === 'volumes' ? 'tutorial-deployment-volumes-tab'
                  : tab === 'history' ? 'tutorial-deployment-history-tab'
                  : undefined
                }
                onClick={() => {
                  setActiveTab(tab) // タブを切り替える
                  // チュートリアル中：各タブクリックで対応するステップを進める
                  const tabToStepIds: Record<string, string[]> = {
                    networking: ['deployment-networking-tab'],
                    'env-vars': ['deployment-envvars-tab', 'adv-envvar-mount-tab'],
                    volumes: ['deployment-volumes-tab', 'adv-storage-mount-tab'],
                    history: ['deployment-history-tab'],
                  }
                  if (tutorialIsActive && tabToStepIds[tab]?.includes(tutorialActualStep?.id ?? '')) {
                    tutorialAdvance()
                  }
                }}
                className={`px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === tab
                    ? 'border-[#00C2D1] text-[#00C2D1]'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                {{ overview: '概要', logs: 'ログ', builds: 'ビルド', settings: '設定', networking: 'ネットワーク', 'env-vars': '環境変数', volumes: 'ボリューム', history: '履歴', metrics: 'メトリクス', webhook: 'Webhook' }[tab]}
              </button>
            ))}
          </nav>
        </div>

        {/* タブコンテンツ */}
        <div>
          {activeTab === 'overview' && <OverviewTab deployment={deployment} projectId={projectId!} />}
          {activeTab === 'logs' && <LogsTab deploymentId={deploymentId!} />}
          {activeTab === 'builds' && <BuildsTab deploymentId={deploymentId!} projectId={projectId!} deployment={deployment} />}
          {activeTab === 'settings' && <SettingsTab deployment={deployment} onSaved={fetchDeployment} />}
          {activeTab === 'networking' && <NetworkingTab deploymentId={deploymentId!} projectId={projectId!} onUpdated={fetchAllPending} />}
          {activeTab === 'env-vars' && <EnvVarsTab deploymentId={deploymentId!} projectId={projectId!} onUpdated={fetchAllPending} />}
          {activeTab === 'volumes' && <VolumesTab deploymentId={deploymentId!} projectId={projectId!} onUpdated={fetchAllPending} />}
          {activeTab === 'history' && <HistoryTab deploymentId={deploymentId!} />}
          {activeTab === 'metrics' && <MetricsTab deploymentId={deploymentId!} />}
          {activeTab === 'webhook' && <WebhookTab deploymentId={deploymentId!} deploymentType={deployment?.type ?? 'image_url'} />}
        </div>
      </div>
      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="デプロイメントを削除"
        description={`「${deployment?.name}」を削除しますか？\nこの操作は取り消せません。`}
        confirmLabel="削除"
        variant="destructive"
        onConfirm={async () => {
          setDeleteConfirmOpen(false) // ダイアログを閉じる
          setDeleting(true) // 削除中フラグを立てる
          setFastPolling(true) // 削除後はポーリングを高速化する
          try {
            await del(`/deployments/${deploymentId}`) // デプロイメントを削除する
            setDeleted(true) // 削除完了画面を表示する（navigate しない）
          } catch (deleteError) {
            console.error(deleteError)
            toast.error(deleteError instanceof Error ? deleteError.message : '削除に失敗しました') // エラーをトーストで表示する
            setFastPolling(false)
          } finally {
            setDeleting(false) // 削除中フラグを下げる
          }
        }}
      />
    </Layout>
  )
}

// ── Overview タブ ─────────────────────────────────────────────

function OverviewTab({ deployment }: { deployment: Deployment; projectId: string }) {
  return (
    <div className="grid grid-cols-2 gap-4">
      <InfoCard label="ステータス" value={<StatusBadge status={deployment.status} size="md" />} />
      <InfoCard label="アプリステータス" value={<StatusBadge status={deployment.app_status} size="md" />} />
      <InfoCard label="タイプ" value={deployment.type} mono />
      <InfoCard label="レプリカ数" value={String(deployment.replicas)} />
      <InfoCard label="インスタンスサイズ" value={deployment.instance_size || '—'} />
      <InfoCard label="最終Apply日時" value={deployment.applied_at ? new Date(deployment.applied_at).toLocaleString('ja-JP') : '未Apply'} />
      {deployment.type === 'image_url' && (
        <InfoCard label="イメージURL" value={deployment.image_url || '—'} mono fullWidth />
      )}
      {deployment.type !== 'image_url' && (
        <>
          <InfoCard label="リポジトリ" value={deployment.github_repo_url || '—'} mono fullWidth />
          <InfoCard label="ブランチ" value={deployment.github_branch || '—'} mono />
          <InfoCard label="コミットSHA" value={deployment.github_commit_sha || '—'} mono />
        </>
      )}
      {deployment.current_build_id && (
        <div className="col-span-2">
          <Link
            to={`/builds/${deployment.current_build_id}/logs`}
            className="inline-flex items-center gap-1.5 text-sm text-[#00C2D1] hover:underline"
          >
            <ExternalLink className="w-3.5 h-3.5" />
            現在のビルドログを表示
          </Link>
        </div>
      )}
    </div>
  )
}

function InfoCard({
  label,
  value,
  mono = false,
  fullWidth = false,
}: {
  label: string
  value: React.ReactNode
  mono?: boolean
  fullWidth?: boolean
}) {
  return (
    <div className={`bg-white rounded-lg border border-gray-200 p-3 ${fullWidth ? 'col-span-2' : ''}`}>
      <p className="text-xs text-gray-400 mb-1">{label}</p>
      <div className={`text-sm text-[#111827] ${mono ? 'font-mono' : 'font-medium'} truncate`}>
        {value}
      </div>
    </div>
  )
}

// ── Logs タブ ─────────────────────────────────────────────────

function LogsTab({ deploymentId }: { deploymentId: string }) {
  const [activePodNames, setActivePodNames] = useState<string[]>([]) // k8s 上で現在稼働中の Pod 名一覧を管理する
  const [podEntries, setPodEntries] = useState<PodLogEntry[]>([]) // Pod ごとのログエントリ一覧を管理する
  const [selectedPodName, setSelectedPodName] = useState<string | null>(null) // 選択中の Pod 名を管理する

  // 初回ロードと差分ポーリングのベース関数
  const fetchAllPodLogs = useCallback(async (since?: string) => {
    const params: Record<string, string> = {}
    if (since) params.since = since // since パラメータを設定する
    const result = await get<PodLogsResponse>(`/deployments/${deploymentId}/logs`, params)
    return result
  }, [deploymentId])

  // 初回ロード：全 Pod のログを取得して状態に反映する
  useEffect(() => {
    const loadInitial = async () => {
      try {
        const result = await fetchAllPodLogs()
        setActivePodNames(result.active_pod_names ?? []) // アクティブな Pod 名一覧を設定する
        setPodEntries(result.pods ?? []) // Pod ログ一覧を設定する
        const allPodNames = result.active_pod_names ?? []
        if (allPodNames.length > 0 && selectedPodName === null) {
          setSelectedPodName(allPodNames[0]) // 初回は最初の Pod を選択する
        }
      } catch (loadError) {
        console.error('Pod ログ取得エラー:', loadError)
      }
    }
    void loadInitial()
  }, [fetchAllPodLogs]) // eslint-disable-line react-hooks/exhaustive-deps

  // 選択中の Pod 用の fetchLogs（LogViewer に渡す）
  const fetchSelectedPodLogs = useCallback(async (since?: string) => {
    if (!selectedPodName) return { logs: '', lastTimestamp: null }
    const result = await fetchAllPodLogs(since)
    const latestActivePodNames = result.active_pod_names ?? []
    const pods = result.pods ?? []

    // アクティブ Pod 一覧を同期する
    setActivePodNames((prev) => {
      const prevKey = prev.join(',')
      const nextKey = latestActivePodNames.join(',')
      return prevKey !== nextKey ? latestActivePodNames : prev // 差分があれば更新する
    })
    setPodEntries((prev) => {
      const latestNames = new Set(pods.map((podEntry) => podEntry.pod_name))
      const prevNames = new Set(prev.map((podEntry) => podEntry.pod_name))
      const hasAdded = pods.some((podEntry) => !prevNames.has(podEntry.pod_name))
      const hasRemoved = prev.some((podEntry) => !latestNames.has(podEntry.pod_name))
      return (hasAdded || hasRemoved) ? pods : prev // 差分があれば最新リストで置き換える
    })

    // 選択中の Pod が消えた場合は先頭のアクティブ Pod に切り替える
    if (!latestActivePodNames.includes(selectedPodName) && latestActivePodNames.length > 0) {
      setSelectedPodName(latestActivePodNames[0]) // 先頭の Pod を選択する
    }

    const pod = pods.find((podEntry) => podEntry.pod_name === selectedPodName) // 選択中の Pod のエントリを検索する
    if (!pod) return { logs: '', lastTimestamp: null }
    return { logs: pod.logs, lastTimestamp: pod.last_timestamp }
  }, [selectedPodName, fetchAllPodLogs])

  // タブに表示する Pod 名一覧（アクティブ Pod 優先、ログがある Pod も含める）
  const tabPodNames = Array.from(new Set([
    ...activePodNames,
    ...podEntries.map((podEntry) => podEntry.pod_name),
  ])) // アクティブ Pod とログがある Pod をマージして重複排除する

  return (
    <div style={{ height: 'calc(100vh - 220px)' }} className="flex flex-col">
      {/* Pod タブ */}
      {tabPodNames.length > 0 && (
        <div className="flex gap-1 px-3 py-2 shrink-0 border-b border-[#30363D] bg-[#161B22] overflow-x-auto">
          {tabPodNames.map((podName) => {
            const isActive = activePodNames.includes(podName) // k8s 上で稼働中かどうか
            return (
              <button
                key={podName}
                onClick={() => setSelectedPodName(podName)}
                className={`text-xs px-3 py-1 rounded-md font-mono whitespace-nowrap transition-colors ${
                  selectedPodName === podName
                    ? 'bg-[#00C2D1]/20 text-[#00C2D1] border border-[#00C2D1]/40'
                    : 'text-[#8B949E] hover:text-[#E6EDF3] border border-transparent hover:border-[#30363D]'
                }`}
              >
                <span className={`inline-block w-1.5 h-1.5 rounded-full mr-1.5 ${isActive ? 'bg-green-400' : 'bg-gray-500'}`} />
                {podName}
              </button>
            )
          })}
        </div>
      )}
      {/* ログビューアー */}
      {selectedPodName ? (
        <div className="flex-1 min-h-0">
          <LogViewer
            key={selectedPodName}
            fetchLogs={fetchSelectedPodLogs}
            title={`${selectedPodName}`}
            pollInterval={POLL_INTERVAL_NORMAL}
            initialLive={true}
          />
        </div>
      ) : (
        <div className="flex items-center justify-center flex-1 text-xs text-[#8B949E]">
          Pod 待機中...
        </div>
      )}
    </div>
  )
}

// ── Builds タブ ───────────────────────────────────────────────

const BUILD_STATUS_META: Record<string, { label: string; icon: React.ReactNode; badge: string }> = {
  pending:   { label: '待機中',     icon: <Clock         className="w-3.5 h-3.5" />, badge: 'bg-yellow-100 text-yellow-700 border-yellow-200' },
  building:  { label: 'ビルド中',   icon: <AlertCircle   className="w-3.5 h-3.5 animate-pulse" />, badge: 'bg-blue-100 text-blue-700 border-blue-200' },
  succeeded: { label: '成功',       icon: <CheckCircle2  className="w-3.5 h-3.5" />, badge: 'bg-green-100 text-green-700 border-green-200' },
  failed:    { label: '失敗',       icon: <XCircle       className="w-3.5 h-3.5" />, badge: 'bg-red-100 text-red-700 border-red-200' },
  cancelled: { label: 'キャンセル', icon: <Ban           className="w-3.5 h-3.5" />, badge: 'bg-gray-100 text-gray-500 border-gray-200' },
}

function BuildsTab({
  deploymentId,
  deployment,
}: {
  deploymentId: string
  projectId: string
  deployment: Deployment
}) {
  const navigate = useNavigate()
  const [building, setBuilding] = useState(false) // ビルド中フラグ
  const [buildList, setBuildList] = useState<Build[]>([]) // ビルド一覧を管理する
  const [cancellingId, setCancellingId] = useState<string | null>(null) // キャンセル中のビルドIDを管理する

  const fetchBuilds = useCallback(async () => {
    try {
      const data = await get<Build[]>(`/deployments/${deploymentId}/builds`) // ビルド一覧を取得する
      setBuildList(data ?? [])
    } catch (fetchError) {
      console.error(fetchError)
    }
  }, [deploymentId])

  useEffect(() => {
    void fetchBuilds() // 初回取得
    const intervalId = setInterval(() => void fetchBuilds(), POLL_INTERVAL_BUILDS) // 定期的にポーリングする
    return () => clearInterval(intervalId)
  }, [fetchBuilds])

  const handleBuild = async () => {
    setBuilding(true)
    try {
      // GitHub リポジトリ情報から最新コミットのメッセージと著者を取得する
      let commitMessage = ''
      let author = ''
      const repoUrl = deployment.pending_github_repo_url || deployment.github_repo_url
      const branch = deployment.pending_github_branch || deployment.github_branch
      if (repoUrl && branch) {
        const repo = extractGitHubRepo(repoUrl) // owner/repo を抽出する
        if (repo) {
          try {
            const commits = await fetchGitHubCommits(repo, branch) // 最新コミット一覧を取得する
            if (commits.length > 0) {
              commitMessage = commits[0].commit.message // 最新コミットメッセージを取得する
              author = commits[0].commit.author.name   // 最新コミット著者を取得する
            }
          } catch {
            // GitHub API 取得失敗は無視してビルドを続行する
          }
        }
      }

      const result = await post<Build>(`/deployments/${deploymentId}/build`, { commit_message: commitMessage, author }) // ビルドを開始する
      await fetchBuilds() // 一覧を即座に更新する
      navigate(`/builds/${result.id}/logs`) // ビルドログページへ遷移する
    } catch (buildError) {
      console.error(buildError)
      toast.error(buildError instanceof Error ? buildError.message : 'ビルドの開始に失敗しました') // エラーをトーストで表示する
    } finally {
      setBuilding(false)
    }
  }

  const handleCancel = async (buildId: string, event: React.MouseEvent) => {
    event.preventDefault() // Link のナビゲーションを阻止する
    event.stopPropagation()
    setCancellingId(buildId)
    try {
      await del(`/builds/${buildId}`) // ビルドをキャンセルする
      await fetchBuilds() // 一覧を即座に更新する
    } catch (cancelError) {
      console.error(cancelError)
      toast.error(cancelError instanceof Error ? cancelError.message : 'キャンセルに失敗しました') // エラーをトーストで表示する
    } finally {
      setCancellingId(null)
    }
  }

  const hasPendingOrBuilding = buildList.some(
    buildItem => buildItem.status === 'pending' || buildItem.status === 'building'
  ) // 進行中のビルドが存在するか確認する

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <button
          onClick={() => void handleBuild()}
          disabled={building || hasPendingOrBuilding}
          title={hasPendingOrBuilding ? 'ビルドが進行中です。完了またはキャンセル後に再試行してください' : undefined}
          className="flex items-center gap-1.5 bg-[#111827] text-white text-sm px-3 py-1.5 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {building ? 'ビルド開始中...' : 'ビルド'}
        </button>
      </div>

      {buildList.length === 0 ? (
        <div className="text-center py-12 bg-white rounded-lg border border-dashed border-gray-200">
          <p className="text-sm text-gray-400">ビルド履歴がありません</p>
        </div>
      ) : (
        <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
          <div className="px-4 py-3 border-b border-gray-100 text-xs font-medium text-gray-400 uppercase tracking-wider">
            ビルド履歴
          </div>
          <div className="divide-y divide-gray-100">
            {buildList.map(buildItem => {
              const statusMeta = BUILD_STATUS_META[buildItem.status] // ステータスメタデータを取得する
              const isCancellable = buildItem.status === 'pending' || buildItem.status === 'building' // キャンセル可能か判定する
              return (
                <div key={buildItem.id} className="flex items-center justify-between px-4 py-3 hover:bg-gray-50 transition-colors">
                  {/* クリック可能なリンク部分 */}
                  <Link
                    to={`/builds/${buildItem.id}/logs`}
                    className="flex items-center gap-3 min-w-0 flex-1"
                  >
                    {/* ステータスバッジ */}
                    <span className={`inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full border ${statusMeta?.badge ?? 'bg-gray-100 text-gray-500 border-gray-200'}`}>
                      {statusMeta?.icon}
                      {statusMeta?.label ?? buildItem.status}
                    </span>
                    {/* ビルドID */}
                    <span className="font-mono text-xs text-[#111827] shrink-0">#{buildItem.id.slice(0, 8)}</span>
                    {/* ブランチ */}
                    {buildItem.branch && (
                      <span className="flex items-center gap-1 text-xs text-gray-500 shrink-0">
                        <GitBranch className="w-3 h-3" />
                        {buildItem.branch}
                      </span>
                    )}
                    {/* コミットSHA + メッセージ */}
                    {buildItem.commit_sha && (
                      <span className="flex items-center gap-1 text-xs text-gray-400 truncate min-w-0">
                        <GitCommit className="w-3 h-3 shrink-0" />
                        <span className="font-mono shrink-0">{buildItem.commit_sha.slice(0, 7)}</span>
                        {buildItem.commit_message && (
                          <span className="truncate text-gray-500">{buildItem.commit_message}</span>
                        )}
                      </span>
                    )}
                  </Link>
                  {/* 右側: 日時・キャンセルボタン・ログリンク */}
                  <div className="flex items-center gap-2 shrink-0 ml-3">
                    {buildItem.created_at && (
                      <span className="text-xs text-gray-400">
                        {new Date(buildItem.created_at).toLocaleString('ja-JP')}
                      </span>
                    )}
                    {/* キャンセルボタン（pending/building のみ表示）*/}
                    {isCancellable && (
                      <button
                        onClick={(event) => void handleCancel(buildItem.id, event)}
                        disabled={cancellingId === buildItem.id}
                        className="flex items-center gap-1 text-xs px-2 py-0.5 rounded border border-red-200 text-red-500 hover:bg-red-50 transition-colors disabled:opacity-50"
                      >
                        <X className="w-3 h-3" />
                        {cancellingId === buildItem.id ? 'キャンセル中...' : 'キャンセル'}
                      </button>
                    )}
                    <Link to={`/builds/${buildItem.id}/logs`} onClick={(event) => event.stopPropagation()}>
                      <ExternalLink className="w-3.5 h-3.5 text-gray-400" />
                    </Link>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}

// ── Settings タブ ─────────────────────────────────────────────

// GitHub URL からオーナー/リポジトリ名を抽出する
function extractGitHubRepo(url: string): string | null {
  try {
    const parsed = new URL(url.trim())
    if (parsed.hostname !== 'github.com') return null // github.com 以外は無効
    const parts = parsed.pathname.split('/').filter(Boolean) // パスを分割する
    if (parts.length < 2) return null // owner/repo の形式でない場合は無効
    return `${parts[0]}/${parts[1]}` // owner/repo を返す
  } catch {
    return null // URL パース失敗時は null を返す
  }
}

type GitHubBranch = { name: string }
type GitHubCommit = { sha: string; commit: { message: string; author: { name: string } } }
type GitHubTree  = { path: string; type: string }

// GitHub API からブランチ一覧を取得する
async function fetchGitHubBranches(repo: string): Promise<GitHubBranch[]> {
  const res = await fetch(`https://api.github.com/repos/${repo}/branches?per_page=${GITHUB_BRANCHES_PER_PAGE}`) // GitHub API を呼ぶ
  if (!res.ok) throw new Error(`branches fetch failed: ${res.status}`)
  return res.json() as Promise<GitHubBranch[]>
}

// GitHub API からブランチの最新コミット一覧を取得する
async function fetchGitHubCommits(repo: string, branch: string): Promise<GitHubCommit[]> {
  const res = await fetch(`https://api.github.com/repos/${repo}/commits?sha=${branch}&per_page=${GITHUB_COMMITS_PER_PAGE}`) // GitHub API を呼ぶ
  if (!res.ok) throw new Error(`commits fetch failed: ${res.status}`)
  return res.json() as Promise<GitHubCommit[]>
}

// GitHub API からルートディレクトリ一覧を取得する
async function fetchGitHubDirs(repo: string, branch: string): Promise<string[]> {
  const res = await fetch(`https://api.github.com/repos/${repo}/git/trees/${branch}?recursive=0`) // ルートツリーを取得する
  if (!res.ok) throw new Error(`tree fetch failed: ${res.status}`)
  const data = await res.json() as { tree: GitHubTree[] }
  return data.tree
    .filter(item => item.type === 'tree') // ディレクトリのみ抽出する
    .map(item => `./${item.path}`) // パス名を ./ スタートで取り出す
}

function SettingsTab({ deployment, onSaved }: { deployment: Deployment; onSaved: () => Promise<void> }) {
  const [formData, setFormData] = useState({
    image_url: deployment.pending_image_url || deployment.image_url || '',
    github_repo_url: deployment.pending_github_repo_url || deployment.github_repo_url || '',
    github_branch: deployment.pending_github_branch || deployment.github_branch || '',
    github_commit_sha: deployment.pending_github_commit_sha || deployment.github_commit_sha || '',
    github_repo_directory: deployment.pending_github_repo_directory || deployment.github_repo_directory || '',
    dockerfile_path: deployment.pending_dockerfile_path || deployment.dockerfile_path || '',
    replicas: String(deployment.pending_replicas || deployment.replicas || 1),
    instance_size: deployment.pending_instance_size || deployment.instance_size || '',
  }) // フォームデータを管理する
  const [saving, setSaving] = useState(false) // 保存中フラグ

  // GitHub API から取得したデータ
  const [ghBranches, setGhBranches]   = useState<GitHubBranch[]>([]) // ブランチ一覧
  const [ghCommits, setGhCommits]     = useState<GitHubCommit[]>([]) // コミット一覧
  const [ghDirs, setGhDirs]           = useState<string[]>([])       // ディレクトリ一覧
  const [ghLoading, setGhLoading]     = useState<'branches' | 'commits' | 'dirs' | null>(null) // ローディング中の対象
  const [ghError, setGhError]         = useState<string | null>(null) // エラーメッセージ

  // コミット・ディレクトリ一覧を取得する共通関数
  const loadCommitsAndDirs = useCallback(async (repoUrl: string, branch: string) => {
    const repo = extractGitHubRepo(repoUrl) // owner/repo を抽出する
    if (!repo || !branch) return
    setGhLoading('commits')
    setGhError(null)
    setGhCommits([])
    setGhDirs([])
    try {
      const [commits, dirs] = await Promise.all([
        fetchGitHubCommits(repo, branch), // コミット一覧を取得する
        fetchGitHubDirs(repo, branch),    // ディレクトリ一覧を取得する
      ])
      setGhCommits(commits)
      setGhDirs(dirs)
    } catch {
      setGhError('コミット一覧の取得に失敗しました。') // エラーを表示する
    } finally {
      setGhLoading(null)
    }
  }, [])

  // リポジトリURLが確定したときにブランチ一覧を取得する（loadCommitsAndDirs より後に定義する必要がある）
  const loadBranches = useCallback(async (repoUrl: string, currentBranch?: string) => {
    const repo = extractGitHubRepo(repoUrl) // owner/repo を抽出する
    if (!repo) return
    setGhLoading('branches') // ブランチ取得中を示す
    setGhError(null)
    setGhBranches([])
    setGhCommits([])
    setGhDirs([])
    try {
      const branches = await fetchGitHubBranches(repo) // ブランチ一覧を取得する
      setGhBranches(branches)
      if (currentBranch) {
        await loadCommitsAndDirs(repoUrl, currentBranch) // 現在のブランチのコミット一覧も取得する
      }
    } catch {
      setGhError('ブランチの取得に失敗しました。リポジトリURLを確認してください。') // エラーを表示する
    } finally {
      setGhLoading(null)
    }
  }, [loadCommitsAndDirs])


  // ブランチが選択されたときにコミット一覧とディレクトリ一覧を取得する
  const handleBranchSelect = async (branch: string) => {
    setFormData(prev => ({ ...prev, github_branch: branch, github_commit_sha: '', github_repo_directory: './' })) // ブランチを設定しコミット・ディレクトリをリセットする
    await loadCommitsAndDirs(formData.github_repo_url, branch) // コミット・ディレクトリ一覧を取得する
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const body: Record<string, unknown> = {
        replicas: parseInt(formData.replicas, 10), // レプリカ数を数値に変換する
        instance_size: formData.instance_size,
      }

      if (deployment.type === 'image_url') {
        body.image_url = formData.image_url // image_url タイプの場合はイメージURLを設定する
      } else {
        body.github_repo_url = formData.github_repo_url || null // 空文字はnullにして更新をスキップする
        body.github_branch = formData.github_branch || null // 空文字はnullにして更新をスキップする
        body.github_commit_sha = formData.github_commit_sha || null // 空文字はnullにして更新をスキップする
        body.github_repo_directory = formData.github_repo_directory || null // 空文字はnullにして更新をスキップする
        if (deployment.type === 'dockerfile') {
          body.dockerfile_path = formData.dockerfile_path // Dockerfileのパスを設定する
        }
      }

      await put(`/deployments/${deployment.id}`, body) // デプロイメントを更新する
      await onSaved() // 保存後にデプロイメント情報を再取得する
    } catch (saveError) {
      console.error(saveError)
      toast.error(saveError instanceof Error ? saveError.message : '保存に失敗しました') // エラーをトーストで表示する
    } finally {
      setSaving(false)
    }
  }

  const inputClass = 'w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#00C2D1]/50 focus:border-[#00C2D1] transition-colors font-mono'
  const labelClass = 'block text-xs font-medium text-gray-500 mb-1'

  return (
    <div className="space-y-4 max-w-3xl">
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-4">
        {/* image_url タイプのフォーム */}
        {deployment.type === 'image_url' && (
          <div>
            <label className={labelClass}>イメージURL</label>
            {deployment.image_url && deployment.pending_image_url && deployment.image_url !== deployment.pending_image_url && (
              <p className="text-xs text-amber-600 mb-1">
                現在: <span className="font-mono">{deployment.image_url}</span> → 保留中: <span className="font-mono">{deployment.pending_image_url}</span>
              </p>
            )}
            <input
              type="text"
              value={formData.image_url}
              onChange={(event) => setFormData((prev) => ({ ...prev, image_url: event.target.value }))}
              placeholder="nginx:latest"
              className={inputClass}
            />
          </div>
        )}

        {/* dockerfile / railpack タイプのフォーム */}
        {deployment.type !== 'image_url' && (
          <>
            {/* リポジトリURL入力 */}
            <div>
              <label className={labelClass}>GitHubリポジトリURL</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={formData.github_repo_url}
                  onChange={(event) => setFormData(prev => ({ ...prev, github_repo_url: event.target.value }))}
                  placeholder="https://github.com/org/repo"
                  className={inputClass}
                />
                <button
                  type="button"
                  onClick={() => void loadBranches(formData.github_repo_url, formData.github_branch || undefined)}
                  disabled={ghLoading === 'branches'}
                  className="shrink-0 px-3 py-2 text-xs rounded-md bg-[#111827] text-white hover:bg-gray-800 transition-colors disabled:opacity-50"
                >
                  {ghLoading === 'branches' ? '取得中...' : '読み込む'}
                </button>
              </div>
              {ghError && <p className="text-xs text-red-500 mt-1">{ghError}</p>}
            </div>

            {/* ブランチ：常にテキスト入力、API取得済みならドロップダウンも表示 */}
            <div>
              <label className={labelClass}>ブランチ</label>
              <input
                type="text"
                value={formData.github_branch}
                onChange={(event) => setFormData(prev => ({ ...prev, github_branch: event.target.value }))}
                placeholder="main"
                className={inputClass}
              />
              {ghBranches.length > 0 && (
                <select
                  value={formData.github_branch}
                  onChange={(event) => void handleBranchSelect(event.target.value)}
                  className={`${inputClass} mt-1`}
                >
                  <option value="">ブランチを選択...</option>
                  {ghBranches.map(branchItem => (
                    <option key={branchItem.name} value={branchItem.name}>{branchItem.name}</option>
                  ))}
                </select>
              )}
            </div>

            {/* コミットSHA：常にテキスト入力、API取得済みならドロップダウンも表示 */}
            {ghLoading === 'commits' && <p className="text-xs text-gray-400">コミット・ディレクトリを取得中...</p>}
            <div>
              <label className={labelClass}>コミットSHA（空欄で最新）</label>
              <input
                type="text"
                value={formData.github_commit_sha}
                onChange={(event) => setFormData(prev => ({ ...prev, github_commit_sha: event.target.value }))}
                placeholder="例: abc1234（空欄で最新）"
                className={inputClass}
              />
              {ghCommits.length > 0 && (
                <select
                  value={formData.github_commit_sha}
                  onChange={(event) => setFormData(prev => ({ ...prev, github_commit_sha: event.target.value }))}
                  className={`${inputClass} mt-1`}
                >
                  <option value="">最新のコミット（HEAD）</option>
                  {ghCommits.map(commitItem => (
                    <option key={commitItem.sha} value={commitItem.sha}>
                      {commitItem.sha.slice(0, 7)} — {commitItem.commit.message.split('\n')[0].slice(0, GITHUB_COMMIT_MESSAGE_MAX_LENGTH)}
                    </option>
                  ))}
                </select>
              )}
            </div>

            {/* ビルドディレクトリ：常にテキスト入力、API取得済みならドロップダウンも表示 */}
            <div>
              <label className={labelClass}>ビルドディレクトリ</label>
              <input
                type="text"
                value={formData.github_repo_directory}
                onChange={(event) => setFormData(prev => ({ ...prev, github_repo_directory: event.target.value }))}
                placeholder="./"
                className={inputClass}
              />
              {ghDirs.length > 0 && (
                <select
                  value={formData.github_repo_directory}
                  onChange={(event) => setFormData(prev => ({ ...prev, github_repo_directory: event.target.value }))}
                  className={`${inputClass} mt-1`}
                >
                  <option value="./">./（ルート）</option>
                  {ghDirs.map(dirPath => (
                    <option key={dirPath} value={dirPath}>{dirPath}</option>
                  ))}
                </select>
              )}
            </div>

            {deployment.type === 'dockerfile' && (
              <div>
                <label className={labelClass}>Dockerfileのパス</label>
                <input
                  type="text"
                  value={formData.dockerfile_path}
                  onChange={(event) => setFormData((prev) => ({ ...prev, dockerfile_path: event.target.value }))}
                  placeholder="./Dockerfile"
                  className={inputClass}
                />
              </div>
            )}
          </>
        )}

        {/* 共通フォームフィールド */}
        <div className="grid grid-cols-2 gap-4 pt-2 border-t border-gray-100">
          <div>
            <label className={labelClass}>レプリカ数</label>
            <input
              type="number"
              min={REPLICAS_MIN}
              max={REPLICAS_MAX}
              value={formData.replicas}
              onChange={(event) => setFormData((prev) => ({ ...prev, replicas: event.target.value }))}
              className={inputClass}
            />
          </div>
          <div>
            <label className={labelClass}>インスタンスサイズ</label>
            <select
              value={formData.instance_size}
              onChange={(event) => setFormData((prev) => ({ ...prev, instance_size: event.target.value }))}
              className={inputClass}
            >
              {INSTANCE_SIZES.map(size => (
                <option key={size} value={size}>{size}</option>
              ))}
            </select>
          </div>
        </div>
      </div>

      <button
        onClick={() => void handleSave()}
        disabled={saving}
        className="bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
      >
        {saving ? '保存中...' : '変更を保存'}
      </button>
    </div>
  )
}

// ── Networking タブ ───────────────────────────────────────────

function NetworkingTab({ deploymentId, projectId, onUpdated }: { deploymentId: string; projectId: string; onUpdated: () => Promise<void> }) {
  const { advance: tutorialAdvance, isActive: tutorialIsActive, actualStep: tutorialActualStep } = useTutorialContext() // チュートリアルの状態を取得する

  const [service, setService] = useState<K8sService | null>(null) // サービス情報を管理する
  const [namespace, setNamespace] = useState<string>('') // プロジェクトの namespace を管理する
  const [svcForm, setSvcForm] = useState({ port: '', target_port: '' }) // サービス設定フォーム
  const [savingSvc, setSavingSvc] = useState(false) // サービス保存中フラグ
  const [deletingSvc, setDeletingSvc] = useState(false) // サービス削除中フラグ
  const [copiedKey, setCopiedKey] = useState<string | null>(null) // コピー済みのキーを管理する
  const [deleteSvcConfirmOpen, setDeleteSvcConfirmOpen] = useState(false) // Service無効化確認ダイアログの開閉を管理する

  const fetchNetworking = async () => {
    const svcResult = await get<K8sService>(`/deployments/${deploymentId}/service`).catch(() => null) // サービス情報を取得する
    setService(svcResult) // サービス情報を設定する
  }

  const handleCopy = (text: string, key: string) => {
    void navigator.clipboard.writeText(text) // クリップボードにコピーする
    setCopiedKey(key) // コピー済みキーを設定する
    setTimeout(() => setCopiedKey(null), 1500) // 1.5秒後にリセットする
  }

  useEffect(() => {
    void fetchNetworking()
    const intervalId = setInterval(() => { void fetchNetworking() }, POLL_INTERVAL_BUILDS)
    return () => clearInterval(intervalId) // クリーンアップ
  }, [deploymentId])

  useEffect(() => {
    get<Project>(`/projects/${projectId}`) // プロジェクト情報を取得して namespace を設定する
      .then(proj => setNamespace(proj.namespace))
      .catch(console.error)
  }, [projectId])

  // status が pending でない（active）かつ port が設定済みの場合のみ「設定済み」とする
  const serviceConfigured = service && service.status !== 'pending'
  // 無効化が保留中：status=pending かつ port が設定済み（無効化を予約して Apply 前の状態）
  const serviceDisablePending = service && service.status === 'pending' && service.port !== 0

  const handleDeleteService = () => {
    setDeleteSvcConfirmOpen(true) // Service無効化確認ダイアログを開く
  }

  const handleSaveService = async () => {
    setSavingSvc(true)
    try {
      const portNum = parseInt(svcForm.port, 10)
      const targetPortNum = parseInt(svcForm.target_port, 10)
      if (service) {
        // 既存 Service の更新は PUT を使う
        await put(`/deployments/${deploymentId}/service`, {
          port: portNum,
          target_port: targetPortNum,
        })
      } else {
        // Service が未作成の場合は POST で新規作成する
        await post(`/deployments/${deploymentId}/service`, {
          port: portNum,
          target_port: targetPortNum,
        })
      }
      await Promise.all([fetchNetworking(), onUpdated()]) // 最新状態を再取得する
      // チュートリアル中なら Service 保存完了でステップを進める
      if (tutorialIsActive && (tutorialActualStep?.id === 'deployment-port-input' || tutorialActualStep?.id === 'deployment-target-port-input' || tutorialActualStep?.id === 'deployment-service-save')) {
        tutorialAdvance() // 次のステップへ進む
      }
    } catch (saveError) {
      console.error(saveError)
      toast.error(saveError instanceof Error ? saveError.message : 'サービスの保存に失敗しました') // エラーをトーストで表示する
    } finally {
      setSavingSvc(false)
    }
  }

  const inputClass = 'w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#00C2D1]/50 focus:border-[#00C2D1] transition-colors font-mono'
  const labelClass = 'block text-xs font-medium text-gray-500 mb-1'

  return (
    <div className="space-y-4 max-w-3xl">
      {/* Service セクション */}
      <div className={`bg-white rounded-lg border p-4 space-y-3 ${serviceDisablePending ? 'border-amber-300' : 'border-gray-200'}`}>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <h3 className="text-sm font-semibold text-[#111827]">Kubernetesサービス</h3>
            {serviceDisablePending && (
              <span className="text-xs bg-amber-100 text-amber-700 px-2 py-0.5 rounded-full font-medium">無効化 Apply 待ち</span>
            )}
          </div>
          {serviceConfigured && !serviceDisablePending && (
            <button onClick={() => void handleDeleteService()} disabled={deletingSvc} className="text-xs text-red-500 hover:text-red-700 disabled:opacity-50">
              {deletingSvc ? '処理中...' : '無効化'}
            </button>
          )}
        </div>
        {serviceDisablePending ? (
          <div className="bg-amber-50 border border-amber-200 rounded-md px-3 py-2 text-sm text-amber-800">
            無効化が保留中です。現在のポート: <span className="font-mono font-medium">{service!.port} → {service!.target_port}</span>。Apply を実行すると k8s から削除されます。
          </div>
        ) : serviceConfigured ? (
          <>
            <div className="space-y-2 text-sm">
              <Row label="ステータス"><StatusBadge status={service!.status} /></Row>
              {service!.port !== 0 && (
                <Row label="ポート"><span className="font-mono">{service!.port} → {service!.target_port}</span></Row>
              )}
              {service!.pending_port !== 0 && service!.pending_port !== service!.port && (
                <Row label="保留中のポート"><span className="font-mono text-amber-600">{service!.pending_port} → {service!.pending_target_port}</span></Row>
              )}
            </div>
            {/* 接続情報カード：ClusterIP が割り当て済みの場合のみ表示する */}
            {service!.cluster_ip && namespace && (
              <div className="bg-gray-50 rounded-md border border-gray-200 p-3 space-y-2">
                <p className="text-xs font-semibold text-gray-500 uppercase tracking-wide">接続情報</p>
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0">
                      <p className="text-xs text-gray-400 mb-0.5">ClusterIP（クラスター内部 IP）</p>
                      <p className="font-mono text-sm text-[#111827] truncate">{service!.cluster_ip}</p>
                    </div>
                    <button
                      onClick={() => handleCopy(service!.cluster_ip, 'cluster_ip')}
                      className="shrink-0 text-xs text-gray-400 hover:text-gray-700 border border-gray-200 rounded px-2 py-1 transition-colors"
                    >
                      {copiedKey === 'cluster_ip' ? 'コピー済み' : 'コピー'}
                    </button>
                  </div>
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0">
                      <p className="text-xs text-gray-400 mb-0.5">DNS 名（クラスター内部）</p>
                      <p className="font-mono text-sm text-[#111827] truncate">{service!.id}-svc.{namespace}.svc.cluster.local{service!.port !== 0 ? `:${service!.port}` : ''}</p>
                    </div>
                    <button
                      onClick={() => handleCopy(`${service!.id}-svc.${namespace}.svc.cluster.local${service!.port !== 0 ? `:${service!.port}` : ''}`, 'dns')}
                      className="shrink-0 text-xs text-gray-400 hover:text-gray-700 border border-gray-200 rounded px-2 py-1 transition-colors"
                    >
                      {copiedKey === 'dns' ? 'コピー済み' : 'コピー'}
                    </button>
                  </div>
                </div>
              </div>
            )}
            <div className="pt-3 border-t border-gray-100 space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className={labelClass}>ポート（外部）</label>
                  <input type="number" className={inputClass} placeholder={String(service!.pending_port || service!.port)} value={svcForm.port} onChange={ev => setSvcForm(prev => ({ ...prev, port: ev.target.value }))} />
                </div>
                <div>
                  <label className={labelClass}>ターゲットポート（コンテナ）</label>
                  <input type="number" className={inputClass} placeholder={String(service!.pending_target_port || service!.target_port)} value={svcForm.target_port} onChange={ev => setSvcForm(prev => ({ ...prev, target_port: ev.target.value }))} />
                </div>
              </div>
              <button onClick={() => void handleSaveService()} disabled={savingSvc} className="bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50">
                {savingSvc ? '保存中...' : '変更を保存'}
              </button>
            </div>
          </>
        ) : (
          <div className="space-y-3">
            <p className="text-sm text-gray-400">Serviceが設定されていません。ポートを設定して有効化します。</p>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className={labelClass}>ポート（外部）</label>
                <input data-tutorial="tutorial-deployment-port-input" type="number" className={inputClass} placeholder="80" value={svcForm.port} onChange={ev => setSvcForm(prev => ({ ...prev, port: ev.target.value }))} />
              </div>
              <div>
                <label className={labelClass}>ターゲットポート（コンテナ）</label>
                <input data-tutorial="tutorial-deployment-target-port-input" type="number" className={inputClass} placeholder="8080" value={svcForm.target_port} onChange={ev => setSvcForm(prev => ({ ...prev, target_port: ev.target.value }))} />
              </div>
            </div>
            <button data-tutorial="tutorial-deployment-service-save" onClick={() => void handleSaveService()} disabled={savingSvc || !svcForm.port || !svcForm.target_port} className="bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50">
              {savingSvc ? '保存中...' : 'Serviceを設定'}
            </button>
          </div>
        )}
      </div>

      {/* IngressRoute */}
      <div className="bg-white rounded-lg border border-gray-200 p-4">
        <h3 className="text-sm font-semibold text-[#111827] mb-2">IngressRoute</h3>
        <p className="text-sm text-gray-400">IngressRoute はプロジェクト単位で管理されます。プロジェクトページから設定してください。</p>
      </div>
      <ConfirmDialog
        open={deleteSvcConfirmOpen}
        onOpenChange={setDeleteSvcConfirmOpen}
        title="Serviceを無効化"
        description="Serviceを無効化しますか？保留中になり、Applyで反映されます。"
        confirmLabel="無効化"
        variant="destructive"
        onConfirm={async () => {
          setDeleteSvcConfirmOpen(false) // ダイアログを閉じる
          setDeletingSvc(true) // Service削除中フラグを立てる
          try {
            await del(`/deployments/${deploymentId}/service`) // pending_port=0 にして無効化を予約する
            await Promise.all([fetchNetworking(), onUpdated()]) // 最新状態を再取得する
          } catch (deleteError) {
            console.error(deleteError)
            toast.error(deleteError instanceof Error ? deleteError.message : 'Serviceの無効化に失敗しました') // エラーをトーストで表示する
          } finally {
            setDeletingSvc(false) // Service削除中フラグを下げる
          }
        }}
      />
    </div>
  )
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-4">
      <span className="text-gray-400 w-28 shrink-0">{label}</span>
      <div className="text-[#111827]">{children}</div>
    </div>
  )
}

// ── EnvVars タブ ─────────────────────────────────────────────

function EnvVarsTab({ deploymentId, projectId, onUpdated }: { deploymentId: string; projectId: string; onUpdated: () => Promise<void> }) {
  const { advance: tutorialAdvance, actualStep: tutorialActualStep } = useTutorialContext() // チュートリアルの進行関数を取得する
  const [envVarList, setEnvVarList] = useState<EnvVar[]>([]) // プロジェクトの環境変数一覧を管理する
  const [mountList, setMountList] = useState<EnvVarMount[]>([]) // デプロイメントのマウント設定一覧を管理する
  const [newMountEnvVarId, setNewMountEnvVarId] = useState('') // マウントする環境変数ID
  const [newOverrideKey, setNewOverrideKey] = useState('') // オーバーライドキー（任意）
  const [addingMount, setAddingMount] = useState(false) // マウント作成中フラグ
  const [deletingMountId, setDeletingMountId] = useState<string | null>(null) // 削除中のマウントID
  const [templateList, setTemplateList] = useState<DeploymentTemplate[]>([]) // テンプレート一覧を管理する
  const [selectedTemplateId, setSelectedTemplateId] = useState('') // 選択中のテンプレートID
  const [applyingTemplate, setApplyingTemplate] = useState(false) // テンプレート適用中フラグ
  const [templateError, setTemplateError] = useState('') // テンプレート適用エラー

  const fetchData = useCallback(async () => {
    const [envVars, mounts] = await Promise.all([
      get<EnvVar[]>(`/projects/${projectId}/env-vars`).catch(() => []), // 環境変数一覧を取得する
      get<EnvVarMount[]>(`/deployments/${deploymentId}/env-var-mounts`).catch(() => []), // マウント設定一覧を取得する
    ])
    setEnvVarList(envVars ?? []) // 環境変数一覧を設定する
    setMountList(mounts ?? []) // マウント設定を設定する
  }, [projectId, deploymentId])

  useEffect(() => {
    void fetchData() // 初回データ取得
    get<DeploymentTemplate[]>('/deployment-templates')
      .then(data => setTemplateList(data ?? []))
      .catch(console.error) // テンプレート一覧を取得する
  }, [fetchData])

  const handleAddMount = async () => {
    if (!newMountEnvVarId) return
    setAddingMount(true) // マウント作成中フラグを立てる
    try {
      await post(`/deployments/${deploymentId}/env-var-mounts`, {
        env_var_id: newMountEnvVarId,
        override_key: newOverrideKey, // 空文字の場合はサーバー側で元のキーを使用する
      })
      setNewMountEnvVarId('') // フォームをリセットする
      setNewOverrideKey('') // オーバーライドキーをリセットする
      await Promise.all([fetchData(), onUpdated()]) // データと pending 一覧を再取得する
      if (tutorialActualStep?.id === 'adv-envvar-mount-add') tutorialAdvance() // マウント追加後にチュートリアルを進める
    } catch (addError) {
      console.error(addError)
      toast.error(addError instanceof Error ? addError.message : 'マウント設定の作成に失敗しました') // エラーをトーストで表示する
    } finally {
      setAddingMount(false) // マウント作成中フラグを下げる
    }
  }

  const handleDeleteMount = async (mountId: string) => {
    setDeletingMountId(mountId) // 削除中のマウントIDを設定する
    try {
      await del(`/env-var-mounts/${mountId}`) // マウント設定を削除する
      await Promise.all([fetchData(), onUpdated()]) // データと pending 一覧を再取得する
    } catch (deleteError) {
      console.error(deleteError)
      toast.error(deleteError instanceof Error ? deleteError.message : 'マウント設定の削除に失敗しました') // エラーをトーストで表示する
    } finally {
      setDeletingMountId(null) // 削除中フラグをリセットする
    }
  }

  const handleApplyEnvTemplate = async () => {
    if (!selectedTemplateId) return
    const tmpl = templateList.find(tl => tl.id === selectedTemplateId) // 選択されたテンプレートを取得する
    if (!tmpl || !tmpl.env_vars || tmpl.env_vars.length === 0) return
    setApplyingTemplate(true) // テンプレート適用中フラグを立てる
    setTemplateError('') // エラーをリセットする
    try {
      for (const envVar of tmpl.env_vars) {
        const created = await post<EnvVar>(`/projects/${projectId}/env-vars`, { // プロジェクトに環境変数を作成する
          key: envVar.key,
          value: envVar.value ?? '',
          is_secret: envVar.is_secret,
        })
        if (created) {
          await post(`/deployments/${deploymentId}/env-var-mounts`, { // 作成した環境変数をマウントする
            env_var_id: created.id,
          })
        }
      }
      setSelectedTemplateId('') // 選択をリセットする
      await Promise.all([fetchData(), onUpdated()]) // データを再取得する
    } catch (applyError) {
      console.error(applyError)
      setTemplateError('テンプレートの適用に失敗しました')
    } finally {
      setApplyingTemplate(false) // テンプレート適用中フラグを下げる
    }
  }

  const inputClass = 'w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#00C2D1]/50 focus:border-[#00C2D1] transition-colors'
  const labelClass = 'block text-xs font-medium text-gray-500 mb-1'
  const unmountedEnvVars = envVarList.filter(ev => !mountList.some(mount => mount.env_var_id === ev.id)) // まだマウントされていない環境変数
  const selectedEnvTemplate = templateList.find(tl => tl.id === selectedTemplateId) // 選択中テンプレート

  return (
    <div className="space-y-6 max-w-3xl">
      {/* ── マウント設定 ─── */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <h3 className="text-sm font-semibold text-[#111827]">環境変数マウント</h3>
        <p className="text-xs text-gray-400">Apply を実行すると Kubernetes の container.env に反映されます。</p>

        {/* マウント一覧 */}
        {mountList.length === 0 ? (
          <p className="text-sm text-gray-400">マウント設定がありません</p>
        ) : (
          <div className="space-y-1.5">
            {mountList.map(mount => {
              const mountedEnvVar = envVarList.find(ev => ev.id === mount.env_var_id) // マウント対象の環境変数を取得する
              const effectiveKey = mount.override_key || mountedEnvVar?.key || mount.env_var_id.slice(0, 8) // 実効キーを決定する
              return (
                <div key={mount.id} className="flex items-center justify-between bg-gray-50 rounded-md px-3 py-2 border border-gray-100">
                  <div className="min-w-0 flex items-center gap-2">
                    <span className="font-mono text-sm text-[#111827] truncate">{effectiveKey}</span>
                    {mount.override_key && mountedEnvVar && (
                      <span className="text-xs text-gray-400">← {mountedEnvVar.key}</span>
                    )}
                    {mountedEnvVar?.is_secret && (
                      <span className="text-[10px] bg-purple-50 text-purple-500 px-1.5 py-0.5 rounded">secret</span>
                    )}
                    <span className={`text-[10px] ${mount.status === 'applied' ? 'text-green-500' : mount.status === 'deleting' ? 'text-red-400' : 'text-amber-500'}`}>
                      {mount.status}
                    </span>
                  </div>
                  <button
                    onClick={() => void handleDeleteMount(mount.id)}
                    disabled={deletingMountId === mount.id}
                    className="p-1 rounded hover:bg-red-50 text-gray-300 hover:text-red-400 transition-colors disabled:opacity-50 shrink-0"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              )
            })}
          </div>
        )}

        {/* マウント追加フォーム */}
        <div className="border-t border-gray-100 pt-3 space-y-3">
          <h4 className="text-xs font-semibold text-[#111827]">マウントを追加</h4>
          {envVarList.length === 0 ? (
            <p className="text-sm text-gray-400">マウントするには、まずプロジェクトページで環境変数を作成してください。</p>
          ) : unmountedEnvVars.length === 0 ? (
            <p className="text-sm text-gray-400">すべての環境変数が既にマウントされています。</p>
          ) : (
            <>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className={labelClass}>環境変数</label>
                  <select
                    data-tutorial="tutorial-envvar-mount-select"
                    value={newMountEnvVarId}
                    onChange={ev => {
                      setNewMountEnvVarId(ev.target.value)
                      if (tutorialActualStep?.id === 'adv-envvar-mount-select' && ev.target.value) tutorialAdvance() // 環境変数を選択したらチュートリアルを進める
                    }}
                    className={inputClass}
                  >
                    <option value="">選択してください</option>
                    {unmountedEnvVars.map(ev => (
                      <option key={ev.id} value={ev.id}>
                        {ev.key}{ev.is_secret ? ' (secret)' : ''}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className={labelClass}>コンテナ側のキー名（任意）</label>
                  <input
                    type="text"
                    value={newOverrideKey}
                    onChange={ev => setNewOverrideKey(ev.target.value)}
                    placeholder="空欄なら元のキーを使用"
                    className={`${inputClass} font-mono`}
                  />
                </div>
              </div>
              <button
                data-tutorial="tutorial-envvar-mount-add-btn"
                onClick={() => void handleAddMount()}
                disabled={addingMount || !newMountEnvVarId}
                className="bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
              >
                {addingMount ? '追加中...' : 'マウントを追加'}
              </button>
            </>
          )}
        </div>
      </div>

      {/* ── プロジェクトの環境変数一覧（参照用） ─── */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold text-[#111827]">プロジェクトの環境変数</h3>
          <span className="text-xs text-gray-400">追加・編集はプロジェクトページの左パネルから</span>
        </div>
        {envVarList.length === 0 ? (
          <p className="text-sm text-gray-400">環境変数がありません</p>
        ) : (
          <div className="space-y-1.5">
            {envVarList.map(ev => {
              const isMounted = mountList.some(mount => mount.env_var_id === ev.id) // マウント済みかどうかを確認する
              return (
                <div key={ev.id} className="flex items-center gap-2 bg-gray-50 rounded-md px-3 py-2 border border-gray-100">
                  <span className="font-mono text-sm font-medium text-[#111827] shrink-0">{ev.key}</span>
                  {ev.is_secret && (
                    <span className="text-[10px] bg-purple-50 text-purple-500 px-1.5 py-0.5 rounded shrink-0">secret</span>
                  )}
                  {isMounted && (
                    <span className="text-[10px] bg-blue-50 text-blue-500 px-1.5 py-0.5 rounded shrink-0">マウント済み</span>
                  )}
                  <span className="font-mono text-xs text-gray-400 truncate ml-auto">
                    {ev.value || '(空)'}
                  </span>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* ── テンプレートから環境変数を追加 ─── */}
      {templateList.some(tl => tl.env_vars && tl.env_vars.length > 0) && (
        <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
          <h3 className="text-sm font-semibold text-[#111827]">テンプレートから環境変数を追加</h3>
          <p className="text-xs text-gray-400">テンプレートを選ぶと、定義された環境変数を一括でプロジェクトに追加してマウントします。</p>
          <div className="flex gap-2">
            <select
              value={selectedTemplateId}
              onChange={ev => setSelectedTemplateId(ev.target.value)}
              className={inputClass}
            >
              <option value="">テンプレートを選択...</option>
              {templateList.filter(tl => tl.env_vars && tl.env_vars.length > 0).map(tl => (
                <option key={tl.id} value={tl.id}>{tl.name}</option>
              ))}
            </select>
            <button
              onClick={() => void handleApplyEnvTemplate()}
              disabled={applyingTemplate || !selectedTemplateId}
              className="shrink-0 bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
            >
              {applyingTemplate ? '追加中...' : '追加'}
            </button>
          </div>
          {selectedEnvTemplate?.env_vars && (
            <div className="space-y-1">
              {selectedEnvTemplate.env_vars.map((envVar, envVarIndex) => (
                <div key={envVarIndex} className="flex items-center gap-2 bg-gray-50 rounded-md px-3 py-1.5 border border-gray-100">
                  <span className="font-mono text-xs text-[#111827]">{envVar.key}</span>
                  {envVar.auto_generate ? (
                    <span className="text-[10px] bg-amber-50 text-amber-600 px-1.5 py-0.5 rounded shrink-0">自動生成</span>
                  ) : (
                    <span className="font-mono text-xs text-gray-400 truncate">{envVar.value || '(空)'}</span>
                  )}
                  {envVar.is_secret && (
                    <span className="text-[10px] bg-purple-50 text-purple-500 px-1.5 py-0.5 rounded shrink-0">secret</span>
                  )}
                </div>
              ))}
            </div>
          )}
          {templateError && <p className="text-xs text-red-500">{templateError}</p>}
        </div>
      )}
    </div>
  )
}

// ── Volumes タブ ──────────────────────────────────────────────

function VolumesTab({ deploymentId, projectId, onUpdated }: { deploymentId: string; projectId: string; onUpdated: () => Promise<void> }) {
  const { advance: tutorialAdvance, actualStep: tutorialActualStep } = useTutorialContext() // チュートリアルの進行関数を取得する
  const [volumeList, setVolumeList] = useState<Volume[]>([]) // プロジェクトのボリューム一覧を管理する
  const [mountList, setMountList] = useState<VolumeMount[]>([]) // デプロイメントのマウント設定一覧を管理する
  const [newMountVolumeId, setNewMountVolumeId] = useState('') // マウントするボリュームID
  const [newMountPath, setNewMountPath] = useState('') // マウントパス
  const [addingMount, setAddingMount] = useState(false) // マウント作成中フラグ
  const [deletingMountId, setDeletingMountId] = useState<string | null>(null) // 削除中のマウントID
  const [templateList, setTemplateList] = useState<DeploymentTemplate[]>([]) // テンプレート一覧を管理する
  const [selectedTemplateId, setSelectedTemplateId] = useState('') // 選択中のテンプレートID
  const [applyingTemplate, setApplyingTemplate] = useState(false) // テンプレート適用中フラグ
  const [templateError, setTemplateError] = useState('') // テンプレート適用エラー

  const fetchData = useCallback(async () => {
    const [volumes, mounts] = await Promise.all([
      get<Volume[]>(`/projects/${projectId}/volumes`).catch(() => []), // プロジェクトのボリューム一覧を取得する
      get<VolumeMount[]>(`/deployments/${deploymentId}/volume-mounts`).catch(() => []), // マウント設定一覧を取得する
    ])
    setVolumeList(volumes ?? []) // ボリューム一覧を設定する
    setMountList(mounts ?? []) // マウント設定を設定する
  }, [projectId, deploymentId])

  useEffect(() => {
    void fetchData() // 初回データ取得
    get<DeploymentTemplate[]>('/deployment-templates')
      .then(data => setTemplateList(data ?? []))
      .catch(console.error) // テンプレート一覧を取得する
  }, [fetchData])

  const handleAddMount = async () => {
    if (!newMountVolumeId || !newMountPath) return
    setAddingMount(true) // マウント作成中フラグを立てる
    try {
      await post(`/deployments/${deploymentId}/volume-mounts`, {
        volume_id: newMountVolumeId,
        mount_path: newMountPath,
      })
      setNewMountVolumeId('') // フォームをリセットする
      setNewMountPath('') // パスをリセットする
      await Promise.all([fetchData(), onUpdated()]) // データと pending 一覧を再取得する
      if (tutorialActualStep?.id === 'adv-storage-mount-add') tutorialAdvance() // マウント追加後にチュートリアルを進める
    } catch (addError) {
      console.error(addError)
      toast.error(addError instanceof Error ? addError.message : 'マウント設定の作成に失敗しました') // エラーをトーストで表示する
    } finally {
      setAddingMount(false) // マウント作成中フラグを下げる
    }
  }

  const handleDeleteMount = async (mountId: string) => {
    setDeletingMountId(mountId) // 削除中のマウントIDを設定する
    try {
      await del(`/volume-mounts/${mountId}`) // マウント設定を削除する
      await Promise.all([fetchData(), onUpdated()]) // データと pending 一覧を再取得する
    } catch (deleteError) {
      console.error(deleteError)
      toast.error(deleteError instanceof Error ? deleteError.message : 'マウント設定の削除に失敗しました') // エラーをトーストで表示する
    } finally {
      setDeletingMountId(null) // 削除中フラグをリセットする
    }
  }

  const handleApplyVolumeTemplate = async () => {
    if (!selectedTemplateId) return
    const tmpl = templateList.find(tl => tl.id === selectedTemplateId) // 選択されたテンプレートを取得する
    if (!tmpl || !tmpl.volumes || tmpl.volumes.length === 0) return
    setApplyingTemplate(true) // テンプレート適用中フラグを立てる
    setTemplateError('') // エラーをリセットする
    try {
      for (const volDef of tmpl.volumes) {
        const created = await post<Volume>(`/projects/${projectId}/volumes`, { // プロジェクトにボリュームを作成する
          name: volDef.name,
          size_mb: volDef.size_mb,
        })
        if (created) {
          await post(`/deployments/${deploymentId}/volume-mounts`, { // 作成したボリュームをマウントする
            volume_id: created.id,
            mount_path: volDef.mount_path,
          })
        }
      }
      setSelectedTemplateId('') // 選択をリセットする
      await Promise.all([fetchData(), onUpdated()]) // データを再取得する
    } catch (applyError) {
      console.error(applyError)
      setTemplateError('テンプレートの適用に失敗しました')
    } finally {
      setApplyingTemplate(false) // テンプレート適用中フラグを下げる
    }
  }

  const inputClass = 'w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#00C2D1]/50 focus:border-[#00C2D1] transition-colors'
  const labelClass = 'block text-xs font-medium text-gray-500 mb-1'
  const unmountedVolumes = volumeList.filter(vol => !mountList.some(mount => mount.volume_id === vol.id)) // まだマウントされていないボリューム
  const selectedVolTemplate = templateList.find(tl => tl.id === selectedTemplateId) // 選択中テンプレート

  return (
    <div className="space-y-6 max-w-3xl">
      {/* ── ボリュームマウント設定 ─── */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <h3 className="text-sm font-semibold text-[#111827]">ボリュームマウント</h3>
        <p className="text-xs text-gray-400">Apply を実行すると Kubernetes の volumeMounts に反映されます。</p>

        {/* マウント一覧 */}
        {mountList.length === 0 ? (
          <p className="text-sm text-gray-400">マウント設定がありません</p>
        ) : (
          <div className="space-y-1.5">
            {mountList.map(mount => {
              const mountedVolume = volumeList.find(vol => vol.id === mount.volume_id) // マウント先ボリュームを取得する
              return (
                <div key={mount.id} className="flex items-center justify-between bg-gray-50 rounded-md px-3 py-2 border border-gray-100">
                  <div className="min-w-0 flex items-center gap-2">
                    <span className="font-mono text-sm text-[#111827] truncate">{mount.mount_path}</span>
                    <span className="text-xs text-gray-400">←</span>
                    <span className="text-xs text-gray-500 truncate">{mountedVolume?.name ?? mount.volume_id.slice(0, 8)}</span>
                    <span className={`text-[10px] ${mount.status === 'mounted' ? 'text-green-500' : mount.status === 'deleting' ? 'text-red-400' : 'text-amber-500'}`}>
                      {mount.status}
                    </span>
                  </div>
                  <button
                    onClick={() => void handleDeleteMount(mount.id)}
                    disabled={deletingMountId === mount.id}
                    className="p-1 rounded hover:bg-red-50 text-gray-300 hover:text-red-400 transition-colors disabled:opacity-50 shrink-0"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              )
            })}
          </div>
        )}

        {/* マウント追加フォーム */}
        <div className="border-t border-gray-100 pt-3 space-y-3">
          <h4 className="text-xs font-semibold text-[#111827]">マウントを追加</h4>
          {volumeList.length === 0 ? (
            <p className="text-sm text-gray-400">マウントするには、まず下のセクションでボリュームを作成してください。</p>
          ) : unmountedVolumes.length === 0 ? (
            <p className="text-sm text-gray-400">すべてのボリュームが既にマウントされています。</p>
          ) : (
            <>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className={labelClass}>ボリューム</label>
                  <select
                    data-tutorial="tutorial-volume-mount-select"
                    value={newMountVolumeId}
                    onChange={ev => {
                      setNewMountVolumeId(ev.target.value)
                      if (tutorialActualStep?.id === 'adv-storage-mount-select' && ev.target.value) tutorialAdvance() // ボリュームを選択したらチュートリアルを進める
                    }}
                    className={inputClass}
                  >
                    <option value="">選択してください</option>
                    {unmountedVolumes.map(vol => (
                      <option key={vol.id} value={vol.id}>{vol.name} ({vol.size_mb} MB)</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className={labelClass}>マウントパス</label>
                  <input
                    data-tutorial="tutorial-volume-mount-path"
                    type="text"
                    value={newMountPath}
                    onChange={ev => {
                      setNewMountPath(ev.target.value)
                      if (tutorialActualStep?.id === 'adv-storage-mount-path' && ev.target.value) tutorialAdvance() // パス入力開始でチュートリアルを進める
                    }}
                    placeholder="/data"
                    className={`${inputClass} font-mono`}
                  />
                </div>
              </div>
              <button
                data-tutorial="tutorial-volume-mount-add-btn"
                onClick={() => void handleAddMount()}
                disabled={addingMount || !newMountVolumeId || !newMountPath}
                className="bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
              >
                {addingMount ? '追加中...' : 'マウントを追加'}
              </button>
            </>
          )}
        </div>
      </div>

      {/* ── プロジェクトのボリューム一覧（参照用） ─── */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold text-[#111827]">プロジェクトのボリューム</h3>
          <span className="text-xs text-gray-400">追加はプロジェクトページの「追加」メニューから</span>
        </div>
        {volumeList.length === 0 ? (
          <p className="text-sm text-gray-400">ボリュームがありません</p>
        ) : (
          <div className="space-y-1.5">
            {volumeList.map(vol => {
              const isMounted = mountList.some(mount => mount.volume_id === vol.id) // マウント済みかどうかを確認する
              return (
                <div key={vol.id} className="flex items-center gap-2 bg-gray-50 rounded-md px-3 py-2 border border-gray-100">
                  <span className="text-sm font-medium text-[#111827] truncate">{vol.name}</span>
                  <span className="text-xs text-gray-400">{vol.size_mb} MB</span>
                  <StatusBadge status={vol.status} size="sm" />
                  {isMounted && (
                    <span className="text-[10px] bg-blue-50 text-blue-500 px-1.5 py-0.5 rounded">マウント済み</span>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* ── テンプレートからボリュームを追加 ─── */}
      {templateList.some(tl => tl.volumes && tl.volumes.length > 0) && (
        <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
          <h3 className="text-sm font-semibold text-[#111827]">テンプレートからボリュームを追加</h3>
          <p className="text-xs text-gray-400">テンプレートを選ぶと、定義されたボリュームを一括でプロジェクトに作成してマウントします。</p>
          <div className="flex gap-2">
            <select
              value={selectedTemplateId}
              onChange={ev => setSelectedTemplateId(ev.target.value)}
              className={inputClass}
            >
              <option value="">テンプレートを選択...</option>
              {templateList.filter(tl => tl.volumes && tl.volumes.length > 0).map(tl => (
                <option key={tl.id} value={tl.id}>{tl.name}</option>
              ))}
            </select>
            <button
              onClick={() => void handleApplyVolumeTemplate()}
              disabled={applyingTemplate || !selectedTemplateId}
              className="shrink-0 bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
            >
              {applyingTemplate ? '追加中...' : '追加'}
            </button>
          </div>
          {selectedVolTemplate?.volumes && (
            <div className="space-y-1">
              {selectedVolTemplate.volumes.map((volDef, volIndex) => (
                <div key={volIndex} className="flex items-center gap-2 bg-gray-50 rounded-md px-3 py-1.5 border border-gray-100">
                  <span className="text-xs font-medium text-[#111827]">{volDef.name}</span>
                  <span className="text-xs text-gray-400">{volDef.size_mb} MB</span>
                  <span className="font-mono text-xs text-gray-400">→ {volDef.mount_path}</span>
                </div>
              ))}
            </div>
          )}
          {templateError && <p className="text-xs text-red-500">{templateError}</p>}
        </div>
      )}
    </div>
  )
}

// ── History タブ ──────────────────────────────────────────────

function HistoryTab({ deploymentId }: { deploymentId: string }) {
  const [historyList, setHistoryList] = useState<ApplyHistory[]>([]) // Apply履歴を管理する
  const [expandedId, setExpandedId] = useState<string | null>(null) // 展開中の履歴ID

  useEffect(() => {
    get<ApplyHistory[]>(`/deployments/${deploymentId}/apply-histories`)
      .then((data) => setHistoryList(data ?? []))
      .catch(console.error)
  }, [deploymentId])

  if (historyList.length === 0) {
    return <div className="py-12 text-center text-sm text-gray-400">Apply 履歴がありません</div>
  }

  return (
    <div className="space-y-2">
      {historyList.map((historyItem) => {
        const isExpanded = expandedId === historyItem.id
        const isSuccess = historyItem.status === 'applied'
        return (
          <div
            key={historyItem.id}
            className={`rounded-lg border overflow-hidden ${isSuccess ? 'border-gray-200' : 'border-red-200'}`}
          >
            {/* ヘッダー行 */}
            <button
              className="w-full flex items-center justify-between px-4 py-3 bg-white hover:bg-gray-50 transition-colors text-left"
              onClick={() => setExpandedId(isExpanded ? null : historyItem.id)}
            >
              <div className="flex items-center gap-3">
                <span className={`w-2 h-2 rounded-full shrink-0 ${isSuccess ? 'bg-green-500' : 'bg-red-500'}`} />
                <span className="text-sm text-gray-700">
                  {new Date(historyItem.applied_at).toLocaleString('ja-JP')}
                </span>
                <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${isSuccess ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-600'}`}>
                  {isSuccess ? '成功' : '失敗'}
                </span>
              </div>
              <span className="font-mono text-xs text-gray-400">{historyItem.id.slice(0, 8)}</span>
            </button>

            {/* 展開コンテンツ */}
            {isExpanded && (
              <div className="border-t border-gray-100 bg-gray-50 px-4 py-3 space-y-3">
                {/* エラーメッセージ */}
                {historyItem.error_message && (
                  <div className="bg-red-50 border border-red-200 rounded-md px-3 py-2 text-sm text-red-700 font-mono whitespace-pre-wrap">
                    {historyItem.error_message}
                  </div>
                )}

                {/* manifest スナップショット */}
                {historyItem.manifests && (
                  <div>
                    <p className="text-xs font-medium text-gray-500 mb-1">適用された Manifest</p>
                    <pre className="text-xs font-mono bg-[#111827] text-gray-200 rounded-md p-3 overflow-x-auto whitespace-pre-wrap max-h-96">
                      {JSON.stringify(historyItem.manifests, null, 2)}
                    </pre>
                  </div>
                )}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

function NotInitScreen({
  deployment,
  deploymentId,
  projectId,
  onDelete,
  deleting,
}: {
  deployment: Deployment
  deploymentId: string
  projectId: string
  onDelete: () => void
  deleting: boolean
}) {
  const navigate = useNavigate()
  const [building, setBuilding] = useState(false)

  const handleBuild = async () => {
    setBuilding(true)
    try {
      const result = await post<Build>(`/deployments/${deploymentId}/build`) // ビルドを開始する
      navigate(`/builds/${result.id}/logs`) // ビルドログページへ遷移する
    } catch (buildError) {
      console.error(buildError)
      toast.error(buildError instanceof Error ? buildError.message : 'ビルドの開始に失敗しました') // エラーをトーストで表示する
    } finally {
      setBuilding(false)
    }
  }

  return (
    <Layout
      breadcrumbs={[
        { label: 'Project', href: `/projects/${projectId}` },
        { label: deployment.name },
      ]}
    >
      <div className="h-96 flex flex-col items-center justify-center gap-6">
        <div className="w-16 h-16 rounded-full bg-purple-50 flex items-center justify-center">
          <Hammer className="w-8 h-8 text-purple-400" />
        </div>
        <div className="text-center">
          <p className="text-lg font-semibold text-[#111827]">初回ビルドが必要です</p>
          <p className="text-sm text-gray-400 mt-2 max-w-sm">
            このデプロイメントはまだビルドが実行されていません。<br />
            ビルドを実行すると、デプロイ可能な状態になります。
          </p>
        </div>
        <div className="flex items-center gap-3">
          {deployment.current_build_id && (
            <Link
              to={`/builds/${deployment.current_build_id}/logs`}
              className="flex items-center gap-2 border border-purple-300 text-purple-600 hover:bg-purple-50 text-sm px-4 py-2 rounded-md transition-colors"
            >
              <ExternalLink className="w-4 h-4" />
              ビルドログを表示
            </Link>
          )}
          <button
            onClick={() => void handleBuild()}
            disabled={building}
            className="flex items-center gap-2 bg-purple-600 hover:bg-purple-700 text-white text-sm px-4 py-2 rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Hammer className="w-4 h-4" />
            {building ? 'ビルド開始中...' : deployment.current_build_id ? '再ビルド' : 'ビルドを実行'}
          </button>
          <button
            onClick={onDelete}
            disabled={deleting}
            className="flex items-center gap-2 border border-red-200 text-red-500 hover:bg-red-50 text-sm px-4 py-2 rounded-md transition-colors disabled:opacity-50"
          >
            <Trash2 className="w-4 h-4" />
            {deleting ? '削除中...' : '削除'}
          </button>
        </div>
      </div>
    </Layout>
  )
}

// ── Metrics タブ ──────────────────────────────────────────────

const METRICS_POLL_INTERVAL = 30_000  // 30 秒ごとにポーリングする（バックエンドの収集間隔に合わせる）

// 表示範囲の選択肢（30 秒ポーリング換算の limit と表示ラベルを保持する）
const METRICS_RANGE_OPTIONS = [
  { label: '1時間',  limit: 120  },  // 30秒×120 = 1時間分
  { label: '3時間',  limit: 360  },  // 30秒×360 = 3時間分
  { label: '24時間', limit: 2880 },  // 30秒×2880 = 24時間分
  { label: '3日',    limit: 8640 },  // 30秒×8640 = 3日分（バックエンドの保持期間上限）
] as const

type MetricsRangeOption = typeof METRICS_RANGE_OPTIONS[number]

function MetricsTab({ deploymentId }: { deploymentId: string }) {
  const [metrics, setMetrics] = useState<DeploymentMetrics[]>([])                    // メトリクス一覧を管理する
  const [metricsLoading, setMetricsLoading] = useState(true)                         // 初回ローディング状態を管理する
  const [metricsError, setMetricsError] = useState<string | null>(null)              // エラーメッセージを管理する
  const [selectedRange, setSelectedRange] = useState<MetricsRangeOption>(METRICS_RANGE_OPTIONS[0])  // 選択中の表示範囲を管理する

  const fetchMetrics = useCallback(async () => {
    try {
      const response = await get<{ metrics: DeploymentMetrics[] }>(
        `/deployments/${deploymentId}/metrics?limit=${selectedRange.limit}`
      )  // 選択範囲の limit でメトリクスを取得する
      setMetrics(response.metrics ?? [])
      setMetricsError(null)  // 取得成功時はエラーをクリアする
    } catch (fetchError) {
      console.error(fetchError)
      setMetricsError('メトリクスの取得に失敗しました')
    } finally {
      setMetricsLoading(false)
    }
  }, [deploymentId, selectedRange])

  useEffect(() => {
    setMetricsLoading(true)                                                              // 範囲切り替え時はローディングをリセットする
    void fetchMetrics()                                                                  // 範囲変更時に即時再取得する
    const intervalId = setInterval(() => { void fetchMetrics() }, METRICS_POLL_INTERVAL) // 定期的にポーリングする
    return () => clearInterval(intervalId)                                               // クリーンアップ
  }, [fetchMetrics])

  // 表示期間に応じてダウンサンプリングしたメトリクスを生成する（グラフ描画用）
  const displayMetrics = downsampleMetrics(metrics, selectedRange.label)

  return (
    <div className="space-y-3">
      {/* ヘッダー: 範囲セレクタ + ダウンロードボタン + 更新ボタン */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-1">
          {METRICS_RANGE_OPTIONS.map((rangeOption) => (
            <button
              key={rangeOption.label}
              onClick={() => setSelectedRange(rangeOption)}  // 表示範囲を切り替える
              className={`text-xs px-3 py-1 rounded-full transition-colors ${
                selectedRange.label === rangeOption.label
                  ? 'bg-[#111827] text-white'
                  : 'text-gray-500 hover:text-gray-700 hover:bg-gray-100'
              }`}
            >
              {rangeOption.label}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-2">
          <MetricsDownloadButton metrics={metrics} deploymentId={deploymentId} />  {/* 生データ（間引きなし）をダウンロードする */}
          <button
            onClick={() => void fetchMetrics()}
            className="text-xs text-gray-400 hover:text-gray-600 underline"
          >
            更新
          </button>
        </div>
      </div>

      {/* コンテンツエリア */}
      {metricsLoading ? (  // 初回ローディング中はスケルトンを表示する
        <div className="h-48 flex items-center justify-center text-sm text-gray-400">
          読み込み中...
        </div>
      ) : metricsError ? (  // エラー時はエラーメッセージを表示する
        <div className="h-48 flex items-center justify-center text-sm text-red-400">
          {metricsError}
        </div>
      ) : (
        <MetricsCharts metrics={displayMetrics} />  // ダウンサンプリング済みデータをグラフに渡す
      )}
    </div>
  )
}

// ── Webhook タブ ──────────────────────────────────────────────

function WebhookTab({ deploymentId, deploymentType }: { deploymentId: string; deploymentType: string }) {
  const [webhook, setWebhook] = useState<Webhook | null>(null) // Webhook 情報を管理する
  const [loading, setLoading] = useState(true) // ローディング状態を管理する
  const [creating, setCreating] = useState(false) // 作成中フラグ
  const [deleting, setDeleting] = useState(false) // 削除中フラグ
  const [secretVisible, setSecretVisible] = useState(false) // シークレット表示フラグ

  const baseUrl = window.location.origin // ベース URL を取得する

  const fetchWebhook = useCallback(async () => {
    try {
      const data = await get<Webhook>(`/deployments/${deploymentId}/webhooks`) // Webhook を取得する
      setWebhook(data)
    } catch (fetchError) {
      if (fetchError instanceof ApiError && fetchError.status === 404) {
        setWebhook(null) // 未作成の場合は null にする
      } else {
        console.error(fetchError)
      }
    } finally {
      setLoading(false)
    }
  }, [deploymentId])

  useEffect(() => {
    void fetchWebhook() // 初回取得
  }, [fetchWebhook])

  const handleCreate = async () => {
    setCreating(true)
    try {
      const data = await post<Webhook>(`/deployments/${deploymentId}/webhooks`, { github_repo_url: '' }) // Webhook を作成する
      setWebhook(data)
      toast.success('Webhook を作成しました')
    } catch (createError) {
      console.error(createError)
      toast.error(createError instanceof Error ? createError.message : 'Webhook の作成に失敗しました')
    } finally {
      setCreating(false)
    }
  }

  const handleDelete = async () => {
    if (!webhook) return
    setDeleting(true)
    try {
      await del(`/webhooks/${webhook.id}`) // Webhook を削除する
      setWebhook(null)
      toast.success('Webhook を削除しました')
    } catch (deleteError) {
      console.error(deleteError)
      toast.error(deleteError instanceof Error ? deleteError.message : 'Webhook の削除に失敗しました')
    } finally {
      setDeleting(false)
    }
  }

  const copyToClipboard = (text: string, label: string) => {
    void navigator.clipboard.writeText(text).then(() => {
      toast.success(`${label}をコピーしました`)
    })
  }

  if (loading) {
    return (
      <div className="h-48 flex items-center justify-center text-sm text-gray-400">読み込み中...</div>
    )
  }

  if (!webhook) {
    return (
      <div className="p-6">
        <div className="max-w-xl">
          <div className="flex items-center gap-3 mb-4">
            <div className="w-10 h-10 rounded-lg bg-[#00C2D1]/10 flex items-center justify-center">
              <WebhookIcon className="w-5 h-5 text-[#00C2D1]" />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-[#111827]">Webhook</h3>
              <p className="text-xs text-gray-500">GitHub Actions からビルド・Deploy を自動化する</p>
            </div>
          </div>
          <p className="text-sm text-gray-600 mb-6">
            Webhook を作成すると、デプロイメント固有のシークレットが発行されます。<br />
            GitHub Actions からそのシークレットを使ってビルドと Apply を呼び出せます。
          </p>
          <button
            onClick={() => void handleCreate()}
            disabled={creating}
            className="px-4 py-2 text-sm font-medium bg-[#00C2D1] text-white rounded-md hover:bg-[#00aab8] disabled:opacity-50 transition-colors"
          >
            {creating ? '作成中...' : 'Webhook を作成する'}
          </button>
        </div>
      </div>
    )
  }

  const isImageType = deploymentType === 'image_url' // image_url タイプかどうかを判定する

  const updateImageEndpoint = `${baseUrl}/webhooks/${deploymentId}/update-image`
  const buildEndpoint = `${baseUrl}/webhooks/${deploymentId}/build`
  const buildStatusEndpoint = `${baseUrl}/webhooks/${deploymentId}/builds/{build_id}`
  const applyEndpoint = `${baseUrl}/webhooks/${deploymentId}/apply`

  const endpointList = isImageType
    ? [{ label: 'イメージ更新 & Apply', method: 'POST', url: updateImageEndpoint }]
    : [
        { label: 'ビルドトリガー', method: 'POST', url: buildEndpoint },
        { label: 'ビルド状態確認', method: 'GET', url: buildStatusEndpoint },
        { label: 'Apply', method: 'POST', url: applyEndpoint },
      ]

  const actionsExample = isImageType
    ? `- name: Deploy
  run: |
    curl -s -X POST \\
      -H "X-Webhook-Secret: \${{ secrets.DEPLOY_SECRET }}" \\
      -H "Content-Type: application/json" \\
      -d '{"image_url": "your-registry/image:\${{ github.sha }}"}' \\
      ${updateImageEndpoint}`
    : `- name: Build
  run: |
    RESULT=$(curl -s -X POST \\
      -H "X-Webhook-Secret: \${{ secrets.DEPLOY_SECRET }}" \\
      -H "Content-Type: application/json" \\
      -d '{"commit_message": "\${{ github.event.head_commit.message }}", "author": "\${{ github.actor }}"}' \\
      ${buildEndpoint})
    echo "BUILD_ID=$(echo \$RESULT | jq -r '.id')" >> \$GITHUB_ENV

- name: Wait for build
  run: |
    for i in \$(seq 1 60); do
      STATUS=$(curl -s -H "X-Webhook-Secret: \${{ secrets.DEPLOY_SECRET }}" \\
        ${baseUrl}/webhooks/${deploymentId}/builds/\$BUILD_ID | jq -r '.status')
      echo "status: \$STATUS"
      if [ "\$STATUS" = "succeeded" ]; then exit 0; fi
      if [ "\$STATUS" = "failed" ] || [ "\$STATUS" = "cancelled" ]; then exit 1; fi
      sleep 10
    done
    exit 1

- name: Apply
  run: |
    curl -s -X POST \\
      -H "X-Webhook-Secret: \${{ secrets.DEPLOY_SECRET }}" \\
      ${applyEndpoint}`

  return (
    <div className="p-6 space-y-6">
      {/* シークレット */}
      <div className="border border-gray-200 rounded-lg p-4">
        <div className="flex items-center justify-between mb-3">
          <h4 className="text-sm font-semibold text-[#111827]">シークレット</h4>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setSecretVisible(prev => !prev)}
              className="text-xs text-gray-500 hover:text-gray-700 flex items-center gap-1"
            >
              {secretVisible ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
              {secretVisible ? '隠す' : '表示'}
            </button>
            <button
              onClick={() => copyToClipboard(webhook.secret, 'シークレット')}
              className="text-xs text-gray-500 hover:text-gray-700 flex items-center gap-1"
            >
              <Copy className="w-3.5 h-3.5" />
              コピー
            </button>
          </div>
        </div>
        <code className="block text-xs font-mono bg-gray-50 border border-gray-200 rounded px-3 py-2 text-gray-800 break-all">
          {secretVisible ? webhook.secret : '•'.repeat(32)}
        </code>
        <p className="text-xs text-gray-400 mt-2">GitHub Actions の Secrets に <code className="bg-gray-100 px-1 rounded">DEPLOY_SECRET</code> として登録してください。</p>
      </div>

      {/* エンドポイント一覧 */}
      <div className="border border-gray-200 rounded-lg p-4">
        <h4 className="text-sm font-semibold text-[#111827] mb-3">エンドポイント</h4>
        <div className="space-y-3">
          {endpointList.map(({ label, method, url }) => (
            <div key={label} className="flex items-center gap-2">
              <span className={`text-xs font-mono font-bold px-1.5 py-0.5 rounded ${method === 'GET' ? 'bg-green-100 text-green-700' : 'bg-blue-100 text-blue-700'}`}>
                {method}
              </span>
              <code className="flex-1 text-xs font-mono text-gray-700 truncate">{url}</code>
              <button
                onClick={() => copyToClipboard(url, label)}
                className="text-gray-400 hover:text-gray-600 flex-shrink-0"
              >
                <Copy className="w-3.5 h-3.5" />
              </button>
            </div>
          ))}
        </div>
        <p className="text-xs text-gray-400 mt-3">全エンドポイントに <code className="bg-gray-100 px-1 rounded">X-Webhook-Secret</code> ヘッダーでシークレットを付与してください。</p>
      </div>

      {/* GitHub Actions 使用例 */}
      <div className="border border-gray-200 rounded-lg p-4">
        <div className="flex items-center justify-between mb-3">
          <h4 className="text-sm font-semibold text-[#111827]">GitHub Actions 使用例</h4>
          <button
            onClick={() => copyToClipboard(actionsExample, 'GitHub Actions 使用例')}
            className="text-xs text-gray-500 hover:text-gray-700 flex items-center gap-1"
          >
            <Copy className="w-3.5 h-3.5" />
            コピー
          </button>
        </div>
        <pre className="text-xs font-mono bg-[#111827] text-gray-200 rounded-md p-3 overflow-x-auto whitespace-pre">
          {actionsExample}
        </pre>
      </div>

      {/* 削除 */}
      <div className="border border-red-100 rounded-lg p-4">
        <h4 className="text-sm font-semibold text-red-700 mb-2">Webhook を削除する</h4>
        <p className="text-xs text-gray-500 mb-3">削除するとシークレットが無効になります。再作成すると新しいシークレットが発行されます。</p>
        <button
          onClick={() => void handleDelete()}
          disabled={deleting}
          className="px-4 py-2 text-sm font-medium bg-red-50 text-red-600 border border-red-200 rounded-md hover:bg-red-100 disabled:opacity-50 transition-colors"
        >
          {deleting ? '削除中...' : 'Webhook を削除する'}
        </button>
      </div>
    </div>
  )
}

