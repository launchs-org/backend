import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  MarkerType,
  type Node,
  type Edge,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { Plus, Trash2, X, Globe, Copy, Check, ExternalLink, Play } from 'lucide-react'
import { Layout } from '@/components/Layout'
import { StatusBadge } from '@/components/StatusBadge'
import { DeploymentNode } from '@/components/flow/DeploymentNode'
import { ServiceNode } from '@/components/flow/ServiceNode'
import { IngressNode } from '@/components/flow/IngressNode'
import { get, post, del } from '@/lib/api'
import type { Project, Deployment, K8sService, IngressRoute, PathRule } from '@/lib/types'

const NODE_TYPES = {
  deployment: DeploymentNode,
  service: ServiceNode,
  ingress: IngressNode,
} // カスタムノードタイプを定義する

const EDGE_STYLE = {
  stroke: '#E5E7EB',
  strokeWidth: 2,
} // エッジのスタイルを定義する

const EDGE_MARKER = {
  type: MarkerType.ArrowClosed,
  color: '#9CA3AF',
} // エッジの矢印マーカーを定義する

type DeploymentWithRelations = {
  deployment: Deployment
  service: K8sService | null
}

type SidebarMode = 'deployment' | 'ingress' | null // サイドバーの表示モードを定義する

export function ProjectDetailPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const navigate = useNavigate()
  const [project, setProject] = useState<Project | null>(null) // プロジェクト情報を管理する
  const [deploymentRelations, setDeploymentRelations] = useState<DeploymentWithRelations[]>([]) // デプロイメントとその関連リソースを管理する
  const [loading, setLoading] = useState(true) // ローディング状態を管理する
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]) // ReactFlowのノードを管理する
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]) // ReactFlowのエッジを管理する
  const [deletingProject, setDeletingProject] = useState(false) // プロジェクト削除中フラグ
  const [ingressRoute, setIngressRoute] = useState<IngressRoute | null>(null) // プロジェクト単位のIngressRouteを管理する
  const [pathRules, setPathRules] = useState<PathRule[]>([]) // IngressRouteのパスルール一覧を管理する
  const [creatingIngress, setCreatingIngress] = useState(false) // IngressRoute作成中フラグ
  const [sidebarMode, setSidebarMode] = useState<SidebarMode>(null) // サイドバーの表示モードを管理する
  const [selectedDeploymentId, setSelectedDeploymentId] = useState<string | null>(null) // 選択中のデプロイメントIDを管理する
  const [iframeLoaded, setIframeLoaded] = useState(false) // iframe読み込み完了フラグ
  const [showAddMenu, setShowAddMenu] = useState(false) // 追加メニューの表示フラグ
  const [sidebarWidth, setSidebarWidth] = useState(420) // サイドバー幅（px）
  const isDragging = useRef(false) // ドラッグ中フラグ
  const dragStartX = useRef(0) // ドラッグ開始X座標
  const dragStartWidth = useRef(420) // ドラッグ開始時の幅

  const openIngressSidebar = useCallback(() => {
    setSidebarMode('ingress') // IngressRoute サイドバーを開く
    setSelectedDeploymentId(null) // デプロイメント選択をリセットする
  }, [])

  const buildGraph = useCallback((
    relations: DeploymentWithRelations[],
    pid: string,
    currentIngressRoute: IngressRoute | null,
    currentPathRules: PathRule[],
  ) => {
    const newNodes: Node[] = []
    const newEdges: Edge[] = []

    const ROW_HEIGHT = 200 // 行の高さを定義する
    const COL_WIDTH = 300 // 列の幅を定義する
    const INGRESS_COL = COL_WIDTH * 2 // IngressRoute ノードのX座標

    relations.forEach((relation, relationIndex) => {
      const baseY = relationIndex * ROW_HEIGHT // Y座標を計算する

      // デプロイメントノードを追加する
      newNodes.push({
        id: `dep-${relation.deployment.id}`,
        type: 'deployment',
        position: { x: 0, y: baseY },
        data: {
          deployment: relation.deployment,
          projectId: pid,
          onSelect: (deploymentId: string) => {
            setSelectedDeploymentId(deploymentId)
            setSidebarMode('deployment')
            setIframeLoaded(false)
          }, // ノードクリック時にデプロイメントサイドバーを開く
        },
      })

      // port が 0 の場合は未設定扱いとしてノードを表示しない
      const serviceConfigured = relation.service && (relation.service.port !== 0 || relation.service.pending_port !== 0)

      if (serviceConfigured && relation.service) {
        // サービスノードを追加する
        newNodes.push({
          id: `svc-${relation.service.id}`,
          type: 'service',
          position: { x: COL_WIDTH, y: baseY + 20 },
          data: { service: relation.service },
        })

        // デプロイメント → サービスのエッジを追加する
        newEdges.push({
          id: `edge-dep-svc-${relation.deployment.id}`,
          source: `dep-${relation.deployment.id}`,
          target: `svc-${relation.service.id}`,
          style: EDGE_STYLE,
          markerEnd: EDGE_MARKER,
        })

        // IngressRoute が存在する場合 Service → IngressRoute のエッジを追加する
        if (currentIngressRoute) {
          newEdges.push({
            id: `edge-svc-ing-${relation.service.id}`,
            source: `svc-${relation.service.id}`,
            target: 'ingress-node',
            style: EDGE_STYLE,
            markerEnd: EDGE_MARKER,
          })
        }
      }
    })

    // IngressRoute ノードをグラフ右側中央に1つ配置する
    if (currentIngressRoute) {
      const ingressY = Math.max(0, ((relations.length - 1) * ROW_HEIGHT) / 2) // 縦方向中央に配置する
      newNodes.push({
        id: 'ingress-node',
        type: 'ingress',
        position: { x: INGRESS_COL, y: ingressY },
        data: {
          ingress: currentIngressRoute,
          pathRules: currentPathRules,
          onSelect: openIngressSidebar, // ノードクリック時にIngressサイドバーを開く
        },
      })
    }

    setNodes(newNodes) // ノードを更新する
    setEdges(newEdges) // エッジを更新する
  }, [setNodes, setEdges, openIngressSidebar])

  const fetchData = useCallback(async () => {
    if (!projectId) return

    try {
      const [projectData, deploymentList] = await Promise.all([
        get<Project>(`/projects/${projectId}`), // プロジェクト情報を取得する
        get<Deployment[]>(`/projects/${projectId}/deployments`), // デプロイメント一覧を取得する
      ])

      setProject(projectData)

      // 各デプロイメントのサービスを並行取得する
      const relations = await Promise.all(
        (deploymentList ?? []).map(async (deployment) => {
          const serviceResult = await get<K8sService>(`/deployments/${deployment.id}/service`).catch(() => null) // サービス情報を取得する
          return { deployment, service: serviceResult } as DeploymentWithRelations
        })
      )

      setDeploymentRelations(relations) // デプロイメント関連リソースを更新する

      // プロジェクト単位のIngressRouteを取得する
      const ingressResult = await get<IngressRoute>(`/projects/${projectId}/ingress-route`).catch(() => null) // IngressRouteを取得する
      setIngressRoute(ingressResult) // IngressRoute情報を設定する

      let currentPathRules: PathRule[] = []
      if (ingressResult) {
        currentPathRules = await get<PathRule[]>(`/ingress-routes/${ingressResult.id}/path-rules`).catch(() => []) ?? [] // パスルール一覧を取得する
        setPathRules(currentPathRules) // パスルールを設定する
      } else {
        setPathRules([]) // パスルールを空にする
      }

      buildGraph(relations, projectId, ingressResult, currentPathRules) // グラフを更新する
    } catch (fetchError) {
      console.error(fetchError)
    } finally {
      setLoading(false)
    }
  }, [projectId, buildGraph])

  useEffect(() => {
    void fetchData() // 初回データ取得

    const intervalId = setInterval(() => {
      void fetchData() // 10秒ごとにポーリングする
    }, 10_000)

    return () => clearInterval(intervalId) // クリーンアップ
  }, [fetchData])

  useEffect(() => {
    const onMouseMove = (ev: MouseEvent) => {
      if (!isDragging.current) return
      const delta = dragStartX.current - ev.clientX // 左へドラッグすると幅が増える
      const next = Math.min(800, Math.max(280, dragStartWidth.current + delta))
      setSidebarWidth(next)
    }
    const onMouseUp = () => {
      if (!isDragging.current) return
      isDragging.current = false
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }
    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', onMouseUp)
    return () => {
      window.removeEventListener('mousemove', onMouseMove)
      window.removeEventListener('mouseup', onMouseUp)
    }
  }, [])

  const handleDragStart = (ev: React.MouseEvent) => {
    isDragging.current = true
    dragStartX.current = ev.clientX
    dragStartWidth.current = sidebarWidth
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
  }

  const handleCreateIngressRoute = async () => {
    if (!projectId) return
    setCreatingIngress(true) // 作成中フラグを立てる
    try {
      await post(`/projects/${projectId}/ingress-route`) // IngressRouteを作成する
      await fetchData() // データを再取得する
      setSidebarMode('ingress') // 作成後にIngressサイドバーを開く
    } catch (createError) {
      console.error(createError)
      alert('IngressRouteの作成に失敗しました')
    } finally {
      setCreatingIngress(false) // 作成中フラグを下げる
    }
  }

  const handleDeleteProject = async () => {
    if (!projectId || !project) return
    if (!confirm(`プロジェクト「${project.name}」を削除しますか？この操作は取り消せません。`)) return

    setDeletingProject(true)
    try {
      await del(`/projects/${projectId}`) // プロジェクトを削除する
      navigate('/') // ダッシュボードへ遷移する
    } catch (deleteError) {
      console.error(deleteError)
      alert('プロジェクトの削除に失敗しました')
    } finally {
      setDeletingProject(false)
    }
  }

  const handleCloseSidebar = () => {
    setSidebarMode(null)
    setSelectedDeploymentId(null)
  }

  const sidebarOpen = sidebarMode !== null // サイドバーが開いているかどうかを確認する

  const sidebarIframeSrc = sidebarMode === 'deployment' && selectedDeploymentId && projectId
    ? `/ui/projects/${projectId}/deployments/${selectedDeploymentId}`
    : null
  const sidebarNavigatePath = sidebarMode === 'deployment' && selectedDeploymentId && projectId
    ? `/projects/${projectId}/deployments/${selectedDeploymentId}`
    : null

  if (loading) {
    return (
      <Layout>
        <div className="h-96 flex items-center justify-center">
          <div className="text-sm text-gray-400">読み込み中...</div>
        </div>
      </Layout>
    )
  }

  return (
    <Layout
      fullWidth
      breadcrumbs={[{ label: project?.name ?? '', sub: project?.namespace }]}
      actions={
        <div className="flex items-center gap-2">
          {/* 追加メニュー */}
          <div className="relative">
            <button
              onClick={() => setShowAddMenu(prev => !prev)}
              className="flex items-center gap-1.5 bg-[#111827] text-white text-sm px-3 py-1.5 rounded-md hover:bg-gray-800 transition-colors"
            >
              <Plus className="w-3.5 h-3.5" />
              追加
            </button>
            {showAddMenu && (
              <>
                {/* 背景クリックで閉じる */}
                <div className="fixed inset-0 z-10" onClick={() => setShowAddMenu(false)} />
                <div className="absolute right-0 top-full mt-1 z-20 bg-white border border-gray-200 rounded-lg shadow-lg overflow-hidden w-48">
                  <button
                    onClick={() => { setShowAddMenu(false); navigate(`/projects/${projectId}/deployments/new`) }}
                    className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-[#111827] hover:bg-gray-50 transition-colors"
                  >
                    <Plus className="w-3.5 h-3.5 text-gray-400" />
                    Deployment
                  </button>
                  <button
                    onClick={() => {
                      setShowAddMenu(false)
                      if (ingressRoute) {
                        setSidebarMode('ingress') // 既存の IngressRoute がある場合はサイドバーを開く
                      } else {
                        void handleCreateIngressRoute() // 未作成の場合は作成する
                      }
                    }}
                    disabled={creatingIngress}
                    className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-[#111827] hover:bg-gray-50 transition-colors disabled:opacity-50"
                  >
                    <Globe className="w-3.5 h-3.5 text-gray-400" />
                    {creatingIngress ? 'IngressRoute作成中...' : 'IngressRoute'}
                  </button>
                </div>
              </>
            )}
          </div>

          <button
            onClick={() => void handleDeleteProject()}
            disabled={deletingProject}
            className="flex items-center gap-1.5 text-red-500 text-sm px-3 py-1.5 rounded-md hover:bg-red-50 border border-red-200 transition-colors disabled:opacity-50"
          >
            <Trash2 className="w-3.5 h-3.5" />
            {deletingProject ? '削除中...' : '削除'}
          </button>
        </div>
      }
    >
      <div>
        {/* デプロイメントがない場合の空状態 */}
        {deploymentRelations.length === 0 ? (
          <div className="text-center py-20 bg-white rounded-lg border border-dashed border-gray-200">
            <p className="text-sm font-medium text-gray-500 mb-1">まだデプロイメントがありません</p>
            <p className="text-xs text-gray-400 mb-4">最初のアプリケーションをデプロイしましょう</p>
            <button
              onClick={() => navigate(`/projects/${projectId}/deployments/new`)}
              className="inline-flex items-center gap-1.5 bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors"
            >
              <Plus className="w-3.5 h-3.5" />
              デプロイ
            </button>
          </div>
        ) : (
          /* ReactFlow グラフ + サイドバー */
          <div className="flex overflow-hidden bg-white" style={{ height: 'calc(100vh - 48px)' }}>
            {/* ReactFlow */}
            <div className="flex-1 min-w-0">
              <ReactFlow
                nodes={nodes}
                edges={edges}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                nodeTypes={NODE_TYPES}
                fitView
                fitViewOptions={{ padding: 0.2 }}
                proOptions={{ hideAttribution: true }}
              >
                <Background color="#E5E7EB" gap={16} />
                <Controls />
                <MiniMap
                  nodeColor={(node) => {
                    if (node.type === 'deployment') return '#00C2D1'
                    if (node.type === 'service') return '#3B82F6'
                    return '#7C3AED'
                  }}
                />
              </ReactFlow>
            </div>

            {/* サイドバーが開いているときのみリサイズハンドルとサイドバーを表示する */}
            {sidebarOpen && (
              <>
                {/* リサイズハンドル */}
                <div
                  onMouseDown={handleDragStart}
                  className="w-1 shrink-0 bg-gray-200 hover:bg-[#00C2D1] cursor-col-resize transition-colors"
                />

                {/* サイドバー */}
                <div className="shrink-0 flex flex-col border-l border-gray-200" style={{ width: sidebarWidth }}>
                  {/* Deployment 詳細（iframe） */}
                  {sidebarMode === 'deployment' && sidebarIframeSrc && (
                    <>
                      <div className="h-10 flex items-center justify-between px-3 border-b border-gray-100 bg-gray-50 shrink-0">
                        <span className="text-xs font-medium text-gray-500">
                          {deploymentRelations.find(rel => rel.deployment.id === selectedDeploymentId)?.deployment.name ?? 'デプロイメント詳細'}
                        </span>
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => sidebarNavigatePath && navigate(sidebarNavigatePath)}
                            className="text-xs text-[#00C2D1] hover:underline px-1"
                          >
                            全画面で開く
                          </button>
                          <button
                            onClick={handleCloseSidebar}
                            className="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors"
                          >
                            <X className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </div>
                      <div className="flex-1 relative">
                        {!iframeLoaded && (
                          <div className="absolute inset-0 flex items-center justify-center bg-white z-10">
                            <div className="w-6 h-6 border-2 border-[#00C2D1] border-t-transparent rounded-full animate-spin" />
                          </div>
                        )}
                        <iframe
                          key={selectedDeploymentId}
                          src={sidebarIframeSrc}
                          className="w-full h-full border-none"
                          title="デプロイメント詳細"
                          onLoad={() => setIframeLoaded(true)}
                        />
                      </div>
                    </>
                  )}

                  {/* IngressRoute サイドバー */}
                  {sidebarMode === 'ingress' && ingressRoute && (
                    <IngressRouteSidebar
                      projectId={projectId!}
                      ingressRoute={ingressRoute}
                      pathRules={pathRules}
                      deploymentRelations={deploymentRelations}
                      onRefresh={fetchData}
                      onClose={handleCloseSidebar}
                    />
                  )}
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </Layout>
  )
}

// ── IngressRouteSidebar ───────────────────────────────────────

type IngressTab = 'overview' | 'paths'

function IngressRouteSidebar({
  projectId,
  ingressRoute,
  pathRules,
  deploymentRelations,
  onRefresh,
  onClose,
}: {
  projectId: string
  ingressRoute: IngressRoute
  pathRules: PathRule[]
  deploymentRelations: DeploymentWithRelations[]
  onRefresh: () => Promise<void>
  onClose: () => void
}) {
  const [activeTab, setActiveTab] = useState<IngressTab>('overview') // アクティブなタブを管理する
  const [applying, setApplying] = useState(false) // apply 中フラグ
  const [deleting, setDeleting] = useState(false) // 削除中フラグ

  const hasPending = pathRules.some(pr => pr.status === 'pending' || pr.status === 'deleting') // 保留中の変更があるかどうか

  const handleApply = async () => {
    setApplying(true) // apply 中フラグを立てる
    try {
      await post(`/projects/${projectId}/apply`) // IngressRoute を k8s に apply する
      await onRefresh() // データを再取得する
    } catch (applyError) {
      console.error(applyError)
      alert('Apply に失敗しました')
    } finally {
      setApplying(false) // apply 中フラグを下げる
    }
  }

  const handleDelete = async () => {
    if (!confirm('IngressRoute を削除しますか？k8s からも即時削除されます。')) return
    setDeleting(true) // 削除中フラグを立てる
    try {
      await del(`/projects/${projectId}/ingress-route`) // IngressRoute を deleting 状態にする
      await post(`/projects/${projectId}/apply`) // Apply して k8s から削除・DB レコードも物理削除する
      await onRefresh() // データを再取得する
      onClose() // サイドバーを閉じる
    } catch (deleteError) {
      console.error(deleteError)
      alert('IngressRoute の削除に失敗しました')
    } finally {
      setDeleting(false) // 削除中フラグを下げる
    }
  }

  return (
    <div className="flex flex-col h-full bg-white">
      {/* ヘッダー：DeploymentDetailPage のアクションバーと同じ構成 */}
      <div className="h-10 flex items-center justify-between px-3 border-b border-gray-100 bg-gray-50 shrink-0">
        <div className="flex items-center gap-2 min-w-0">
          <Globe className="w-3.5 h-3.5 text-purple-500 shrink-0" />
          <span className="text-xs font-medium text-[#111827] truncate">IngressRoute</span>
          <StatusBadge status={ingressRoute.status} />
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <button
            onClick={() => void handleApply()}
            disabled={applying || ingressRoute.status === 'deleting'}
            className={`flex items-center gap-1 text-xs px-2.5 py-1 rounded transition-colors disabled:opacity-50 ${
              hasPending
                ? 'bg-amber-500 hover:bg-amber-600 text-white'
                : 'bg-[#111827] hover:bg-gray-800 text-white'
            }`}
          >
            <Play className="w-3 h-3" />
            {applying ? 'Apply中...' : 'Apply'}
          </button>
          <button
            onClick={() => void handleDelete()}
            disabled={deleting || ingressRoute.status === 'deleting'}
            className="flex items-center gap-1 text-xs px-2 py-1 rounded text-red-500 hover:bg-red-50 border border-red-200 transition-colors disabled:opacity-50"
          >
            <Trash2 className="w-3 h-3" />
            {deleting ? '削除中...' : '削除'}
          </button>
          <button
            onClick={onClose}
            className="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* 保留中バナー */}
      {hasPending && (
        <div className="bg-amber-50 border-b border-amber-200 px-4 py-2.5 flex items-center justify-between">
          <div className="flex items-center gap-2 text-xs text-amber-800">
            <span className="w-1.5 h-1.5 rounded-full bg-amber-500 shrink-0" />
            設定変更が保留中です。Apply を実行すると Kubernetes に反映されます。
          </div>
          <button
            onClick={() => void handleApply()}
            disabled={applying}
            className="text-xs font-medium text-amber-700 hover:text-amber-900 underline shrink-0 ml-2"
          >
            今すぐ Apply
          </button>
        </div>
      )}

      {/* タブナビゲーション */}
      <div className="border-b border-gray-200 shrink-0">
        <nav className="flex gap-0">
          {(['overview', 'paths'] as IngressTab[]).map(tab => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${
                activeTab === tab
                  ? 'border-[#00C2D1] text-[#00C2D1]'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              {{ overview: '概要', paths: 'パスルール' }[tab]}
            </button>
          ))}
        </nav>
      </div>

      {/* タブコンテンツ */}
      <div className="flex-1 overflow-y-auto">
        {activeTab === 'overview' && (
          <IngressOverviewTab ingressRoute={ingressRoute} />
        )}
        {activeTab === 'paths' && (
          <IngressPathsTab
            ingressRoute={ingressRoute}
            pathRules={pathRules}
            deploymentRelations={deploymentRelations}
            onRefresh={onRefresh}
          />
        )}
      </div>
    </div>
  )
}

// ── 概要タブ ──────────────────────────────────────────────────

function IngressOverviewTab({ ingressRoute }: { ingressRoute: IngressRoute }) {
  const [copied, setCopied] = useState(false) // コピー完了フラグ

  const handleCopyHost = async () => {
    await navigator.clipboard.writeText(ingressRoute.host) // ホスト名をクリップボードにコピーする
    setCopied(true)
    setTimeout(() => setCopied(false), 2000) // 2秒後にリセットする
  }

  return (
    <div className="p-4 space-y-4">
      <div className="space-y-3">
        <Row label="ステータス"><StatusBadge status={ingressRoute.status} /></Row>
        <Row label="ホスト">
          <div className="flex items-center gap-1 min-w-0">
            <span className="font-mono text-sm text-[#111827] truncate">{ingressRoute.host}</span>
            <button
              onClick={() => void handleCopyHost()}
              className="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors shrink-0"
              title="コピー"
            >
              {copied ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
            <a
              href={`http://${ingressRoute.host}`}
              target="_blank"
              rel="noopener noreferrer"
              className="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors shrink-0"
              title="ブラウザで開く"
            >
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
          </div>
        </Row>
        <Row label="作成日時"><span className="text-sm text-gray-600">{new Date(ingressRoute.created_at).toLocaleString('ja-JP')}</span></Row>
      </div>
    </div>
  )
}

// ── パスルールタブ ────────────────────────────────────────────

function IngressPathsTab({
  ingressRoute,
  pathRules,
  deploymentRelations,
  onRefresh,
}: {
  ingressRoute: IngressRoute
  pathRules: PathRule[]
  deploymentRelations: DeploymentWithRelations[]
  onRefresh: () => Promise<void>
}) {
  const [deletingPathRuleId, setDeletingPathRuleId] = useState<string | null>(null) // 削除中のパスルールID
  const [pathPrefix, setPathPrefix] = useState('/') // 追加するパスプレフィックス
  const [serviceId, setServiceId] = useState('') // 追加する転送先サービスID
  const [addingPathRule, setAddingPathRule] = useState(false) // パスルール追加中フラグ

  const servicesWithLabel = deploymentRelations
    .filter(rel => rel.service)
    .map(rel => ({ id: rel.service!.id, label: rel.deployment.name })) // Service が設定済みのデプロイメントを抽出する

  const handleDeletePathRule = async (pathRuleId: string) => {
    setDeletingPathRuleId(pathRuleId) // 削除中のパスルールIDを設定する
    try {
      await del(`/ingress-routes/${ingressRoute.id}/path-rules/${pathRuleId}`) // パスルールを削除する
      await onRefresh() // データを再取得する
    } catch (deleteError) {
      console.error(deleteError)
      alert('パスルールの削除に失敗しました')
    } finally {
      setDeletingPathRuleId(null) // 削除中フラグをリセットする
    }
  }

  const handleAddPathRule = async () => {
    if (!serviceId || !pathPrefix) return
    setAddingPathRule(true) // 追加中フラグを立てる
    try {
      await post(`/ingress-routes/${ingressRoute.id}/path-rules`, {
        path_prefix: pathPrefix,
        service_id: serviceId,
      }) // パスルールを追加する
      setPathPrefix('/') // フォームをリセットする
      setServiceId('') // 選択をリセットする
      await onRefresh() // データを再取得する
    } catch (addError) {
      console.error(addError)
      alert('パスルールの追加に失敗しました')
    } finally {
      setAddingPathRule(false) // 追加中フラグを下げる
    }
  }

  const inputClass = 'w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#00C2D1]/50 focus:border-[#00C2D1] transition-colors'
  const labelClass = 'block text-xs font-medium text-gray-500 mb-1'

  return (
    <div className="p-4 space-y-4">
      {/* パスルール一覧 */}
      {pathRules.length === 0 ? (
        <p className="text-sm text-gray-400">パスルールがありません</p>
      ) : (
        <div className="space-y-1.5">
          {pathRules.map(pathRule => {
            const targetDeployment = deploymentRelations.find(rel => rel.service?.id === pathRule.service_id) // 対象デプロイメントを取得する
            return (
              <div key={pathRule.id} className="flex items-center justify-between bg-gray-50 rounded-md px-3 py-2 border border-gray-100">
                <div className="min-w-0">
                  <span className="font-mono text-sm text-[#111827]">{pathRule.path_prefix}</span>
                  {targetDeployment && (
                    <span className="text-xs text-gray-400 ml-2">→ {targetDeployment.deployment.name}</span>
                  )}
                  <span className={`ml-2 text-[10px] ${pathRule.status === 'active' ? 'text-green-500' : pathRule.status === 'deleting' ? 'text-red-400' : 'text-amber-500'}`}>
                    {pathRule.status}
                  </span>
                </div>
                <button
                  onClick={() => void handleDeletePathRule(pathRule.id)}
                  disabled={deletingPathRuleId === pathRule.id}
                  className="p-1 rounded hover:bg-red-50 text-gray-300 hover:text-red-400 transition-colors disabled:opacity-50 shrink-0"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              </div>
            )
          })}
        </div>
      )}

      {/* パスルール追加フォーム */}
      <div className="border-t border-gray-100 pt-4 space-y-3">
        <h3 className="text-sm font-semibold text-[#111827]">パスルールを追加</h3>
        {servicesWithLabel.length === 0 ? (
          <p className="text-sm text-gray-400">パスルールを追加するには、Deployment の Networking タブで Service を設定してください。</p>
        ) : (
          <>
            <div>
              <label className={labelClass}>パスプレフィックス</label>
              <input
                type="text"
                value={pathPrefix}
                onChange={ev => setPathPrefix(ev.target.value)}
                placeholder="/"
                className={`${inputClass} font-mono`}
              />
            </div>
            <div>
              <label className={labelClass}>転送先 Service</label>
              <select
                value={serviceId}
                onChange={ev => setServiceId(ev.target.value)}
                className={inputClass}
              >
                <option value="">デプロイメントを選択</option>
                {servicesWithLabel.map(svc => (
                  <option key={svc.id} value={svc.id}>{svc.label}</option>
                ))}
              </select>
            </div>
            <button
              onClick={() => void handleAddPathRule()}
              disabled={addingPathRule || !serviceId || !pathPrefix}
              className="bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
            >
              {addingPathRule ? '追加中...' : '追加'}
            </button>
          </>
        )}
      </div>
    </div>
  )
}

// ── 共通ユーティリティ ────────────────────────────────────────

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 text-sm">
      <span className="text-gray-400 shrink-0">{label}</span>
      <span className="text-right">{children}</span>
    </div>
  )
}
