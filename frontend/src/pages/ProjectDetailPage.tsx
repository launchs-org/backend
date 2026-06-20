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
import { Plus, Trash2, X } from 'lucide-react'
import { Layout } from '@/components/Layout'
import { DeploymentNode } from '@/components/flow/DeploymentNode'
import { ServiceNode } from '@/components/flow/ServiceNode'
import { IngressNode } from '@/components/flow/IngressNode'
import { get, post, del } from '@/lib/api'
import { StatusBadge } from '@/components/StatusBadge'
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

const MIN_SIDEBAR_WIDTH = 280 // サイドバー最小幅
const MAX_SIDEBAR_WIDTH = 1200 // サイドバー最大幅
const DEFAULT_SIDEBAR_WIDTH = 760 // サイドバーデフォルト幅

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
  const [ingressLoaded, setIngressLoaded] = useState(false) // IngressRoute読み込み完了フラグ
  const [creatingIngress, setCreatingIngress] = useState(false) // IngressRoute作成中フラグ
  const [deletingIngress, setDeletingIngress] = useState(false) // IngressRoute削除中フラグ
  const [selectedDeploymentId, setSelectedDeploymentId] = useState<string | null>(null) // 選択中のデプロイメントIDを管理する
  const [iframeLoaded, setIframeLoaded] = useState(false) // iframe読み込み完了フラグ
  const [sidebarWidth, setSidebarWidth] = useState(DEFAULT_SIDEBAR_WIDTH) // サイドバー幅を管理する
  const resizingRef = useRef(false) // リサイズ中フラグ
  const startXRef = useRef(0) // リサイズ開始X座標
  const startWidthRef = useRef(DEFAULT_SIDEBAR_WIDTH) // リサイズ開始時の幅

  const buildGraph = useCallback((relations: DeploymentWithRelations[], pid: string) => {
    const newNodes: Node[] = []
    const newEdges: Edge[] = []

    const ROW_HEIGHT = 200 // 行の高さを定義する
    const COL_WIDTH = 300 // 列の幅を定義する

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
          onSelect: (deploymentId: string) => { setSelectedDeploymentId(deploymentId); setIframeLoaded(false) }, // ノードクリック時にサイドバーを開く
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
      }

    })

    setNodes(newNodes) // ノードを更新する
    setEdges(newEdges) // エッジを更新する
  }, [setNodes, setEdges])

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
          return {
            deployment,
            service: serviceResult,
          } as DeploymentWithRelations
        })
      )

      setDeploymentRelations(relations) // デプロイメント関連リソースを更新する
      buildGraph(relations, projectId) // グラフを更新する

      // プロジェクト単位のIngressRouteを取得する
      const ingressResult = await get<IngressRoute>(`/projects/${projectId}/ingress-route`).catch(() => null) // IngressRouteを取得する
      setIngressRoute(ingressResult) // IngressRoute情報を設定する
      if (ingressResult) {
        const pathRuleResult = await get<PathRule[]>(`/ingress-routes/${ingressResult.id}/path-rules`).catch(() => []) // パスルール一覧を取得する
        setPathRules(pathRuleResult ?? []) // パスルールを設定する
      } else {
        setPathRules([]) // パスルールを空にする
      }
      setIngressLoaded(true) // IngressRoute読み込み完了を記録する
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

  // サイドバーリサイズのマウスイベントを登録する
  useEffect(() => {
    const handleMouseMove = (mouseEvent: MouseEvent) => {
      if (!resizingRef.current) return
      const delta = startXRef.current - mouseEvent.clientX // 左方向ドラッグで幅を増やす
      const newWidth = Math.min(MAX_SIDEBAR_WIDTH, Math.max(MIN_SIDEBAR_WIDTH, startWidthRef.current + delta))
      setSidebarWidth(newWidth)
    }

    const handleMouseUp = () => {
      resizingRef.current = false
      document.body.style.cursor = '' // カーソルをリセットする
      document.body.style.userSelect = '' // テキスト選択を再有効化する
    }

    window.addEventListener('mousemove', handleMouseMove)
    window.addEventListener('mouseup', handleMouseUp)

    return () => {
      window.removeEventListener('mousemove', handleMouseMove)
      window.removeEventListener('mouseup', handleMouseUp)
    }
  }, [])

  const handleResizeStart = (mouseEvent: React.MouseEvent) => {
    resizingRef.current = true
    startXRef.current = mouseEvent.clientX // リサイズ開始位置を記録する
    startWidthRef.current = sidebarWidth
    document.body.style.cursor = 'col-resize' // リサイズカーソルを設定する
    document.body.style.userSelect = 'none' // ドラッグ中のテキスト選択を無効化する
  }

  const handleCreateIngressRoute = async () => {
    if (!projectId) return
    setCreatingIngress(true) // 作成中フラグを立てる
    try {
      await post(`/projects/${projectId}/ingress-route`) // IngressRouteを作成する
      await fetchData() // データを再取得する
    } catch (createError) {
      console.error(createError)
      alert('IngressRouteの作成に失敗しました')
    } finally {
      setCreatingIngress(false) // 作成中フラグを下げる
    }
  }

  const handleDeleteIngressRoute = async () => {
    if (!projectId || !ingressRoute) return
    if (!confirm('IngressRouteを削除しますか？Apply後にk8sから削除されます。')) return
    setDeletingIngress(true) // 削除中フラグを立てる
    try {
      await del(`/projects/${projectId}/ingress-route`) // IngressRouteを削除する
      await fetchData() // データを再取得する
    } catch (deleteError) {
      console.error(deleteError)
      alert('IngressRouteの削除に失敗しました')
    } finally {
      setDeletingIngress(false) // 削除中フラグを下げる
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

  // 選択中のデプロイメントのiframe URLを生成する
  const sidebarIframeUrl = selectedDeploymentId
    ? `/ui/projects/${projectId}/deployments/${selectedDeploymentId}`
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
          <button
            onClick={() => navigate(`/projects/${projectId}/deployments/new`)}
            className="flex items-center gap-1.5 bg-[#111827] text-white text-sm px-3 py-1.5 rounded-md hover:bg-gray-800 transition-colors"
          >
            <Plus className="w-3.5 h-3.5" />
            デプロイ
          </button>
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
            {/* ReactFlow 全画面 */}
            <div className="flex-1 min-w-0 relative">
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

              {/* IngressRoute オーバーレイパネル */}
              {ingressLoaded && (
                <div className="absolute top-3 right-3 z-10 bg-white rounded-lg border border-gray-200 shadow-sm p-3 w-72">
                  <h3 className="text-xs font-semibold text-[#111827] mb-2">IngressRoute</h3>
                  {ingressRoute ? (
                    <div className="space-y-1 text-xs">
                      <div className="flex items-center gap-1">
                        <span className="text-gray-400">ホスト:</span>
                        <a href={`http://${ingressRoute.host}`} target="_blank" rel="noopener noreferrer" className="font-mono text-[#00C2D1] hover:underline truncate">
                          {ingressRoute.host}
                        </a>
                      </div>
                      <div className="flex items-center gap-1">
                        <span className="text-gray-400">ステータス:</span>
                        <StatusBadge status={ingressRoute.status} size="sm" />
                      </div>
                      {pathRules.length > 0 && (
                        <div className="pt-1 border-t border-gray-100 mt-1">
                          <p className="text-gray-400 mb-1">パスルール ({pathRules.length})</p>
                          {pathRules.map(pathRule => (
                            <div key={pathRule.id} className="font-mono text-xs text-gray-600 flex items-center justify-between">
                              <span>{pathRule.path_prefix}</span>
                              <span className="text-gray-300 text-[10px]">{pathRule.status}</span>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  ) : (
                    <div className="space-y-2">
                      <p className="text-xs text-gray-400">IngressRouteが未設定です</p>
                      <button
                        onClick={() => void handleCreateIngressRoute()}
                        disabled={creatingIngress}
                        className="w-full bg-[#111827] text-white text-xs px-3 py-1.5 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
                      >
                        {creatingIngress ? '作成中...' : 'IngressRouteを作成'}
                      </button>
                    </div>
                  )}
                  {ingressRoute && (
                    <div className="pt-2 border-t border-gray-100 mt-2 space-y-2">
                      <IngressPathRuleForm
                        ingressRouteId={ingressRoute.id}
                        deploymentRelations={deploymentRelations}
                        onCreated={() => void fetchData()}
                      />
                      {ingressRoute.status !== 'deleting' && (
                        <button
                          onClick={() => void handleDeleteIngressRoute()}
                          disabled={deletingIngress}
                          className="w-full text-xs text-red-500 hover:text-red-700 disabled:opacity-50 py-1"
                        >
                          {deletingIngress ? '削除中...' : 'IngressRouteを削除'}
                        </button>
                      )}
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* リサイズハンドル */}
            {selectedDeploymentId && (
              <div
                onMouseDown={handleResizeStart}
                className="w-1 bg-gray-200 hover:bg-[#00C2D1] cursor-col-resize transition-colors shrink-0"
              />
            )}

            {/* サイドバー */}
            {selectedDeploymentId && sidebarIframeUrl && (
              <div
                className="shrink-0 flex flex-col border-l border-gray-200"
                style={{ width: sidebarWidth }}
              >
                {/* サイドバーヘッダー */}
                <div className="h-10 flex items-center justify-between px-3 border-b border-gray-100 bg-gray-50 shrink-0">
                  <span className="text-xs font-medium text-gray-500">
                    {deploymentRelations.find(rel => rel.deployment.id === selectedDeploymentId)?.deployment.name ?? 'デプロイメント詳細'}
                  </span>
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => navigate(sidebarIframeUrl)}
                      className="text-xs text-[#00C2D1] hover:underline px-1"
                    >
                      全画面で開く
                    </button>
                    <button
                      onClick={() => setSelectedDeploymentId(null)}
                      className="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
                {/* iframe */}
                <div className="flex-1 relative">
                  {!iframeLoaded && (
                    <div className="absolute inset-0 flex items-center justify-center bg-white z-10">
                      <div className="w-6 h-6 border-2 border-[#00C2D1] border-t-transparent rounded-full animate-spin" />
                    </div>
                  )}
                  <iframe
                    key={selectedDeploymentId}
                    src={sidebarIframeUrl}
                    className="w-full h-full border-none"
                    title="デプロイメント詳細"
                    onLoad={() => setIframeLoaded(true)}
                  />
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </Layout>
  )
}

// ── IngressPathRuleForm ────────────────────────────────────────

function IngressPathRuleForm({
  ingressRouteId,
  deploymentRelations,
  onCreated,
}: {
  ingressRouteId: string
  deploymentRelations: DeploymentWithRelations[]
  onCreated: () => void
}) {
  const [pathPrefix, setPathPrefix] = useState('/') // パスプレフィックスを管理する
  const [serviceId, setServiceId] = useState('') // 対象サービスIDを管理する
  const [saving, setSaving] = useState(false) // 保存中フラグ

  const servicesWithId = deploymentRelations
    .filter(rel => rel.service)
    .map(rel => ({ id: rel.service!.id, name: rel.deployment.name })) // サービスが存在するデプロイメントを抽出する

  const handleAdd = async () => {
    if (!serviceId) return
    setSaving(true) // 保存中フラグを立てる
    try {
      await post(`/ingress-routes/${ingressRouteId}/path-rules`, {
        path_prefix: pathPrefix,
        service_id: serviceId,
      }) // パスルールを追加する
      setPathPrefix('/') // フォームをリセットする
      setServiceId('') // 選択をリセットする
      onCreated() // 親コンポーネントにデータ再取得を通知する
    } catch (saveError) {
      console.error(saveError)
      alert('パスルールの追加に失敗しました')
    } finally {
      setSaving(false) // 保存中フラグを下げる
    }
  }

  if (servicesWithId.length === 0) {
    return <p className="text-xs text-gray-400">パスルールを追加するにはまず Service を設定してください</p>
  }

  return (
    <div className="space-y-1.5">
      <p className="text-xs font-medium text-gray-500">パスルールを追加</p>
      <input
        type="text"
        value={pathPrefix}
        onChange={ev => setPathPrefix(ev.target.value)}
        placeholder="/"
        className="w-full rounded border border-gray-200 px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-[#00C2D1]"
      />
      <select
        value={serviceId}
        onChange={ev => setServiceId(ev.target.value)}
        className="w-full rounded border border-gray-200 px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-[#00C2D1]"
      >
        <option value="">対象 Service を選択</option>
        {servicesWithId.map(svc => (
          <option key={svc.id} value={svc.id}>{svc.name}</option>
        ))}
      </select>
      <button
        onClick={() => void handleAdd()}
        disabled={saving || !serviceId}
        className="w-full bg-[#00C2D1] text-white text-xs px-3 py-1.5 rounded-md hover:bg-[#00b3c0] transition-colors disabled:opacity-50"
      >
        {saving ? '追加中...' : '追加'}
      </button>
    </div>
  )
}
