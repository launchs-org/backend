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
import { get, del } from '@/lib/api'
import type { Project, Deployment, K8sService, IngressRoute } from '@/lib/types'

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
  ingress: IngressRoute | null
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

      if (relation.ingress) {
        // イングレスノードを追加する（Service の有無に関わらず表示する）
        newNodes.push({
          id: `ing-${relation.ingress.id}`,
          type: 'ingress',
          position: { x: COL_WIDTH * 2, y: baseY },
          data: { ingress: relation.ingress },
        })

        // Service が設定済みの場合は Service → Ingress、未設定の場合は Deployment → Ingress のエッジを追加する
        const ingressEdgeSource = serviceConfigured && relation.service
          ? `svc-${relation.service.id}`
          : `dep-${relation.deployment.id}`
        newEdges.push({
          id: `edge-src-ing-${relation.ingress.id}`,
          source: ingressEdgeSource,
          target: `ing-${relation.ingress.id}`,
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

      // 各デプロイメントのサービスとイングレスを並行取得する
      const relations = await Promise.all(
        (deploymentList ?? []).map(async (deployment) => {
          const [serviceData, ingressData] = await Promise.allSettled([
            get<K8sService>(`/deployments/${deployment.id}/service`),
            get<IngressRoute>(`/deployments/${deployment.id}/ingress-route`),
          ])

          return {
            deployment,
            service: serviceData.status === 'fulfilled' ? serviceData.value : null,
            ingress: ingressData.status === 'fulfilled' ? ingressData.value : null,
          } as DeploymentWithRelations
        })
      )

      setDeploymentRelations(relations) // デプロイメント関連リソースを更新する
      buildGraph(relations, projectId) // グラフを更新する
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
