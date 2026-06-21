import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { Plus, Trash2, X, Globe, Copy, Check, ExternalLink, Play, HardDrive, KeyRound } from 'lucide-react'
import { Layout } from '@/components/Layout'
import { StatusBadge } from '@/components/StatusBadge'
import { DeploymentNode } from '@/components/flow/DeploymentNode'
import { ServiceNode } from '@/components/flow/ServiceNode'
import { IngressNode } from '@/components/flow/IngressNode'
import { InternetNode } from '@/components/flow/InternetNode'
import { VolumeNode } from '@/components/flow/VolumeNode'
import { get, post, del } from '@/lib/api'
import { put } from '@/lib/api'
import type { Project, Deployment, K8sService, IngressRoute, PathRule, Volume, EnvVar, VolumeMount } from '@/lib/types'
import { SIDEBAR_INITIAL_WIDTH, SIDEBAR_MIN_WIDTH, SIDEBAR_MAX_WIDTH, FLOW_ROW_HEIGHT } from '@/lib/config'

const NODE_TYPES = {
  deployment: DeploymentNode,
  service: ServiceNode,
  ingress: IngressNode,
  internet: InternetNode,
  volume: VolumeNode,
} // カスタムノードタイプを定義する

const EDGE_DEFAULTS = {
  type: 'straight', // 直線エッジで矢印を水平に保つ
  // animated は使わず CSS アニメーションで制御する（animated=true は再レンダリングのたびにリセットされてガクつく）
  style: {
    stroke: '#00C2D1',
    strokeWidth: 2,
    strokeDasharray: '6 3',
    animation: 'dashmove 0.6s linear infinite', // CSS アニメーションで常時流れる破線を実現する
  },
} // エッジ共通オプション（矢印なし・破線アニメーション）

