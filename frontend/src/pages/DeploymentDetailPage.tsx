import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { Play, Trash2, GitBranch, Container, Package, ExternalLink } from 'lucide-react'
import { Layout } from '@/components/Layout'
import { StatusBadge } from '@/components/StatusBadge'
import { LogViewer } from '@/components/LogViewer'
import { get, post, put, del } from '@/lib/api'
import type {
  Deployment,
  Build,
  K8sService,
  ApplyHistory,
  PodLogsResponse,
} from '@/lib/types'

type Tab = 'overview' | 'logs' | 'builds' | 'settings' | 'networking' | 'history'

const TYPE_ICON = {
  image_url: Container,
  dockerfile: GitBranch,
  railpack: Package,
}

export function DeploymentDetailPage() {
  const { projectId, deploymentId } = useParams<{ projectId: string; deploymentId: string }>()
  const navigate = useNavigate()

  const [deployment, setDeployment] = useState<Deployment | null>(null) // デプロイメント情報を管理する
  const [activeTab, setActiveTab] = useState<Tab>('overview') // アクティブなタブを管理する
  const [loading, setLoading] = useState(true) // ローディング状態を管理する
  const [applying, setApplying] = useState(false) // Apply中フラグ
  const [deleting, setDeleting] = useState(false) // 削除中フラグ

  const fetchDeployment = useCallback(async () => {
    if (!deploymentId) return
    try {
      const data = await get<Deployment>(`/deployments/${deploymentId}`) // デプロイメント情報を取得する
      setDeployment(data)
    } catch (fetchError) {
      console.error(fetchError)
    } finally {
      setLoading(false)
    }
  }, [deploymentId])

  useEffect(() => {
    void fetchDeployment() // 初回データ取得

    const intervalId = setInterval(() => {
      void fetchDeployment() // 10秒ごとにポーリングする
    }, 10_000)

    return () => clearInterval(intervalId) // クリーンアップ
  }, [fetchDeployment])

  const hasPending = deployment && !!(
    deployment.pending_image_url ||
    deployment.pending_github_repo_url ||
    deployment.pending_github_branch ||
    deployment.pending_replicas ||
    deployment.pending_instance_size ||
    deployment.pending_command?.length ||
    deployment.pending_dockerfile_path
  ) // 保留中の変更があるかどうかを確認する

  const handleApply = async () => {
    if (!deploymentId) return
    setApplying(true)
    try {
      await post(`/deployments/${deploymentId}/apply`) // Applyを実行する
      await fetchDeployment() // デプロイメント情報を再取得する
    } catch (applyError) {
      console.error(applyError)
      alert('Apply に失敗しました')
    } finally {
      setApplying(false)
    }
  }

  const handleDelete = async () => {
    if (!deploymentId || !deployment) return
    if (!confirm(`「${deployment.name}」を削除しますか？この操作は取り消せません。`)) return

    setDeleting(true)
    try {
      await del(`/deployments/${deploymentId}`) // デプロイメントを削除する
      navigate(`/projects/${projectId}`) // プロジェクト詳細へ遷移する
    } catch (deleteError) {
      console.error(deleteError)
      alert('削除に失敗しました')
    } finally {
      setDeleting(false)
    }
  }

  // type によって表示するタブを決定する
  const availableTabs: Tab[] = deployment
    ? [
        'overview',
        'logs',
        ...(deployment.type !== 'image_url' ? (['builds'] as Tab[]) : []),
        'settings',
        'networking',
        'history',
      ]
    : ['overview']

  if (loading) {
    return (
      <Layout>
        <div className="h-48 flex items-center justify-center text-sm text-gray-400">読み込み中...</div>
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
          <div className="bg-amber-50 border border-amber-200 rounded-lg px-4 py-3 flex items-center justify-between">
            <div className="flex items-center gap-2 text-sm text-amber-800">
              <span className="w-2 h-2 rounded-full bg-amber-500 shrink-0" />
              設定変更が保留中です。Apply を実行すると Kubernetes リソースに反映されます。
            </div>
            <button
              onClick={() => void handleApply()}
              disabled={applying}
              className="text-xs font-medium text-amber-700 hover:text-amber-900 underline"
            >
              今すぐ Apply
            </button>
          </div>
        )}

        {/* タブナビゲーション */}
        <div className="border-b border-gray-200">
          <nav className="flex gap-0">
            {availableTabs.map((tab) => (
              <button
                key={tab}
                onClick={() => setActiveTab(tab)}
                className={`px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === tab
                    ? 'border-[#00C2D1] text-[#00C2D1]'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                {{ overview: '概要', logs: 'ログ', builds: 'ビルド', settings: '設定', networking: 'ネットワーク', history: '履歴' }[tab]}
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
          {activeTab === 'networking' && <NetworkingTab deploymentId={deploymentId!} />}
          {activeTab === 'history' && <HistoryTab deploymentId={deploymentId!} />}
        </div>
      </div>
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
  const fetchPodLogs = useCallback(async (since?: string) => {
    const params: Record<string, string> = {}
    if (since) params.since = since // since パラメータを設定する
    const result = await get<PodLogsResponse>(`/deployments/${deploymentId}/logs`, params)
    return { logs: result.logs ?? '', lastTimestamp: result.last_timestamp }
  }, [deploymentId])

  return (
    <div style={{ height: 'calc(100vh - 220px)' }}>
      <LogViewer
        fetchLogs={fetchPodLogs}
        title={`Pod Logs — ${deploymentId}`}
        pollInterval={5_000}
        initialLive={true}
      />
    </div>
  )
}

// ── Builds タブ ───────────────────────────────────────────────

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

  const handleBuild = async () => {
    setBuilding(true)
    try {
      const result = await post<Build>(`/deployments/${deploymentId}/build`) // ビルドを開始する
      navigate(`/builds/${result.id}/logs`) // ビルドログページへ遷移する
    } catch (buildError) {
      console.error(buildError)
      alert('ビルドの開始に失敗しました')
    } finally {
      setBuilding(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <button
          onClick={() => void handleBuild()}
          disabled={building}
          className="flex items-center gap-1.5 bg-[#111827] text-white text-sm px-3 py-1.5 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
        >
          {building ? 'ビルド開始中...' : 'ビルド'}
        </button>
      </div>

      {deployment.current_build_id ? (
        <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
          <div className="px-4 py-3 border-b border-gray-100 text-xs font-medium text-gray-400 uppercase tracking-wider">
            最新ビルド
          </div>
          <Link
            to={`/builds/${deployment.current_build_id}/logs`}
            className="block px-4 py-3 hover:bg-gray-50 transition-colors"
          >
            <div className="flex items-center justify-between">
              <span className="font-mono text-sm text-[#111827]">
                #{deployment.current_build_id.slice(0, 8)}
              </span>
              <ExternalLink className="w-3.5 h-3.5 text-gray-400" />
            </div>
            <p className="text-xs text-gray-400 mt-1">クリックしてビルドログを表示</p>
          </Link>
        </div>
      ) : (
        <div className="text-center py-12 bg-white rounded-lg border border-dashed border-gray-200">
          <p className="text-sm text-gray-400">ビルド履歴がありません</p>
        </div>
      )}
    </div>
  )
}

// ── Settings タブ ─────────────────────────────────────────────

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
        body.github_repo_url = formData.github_repo_url // GitHub リポジトリURLを設定する
        body.github_branch = formData.github_branch // ブランチを設定する
        body.github_commit_sha = formData.github_commit_sha // コミットSHAを設定する
        body.github_repo_directory = formData.github_repo_directory // ディレクトリを設定する
        if (deployment.type === 'dockerfile') {
          body.dockerfile_path = formData.dockerfile_path // Dockerfileのパスを設定する
        }
      }

      await put(`/deployments/${deployment.id}`, body) // デプロイメントを更新する
      await onSaved() // 保存後にデプロイメント情報を再取得する
    } catch (saveError) {
      console.error(saveError)
      alert('保存に失敗しました')
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
            <div>
              <label className={labelClass}>GitHubリポジトリURL</label>
              <input
                type="text"
                value={formData.github_repo_url}
                onChange={(event) => setFormData((prev) => ({ ...prev, github_repo_url: event.target.value }))}
                placeholder="https://github.com/org/repo"
                className={inputClass}
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className={labelClass}>ブランチ</label>
                <input
                  type="text"
                  value={formData.github_branch}
                  onChange={(event) => setFormData((prev) => ({ ...prev, github_branch: event.target.value }))}
                  placeholder="main"
                  className={inputClass}
                />
              </div>
              <div>
                <label className={labelClass}>コミットSHA</label>
                <input
                  type="text"
                  value={formData.github_commit_sha}
                  onChange={(event) => setFormData((prev) => ({ ...prev, github_commit_sha: event.target.value }))}
                  placeholder="HEAD"
                  className={inputClass}
                />
              </div>
            </div>
            <div>
              <label className={labelClass}>ビルドディレクトリ</label>
              <input
                type="text"
                value={formData.github_repo_directory}
                onChange={(event) => setFormData((prev) => ({ ...prev, github_repo_directory: event.target.value }))}
                placeholder="./"
                className={inputClass}
              />
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
              min={0}
              max={10}
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
              <option value="small">small</option>
              <option value="medium">medium</option>
              <option value="large">large</option>
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

function NetworkingTab({ deploymentId }: { deploymentId: string }) {
  const [service, setService] = useState<K8sService | null>(null) // サービス情報を管理する
  const [svcForm, setSvcForm] = useState({ port: '', target_port: '' }) // サービス設定フォーム
  const [savingSvc, setSavingSvc] = useState(false) // サービス保存中フラグ
  const [deletingSvc, setDeletingSvc] = useState(false) // サービス削除中フラグ

  const fetchNetworking = async () => {
    const svcResult = await get<K8sService>(`/deployments/${deploymentId}/service`).catch(() => null) // サービス情報を取得する
    setService(svcResult) // サービス情報を設定する
  }

  useEffect(() => {
    void fetchNetworking()
    const intervalId = setInterval(() => { void fetchNetworking() }, 10_000)
    return () => clearInterval(intervalId) // クリーンアップ
  }, [deploymentId])

  // port と pending_port が両方 0 の場合のみ「未設定」とする
  const serviceConfigured = service && (service.port !== 0 || service.pending_port !== 0)
  // 無効化が保留中（port は設定済みだが pending_port=0 に変更された）
  const serviceDisablePending = service && service.port !== 0 && service.pending_port === 0

  const handleDeleteService = async () => {
    if (!confirm('Serviceを無効化しますか？保留中になり、Applyで反映されます。')) return
    setDeletingSvc(true)
    try {
      await del(`/deployments/${deploymentId}/service`) // pending_port=0 にして無効化を予約する
      await fetchNetworking() // 最新状態を再取得する
    } catch (deleteError) {
      console.error(deleteError)
      alert('Serviceの無効化に失敗しました')
    } finally {
      setDeletingSvc(false)
    }
  }

  const handleSaveService = async () => {
    setSavingSvc(true)
    try {
      await put(`/deployments/${deploymentId}/service`, {
        port: parseInt(svcForm.port, 10),
        target_port: parseInt(svcForm.target_port, 10),
      })
      await fetchNetworking()
    } catch (saveError) {
      console.error(saveError)
      alert('サービスの保存に失敗しました')
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
                <input type="number" className={inputClass} placeholder="80" value={svcForm.port} onChange={ev => setSvcForm(prev => ({ ...prev, port: ev.target.value }))} />
              </div>
              <div>
                <label className={labelClass}>ターゲットポート（コンテナ）</label>
                <input type="number" className={inputClass} placeholder="8080" value={svcForm.target_port} onChange={ev => setSvcForm(prev => ({ ...prev, target_port: ev.target.value }))} />
              </div>
            </div>
            <button onClick={() => void handleSaveService()} disabled={savingSvc || !svcForm.port || !svcForm.target_port} className="bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50">
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

// ── History タブ ──────────────────────────────────────────────

function HistoryTab({ deploymentId }: { deploymentId: string }) {
  const [historyList, setHistoryList] = useState<ApplyHistory[]>([]) // Apply履歴を管理する

  useEffect(() => {
    get<ApplyHistory[]>(`/deployments/${deploymentId}/apply-histories`)
      .then((data) => setHistoryList(data ?? []))
      .catch(console.error)
  }, [deploymentId])

  return (
    <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
      {historyList.length === 0 ? (
        <div className="py-12 text-center text-sm text-gray-400">Apply 履歴がありません</div>
      ) : (
        <table className="w-full text-sm">
          <thead className="bg-gray-50 border-b border-gray-100">
            <tr>
              <th className="px-4 py-2 text-left text-xs font-medium text-gray-400">Apply日時</th>
              <th className="px-4 py-2 text-left text-xs font-medium text-gray-400">ID</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-50">
            {historyList.map((historyItem) => (
              <tr key={historyItem.id} className="hover:bg-gray-50">
                <td className="px-4 py-2.5 text-gray-600">
                  {new Date(historyItem.applied_at).toLocaleString('ja-JP')}
                </td>
                <td className="px-4 py-2.5 font-mono text-gray-400 text-xs">
                  {historyItem.id.slice(0, 12)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