type DeploymentWithRelations = {
  deployment: Deployment
  service: K8sService | null
  volumeMounts: VolumeMount[] // このデプロイメントにマウントされているボリューム一覧
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
  const [showVolumeSidebar, setShowVolumeSidebar] = useState(false) // ボリュームサイドバーの表示フラグ
  const [volumeList, setVolumeList] = useState<Volume[]>([]) // プロジェクトのボリューム一覧を管理する
  const [deletingVolumeId, setDeletingVolumeId] = useState<string | null>(null) // 削除中のボリュームID
  const [showEnvVarSidebar, setShowEnvVarSidebar] = useState(false) // 環境変数サイドバーの表示フラグ
  const [envVarList, setEnvVarList] = useState<EnvVar[]>([]) // プロジェクトの環境変数一覧を管理する
  const [deletingEnvVarId, setDeletingEnvVarId] = useState<string | null>(null) // 削除中の環境変数ID
  const [sidebarWidth, setSidebarWidth] = useState(SIDEBAR_INITIAL_WIDTH) // サイドバー幅（px）
  const isDragging = useRef(false) // ドラッグ中フラグ
  const dragStartX = useRef(0) // ドラッグ開始X座標
  const dragStartWidth = useRef(SIDEBAR_INITIAL_WIDTH) // ドラッグ開始時の幅

  const openIngressSidebar = useCallback(() => {
    setSidebarMode('ingress') // IngressRoute サイドバーを開く
    setSelectedDeploymentId(null) // デプロイメント選択をリセットする
  }, [])

  const buildGraph = useCallback((
    relations: DeploymentWithRelations[],
    pid: string,
    currentIngressRoute: IngressRoute | null,
    currentPathRules: PathRule[],
    currentVolumeList: Volume[],
  ) => {
    const newNodes: Node[] = []
    const newEdges: Edge[] = []

    const ROW_HEIGHT = FLOW_ROW_HEIGHT // 行の高さを定義する
    const COL_WIDTH = 360 // 列の幅を定義する

    // レイアウト: Internet(x=0) → Ingress(x=COL) → Service(x=COL*2) → Deployment(x=COL*3) → Volume(x=COL*4)
    const INTERNET_COL = 0 // Internet ノードのX座標
    const INGRESS_COL = COL_WIDTH // IngressRoute ノードのX座標
    const SERVICE_COL = COL_WIDTH * 2 // Service ノードのX座標
    const DEPLOY_COL = COL_WIDTH * 3 // Deployment ノードのX座標
    const VOLUME_COL = COL_WIDTH * 4 // Volume ノードのX座標

    // ノード高さ定数（ハンドルはノード縦中央に出るため、中央Y = y + height/2 が一致するよう offsetY を計算する）
    const DEP_H = 160   // Deployment ノードの概算高さ
    const SVC_H = 110   // Service ノードの概算高さ
    const ING_H = 115   // IngressRoute ノードの概算高さ
    const NET_H = 80    // Internet ノード（円）の概算高さ

    relations.forEach((relation, relationIndex) => {
      const rowBaseY = relationIndex * ROW_HEIGHT // 行のベースY座標
      const depY = rowBaseY // Deployment を基準（y=rowBaseY）にする

      // デプロイメントノードを右端に追加する
      newNodes.push({
        id: `dep-${relation.deployment.id}`,
        type: 'deployment',
        position: { x: DEPLOY_COL, y: depY },
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
        // サービスノードのY: Deployment の縦中央に Service の縦中央を合わせる
        const svcY = depY + (DEP_H - SVC_H) / 2
        newNodes.push({
          id: `svc-${relation.service.id}`,
          type: 'service',
          position: { x: SERVICE_COL, y: svcY },
          data: { service: relation.service },
        })

        // Service → Deployment のエッジを追加する（トラフィックの流れ: ingress → svc → dep）
        newEdges.push({
          id: `edge-svc-dep-${relation.deployment.id}`,
          source: `svc-${relation.service.id}`,
          target: `dep-${relation.deployment.id}`,
          ...EDGE_DEFAULTS,
        })

        // pathRules でこの Service が IngressRoute に登録済みの場合のみ Ingress → Service のエッジを追加する
        const isLinkedToIngress = currentIngressRoute && currentPathRules.some(pr => pr.service_id === relation.service!.id)
        if (isLinkedToIngress) {
          newEdges.push({
            id: `edge-ing-svc-${relation.service.id}`,
            source: 'ingress-node',
            target: `svc-${relation.service.id}`,
            ...EDGE_DEFAULTS,
          })
        }
      }
    })

    // ボリュームをグラフに追加する（マウント有無に関わらず全件表示する）
    const VOL_H = 120 // Volume ノードの概算高さ
    const VOL_ROW_HEIGHT = 140 // Volume ノード間の縦間隔
    currentVolumeList.forEach((volume, volumeIndex) => {
      // このボリュームをマウントしている Deployment のインデックスを収集する
      const mountedRelationIndices = relations
        .map((relation, relationIndex) => ({ relation, relationIndex }))
        .filter(({ relation }) => relation.volumeMounts.some(vm => vm.volume_id === volume.id && vm.status !== 'deleting')) // deleting 状態は接続しない

      // Volume ノードのY座標: マウント先がある場合はその縦中央平均、ない場合は縦に並べる
      const volY = mountedRelationIndices.length > 0
        ? mountedRelationIndices.reduce((sum, { relationIndex }) => sum + relationIndex * ROW_HEIGHT, 0) / mountedRelationIndices.length + (DEP_H - VOL_H) / 2
        : volumeIndex * VOL_ROW_HEIGHT // マウントなしは縦に並べる

      newNodes.push({
        id: `vol-${volume.id}`,
        type: 'volume',
        position: { x: VOLUME_COL, y: volY },
        data: { volume },
      })

      // マウント先の各 Deployment から Volume へ破線エッジを追加する
      mountedRelationIndices.forEach(({ relation }) => {
        newEdges.push({
          id: `edge-dep-vol-${relation.deployment.id}-${volume.id}`,
          source: `dep-${relation.deployment.id}`,
          target: `vol-${volume.id}`,
          type: 'straight',
          style: {
            stroke: '#F59E0B', // アンバー色でストレージを表現する
            strokeWidth: 1.5,
            strokeDasharray: '4 4',
            animation: 'dashmove 0.6s linear infinite', // CSS アニメーションで常時流れる破線を実現する
          },
        })
      })
    })

    // Ingress / Internet は全行の縦中央に1つ配置する
    // 全行の縦中央Y = 全行の高さ/2 - DEP_H/2（Deployment 基準）
    const totalDepSpan = relations.length * ROW_HEIGHT // 全Deploymentが占める縦幅
    const depMidY = (totalDepSpan - ROW_HEIGHT) / 2 // 縦中央行の Deployment Y座標
    const ingY = depMidY + (DEP_H - ING_H) / 2   // IngressRoute の縦中央を Deployment に合わせる
    const netY = depMidY + (DEP_H - NET_H) / 2   // Internet の縦中央を Deployment に合わせる

    if (currentIngressRoute) {
      newNodes.push({
        id: 'ingress-node',
        type: 'ingress',
        position: { x: INGRESS_COL, y: ingY },
        data: {
          ingress: currentIngressRoute,
          pathRules: currentPathRules,
          onSelect: openIngressSidebar, // ノードクリック時にIngressサイドバーを開く
        },
      })

      // Internet → IngressRoute のエッジを追加する
      newEdges.push({
        id: 'edge-internet-ingress',
        source: 'internet-node',
        target: 'ingress-node',
        ...EDGE_DEFAULTS,
      })

      // Internet ノードの縦中央を Deployment に合わせる
      newNodes.push({
        id: 'internet-node',
        type: 'internet',
        position: { x: INTERNET_COL, y: netY },
        data: {},
      })
    }

    setNodes(newNodes) // ノードを更新する
    setEdges(newEdges) // エッジを更新する
  }, [setNodes, setEdges, openIngressSidebar])

  // グラフ再描画の判定に使う前回データを保持する
  const prevGraphKey = useRef<string | null>(null)

  const fetchData = useCallback(async () => {
    if (!projectId) return

    try {
      const [projectData, deploymentList] = await Promise.all([
        get<Project>(`/projects/${projectId}`), // プロジェクト情報を取得する
        get<Deployment[]>(`/projects/${projectId}/deployments`), // デプロイメント一覧を取得する
      ])

      setProject(projectData)

      // 各デプロイメントのサービスと VolumeMounts を並行取得する
      const relations = await Promise.all(
        (deploymentList ?? []).map(async (deployment) => {
          const [serviceResult, volumeMountResult] = await Promise.all([
            get<K8sService>(`/deployments/${deployment.id}/service`).catch(() => null), // サービス情報を取得する
            get<VolumeMount[]>(`/deployments/${deployment.id}/volume-mounts`).catch(() => [] as VolumeMount[]), // VolumeMounts を取得する
          ])
          return { deployment, service: serviceResult, volumeMounts: volumeMountResult ?? [] } as DeploymentWithRelations
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

      const volumes = await get<Volume[]>(`/projects/${projectId}/volumes`).catch(() => []) // ボリューム一覧を取得する
      setVolumeList(volumes ?? []) // ボリューム一覧を設定する

      // グラフに関わるデータのハッシュキーを生成し、前回から変化があった場合のみ再描画する
      const nextGraphKey = JSON.stringify({ relations, ingressResult, currentPathRules, volumes })
      if (nextGraphKey !== prevGraphKey.current) {
        prevGraphKey.current = nextGraphKey // キーを更新する
        buildGraph(relations, projectId, ingressResult, currentPathRules, volumes ?? []) // グラフを再描画する
      }

      const envVars = await get<EnvVar[]>(`/projects/${projectId}/env-vars`).catch(() => []) // 環境変数一覧を取得する
      setEnvVarList(envVars ?? []) // 環境変数一覧を設定する
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
      const next = Math.min(SIDEBAR_MAX_WIDTH, Math.max(SIDEBAR_MIN_WIDTH, dragStartWidth.current + delta))
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

  const handleDeleteVolume = async (volumeId: string) => {
    if (!confirm('ボリュームを削除しますか？この操作は取り消せません。')) return
    setDeletingVolumeId(volumeId) // 削除中のボリュームIDを設定する
    try {
      await del(`/volumes/${volumeId}`) // ボリュームを削除する
      await fetchData() // データを再取得する
    } catch (deleteError) {
      console.error(deleteError)
      alert('ボリュームの削除に失敗しました')
    } finally {
      setDeletingVolumeId(null) // 削除中フラグをリセットする
    }
  }

  const handleDeleteEnvVar = async (envVarId: string) => {
    if (!confirm('環境変数を削除しますか？この操作は取り消せません。')) return
    setDeletingEnvVarId(envVarId) // 削除中の環境変数IDを設定する
    try {
      await del(`/env-vars/${envVarId}`) // 環境変数を削除する
      await fetchData() // データを再取得する
    } catch (deleteError) {
      console.error(deleteError)
      alert('環境変数の削除に失敗しました')
    } finally {
      setDeletingEnvVarId(null) // 削除中フラグをリセットする
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
                  <button
                    onClick={() => { setShowAddMenu(false); setShowVolumeSidebar(true) }}
                    className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-[#111827] hover:bg-gray-50 transition-colors"
                  >
                    <HardDrive className="w-3.5 h-3.5 text-gray-400" />
                    Volume
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
            {/* 左アイコンレール */}
            <div className="w-14 shrink-0 flex flex-col items-center pt-3 gap-2 border-r border-gray-100 bg-gray-50 z-10">
              <button
                onClick={() => { setShowVolumeSidebar(prev => !prev); setShowEnvVarSidebar(false) }}
                title="ボリューム"
                className={`w-10 h-10 flex items-center justify-center rounded-lg transition-all ${
                  showVolumeSidebar
                    ? 'bg-amber-500 text-white shadow-md'
                    : 'bg-amber-50 text-amber-500 hover:bg-amber-100'
                }`}
              >
                <HardDrive className="w-5 h-5" />
              </button>
              <button
                onClick={() => { setShowEnvVarSidebar(prev => !prev); setShowVolumeSidebar(false) }}
                title="環境変数"
                className={`w-10 h-10 flex items-center justify-center rounded-lg transition-all ${
                  showEnvVarSidebar
                    ? 'bg-purple-500 text-white shadow-md'
                    : 'bg-purple-50 text-purple-500 hover:bg-purple-100'
                }`}
              >
                <KeyRound className="w-5 h-5" />
              </button>
            </div>

            {/* ボリュームサイドバー（左から開く） */}
            {showVolumeSidebar && (
              <div className="w-72 shrink-0 flex flex-col border-r border-gray-200 bg-white z-10">
                <VolumeSidebar
                  projectId={projectId!}
                  volumeList={volumeList}
                  deletingVolumeId={deletingVolumeId}
                  onDelete={handleDeleteVolume}
                  onClose={() => setShowVolumeSidebar(false)}
                />
              </div>
            )}

            {/* 環境変数サイドバー（左から開く） */}
            {showEnvVarSidebar && (
              <div className="w-80 shrink-0 flex flex-col border-r border-gray-200 bg-white z-10">
                <EnvVarSidebar
                  projectId={projectId!}
                  envVarList={envVarList}
                  deletingEnvVarId={deletingEnvVarId}
                  onDelete={handleDeleteEnvVar}
                  onRefresh={fetchData}
                  onClose={() => setShowEnvVarSidebar(false)}
                />
              </div>
            )}

            {/* ReactFlow */}
            <div className="flex-1 min-w-0">
              <ReactFlow
                nodes={nodes}
                edges={edges}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                nodeTypes={NODE_TYPES}
                nodesDraggable={false}
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

// ── EnvVarSidebar ────────────────────────────────────────────

function EnvVarSidebar({
  projectId,
  envVarList,
  deletingEnvVarId,
  onDelete,
  onRefresh,
  onClose,
}: {
  projectId: string
  envVarList: EnvVar[]
  deletingEnvVarId: string | null
  onDelete: (id: string) => Promise<void>
  onRefresh: () => Promise<void>
  onClose: () => void
}) {
  const [newKey, setNewKey] = useState('') // 新規環境変数キー
  const [newValue, setNewValue] = useState('') // 新規環境変数値
  const [newIsSecret, setNewIsSecret] = useState(false) // シークレットフラグ
  const [adding, setAdding] = useState(false) // 作成中フラグ
  const [editingId, setEditingId] = useState<string | null>(null) // 編集中の環境変数ID
  const [editValue, setEditValue] = useState('') // 編集中の値
  const [savingId, setSavingId] = useState<string | null>(null) // 保存中の環境変数ID
  const [showValues, setShowValues] = useState<Set<string>>(new Set()) // 値を表示中のID一覧

  const handleAdd = async () => {
    if (!newKey) return
    setAdding(true) // 作成中フラグを立てる
    try {
      await post(`/projects/${projectId}/env-vars`, {
        key: newKey,
        value: newValue,
        is_secret: newIsSecret,
      })
      setNewKey('') // フォームをリセットする
      setNewValue('') // 値をリセットする
      setNewIsSecret(false) // シークレットフラグをリセットする
      await onRefresh() // 一覧を再取得する
    } catch (addError) {
      console.error(addError)
      alert('環境変数の作成に失敗しました')
    } finally {
      setAdding(false) // 作成中フラグを下げる
    }
  }

  const handleStartEdit = (envVar: EnvVar) => {
    setEditingId(envVar.id) // 編集対象IDを設定する
    setEditValue(envVar.is_secret ? '' : envVar.value) // シークレットは空欄で編集開始する
  }

  const handleSaveEdit = async (envVarId: string) => {
    setSavingId(envVarId) // 保存中IDを設定する
    try {
      await put(`/env-vars/${envVarId}`, { value: editValue }) // 値を更新する
      setEditingId(null) // 編集モードを終了する
      await onRefresh() // 一覧を再取得する
    } catch (saveError) {
      console.error(saveError)
      alert('環境変数の更新に失敗しました')
    } finally {
      setSavingId(null) // 保存中フラグをリセットする
    }
  }

  const toggleShowValue = (id: string) => {
    setShowValues(prev => {
      const next = new Set(prev) // 新しい Set を生成する
      if (next.has(id)) {
        next.delete(id) // 表示中の場合は非表示にする
      } else {
        next.add(id) // 非表示の場合は表示する
      }
      return next
    })
  }

  const inputClass = 'w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#00C2D1]/50 focus:border-[#00C2D1] transition-colors font-mono'
  const labelClass = 'block text-xs font-medium text-gray-500 mb-1'

  return (
    <div className="flex flex-col h-full">
      {/* ヘッダー */}
      <div className="h-10 flex items-center justify-between px-3 border-b border-gray-100 bg-gray-50 shrink-0">
        <div className="flex items-center gap-2">
          <KeyRound className="w-3.5 h-3.5 text-gray-500 shrink-0" />
          <span className="text-xs font-medium text-[#111827]">環境変数</span>
          <span className="text-xs text-gray-400">{envVarList.length}</span>
        </div>
        <button
          onClick={onClose}
          className="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>

      {/* コンテンツ */}
      <div className="flex-1 overflow-y-auto p-3 space-y-4">
        {/* apply の説明 */}
        <div className="bg-blue-50 border border-blue-100 rounded-md px-3 py-2 text-xs text-blue-700 leading-relaxed">
          環境変数はデプロイメントのボリュームタブでマウント設定を追加し、Deployment から Apply すると k8s に反映されます。
        </div>

        {/* 環境変数一覧 */}
        <div className="space-y-1.5">
          {envVarList.length === 0 ? (
            <p className="text-xs text-gray-400 py-2">環境変数がありません</p>
          ) : (
            envVarList.map(envVar => (
              <div key={envVar.id} className="bg-gray-50 rounded-md px-3 py-2 border border-gray-100">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 flex-1 space-y-1">
                    <div className="flex items-center gap-1.5">
                      <span className="text-xs font-mono font-semibold text-[#111827] truncate">{envVar.key}</span>
                      {envVar.is_secret && (
                        <span className="text-[10px] bg-purple-50 text-purple-500 px-1.5 py-0.5 rounded shrink-0">secret</span>
                      )}
                    </div>
                    {editingId === envVar.id ? (
                      <div className="flex items-center gap-1.5">
                        <input
                          type={envVar.is_secret ? 'password' : 'text'}
                          value={editValue}
                          onChange={ev => setEditValue(ev.target.value)}
                          placeholder={envVar.is_secret ? '新しい値を入力' : envVar.value}
                          className="flex-1 min-w-0 rounded border border-gray-200 px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-[#00C2D1]"
                          autoFocus
                        />
                        <button
                          onClick={() => void handleSaveEdit(envVar.id)}
                          disabled={savingId === envVar.id}
                          className="text-[10px] bg-[#111827] text-white px-2 py-1 rounded hover:bg-gray-800 disabled:opacity-50 shrink-0"
                        >
                          {savingId === envVar.id ? '...' : '保存'}
                        </button>
                        <button
                          onClick={() => setEditingId(null)}
                          className="text-[10px] text-gray-400 hover:text-gray-600 px-1 shrink-0"
                        >
                          <X className="w-3 h-3" />
                        </button>
                      </div>
                    ) : (
                      <div className="flex items-center gap-1">
                        <span className="text-[11px] font-mono text-gray-500 truncate">
                          {envVar.is_secret
                            ? (showValues.has(envVar.id) ? envVar.value : '••••••••')
                            : envVar.value || <span className="text-gray-300 italic">空</span>
                          }
                        </span>
                        {envVar.is_secret && (
                          <button
                            onClick={() => toggleShowValue(envVar.id)}
                            className="text-[10px] text-gray-400 hover:text-gray-600 shrink-0"
                          >
                            {showValues.has(envVar.id) ? '隠す' : '表示'}
                          </button>
                        )}
                      </div>
                    )}
                  </div>
                  <div className="flex items-center gap-0.5 shrink-0">
                    {editingId !== envVar.id && (
                      <button
                        onClick={() => handleStartEdit(envVar)}
                        className="p-1 rounded text-gray-300 hover:text-gray-500 hover:bg-gray-200 transition-colors"
                        title="編集"
                      >
                        <Check className="w-3 h-3" />
                      </button>
                    )}
                    <button
                      onClick={() => void onDelete(envVar.id)}
                      disabled={deletingEnvVarId === envVar.id}
                      className="p-1 rounded text-gray-300 hover:text-red-400 hover:bg-red-50 transition-colors disabled:opacity-50"
                      title="削除"
                    >
                      <Trash2 className="w-3 h-3" />
                    </button>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>

        {/* 作成フォーム */}
        <div className="border-t border-gray-100 pt-3 space-y-2.5">
          <p className="text-xs font-semibold text-gray-500">新しい環境変数</p>
          <div>
            <label className={labelClass}>キー</label>
            <input
              type="text"
              value={newKey}
              onChange={ev => setNewKey(ev.target.value)}
              placeholder="API_KEY"
              className={inputClass}
            />
          </div>
          <div>
            <label className={labelClass}>値</label>
            <input
              type={newIsSecret ? 'password' : 'text'}
              value={newValue}
              onChange={ev => setNewValue(ev.target.value)}
              placeholder="値を入力"
              className={inputClass}
            />
          </div>
          <label className="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={newIsSecret}
              onChange={ev => setNewIsSecret(ev.target.checked)}
              className="rounded border-gray-300 text-[#00C2D1] focus:ring-[#00C2D1]"
            />
            <span className="text-xs text-gray-600">シークレット（k8s Secret に格納）</span>
          </label>
          <button
            onClick={() => void handleAdd()}
            disabled={adding || !newKey}
            className="w-full bg-[#111827] text-white text-sm py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
          >
            {adding ? '作成中...' : '作成'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ── VolumeSidebar ─────────────────────────────────────────────

function VolumeSidebar({
  projectId,
  volumeList,
  deletingVolumeId,
  onDelete,
  onClose,
}: {
  projectId: string
  volumeList: Volume[]
  deletingVolumeId: string | null
  onDelete: (volumeId: string) => Promise<void>
  onClose: () => void
}) {
  const [newVolumeName, setNewVolumeName] = useState('') // 新規ボリューム名
  const [newVolumeSizeMb, setNewVolumeSizeMb] = useState('') // 新規ボリュームサイズ（MB）
  const [adding, setAdding] = useState(false) // 作成中フラグ

  const handleAdd = async () => {
    if (!newVolumeName || !newVolumeSizeMb) return
    setAdding(true) // 作成中フラグを立てる
    try {
      await post(`/projects/${projectId}/volumes`, {
        name: newVolumeName,
        size_mb: parseInt(newVolumeSizeMb, 10), // サイズを数値に変換する
      })
      setNewVolumeName('') // フォームをリセットする
      setNewVolumeSizeMb('') // サイズをリセットする
    } catch (addError) {
      console.error(addError)
      alert('ボリュームの作成に失敗しました')
    } finally {
      setAdding(false) // 作成中フラグを下げる
    }
  }

  const inputClass = 'w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#00C2D1]/50 focus:border-[#00C2D1] transition-colors'
  const labelClass = 'block text-xs font-medium text-gray-500 mb-1'

  return (
    <div className="flex flex-col h-full">
      {/* ヘッダー */}
      <div className="h-10 flex items-center justify-between px-3 border-b border-gray-100 bg-gray-50 shrink-0">
        <div className="flex items-center gap-2">
          <HardDrive className="w-3.5 h-3.5 text-gray-500 shrink-0" />
          <span className="text-xs font-medium text-[#111827]">ボリューム</span>
          <span className="text-xs text-gray-400">{volumeList.length}</span>
        </div>
        <button
          onClick={onClose}
          className="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>

      {/* コンテンツ */}
      <div className="flex-1 overflow-y-auto p-3 space-y-4">
        {/* apply の説明 */}
        <div className="bg-blue-50 border border-blue-100 rounded-md px-3 py-2 text-xs text-blue-700 leading-relaxed">
          ボリュームは Deployment のボリュームタブでマウント設定を追加し、Deployment から Apply すると k8s PVC として反映されます。
        </div>

        {/* ボリューム一覧 */}
        <div className="space-y-1.5">
          {volumeList.length === 0 ? (
            <p className="text-xs text-gray-400 py-2">ボリュームがありません</p>
          ) : (
            volumeList.map(vol => (
              <div key={vol.id} className="flex items-center justify-between bg-gray-50 rounded-md px-3 py-2 border border-gray-100">
                <div className="min-w-0 space-y-0.5">
                  <p className="text-sm font-medium text-[#111827] truncate">{vol.name}</p>
                  <div className="flex items-center gap-1.5">
                    <span className="text-[10px] text-gray-400">{vol.size_mb} MB</span>
                    <span className={`text-[10px] ${vol.status === 'bound' ? 'text-green-500' : vol.status === 'deleting' ? 'text-red-400' : 'text-amber-500'}`}>
                      {vol.status}
                    </span>
                  </div>
                </div>
                <button
                  onClick={() => void onDelete(vol.id)}
                  disabled={deletingVolumeId === vol.id}
                  className="p-1 rounded hover:bg-red-50 text-gray-300 hover:text-red-400 transition-colors disabled:opacity-50 shrink-0 ml-2"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              </div>
            ))
          )}
        </div>

        {/* 作成フォーム */}
        <div className="border-t border-gray-100 pt-3 space-y-2.5">
          <p className="text-xs font-semibold text-gray-500">新しいボリューム</p>
          <div>
            <label className={labelClass}>名前</label>
            <input
              type="text"
              value={newVolumeName}
              onChange={ev => setNewVolumeName(ev.target.value)}
              placeholder="my-volume"
              className={inputClass}
            />
          </div>
          <div>
            <label className={labelClass}>サイズ（MB）</label>
            <input
              type="number"
              min={1}
              value={newVolumeSizeMb}
              onChange={ev => setNewVolumeSizeMb(ev.target.value)}
              placeholder="1024"
              className={inputClass}
            />
          </div>
          <button
            onClick={() => void handleAdd()}
            disabled={adding || !newVolumeName || !newVolumeSizeMb}
            className="w-full bg-[#111827] text-white text-sm py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
          >
            {adding ? '作成中...' : '作成'}
          </button>
        </div>
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
