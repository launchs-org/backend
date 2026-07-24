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
import { Plus, Trash2, X, Globe, Copy, Check, ExternalLink, Play, HardDrive, KeyRound, Layers, ChevronRight, Package, GitBranch, GitCommit, FolderOpen, ScrollText } from 'lucide-react'
import { Layout } from '@/components/Layout'
import { StatusBadge } from '@/components/StatusBadge'
import { DeploymentNode } from '@/components/flow/DeploymentNode'
import { ServiceNode } from '@/components/flow/ServiceNode'
import { IngressNode } from '@/components/flow/IngressNode'
import { InternetNode } from '@/components/flow/InternetNode'
import { VolumeNode } from '@/components/flow/VolumeNode'
import { EnvVarNode } from '@/components/flow/EnvVarNode'
import { get, post, put, del, patch } from '@/lib/api'
import type { Project, Deployment, K8sService, IngressRoute, PathRule, Volume, EnvVar, VolumeMount, EnvVarMount, Image, ProjectQuota, ProjectPendingSummary, ApplyProjectResult } from '@/lib/types'
import { SIDEBAR_INITIAL_WIDTH, SIDEBAR_MIN_WIDTH, SIDEBAR_MAX_WIDTH, FLOW_ROW_HEIGHT, APPLY_PROJECT_POLL_INTERVAL, APPLY_PROJECT_POLL_TIMEOUT } from '@/lib/config'
import { toast } from 'sonner' // トースト通知をインポートする
import { ConfirmDialog } from '@/components/ui/confirm-dialog' // 確認ダイアログをインポートする
import { ProjectApplyBar } from '@/components/ProjectApplyBar' // 一括Applyフローティングバーをインポートする
import { useTutorialContext } from '@/tutorial/TutorialContext' // チュートリアル Context をインポートする

const NODE_TYPES = {
  deployment: DeploymentNode,
  service: ServiceNode,
  ingress: IngressNode,
  internet: InternetNode,
  volume: VolumeNode,
  envVar: EnvVarNode,
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
  volumeMounts: VolumeMount[]    // このデプロイメントにマウントされているボリューム一覧
  envVarMounts: EnvVarMount[]    // このデプロイメントにマウントされている環境変数一覧
}

type SidebarMode = 'deployment' | 'deployment-new' | 'ingress' | null // サイドバーの表示モードを定義する

export function ProjectDetailPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const navigate = useNavigate()
  const { advance: tutorialAdvance, isActive: tutorialIsActive, actualStep: tutorialActualStep, pause: tutorialPause, resume: tutorialResume } = useTutorialContext() // チュートリアルの進行関数を取得する
  const [project, setProject] = useState<Project | null>(null) // プロジェクト情報を管理する
  const [deploymentRelations, setDeploymentRelations] = useState<DeploymentWithRelations[]>([]) // デプロイメントとその関連リソースを管理する
  const [loading, setLoading] = useState(true) // ローディング状態を管理する
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]) // ReactFlowのノードを管理する
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]) // ReactFlowのエッジを管理する
  const [deletingProject, setDeletingProject] = useState(false) // プロジェクト削除中フラグ
  const [ingressRouteList, setIngressRouteList] = useState<IngressRoute[]>([]) // プロジェクトのIngressRoute一覧を管理する
  const [pathRulesByIngressRouteId, setPathRulesByIngressRouteId] = useState<Record<string, PathRule[]>>({}) // IngressRoute IDをキーにしたパスルールマップを管理する
  const [creatingIngress, setCreatingIngress] = useState(false) // IngressRoute作成中フラグ
  const [selectedIngressRouteId, setSelectedIngressRouteId] = useState<string | null>(null) // 選択中のIngressRoute IDを管理する
  const [sidebarMode, setSidebarMode] = useState<SidebarMode>(null) // サイドバーの表示モードを管理する
  const [selectedDeploymentId, setSelectedDeploymentId] = useState<string | null>(null) // 選択中のデプロイメントIDを管理する
  const [iframeLoaded, setIframeLoaded] = useState(false) // iframe読み込み完了フラグ
  const [iframeError, setIframeError] = useState(false) // iframe読み込みエラーフラグ
  const iframeLoadTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null) // ローディングタイムアウトの ref
  const [showAddMenu, setShowAddMenu] = useState(false) // 追加メニューの表示フラグ
  const [showVolumeSidebar, setShowVolumeSidebar] = useState(false) // ボリュームサイドバーの表示フラグ
  const [volumeList, setVolumeList] = useState<Volume[]>([]) // プロジェクトのボリューム一覧を管理する
  const [deletingVolumeId, setDeletingVolumeId] = useState<string | null>(null) // 削除中のボリュームID
  const [showEnvVarSidebar, setShowEnvVarSidebar] = useState(false) // 環境変数サイドバーの表示フラグ
  const [selectedEnvVarId, setSelectedEnvVarId] = useState<string | null>(null) // クリックされた環境変数ID（ハイライト用）
  const [showDeploymentListSidebar, setShowDeploymentListSidebar] = useState(false) // デプロイメント一覧サイドバーの表示フラグ
  const [showIngressListSidebar, setShowIngressListSidebar] = useState(false) // IngressRoute一覧サイドバーの表示フラグ
  const [showImageSidebar, setShowImageSidebar] = useState(false) // イメージ一覧サイドバーの表示フラグ
  const [sidebarNewQuery, setSidebarNewQuery] = useState('') // デプロイメント新規作成サイドバーのクエリパラメータ
  const [imageList, setImageList] = useState<Image[]>([]) // プロジェクトのイメージ一覧を管理する
  const [projectQuota, setProjectQuota] = useState<ProjectQuota | null>(null) // Harborストレージクォータを管理する
  const [envVarList, setEnvVarList] = useState<EnvVar[]>([]) // プロジェクトの環境変数一覧を管理する
  const [deletingEnvVarId, setDeletingEnvVarId] = useState<string | null>(null) // 削除中の環境変数ID
  const [deleteProjectConfirmOpen, setDeleteProjectConfirmOpen] = useState(false) // プロジェクト削除確認ダイアログの表示フラグ
  const [pendingSummary, setPendingSummary] = useState<ProjectPendingSummary | null>(null) // プロジェクト配下のpending集計を管理する
  const [applyProjectDetailsOpen, setApplyProjectDetailsOpen] = useState(false) // 一括Apply詳細ダイアログの表示フラグ
  const [applyProjectProgress, setApplyProjectProgress] = useState<{ done: number; total: number } | null>(null) // 一括Apply完了待機の進捗（完了件数/対象件数）
  const [applyingProject, setApplyingProject] = useState(false) // 一括Apply実行中フラグ
  const [deleteVolumeConfirmId, setDeleteVolumeConfirmId] = useState<string | null>(null) // ボリューム削除確認ダイアログ対象ID
  const [deleteEnvVarConfirmId, setDeleteEnvVarConfirmId] = useState<string | null>(null) // 環境変数削除確認ダイアログ対象ID
  const [createIngressDialogOpen, setCreateIngressDialogOpen] = useState(false) // IngressRoute作成ダイアログの表示フラグ
  const [createIngressNameInput, setCreateIngressNameInput] = useState('') // IngressRoute作成時の名前入力値
  const sidebarWidth = useRef(SIDEBAR_INITIAL_WIDTH) // サイドバー幅（px）: re-render を避けるため ref で管理する
  const sidebarElRef = useRef<HTMLDivElement>(null) // サイドバー DOM 要素への参照
  const [isResizing, setIsResizing] = useState(false) // リサイズ中フラグ（iframe のイベント遮断に使う）
  const isDragging = useRef(false) // ドラッグ中フラグ
  const dragStartX = useRef(0) // ドラッグ開始X座標
  const dragStartWidth = useRef(SIDEBAR_INITIAL_WIDTH) // ドラッグ開始時の幅

  const openDeploymentNewSidebar = useCallback((query?: string) => {
    setSidebarMode('deployment-new') // デプロイメント作成サイドバーを開く
    setSelectedDeploymentId(null) // デプロイメント選択をリセットする
    setIframeLoaded(false) // iframe読み込みフラグをリセットする
    setIframeError(false) // iframeエラーフラグをリセットする
    if (query) setSidebarNewQuery(query) // クエリパラメータを設定する
    else setSidebarNewQuery('') // クエリパラメータをリセットする
  }, [])

  const openIngressSidebar = useCallback((ingressRouteId: string) => {
    setSelectedIngressRouteId(ingressRouteId) // 選択した IngressRoute ID を設定する
    setSidebarMode('ingress') // IngressRoute サイドバーを開く
    setSelectedDeploymentId(null) // デプロイメント選択をリセットする
  }, [])

  const buildGraph = useCallback((
    relations: DeploymentWithRelations[],
    pid: string,
    currentIngressRouteList: IngressRoute[],
    currentPathRulesByIngressRouteId: Record<string, PathRule[]>,
    currentVolumeList: Volume[],
    _currentEnvVarList: EnvVar[],
    currentSelectedEnvVarId: string | null,
  ) => {
    const newNodes: Node[] = []
    const newEdges: Edge[] = []

    const ROW_HEIGHT = FLOW_ROW_HEIGHT // 行の高さを定義する
    const COL_WIDTH = 360 // 列の幅を定義する

    // レイアウト: Internet(x=0) → Ingress(x=COL) → Service(x=COL*2) → Deployment(x=COL*3) → Volume(x=COL*4) / EnvVar(Volume列の下)
    const INTERNET_COL = 0 // Internet ノードのX座標
    const INGRESS_COL = COL_WIDTH // IngressRoute ノードのX座標
    const SERVICE_COL = COL_WIDTH * 2 // Service ノードのX座標
    const DEPLOY_COL = COL_WIDTH * 3 // Deployment ノードのX座標
    const VOLUME_COL = COL_WIDTH * 4 // Volume ノードのX座標

    // 各ノードの固定高さ（ノードコンポーネントの style.height と必ず一致させること）
    const SVC_H = 98   // Service ノード高さ
    const ING_H = 116  // IngressRoute ノード高さ
    const NET_H = 80   // Internet ノード高さ（h-20 固定）

    relations.forEach((relation, relationIndex) => {
      const rowCenterY = relationIndex * ROW_HEIGHT + ROW_HEIGHT / 2 // 行の縦中央Y座標

      // Deployment の高さは k8s_status の有無で変わる（ノードコンポーネントと同じ条件分岐）
      const depH = relation.deployment.k8s_status ? 148 : 132

      // デプロイメントノード: 縦中央を rowCenterY に合わせる
      // このDeploymentが選択中のEnvVarにマウントされているかを判定する
      const depHighlighted = currentSelectedEnvVarId !== null &&
        relation.envVarMounts.some(evm => evm.env_var_id === currentSelectedEnvVarId && evm.status !== 'deleting')

      newNodes.push({
        id: `dep-${relation.deployment.id}`,
        type: 'deployment',
        position: { x: DEPLOY_COL, y: rowCenterY - depH / 2 },
        data: {
          deployment: relation.deployment,
          projectId: pid,
          highlighted: depHighlighted, // EnvVarハイライト時に強調する
          onSelect: (deploymentId: string) => {
            setSelectedDeploymentId(deploymentId)
            setSidebarMode('deployment')
            setIframeLoaded(false)
            setIframeError(false) // iframeエラーフラグをリセットする
            // チュートリアル中にカードをクリックしたらステップを進める
            if (tutorialIsActive && (tutorialActualStep?.id === 'deployment-open-card' || tutorialActualStep?.id === 'adv-storage-open-card' || tutorialActualStep?.id === 'adv-envvar-open-card')) {
              tutorialAdvance()
            }
          }, // ノードクリック時にデプロイメントサイドバーを開く
        },
      })

      // port が 0 の場合は未設定扱いとしてノードを表示しない
      const serviceConfigured = relation.service && (relation.service.port !== 0 || relation.service.pending_port !== 0)

      if (serviceConfigured && relation.service) {
        // ClusterIP が割り当て済みの場合はノード高さが増える（ServiceNode と同じ条件）
        const svcH = relation.service.cluster_ip ? 118 : SVC_H
        // サービスノード: 縦中央を rowCenterY に合わせる
        newNodes.push({
          id: `svc-${relation.service.id}`,
          type: 'service',
          position: { x: SERVICE_COL, y: rowCenterY - svcH / 2 },
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
        const linkedIngressRouteList = currentIngressRouteList.filter(ing =>
          (currentPathRulesByIngressRouteId[ing.id] ?? []).some(pr => pr.service_id === relation.service!.id)
        ) // この Service を参照している IngressRoute を収集する
        linkedIngressRouteList.forEach(linkedIngressRoute => {
          newEdges.push({
            id: `edge-ing-svc-${linkedIngressRoute.id}-${relation.service!.id}`,
            source: `ingress-node-${linkedIngressRoute.id}`,
            target: `svc-${relation.service!.id}`,
            ...EDGE_DEFAULTS,
          })
        })
      }
    })

    // ボリュームをグラフに追加する（マウント有無に関わらず全件表示する）
    const VOL_H = 114  // Volume ノード高さ
    const VOL_GAP = 80 // Volume カード間の最小余白

    const firstDepTop = ROW_HEIGHT / 2 - 148 / 2 + 16 // 先頭Deploymentカードの上端より少し下から開始する

    currentVolumeList.forEach((volume, volumeIndex) => {
      // このボリュームをマウントしている Deployment のインデックスを収集する
      const mountedRelationIndices = relations
        .map((relation, relationIndex) => ({ relation, relationIndex }))
        .filter(({ relation }) => relation.volumeMounts.some(vm => vm.volume_id === volume.id && vm.status !== 'deleting')) // deleting 状態は接続しない

      // Volume ノードのY座標: マウント状態に関係なくインデックス順の固定位置に並べる
      const volY = firstDepTop + volumeIndex * (VOL_H + VOL_GAP)

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

    // IngressRoute を ROW_HEIGHT 間隔で縦に並べて配置する
    if (currentIngressRouteList.length > 0) {
      const totalIngSpan = currentIngressRouteList.length * ROW_HEIGHT // 全IngressRouteが占める縦幅
      const totalDepSpan = relations.length * ROW_HEIGHT // 全Deploymentが占める縦幅
      // IngressRoute 群の縦中央を Deployment 群の縦中央に揃える
      const ingGroupCenterY = totalDepSpan / 2 // Deployment群の縦中央
      const ingStartCenterY = ingGroupCenterY - totalIngSpan / 2 + ROW_HEIGHT / 2 // 最初のIngressRouteの縦中央Y

      currentIngressRouteList.forEach((ingressRoute, ingressIndex) => {
        const ingCenterY = ingStartCenterY + ingressIndex * ROW_HEIGHT // 各IngressRouteの縦中央Y
        const currentPathRules = currentPathRulesByIngressRouteId[ingressRoute.id] ?? [] // この IngressRoute のパスルールを取得する
        newNodes.push({
          id: `ingress-node-${ingressRoute.id}`,
          type: 'ingress',
          position: { x: INGRESS_COL, y: ingCenterY - ING_H / 2 }, // 縦中央を基準にY座標を決める
          data: {
            ingress: ingressRoute,
            pathRules: currentPathRules,
            onSelect: () => openIngressSidebar(ingressRoute.id), // ノードクリック時にIngressサイドバーを開く
          },
        })

        // Internet → IngressRoute のエッジを追加する
        newEdges.push({
          id: `edge-internet-ingress-${ingressRoute.id}`,
          source: 'internet-node',
          target: `ingress-node-${ingressRoute.id}`,
          ...EDGE_DEFAULTS,
        })
      })

      // Internet ノードを IngressRoute 群の縦中央に配置する
      newNodes.push({
        id: 'internet-node',
        type: 'internet',
        position: { x: INTERNET_COL, y: ingGroupCenterY - NET_H / 2 }, // Deployment群縦中央に Internet を置く
        data: {},
      })
    }

    // EnvVar はグラフに表示しない（サイドバーでのみ管理する）

    setNodes(newNodes) // ノードを更新する
    setEdges(newEdges) // エッジを更新する
  }, [setNodes, setEdges, openIngressSidebar, setSelectedEnvVarId])

  const fetchData = useCallback(async () => {
    if (!projectId) return

    try {
      const [projectData, deploymentList] = await Promise.all([
        get<Project>(`/projects/${projectId}`), // プロジェクト情報を取得する
        get<Deployment[]>(`/projects/${projectId}/deployments`), // デプロイメント一覧を取得する
      ])

      setProject(projectData)

      // 各デプロイメントのサービス・VolumeMounts・EnvVarMounts を並行取得する
      const relations = await Promise.all(
        (deploymentList ?? []).map(async (deployment) => {
          const [serviceResult, volumeMountResult, envVarMountResult] = await Promise.all([
            get<K8sService>(`/deployments/${deployment.id}/service`).catch(() => null), // サービス情報を取得する
            get<VolumeMount[]>(`/deployments/${deployment.id}/volume-mounts`).catch(() => [] as VolumeMount[]), // VolumeMounts を取得する
            get<EnvVarMount[]>(`/deployments/${deployment.id}/env-var-mounts`).catch(() => [] as EnvVarMount[]), // EnvVarMounts を取得する
          ])
          return { deployment, service: serviceResult, volumeMounts: volumeMountResult ?? [], envVarMounts: envVarMountResult ?? [] } as DeploymentWithRelations
        })
      )

      setDeploymentRelations(relations) // デプロイメント関連リソースを更新する

      // プロジェクトの IngressRoute 一覧を取得する
      const ingressList = await get<IngressRoute[]>(`/projects/${projectId}/ingress-routes`).catch(() => []) ?? [] // IngressRoute 一覧を取得する
      setIngressRouteList(ingressList) // IngressRoute 一覧を設定する

      // 各 IngressRoute のパスルールを並行取得する
      const pathRulesEntries = await Promise.all(
        ingressList.map(async (ingressRoute) => {
          const rules = await get<PathRule[]>(`/ingress-routes/${ingressRoute.id}/path-rules`).catch(() => []) ?? [] // パスルール一覧を取得する
          return [ingressRoute.id, rules] as [string, PathRule[]]
        })
      )
      const currentPathRulesByIngressRouteId = Object.fromEntries(pathRulesEntries) // IngressRoute IDをキーにしたパスルールマップを生成する
      setPathRulesByIngressRouteId(currentPathRulesByIngressRouteId) // パスルールマップを設定する

      const volumes = await get<Volume[]>(`/projects/${projectId}/volumes`).catch(() => []) // ボリューム一覧を取得する
      setVolumeList(volumes ?? []) // ボリューム一覧を設定する

      // グラフキーは envVar 追加後に再描画するため、ここではスキップする

      const envVars = await get<EnvVar[]>(`/projects/${projectId}/env-vars`).catch(() => []) // 環境変数一覧を取得する
      setEnvVarList(envVars ?? []) // 環境変数一覧を設定する

      // イメージ一覧とクォータを並行取得する
      const [images, quota] = await Promise.all([
        get<Image[]>(`/projects/${projectId}/images`).catch(() => []), // イメージ一覧を取得する
        get<ProjectQuota>(`/projects/${projectId}/quota`).catch(() => null), // Harbor クォータを取得する
      ])
      setImageList(images ?? []) // イメージ一覧を設定する
      setProjectQuota(quota) // クォータを設定する

      // envVar を含めてグラフを再描画する（selectedEnvVarId は useEffect 側で監視）
      buildGraph(relations, projectId, ingressList, currentPathRulesByIngressRouteId, volumes ?? [], envVars ?? [], selectedEnvVarId)
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

  const fetchPendingSummary = useCallback(async () => {
    if (!projectId) return
    const summary = await get<ProjectPendingSummary>(`/projects/${projectId}/pending-summary`).catch(() => null) // pending集計を取得する
    setPendingSummary(summary) // pending集計を更新する
  }, [projectId])

  useEffect(() => {
    void fetchPendingSummary() // 初回pending集計取得

    const intervalId = setInterval(() => {
      void fetchPendingSummary() // 10秒ごとにポーリングする
    }, 10_000)

    return () => clearInterval(intervalId) // クリーンアップ
  }, [fetchPendingSummary])

  // waitForDeploymentsCompletion は対象 Deployment 一覧の app_status が deploying でなくなるまでポーリングして待機する
  const waitForDeploymentsCompletion = async (deploymentIdList: string[]) => {
    const pendingIdSet = new Set(deploymentIdList) // 未完了の deployment ID を管理する
    setApplyProjectProgress({ done: 0, total: deploymentIdList.length }) // 進捗を初期化する
    const startedAt = Date.now()

    while (pendingIdSet.size > 0) {
      if (Date.now() - startedAt > APPLY_PROJECT_POLL_TIMEOUT) { // タイムアウトした場合は待機を打ち切る
        toast.warning('一部のDeploymentの完了確認がタイムアウトしました。状況は画面上で確認してください')
        break
      }
      await new Promise(resolve => setTimeout(resolve, APPLY_PROJECT_POLL_INTERVAL)) // 一定間隔で待機する

      await Promise.all(
        Array.from(pendingIdSet).map(async (deploymentId) => {
          const deploymentData = await get<Deployment>(`/deployments/${deploymentId}`).catch(() => null) // 最新のDeployment情報を取得する
          if (deploymentData && deploymentData.app_status !== 'deploying') { // deploying でなくなったら完了扱いにする
            pendingIdSet.delete(deploymentId)
          }
        })
      )
      setApplyProjectProgress({ done: deploymentIdList.length - pendingIdSet.size, total: deploymentIdList.length }) // 進捗を更新する
    }
  }

  const handleApplyProject = async () => {
    if (!projectId) return
    setApplyingProject(true) // 一括Apply実行中フラグを立てる
    try {
      const result = await post<ApplyProjectResult>(`/projects/${projectId}/apply`) // プロジェクト配下を一括applyする
      if (result && result.applied_deployment_id_list.length > 0) { // apply起動が成功したDeploymentの完了を待つ
        await waitForDeploymentsCompletion(result.applied_deployment_id_list)
      }
      if (result && result.failed_deployment_list.length > 0) { // 一部失敗した場合は警告を表示する
        toast.warning(`${result.applied_deployment_count}件完了、${result.failed_deployment_list.length}件失敗しました`)
      } else {
        toast.success('プロジェクトの変更を一括適用しました')
      }
      await fetchData() // データを再取得する
      await fetchPendingSummary() // pending集計を再取得する
    } catch (applyError) {
      console.error(applyError)
      toast.error(applyError instanceof Error ? applyError.message : 'Apply に失敗しました') // エラートーストを表示する
    } finally {
      setApplyingProject(false) // 一括Apply実行中フラグを下げる
      setApplyProjectProgress(null) // 進捗表示をクリアする
    }
  }

  // selectedEnvVarId が変わったらハイライト状態でグラフを再描画する
  useEffect(() => {
    if (deploymentRelations.length === 0) return
    buildGraph(deploymentRelations, projectId!, ingressRouteList, pathRulesByIngressRouteId, volumeList, envVarList, selectedEnvVarId)
  }, [selectedEnvVarId]) // eslint-disable-line react-hooks/exhaustive-deps

  // deployment-new サイドバーが開いている間だけチュートリアルを一時停止する
  // deployment（詳細）サイドバーは pause しない（close-sidebar ステップを親側で表示するため）
  useEffect(() => {
    if (sidebarMode === 'deployment-new') {
      tutorialPause() // 新規作成 iframe が開いている間はオーバーレイを非表示にする
    } else {
      tutorialResume() // それ以外は再表示する
    }
  }, [sidebarMode, tutorialPause, tutorialResume])

  // iframe から postMessage でデプロイメント作成完了を受信したらサイドバーを閉じてデータを再取得する
  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      if (event.data?.type !== 'deployment-created') return
      setSidebarMode(null) // サイドバーを閉じる
      setSelectedDeploymentId(null) // 選択をリセットする
      void fetchData() // データを再取得する
      // チュートリアル中の場合はデプロイメント作成完了でステップを進める（deployment-open-card へ）
      const deploymentFlowStepIds = ['add-deployment-menu', 'deployment-type-select', 'deployment-name-input', 'deployment-image-input', 'deployment-create-button']
      if (tutorialIsActive && deploymentFlowStepIds.includes(tutorialActualStep?.id ?? '')) {
        tutorialAdvance() // 次のステップ（deployment-open-card）へ進む
      }
    }
    window.addEventListener('message', handleMessage)
    return () => window.removeEventListener('message', handleMessage)
  }, [fetchData, tutorialActualStep, tutorialAdvance, tutorialIsActive])

  useEffect(() => {
    const onMouseMove = (ev: MouseEvent) => {
      if (!isDragging.current) return
      const delta = dragStartX.current - ev.clientX // 左へドラッグすると幅が増える
      const next = Math.min(SIDEBAR_MAX_WIDTH, Math.max(SIDEBAR_MIN_WIDTH, dragStartWidth.current + delta))
      sidebarWidth.current = next // ref を更新する
      if (sidebarElRef.current) {
        sidebarElRef.current.style.width = `${next}px` // re-render せず直接 DOM に反映する
      }
    }
    const onMouseUp = () => {
      if (!isDragging.current) return
      isDragging.current = false
      setIsResizing(false) // オーバーレイを外す
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
    dragStartWidth.current = sidebarWidth.current // 現在の幅をドラッグ開始幅として記録する
    setIsResizing(true) // iframe 上にオーバーレイを表示してマウスイベントを横取りさせない
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
  }

  const handleCreateIngressRoute = async (name?: string) => {
    if (!projectId) return
    setCreatingIngress(true) // 作成中フラグを立てる
    try {
      const body = name ? { name } : undefined // 名前が指定されている場合のみボディに含める
      const created = await post<IngressRoute>(`/projects/${projectId}/ingress-routes`, body) // IngressRouteを作成する
      await fetchData() // データを再取得する
      if (created) {
        openIngressSidebar(created.id) // 作成した IngressRoute のサイドバーを開く
      }
    } catch (createError) {
      console.error(createError)
      toast.error(createError instanceof Error ? createError.message : 'IngressRouteの作成に失敗しました') // エラートーストを表示する
    } finally {
      setCreatingIngress(false) // 作成中フラグを下げる
    }
  }

  const handleOpenCreateIngressDialog = () => {
    setCreateIngressNameInput('') // 名前入力をリセットする
    setCreateIngressDialogOpen(true) // ダイアログを開く
    tutorialPause() // ダイアログが開いている間はチュートリアルオーバーレイを非表示にする
  }

  const handleConfirmCreateIngress = async () => {
    setCreateIngressDialogOpen(false) // ダイアログを閉じる
    tutorialResume() // チュートリアルオーバーレイを再表示する
    // チュートリアル中の ingress-menu ステップで作成した場合は次のステップに進む
    if (tutorialIsActive && tutorialActualStep?.id === 'ingress-menu') {
      tutorialAdvance() // ingress-overview ステップへ進む
    }
    await handleCreateIngressRoute(createIngressNameInput.trim() || undefined) // 名前を渡して作成する
  }

  const handleCancelCreateIngressDialog = () => {
    setCreateIngressDialogOpen(false) // ダイアログを閉じる
    tutorialResume() // チュートリアルオーバーレイを再表示する
  }

  const handleDeleteProject = async () => {
    if (!projectId || !project) return
    setDeletingProject(true) // 削除中フラグを立てる
    try {
      await del(`/projects/${projectId}`) // プロジェクトを削除する
      navigate('/') // ダッシュボードへ遷移する
    } catch (deleteError) {
      console.error(deleteError)
      toast.error(deleteError instanceof Error ? deleteError.message : 'プロジェクトの削除に失敗しました') // エラートーストを表示する
    } finally {
      setDeletingProject(false)
    }
  }

  const handleDeleteVolume = async (volumeId: string) => {
    setDeletingVolumeId(volumeId) // 削除中のボリュームIDを設定する
    try {
      await del(`/volumes/${volumeId}`) // ボリュームを削除する
      await fetchData() // データを再取得する
    } catch (deleteError) {
      console.error(deleteError)
      toast.error(deleteError instanceof Error ? deleteError.message : 'ボリュームの削除に失敗しました') // エラートーストを表示する
    } finally {
      setDeletingVolumeId(null) // 削除中フラグをリセットする
    }
  }

  const handleDeleteEnvVar = async (envVarId: string) => {
    setDeletingEnvVarId(envVarId) // 削除中の環境変数IDを設定する
    try {
      await del(`/env-vars/${envVarId}`) // 環境変数を削除する
      await fetchData() // データを再取得する
    } catch (deleteError) {
      console.error(deleteError)
      toast.error(deleteError instanceof Error ? deleteError.message : '環境変数の削除に失敗しました') // エラートーストを表示する
    } finally {
      setDeletingEnvVarId(null) // 削除中フラグをリセットする
    }
  }

  const handleCloseSidebar = () => {
    setSidebarMode(null)
    setSelectedDeploymentId(null)
  }

  const sidebarOpen = sidebarMode !== null // サイドバーが開いているかどうかを確認する

  const sidebarIframeSrc = (() => {
    if (sidebarMode === 'deployment' && selectedDeploymentId && projectId) {
      return `/ui/projects/${projectId}/deployments/${selectedDeploymentId}` // デプロイメント詳細の iframe URL
    }
    if (sidebarMode === 'deployment-new' && projectId) {
      const query = sidebarNewQuery ? `?${sidebarNewQuery}` : '' // クエリパラメータを付与する
      return `/ui/projects/${projectId}/deployments/new${query}` // デプロイメント作成フォームの iframe URL
    }
    return null
  })()
  const sidebarNavigatePath = sidebarMode === 'deployment' && selectedDeploymentId && projectId
    ? `/projects/${projectId}/deployments/${selectedDeploymentId}`
    : sidebarMode === 'deployment-new' && projectId
      ? `/projects/${projectId}/deployments/new`
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
    <>
    <Layout
      fullWidth
      breadcrumbs={[{ label: project?.name ?? '', sub: project?.namespace }]}
      actions={
        <div className="flex items-center gap-2">
          {/* 追加メニュー */}
          <div className="relative" data-tutorial="tutorial-add-button">
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
                <div className="fixed inset-0" style={{ zIndex: 9999 }} onClick={() => setShowAddMenu(false)} />
                <div className="absolute right-0 top-full mt-1 bg-white border border-gray-200 rounded-lg shadow-lg overflow-hidden w-48" style={{ zIndex: 10000 }}>
                  <button
                    data-tutorial="tutorial-add-deployment-menu"
                    onClick={() => { setShowAddMenu(false); openDeploymentNewSidebar() }}
                    className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-[#111827] hover:bg-gray-50 transition-colors"
                  >
                    <Plus className="w-3.5 h-3.5 text-gray-400" />
                    Deployment
                  </button>
                  <button
                    data-tutorial="tutorial-add-ingress-menu"
                    onClick={() => {
                      setShowAddMenu(false)
                      handleOpenCreateIngressDialog() // 名前入力ダイアログを開く
                    }}
                    disabled={creatingIngress}
                    className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-[#111827] hover:bg-gray-50 transition-colors disabled:opacity-50"
                  >
                    <Globe className="w-3.5 h-3.5 text-gray-400" />
                    {creatingIngress ? 'IngressRoute作成中...' : 'IngressRoute'}
                  </button>
                  <button
                    data-tutorial="tutorial-add-volume-menu"
                    onClick={() => {
                      setShowAddMenu(false)
                      setShowVolumeSidebar(true)
                      if (tutorialActualStep?.id === 'adv-storage-menu') tutorialAdvance() // ボリュームサイドバーを開いたらチュートリアルを進める
                    }}
                    className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-[#111827] hover:bg-gray-50 transition-colors"
                  >
                    <HardDrive className="w-3.5 h-3.5 text-gray-400" />
                    Volume
                  </button>
                  <button
                    data-tutorial="tutorial-add-envvar-menu"
                    onClick={() => {
                      setShowAddMenu(false)
                      setShowEnvVarSidebar(true)
                      if (tutorialActualStep?.id === 'adv-envvar-menu') tutorialAdvance() // 環境変数サイドバーを開いたらチュートリアルを進める
                    }}
                    className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-[#111827] hover:bg-gray-50 transition-colors"
                  >
                    <KeyRound className="w-3.5 h-3.5 text-gray-400" />
                    EnvVar
                  </button>
                </div>
              </>
            )}
          </div>

          <button
            onClick={() => setDeleteProjectConfirmOpen(true)} // プロジェクト削除確認ダイアログを開く
            disabled={deletingProject}
            className="flex items-center gap-1.5 text-red-500 text-sm px-3 py-1.5 rounded-md hover:bg-red-50 border border-red-200 transition-colors disabled:opacity-50"
          >
            <Trash2 className="w-3.5 h-3.5" />
            {deletingProject ? '削除中...' : '削除'}
          </button>
        </div>
      }
    >
      <div className="h-full">
        {/* ReactFlow グラフ + サイドバー（空状態でもサイドバーを共有する） */}
        <div className="flex overflow-hidden bg-white h-full">
            {/* 左アイコンレール */}
            <div className="w-14 shrink-0 flex flex-col items-center pt-3 gap-2 border-r border-gray-100 bg-gray-50 z-10">
              <button
                onClick={() => { setShowDeploymentListSidebar(prev => !prev); setShowIngressListSidebar(false); setShowVolumeSidebar(false); setShowEnvVarSidebar(false); setShowImageSidebar(false) }}
                title="デプロイメント"
                className={`w-10 h-10 flex items-center justify-center rounded-lg transition-all ${
                  showDeploymentListSidebar
                    ? 'bg-[#00C2D1] text-white shadow-md'
                    : 'bg-cyan-50 text-[#00C2D1] hover:bg-cyan-100'
                }`}
              >
                <Layers className="w-5 h-5" />
              </button>
              <button
                onClick={() => { setShowIngressListSidebar(prev => !prev); setShowDeploymentListSidebar(false); setShowVolumeSidebar(false); setShowEnvVarSidebar(false); setShowImageSidebar(false) }}
                title="IngressRoute"
                className={`w-10 h-10 flex items-center justify-center rounded-lg transition-all ${
                  showIngressListSidebar
                    ? 'bg-indigo-500 text-white shadow-md'
                    : 'bg-indigo-50 text-indigo-500 hover:bg-indigo-100'
                }`}
              >
                <Globe className="w-5 h-5" />
              </button>
              <button
                onClick={() => { setShowVolumeSidebar(prev => !prev); setShowEnvVarSidebar(false); setShowDeploymentListSidebar(false); setShowIngressListSidebar(false); setShowImageSidebar(false) }}
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
                onClick={() => { setShowEnvVarSidebar(prev => !prev); setShowVolumeSidebar(false); setShowDeploymentListSidebar(false); setShowIngressListSidebar(false); setShowImageSidebar(false) }}
                title="環境変数"
                className={`w-10 h-10 flex items-center justify-center rounded-lg transition-all ${
                  showEnvVarSidebar
                    ? 'bg-purple-500 text-white shadow-md'
                    : 'bg-purple-50 text-purple-500 hover:bg-purple-100'
                }`}
              >
                <KeyRound className="w-5 h-5" />
              </button>
              <button
                onClick={() => { setShowImageSidebar(prev => !prev); setShowEnvVarSidebar(false); setShowVolumeSidebar(false); setShowDeploymentListSidebar(false); setShowIngressListSidebar(false) }}
                title="イメージ"
                className={`w-10 h-10 flex items-center justify-center rounded-lg transition-all ${
                  showImageSidebar
                    ? 'bg-emerald-500 text-white shadow-md'
                    : 'bg-emerald-50 text-emerald-500 hover:bg-emerald-100'
                }`}
              >
                <Package className="w-5 h-5" />
              </button>
            </div>

            {/* デプロイメント一覧サイドバー（左から開く） */}
            {showDeploymentListSidebar && (
              <div className="w-72 shrink-0 flex flex-col border-r border-gray-200 bg-white z-10">
                <DeploymentListSidebar
                  deploymentRelations={deploymentRelations}
                  onSelect={(deploymentId: string) => {
                    setSelectedDeploymentId(deploymentId) // 選択したデプロイメントIDを設定する
                    setSidebarMode('deployment') // デプロイメントサイドバーを開く
                    setIframeLoaded(false) // iframe読み込みフラグをリセットする
                    setIframeError(false) // iframeエラーフラグをリセットする
                    // チュートリアル中にカードをクリックしたらステップを進める
                    if (tutorialIsActive && (tutorialActualStep?.id === 'deployment-open-card' || tutorialActualStep?.id === 'adv-storage-open-card' || tutorialActualStep?.id === 'adv-envvar-open-card')) {
                      tutorialAdvance()
                    }
                  }}
                  selectedDeploymentId={selectedDeploymentId}
                  onClose={() => setShowDeploymentListSidebar(false)}
                  onAddNew={openDeploymentNewSidebar} // サイドバーで新規作成フォームを開く
                />
              </div>
            )}

            {/* IngressRoute一覧サイドバー（左から開く） */}
            {showIngressListSidebar && (
              <div className="w-72 shrink-0 flex flex-col border-r border-gray-200 bg-white z-10">
                <IngressListSidebar
                  ingressRouteList={ingressRouteList}
                  pathRulesByIngressRouteId={pathRulesByIngressRouteId}
                  selectedIngressRouteId={selectedIngressRouteId}
                  onSelect={openIngressSidebar}
                  onCreateIngress={handleOpenCreateIngressDialog}
                  creatingIngress={creatingIngress}
                  onClose={() => setShowIngressListSidebar(false)}
                />
              </div>
            )}

            {/* ボリュームサイドバー（左から開く） */}
            {showVolumeSidebar && (
              <div className="w-72 shrink-0 flex flex-col border-r border-gray-200 bg-white z-10">
                <VolumeSidebar
                  projectId={projectId!}
                  volumeList={volumeList}
                  deletingVolumeId={deletingVolumeId}
                  onDelete={async (volumeId: string) => { setDeleteVolumeConfirmId(volumeId) }} // 確認ダイアログを開く
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
                  onDelete={async (envVarId: string) => { setDeleteEnvVarConfirmId(envVarId) }} // 確認ダイアログを開く
                  onRefresh={fetchData}
                  onClose={() => setShowEnvVarSidebar(false)}
                />
              </div>
            )}

            {/* イメージサイドバー（左から開く） */}
            {showImageSidebar && (
              <div className="w-96 shrink-0 flex flex-col border-r border-gray-200 bg-white z-10">
                <ImageSidebar
                  projectId={projectId!}
                  imageList={imageList}
                  projectQuota={projectQuota}
                  onClose={() => setShowImageSidebar(false)}
                  onImageDeleted={(imageId) => setImageList(prev => prev.filter(i => i.id !== imageId))}
                  onDeployFromImage={(imageUrl) => {
                    setShowImageSidebar(false) // イメージサイドバーを閉じる
                    openDeploymentNewSidebar(new URLSearchParams({ image_url: imageUrl }).toString()) // デプロイメント作成サイドバーを開く
                  }}
                />
              </div>
            )}

            {/* 中央コンテンツ：空状態 or ReactFlow */}
            <div className="flex-1 min-w-0">
              {deploymentRelations.length === 0 ? (
                <div className="h-full flex items-center justify-center">
                  <div className="text-center">
                    <p className="text-sm font-medium text-gray-500 mb-1">まだデプロイメントがありません</p>
                    <p className="text-xs text-gray-400 mb-4">最初のアプリケーションをデプロイしましょう</p>
                    <button
                      onClick={() => openDeploymentNewSidebar()}
                      className="inline-flex items-center gap-1.5 bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors"
                    >
                      <Plus className="w-3.5 h-3.5" />
                      デプロイ
                    </button>
                  </div>
                </div>
              ) : (
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
              )}
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
                <div ref={sidebarElRef} className="shrink-0 flex flex-col border-l border-gray-200 overflow-hidden" style={{ width: sidebarWidth.current }}>
                  {/* Deployment 詳細・作成フォーム（iframe） */}
                  {(sidebarMode === 'deployment' || sidebarMode === 'deployment-new') && sidebarIframeSrc && (
                    <>
                      <div className="h-10 flex items-center justify-between px-3 border-b border-gray-100 bg-gray-50 shrink-0">
                        <span className="text-xs font-medium text-gray-500">
                          {sidebarMode === 'deployment-new'
                            ? '新規デプロイメント'
                            : (deploymentRelations.find(rel => rel.deployment.id === selectedDeploymentId)?.deployment.name ?? 'デプロイメント詳細')
                          }
                        </span>
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => sidebarNavigatePath && navigate(sidebarNavigatePath)}
                            className="text-xs text-[#00C2D1] hover:underline px-1"
                          >
                            全画面で開く
                          </button>
                          <button
                            data-tutorial="tutorial-deployment-close-btn"
                            onClick={() => {
                              handleCloseSidebar() // サイドバーを閉じる
                              // チュートリアル中なら×で閉じたタイミングでステップを進める
                              if (tutorialIsActive && tutorialActualStep?.id === 'deployment-close-sidebar') {
                                tutorialAdvance()
                              }
                            }}
                            className="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors"
                          >
                            <X className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </div>
                      <div className="flex-1 relative min-h-0 overflow-hidden">
                        {!iframeLoaded && !iframeError && (
                          <div className="absolute inset-0 flex items-center justify-center bg-white z-10">
                            <div className="w-6 h-6 border-2 border-[#00C2D1] border-t-transparent rounded-full animate-spin" />
                          </div>
                        )}
                        {iframeError && (
                          <div className="absolute inset-0 flex flex-col items-center justify-center bg-white z-10 gap-3">
                            <p className="text-sm text-gray-500">読み込みに失敗しました</p>
                            <button
                              onClick={() => {
                                if (iframeLoadTimeoutRef.current) clearTimeout(iframeLoadTimeoutRef.current) // 既存タイムアウトをクリアする
                                setIframeError(false) // エラーフラグをリセットする
                                setIframeLoaded(false) // ローディング状態に戻す
                              }}
                              className="text-xs px-3 py-1.5 bg-[#00C2D1] text-white rounded hover:bg-[#00A8B5] transition-colors"
                            >
                              再読み込み
                            </button>
                          </div>
                        )}
                        {/* リサイズ中は iframe 上にオーバーレイを被せてマウスイベントの横取りを防ぐ */}
                        {isResizing && (
                          <div className="absolute inset-0 z-20" />
                        )}
                        <iframe
                          key={iframeError ? `${sidebarMode === 'deployment-new' ? 'deployment-new' : selectedDeploymentId}-retry-${Date.now()}` : (sidebarMode === 'deployment-new' ? 'deployment-new' : selectedDeploymentId)}
                          src={sidebarIframeSrc}
                          className="w-full h-full border-none"
                          title={sidebarMode === 'deployment-new' ? '新規デプロイメント' : 'デプロイメント詳細'}
                          onLoad={() => {
                            if (iframeLoadTimeoutRef.current) clearTimeout(iframeLoadTimeoutRef.current) // タイムアウトをクリアする
                            setIframeLoaded(true) // ロード完了フラグをセットする
                          }}
                          ref={(iframeEl) => {
                            if (iframeEl && !iframeLoaded && !iframeError) {
                              if (iframeLoadTimeoutRef.current) clearTimeout(iframeLoadTimeoutRef.current) // 既存タイムアウトをクリアする
                              iframeLoadTimeoutRef.current = setTimeout(() => {
                                setIframeError(true) // 10秒以内にロードされなければエラー扱いにする
                              }, 10000)
                            }
                          }}
                        />
                      </div>
                    </>
                  )}

                  {/* IngressRoute サイドバー */}
                  {sidebarMode === 'ingress' && selectedIngressRouteId && (() => {
                    const selectedIngressRoute = ingressRouteList.find(ir => ir.id === selectedIngressRouteId) // 選択中の IngressRoute を取得する
                    if (!selectedIngressRoute) return null
                    return (
                      <IngressRouteSidebar
                        projectId={projectId!}
                        ingressRoute={selectedIngressRoute}
                        pathRules={pathRulesByIngressRouteId[selectedIngressRouteId] ?? []}
                        deploymentRelations={deploymentRelations}
                        onRefresh={fetchData}
                        onClose={handleCloseSidebar}
                      />
                    )
                  })()}
                </div>
              </>
            )}
          </div>
      </div>

    </Layout>

    {/* プロジェクト一括Apply詳細ダイアログ */}
    <ConfirmDialog
      open={applyProjectDetailsOpen}
      onOpenChange={setApplyProjectDetailsOpen}
      title="保留中の変更"
      description={`Deployment ${pendingSummary?.pending_deployment_count ?? 0}件、IngressRoute ${pendingSummary?.pending_ingress_route_count ?? 0}件の変更が保留中です。\nApply を実行すると Kubernetes に反映され、実行中のアプリケーションに影響する場合があります。`}
      confirmLabel="Deploy"
      variant="default"
      loading={applyingProject}
      onConfirm={async () => {
        await handleApplyProject() // プロジェクトを一括applyする
        setApplyProjectDetailsOpen(false) // ダイアログを閉じる
      }}
    />

    {/* プロジェクト一括Applyフローティングバー */}
    {(pendingSummary?.has_pending || applyingProject) && ( // pendingが1件以上ある場合、またはApply完了待機中は表示する
      <ProjectApplyBar
        pendingDeploymentCount={pendingSummary?.pending_deployment_count ?? 0}
        pendingIngressRouteCount={pendingSummary?.pending_ingress_route_count ?? 0}
        applying={applyingProject}
        progress={applyProjectProgress}
        onApply={handleApplyProject}
        onShowDetails={() => setApplyProjectDetailsOpen(true)}
      />
    )}

    {/* プロジェクト削除確認ダイアログ */}
    <ConfirmDialog
      open={deleteProjectConfirmOpen}
      onOpenChange={setDeleteProjectConfirmOpen}
      title="プロジェクトを削除"
      description={`プロジェクト「${project?.name}」を削除しますか？\nこの操作は取り消せません。`}
      confirmLabel="削除"
      variant="destructive"
      onConfirm={async () => {
        setDeleteProjectConfirmOpen(false) // ダイアログを閉じる
        await handleDeleteProject() // プロジェクトを削除する
      }}
    />

    {/* ボリューム削除確認ダイアログ */}
    <ConfirmDialog
      open={deleteVolumeConfirmId !== null}
      onOpenChange={open => { if (!open) setDeleteVolumeConfirmId(null) }} // ダイアログを閉じる
      title="ボリュームを削除"
      description="ボリュームを削除しますか？この操作は取り消せません。"
      confirmLabel="削除"
      variant="destructive"
      onConfirm={async () => {
        const targetId = deleteVolumeConfirmId // 削除対象IDを保持する
        setDeleteVolumeConfirmId(null) // ダイアログを閉じる
        if (targetId) await handleDeleteVolume(targetId) // ボリュームを削除する
      }}
    />

    {/* 環境変数削除確認ダイアログ */}
    <ConfirmDialog
      open={deleteEnvVarConfirmId !== null}
      onOpenChange={open => { if (!open) setDeleteEnvVarConfirmId(null) }} // ダイアログを閉じる
      title="環境変数を削除"
      description="環境変数を削除しますか？この操作は取り消せません。"
      confirmLabel="削除"
      variant="destructive"
      onConfirm={async () => {
        const targetId = deleteEnvVarConfirmId // 削除対象IDを保持する
        setDeleteEnvVarConfirmId(null) // ダイアログを閉じる
        if (targetId) await handleDeleteEnvVar(targetId) // 環境変数を削除する
      }}
    />
    {/* IngressRoute作成ダイアログ */}
    {createIngressDialogOpen && (
      <div className="fixed inset-0 z-50 flex items-center justify-center">
        <div className="absolute inset-0 bg-black/40" onClick={handleCancelCreateIngressDialog} />
        <div className="relative bg-white rounded-xl shadow-xl w-80 p-5 space-y-4">
          <div className="space-y-1">
            <h2 className="text-sm font-semibold text-[#111827]">IngressRoute を作成</h2>
            <p className="text-xs text-gray-500">名前を入力します（省略可）。省略するとプロジェクト名から自動生成されます。</p>
          </div>
          <div className="space-y-1">
            <label className="block text-xs font-medium text-gray-500">名前（英小文字・数字・ハイフン、最大20文字）</label>
            <input
              type="text"
              value={createIngressNameInput}
              onChange={ev => setCreateIngressNameInput(ev.target.value)}
              placeholder="my-api（省略可）"
              maxLength={20}
              className="w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-[#00C2D1]/50 focus:border-[#00C2D1] transition-colors"
              autoFocus
              onKeyDown={ev => { if (ev.key === 'Enter') void handleConfirmCreateIngress() }} // Enter で確定する
            />
          </div>
          <div className="flex items-center justify-end gap-2">
            <button
              onClick={handleCancelCreateIngressDialog}
              className="text-sm text-gray-500 hover:text-gray-700 px-3 py-1.5 rounded hover:bg-gray-100 transition-colors"
            >
              キャンセル
            </button>
            <button
              onClick={() => void handleConfirmCreateIngress()}
              disabled={creatingIngress}
              className="text-sm bg-[#111827] text-white px-4 py-1.5 rounded hover:bg-gray-800 transition-colors disabled:opacity-50"
            >
              {creatingIngress ? '作成中...' : '作成'}
            </button>
          </div>
        </div>
      </div>
    )}
    </>
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
  const { advance: tutorialAdvance, isActive: tutorialIsActive, actualStep: tutorialActualStep } = useTutorialContext() // チュートリアルの状態を取得する

  const [activeTab, setActiveTab] = useState<IngressTab>('overview') // アクティブなタブを管理する
  const [applying, setApplying] = useState(false) // apply 中フラグ
  const [deleting, setDeleting] = useState(false) // 削除中フラグ
  const [deleteIngressConfirmOpen, setDeleteIngressConfirmOpen] = useState(false) // IngressRoute削除確認ダイアログの表示フラグ

  const hasPending = !!ingressRoute.pending_name || pathRules.some(pr => pr.status === 'pending' || pr.status === 'deleting') // 保留中の変更があるかどうか

  const handleApply = async () => {
    setApplying(true) // apply 中フラグを立てる
    try {
      await post(`/projects/${projectId}/apply`) // IngressRoute を k8s に apply する
      await onRefresh() // データを再取得する
      // チュートリアル中なら Apply 完了で完了ステップへ進む
      if (tutorialIsActive && tutorialActualStep?.id === 'ingress-apply') {
        tutorialAdvance()
      }
    } catch (applyError) {
      console.error(applyError)
      toast.error(applyError instanceof Error ? applyError.message : 'Apply に失敗しました') // エラートーストを表示する
    } finally {
      setApplying(false) // apply 中フラグを下げる
    }
  }

  const handleDelete = async () => {
    setDeleting(true) // 削除中フラグを立てる
    try {
      await del(`/ingress-routes/${ingressRoute.id}`) // IngressRoute を deleting 状態にする
      await post(`/projects/${projectId}/apply`) // Apply して k8s から削除・DB レコードも物理削除する
      await onRefresh() // データを再取得する
      onClose() // サイドバーを閉じる
    } catch (deleteError) {
      console.error(deleteError)
      toast.error(deleteError instanceof Error ? deleteError.message : 'IngressRoute の削除に失敗しました') // エラートーストを表示する
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
            data-tutorial="tutorial-ingress-apply-btn"
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
            onClick={() => setDeleteIngressConfirmOpen(true)} // IngressRoute削除確認ダイアログを開く
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
              data-tutorial={tab === 'paths' ? 'tutorial-ingress-paths-tab' : undefined}
              onClick={() => {
                setActiveTab(tab) // タブを切り替える
                // チュートリアル中にパスルールタブをクリックしたらステップを進める
                if (tab === 'paths' && tutorialIsActive && tutorialActualStep?.id === 'ingress-paths-tab') {
                  tutorialAdvance()
                }
              }}
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
          <IngressOverviewTab ingressRoute={ingressRoute} onRefresh={onRefresh} />
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

      {/* IngressRoute削除確認ダイアログ */}
      <ConfirmDialog
        open={deleteIngressConfirmOpen}
        onOpenChange={setDeleteIngressConfirmOpen}
        title="IngressRoute を削除"
        description="IngressRoute を削除しますか？k8s からも即時削除されます。"
        confirmLabel="削除"
        variant="destructive"
        onConfirm={async () => {
          setDeleteIngressConfirmOpen(false) // ダイアログを閉じる
          await handleDelete() // IngressRoute を削除する
        }}
      />
    </div>
  )
}

// ── 概要タブ ──────────────────────────────────────────────────

function IngressOverviewTab({ ingressRoute, onRefresh }: { ingressRoute: IngressRoute; onRefresh: () => Promise<void> }) {
  const [copied, setCopied] = useState(false) // コピー完了フラグ
  const [editingName, setEditingName] = useState(false) // 名前編集モードフラグ
  const [nameInput, setNameInput] = useState('') // 名前入力値
  const [savingName, setSavingName] = useState(false) // 名前保存中フラグ

  const handleCopyHost = async () => {
    await navigator.clipboard.writeText(ingressRoute.host) // ホスト名をクリップボードにコピーする
    setCopied(true)
    setTimeout(() => setCopied(false), 2000) // 2秒後にリセットする
  }

  const handleStartEditName = () => {
    setNameInput(ingressRoute.pending_name || ingressRoute.name) // 現在の名前をフォームに設定する
    setEditingName(true) // 編集モードを開始する
  }

  const handleSaveName = async () => {
    if (!nameInput.trim()) return
    setSavingName(true) // 保存中フラグを立てる
    try {
      await patch(`/ingress-routes/${ingressRoute.id}/name`, { name: nameInput.trim() }) // 名前変更APIを呼び出す
      await onRefresh() // データを再取得する
      setEditingName(false) // 編集モードを終了する
      toast.success('名前を変更しました。Apply すると反映されます。') // 成功トーストを表示する
    } catch (saveError) {
      toast.error(saveError instanceof Error ? saveError.message : '名前の変更に失敗しました') // エラートーストを表示する
    } finally {
      setSavingName(false) // 保存中フラグを下げる
    }
  }

  return (
    <div className="p-4 space-y-4">
      <div className="space-y-3">
        <Row label="ステータス"><StatusBadge status={ingressRoute.status} /></Row>

        {/* 名前フィールド */}
        <div className="flex items-start justify-between gap-4 text-sm">
          <span className="text-gray-400 shrink-0">名前</span>
          <div className="text-right min-w-0 flex-1">
            {editingName ? (
              <div className="flex items-center gap-1.5">
                <input
                  type="text"
                  value={nameInput}
                  onChange={ev => setNameInput(ev.target.value)}
                  placeholder="my-api"
                  maxLength={20}
                  className="flex-1 min-w-0 rounded border border-gray-200 px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-[#00C2D1]"
                  autoFocus
                  onKeyDown={ev => { if (ev.key === 'Enter') void handleSaveName() }} // Enter で保存する
                />
                <button
                  onClick={() => void handleSaveName()}
                  disabled={savingName || !nameInput.trim()}
                  className="text-[10px] bg-[#111827] text-white px-2 py-1 rounded hover:bg-gray-800 disabled:opacity-50 shrink-0"
                >
                  {savingName ? '...' : '保存'}
                </button>
                <button
                  onClick={() => setEditingName(false)}
                  className="text-[10px] text-gray-400 hover:text-gray-600 px-1 shrink-0"
                >
                  <X className="w-3 h-3" />
                </button>
              </div>
            ) : (
              <div className="flex items-center gap-1 justify-end">
                <span className="font-mono text-sm text-[#111827] truncate">{ingressRoute.name || '(未設定)'}</span>
                {ingressRoute.pending_name && (
                  <span className="text-[10px] bg-amber-50 text-amber-600 border border-amber-200 px-1.5 py-0.5 rounded shrink-0">
                    → {ingressRoute.pending_name}
                  </span>
                )}
                <button
                  onClick={handleStartEditName}
                  className="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors shrink-0"
                  title="名前を変更"
                >
                  <Check className="w-3.5 h-3.5" />
                </button>
              </div>
            )}
          </div>
        </div>

        <Row label="ホスト">
          <div data-tutorial="tutorial-ingress-host" className="flex items-center gap-1 min-w-0">
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
  const { advance: tutorialAdvance, isActive: tutorialIsActive, actualStep: tutorialActualStep } = useTutorialContext() // チュートリアルの状態を取得する

  const [deletingPathRuleId, setDeletingPathRuleId] = useState<string | null>(null) // 削除中のパスルールID
  const [pathPrefix, setPathPrefix] = useState('/') // 追加するパスプレフィックス
  const [serviceId, setServiceId] = useState('') // 追加する転送先サービスID
  const [stripPrefix, setStripPrefix] = useState(false) // strip_prefix フラグ
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
      toast.error(deleteError instanceof Error ? deleteError.message : 'パスルールの削除に失敗しました') // エラートーストを表示する
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
        strip_prefix: stripPrefix,
      }) // パスルールを追加する
      setPathPrefix('/') // フォームをリセットする
      setServiceId('') // 選択をリセットする
      setStripPrefix(false) // strip_prefix フラグをリセットする
      await onRefresh() // データを再取得する
      // チュートリアル中ならパスルール追加完了でステップを進める
      if (tutorialIsActive && (tutorialActualStep?.id === 'ingress-service-select' || tutorialActualStep?.id === 'ingress-add-path-rule')) {
        tutorialAdvance()
      }
    } catch (addError) {
      console.error(addError)
      toast.error(addError instanceof Error ? addError.message : 'パスルールの追加に失敗しました') // エラートーストを表示する
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
                  {pathRule.strip_prefix && (
                    <span className="ml-2 text-[10px] bg-[#00C2D1]/10 text-[#00C2D1] px-1.5 py-0.5 rounded font-medium">strip</span>
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
                data-tutorial="tutorial-ingress-service-select"
                value={serviceId}
                onChange={ev => {
                  setServiceId(ev.target.value) // 転送先 Service を選択する
                  // チュートリアル中に Service を選択したらステップを進める
                  if (ev.target.value && tutorialIsActive && tutorialActualStep?.id === 'ingress-service-select') {
                    tutorialAdvance()
                  }
                }}
                className={inputClass}
              >
                <option value="">デプロイメントを選択</option>
                {servicesWithLabel.map(svc => (
                  <option key={svc.id} value={svc.id}>{svc.label}</option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="strip-prefix-checkbox"
                checked={stripPrefix}
                onChange={ev => setStripPrefix(ev.target.checked)}
                className="w-4 h-4 rounded border-gray-300 text-[#00C2D1] focus:ring-[#00C2D1]/50 cursor-pointer"
              />
              <label htmlFor="strip-prefix-checkbox" className="text-sm text-gray-600 cursor-pointer select-none">
                パスプレフィックスを strip する
              </label>
            </div>
            <button
              data-tutorial="tutorial-ingress-add-path-rule"
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
  const { advance: tutorialAdvance, actualStep: tutorialActualStep } = useTutorialContext() // チュートリアルの進行関数を取得する
  const [newKey, setNewKey] = useState('') // 新規環境変数キー
  const [newValue, setNewValue] = useState('') // 新規環境変数値
  const [newIsSecret, setNewIsSecret] = useState(false) // シークレットフラグ
  const [adding, setAdding] = useState(false) // 作成中フラグ
  const [editingId, setEditingId] = useState<string | null>(null) // 編集中の環境変数ID
  const [editValue, setEditValue] = useState('') // 編集中の値
  const [savingId, setSavingId] = useState<string | null>(null) // 保存中の環境変数ID
  const [showValues, setShowValues] = useState<Set<string>>(new Set()) // 値を表示中のID一覧
  const [copiedId, setCopiedId] = useState<string | null>(null) // コピー完了フィードバック用ID

  const handleCopy = async (envVar: EnvVar) => {
    if (!envVar.value) { // 値が空（バックエンドがマスクして空を返す場合）はアラートを表示する
      toast.info('値が取得できません（シークレット値はバックエンドによってマスクされています）') // インフォトーストを表示する
      return
    }
    await navigator.clipboard.writeText(envVar.value) // クリップボードに書き込む
    setCopiedId(envVar.id) // コピー完了フィードバックを表示する
    setTimeout(() => setCopiedId(null), 1500) // 1.5秒後にフィードバックをリセットする
  }

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
      if (tutorialActualStep?.id === 'adv-envvar-create') tutorialAdvance() // 環境変数作成後にチュートリアルを進める
    } catch (addError) {
      console.error(addError)
      toast.error(addError instanceof Error ? addError.message : '環境変数の作成に失敗しました') // エラートーストを表示する
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
      toast.error(saveError instanceof Error ? saveError.message : '環境変数の更新に失敗しました') // エラートーストを表示する
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
                      <>
                        <button
                          onClick={() => void handleCopy(envVar)}
                          className="p-1 rounded text-gray-300 hover:text-gray-500 hover:bg-gray-200 transition-colors"
                          title="値をコピー"
                        >
                          {copiedId === envVar.id ? <Check className="w-3 h-3 text-green-500" /> : <Copy className="w-3 h-3" />}
                        </button>
                        <button
                          onClick={() => handleStartEdit(envVar)}
                          className="p-1 rounded text-gray-300 hover:text-gray-500 hover:bg-gray-200 transition-colors"
                          title="編集"
                        >
                          <Check className="w-3 h-3" />
                        </button>
                      </>
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
              data-tutorial="tutorial-envvar-key-input"
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
              data-tutorial="tutorial-envvar-value-input"
              type={newIsSecret ? 'password' : 'text'}
              value={newValue}
              onChange={ev => setNewValue(ev.target.value)}
              placeholder="値を入力"
              className={inputClass}
            />
          </div>
          <label data-tutorial="tutorial-envvar-secret-checkbox" className="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={newIsSecret}
              onChange={ev => setNewIsSecret(ev.target.checked)}
              className="rounded border-gray-300 text-[#00C2D1] focus:ring-[#00C2D1]"
            />
            <span className="text-xs text-gray-600">シークレット（k8s Secret に格納）</span>
          </label>
          <button
            data-tutorial="tutorial-envvar-create-btn"
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
  const { advance: tutorialAdvance, actualStep: tutorialActualStep } = useTutorialContext() // チュートリアルの進行関数を取得する
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
      if (tutorialActualStep?.id === 'adv-storage-create') tutorialAdvance() // ボリューム作成後にチュートリアルを進める
    } catch (addError) {
      console.error(addError)
      toast.error(addError instanceof Error ? addError.message : 'ボリュームの作成に失敗しました') // エラートーストを表示する
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
                    <StatusBadge status={vol.status} size="sm" />
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
              data-tutorial="tutorial-volume-name-input"
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
              data-tutorial="tutorial-volume-size-input"
              type="number"
              min={1}
              value={newVolumeSizeMb}
              onChange={ev => setNewVolumeSizeMb(ev.target.value)}
              placeholder="1024"
              className={inputClass}
            />
          </div>
          <button
            data-tutorial="tutorial-volume-create-btn"
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

// ── DeploymentListSidebar ─────────────────────────────────────

function DeploymentListSidebar({
  deploymentRelations,
  onSelect,
  selectedDeploymentId,
  onClose,
  onAddNew,
}: {
  deploymentRelations: DeploymentWithRelations[]
  onSelect: (deploymentId: string) => void
  selectedDeploymentId: string | null
  onClose: () => void
  onAddNew: () => void
}) {
  return (
    <div className="flex flex-col h-full">
      {/* ヘッダー */}
      <div className="h-10 flex items-center justify-between px-3 border-b border-gray-100 bg-gray-50 shrink-0">
        <div className="flex items-center gap-2">
          <Layers className="w-3.5 h-3.5 text-[#00C2D1] shrink-0" />
          <span className="text-xs font-medium text-[#111827]">デプロイメント</span>
          <span className="text-xs text-gray-400">{deploymentRelations.length}</span>
        </div>
        <button
          onClick={onClose}
          className="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>

      {/* 一覧 */}
      <div className="flex-1 overflow-y-auto p-3 space-y-1.5">
        {deploymentRelations.length === 0 ? (
          <p className="text-xs text-gray-400 py-2">デプロイメントがありません</p>
        ) : (
          deploymentRelations.map(({ deployment }) => (
            <button
              key={deployment.id}
              onClick={() => onSelect(deployment.id)} // クリックで右サイドバーに詳細を表示する
              className={`w-full flex items-center justify-between px-3 py-2.5 rounded-md border text-left transition-all ${
                selectedDeploymentId === deployment.id
                  ? 'border-[#00C2D1] bg-cyan-50'
                  : 'border-gray-100 bg-gray-50 hover:border-gray-200 hover:bg-gray-100'
              }`}
            >
              <div className="min-w-0 flex-1 space-y-0.5">
                <p className="text-sm font-medium text-[#111827] truncate">{deployment.name}</p>
                <StatusBadge status={deployment.status} />
              </div>
              <ChevronRight className="w-3.5 h-3.5 text-gray-300 shrink-0 ml-2" />
            </button>
          ))
        )}
      </div>

      {/* フッター：新規作成ボタン */}
      <div className="shrink-0 border-t border-gray-100 p-3">
        <button
          onClick={onAddNew} // サイドバーで新規デプロイメント作成フォームを開く
          className="w-full flex items-center justify-center gap-1.5 bg-[#111827] text-white text-sm py-2 rounded-md hover:bg-gray-800 transition-colors"
        >
          <Plus className="w-3.5 h-3.5" />
          新規デプロイメント
        </button>
      </div>
    </div>
  )
}

// ── IngressListSidebar ────────────────────────────────────────

function IngressListSidebar({
  ingressRouteList,
  pathRulesByIngressRouteId,
  selectedIngressRouteId,
  onSelect,
  onCreateIngress,
  creatingIngress,
  onClose,
}: {
  ingressRouteList: IngressRoute[]
  pathRulesByIngressRouteId: Record<string, PathRule[]>
  selectedIngressRouteId: string | null
  onSelect: (ingressRouteId: string) => void
  onCreateIngress: () => void
  creatingIngress: boolean
  onClose: () => void
}) {
  return (
    <div className="flex flex-col h-full">
      {/* ヘッダー */}
      <div className="h-10 flex items-center justify-between px-3 border-b border-gray-100 bg-gray-50 shrink-0">
        <div className="flex items-center gap-2">
          <Globe className="w-3.5 h-3.5 text-indigo-500 shrink-0" />
          <span className="text-xs font-medium text-[#111827]">IngressRoute</span>
          <span className="text-xs text-gray-400">{ingressRouteList.length}</span>
        </div>
        <button
          onClick={onClose}
          className="p-1 rounded hover:bg-gray-200 text-gray-400 hover:text-gray-600 transition-colors"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>

      {/* 一覧 */}
      <div className="flex-1 overflow-y-auto p-3 space-y-1.5">
        {ingressRouteList.length === 0 ? (
          <p className="text-xs text-gray-400 py-2">IngressRouteがありません</p>
        ) : (
          ingressRouteList.map(ingressRoute => {
            const ruleCount = (pathRulesByIngressRouteId[ingressRoute.id] ?? []).length // このIngressRouteのパスルール数を取得する
            return (
              <button
                key={ingressRoute.id}
                onClick={() => onSelect(ingressRoute.id)} // クリックで右サイドバーに詳細を表示する
                className={`w-full flex items-center justify-between px-3 py-2.5 rounded-md border text-left transition-all ${
                  selectedIngressRouteId === ingressRoute.id
                    ? 'border-indigo-400 bg-indigo-50'
                    : 'border-gray-100 bg-gray-50 hover:border-gray-200 hover:bg-gray-100'
                }`}
              >
                <div className="min-w-0 flex-1 space-y-0.5">
                  <p className="text-sm font-medium text-[#111827] truncate">
                    {ingressRoute.name || '(名前未設定)'}
                    {ingressRoute.pending_name && (
                      <span className="ml-1 text-[10px] text-amber-500">({ingressRoute.pending_name}に変更予定)</span>
                    )}
                  </p>
                  <p className="text-[10px] font-mono text-gray-400 truncate">{ingressRoute.host || '(ホスト未設定)'}</p>
                  <div className="flex items-center gap-2">
                    <StatusBadge status={ingressRoute.status} />
                    {ruleCount > 0 && (
                      <span className="text-[10px] text-gray-400">{ruleCount}ルール</span>
                    )}
                  </div>
                </div>
                <ChevronRight className="w-3.5 h-3.5 text-gray-300 shrink-0 ml-2" />
              </button>
            )
          })
        )}
      </div>

      {/* フッター：新規作成ボタン */}
      <div className="shrink-0 border-t border-gray-100 p-3">
        <button
          onClick={onCreateIngress} // 名前入力ダイアログを開く
          disabled={creatingIngress}
          className="w-full flex items-center justify-center gap-1.5 bg-[#111827] text-white text-sm py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
        >
          <Plus className="w-3.5 h-3.5" />
          {creatingIngress ? '作成中...' : '新規IngressRoute'}
        </button>
      </div>
    </div>
  )
}

// ── ImageSidebar ──────────────────────────────────────────────

function ImageSidebar({
  projectId,
  imageList,
  projectQuota,
  onClose,
  onImageDeleted,
  onDeployFromImage,
}: {
  projectId: string
  imageList: Image[]
  projectQuota: ProjectQuota | null
  onClose: () => void
  onImageDeleted: (imageId: string) => void
  onDeployFromImage: (imageUrl: string) => void
}) {
  const navigate = useNavigate()
  const [deletingImageId, setDeletingImageId] = useState<string | null>(null) // 削除中のイメージ ID を管理する
  const [deleteImageConfirmId, setDeleteImageConfirmId] = useState<string | null>(null) // イメージ削除確認ダイアログ対象ID
  const [copiedImageId, setCopiedImageId] = useState<string | null>(null) // URLコピー直後のイメージIDを管理する

  const handleCopyImageUrl = (image: Image) => {
    void navigator.clipboard.writeText(image.image_url) // イメージURLをクリップボードにコピーする
    setCopiedImageId(image.id) // コピー完了表示を出す
    setTimeout(() => setCopiedImageId(prev => (prev === image.id ? null : prev)), 1500) // 1.5秒後に表示を戻す
  }

  const buildStatusBorder: Record<string, string> = { // ビルドステータスに対応する左ボーダー色を定義する
    pending: 'border-l-gray-300',
    building: 'border-l-blue-400',
    succeeded: 'border-l-emerald-400',
    failed: 'border-l-red-400',
    cancelled: 'border-l-gray-300',
  }

  const buildStatusColor: Record<string, string> = { // ビルドステータスに対応する色を定義する
    pending: 'bg-gray-100 text-gray-500',
    building: 'bg-blue-100 text-blue-600',
    succeeded: 'bg-emerald-100 text-emerald-700',
    failed: 'bg-red-100 text-red-600',
    cancelled: 'bg-gray-100 text-gray-400',
  }

  const buildStatusLabel: Record<string, string> = { // ビルドステータスの日本語ラベルを定義する
    pending: '待機中',
    building: 'ビルド中',
    succeeded: '成功',
    failed: '失敗',
    cancelled: 'キャンセル',
  }

  const handleDeployFromImage = (image: Image) => {
    onDeployFromImage(image.image_url) // 親コンポーネントのサイドバーを開く
  }

  const handleDeleteImage = async (image: Image) => {
    setDeletingImageId(image.id) // 削除中フラグを立てる
    try {
      await del(`/projects/${projectId}/images/${image.id}`) // DELETE API を呼び出す
      onImageDeleted(image.id) // 親コンポーネントに削除を通知する
    } catch (deleteImageError) {
      console.error(deleteImageError)
      // 409（使用中）はメッセージを分けて分かりやすく表示する
      const isConflict = deleteImageError instanceof Error && deleteImageError.message.includes('409')
      toast.error(isConflict ? 'このイメージは使用中のため削除できません' : (deleteImageError instanceof Error ? deleteImageError.message : '削除に失敗しました')) // エラートーストを表示する
    } finally {
      setDeletingImageId(null) // 削除中フラグを解除する
    }
  }

  const formatBytes = (bytes: number): string => { // バイト数を人が読みやすい形式に変換する
    if (bytes <= 0) return ''
    if (bytes < 1_048_576) return `${(bytes / 1_024).toFixed(1)} KB`
    if (bytes < 1_073_741_824) return `${(bytes / 1_048_576).toFixed(1)} MB`
    return `${(bytes / 1_073_741_824).toFixed(2)} GB`
  }

  const usedGb = projectQuota ? (projectQuota.used_bytes / 1_073_741_824).toFixed(2) : null // 使用量をGBに変換する
  const limitGb = projectQuota ? (projectQuota.limit_bytes / 1_073_741_824).toFixed(1) : null // 上限をGBに変換する
  const usagePercent = projectQuota && projectQuota.limit_bytes > 0
    ? Math.min(100, Math.round((projectQuota.used_bytes / projectQuota.limit_bytes) * 100))
    : 0 // 使用率を計算する（最大100%）

  const targetImage = imageList.find(image => image.id === deleteImageConfirmId) // 削除確認ダイアログ対象のイメージを取得する
  const isExternalImage = targetImage ? targetImage.build_id === null : false // ビルド経由でない（外部URL直接指定）かどうかを判定する

  return (
    <div className="flex flex-col h-full">
      {/* ヘッダー */}
      <div className="h-10 flex items-center justify-between px-3 border-b border-gray-100 bg-gray-50 shrink-0">
        <div className="flex items-center gap-2">
          <Package className="w-3.5 h-3.5 text-emerald-500 shrink-0" />
          <span className="text-xs font-medium text-[#111827]">イメージ</span>
          <span className="text-xs text-gray-400">{imageList.length}</span>
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
        {/* Harbor ストレージクォータ */}
        {projectQuota && (
          <div className="bg-gray-50 border border-gray-100 rounded-md p-3 space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-gray-600">Harborストレージ使用量</span>
              <span className="text-xs text-gray-500">{usedGb} GB / {limitGb} GB</span>
            </div>
            <div className="w-full bg-gray-200 rounded-full h-2">
              <div
                className={`h-2 rounded-full transition-all ${usagePercent >= 90 ? 'bg-red-500' : usagePercent >= 70 ? 'bg-amber-500' : 'bg-emerald-500'}`}
                style={{ width: `${usagePercent}%` }}
              />
            </div>
            <p className="text-[10px] text-gray-400">{usagePercent}% 使用中</p>
          </div>
        )}

        {/* イメージ一覧 */}
        {imageList.length === 0 ? (
          <p className="text-xs text-gray-400 py-2">イメージがありません</p>
        ) : (
          <div className="space-y-2">
            {imageList.map(image => {
              const build = image.build // Preload されたビルド情報（存在しない場合は外部URL直接指定）
              const directory = build?.directory ? (build.directory === './' ? 'プロジェクトルート' : build.directory) : null // ./ はプロジェクトルートとして表示する
              const hasMetaRow = !!(build?.branch || build?.commit_sha || directory || build?.author) // メタ情報行を描画するか判定する
              return (
                <div
                  key={image.id}
                  className={`bg-white rounded-lg border border-gray-100 border-l-[3px] ${build ? (buildStatusBorder[build.status] ?? 'border-l-gray-300') : 'border-l-gray-300'} shadow-sm overflow-hidden`}
                >
                  {/* ヘッダー行: ステータス・サイズ・日時・削除 */}
                  <div className="flex items-center justify-between gap-2 px-3 pt-2.5">
                    <div className="flex items-center gap-1.5 min-w-0">
                      {build && (
                        <span className={`inline-flex items-center text-[10px] font-medium px-1.5 py-0.5 rounded shrink-0 ${buildStatusColor[build.status] ?? 'bg-gray-100 text-gray-500'}`}>
                          {buildStatusLabel[build.status] ?? build.status}
                        </span>
                      )}
                      {!build && (
                        <span className="inline-flex items-center text-[10px] font-medium px-1.5 py-0.5 rounded shrink-0 bg-gray-100 text-gray-500">外部イメージ</span>
                      )}
                      {image.size_bytes > 0 && (
                        <span className="text-[10px] text-gray-400 shrink-0">{formatBytes(image.size_bytes)}</span>
                      )}
                    </div>
                    <div className="flex items-center gap-0.5 shrink-0">
                      <span className="text-[10px] text-gray-400 mr-1">
                        {new Date(image.created_at).toLocaleDateString('ja-JP', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                      </span>
                      <button
                        onClick={() => setDeleteImageConfirmId(image.id)} // イメージ削除確認ダイアログを開く
                        disabled={deletingImageId === image.id}
                        className="p-1 rounded hover:bg-red-50 text-gray-300 hover:text-red-500 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
                        title="イメージを削除"
                      >
                        <Trash2 className="w-3 h-3" />
                      </button>
                    </div>
                  </div>

                  {/* 本文: コミットメッセージ */}
                  <div className="px-3 pt-1.5">
                    <p className="text-xs font-medium text-[#111827] truncate" title={build?.commit_message || image.image_url}>
                      {build?.commit_message || image.image_url}
                    </p>
                  </div>

                  {/* メタ情報行: ブランチ・SHA・ディレクトリ・作者 */}
                  {hasMetaRow && (
                    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 pt-1.5 text-[10px] text-gray-500">
                      {build?.branch && (
                        <span className="inline-flex items-center gap-1">
                          <GitBranch className="w-3 h-3 text-gray-300 shrink-0" />
                          {build.github_repo_url ? (
                            <a
                              href={`${build.github_repo_url}/tree/${build.branch}`}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="hover:underline hover:text-[#00C2D1] font-mono"
                            >
                              {build.branch}
                            </a>
                          ) : <span className="font-mono">{build.branch}</span>}
                        </span>
                      )}
                      {build?.commit_sha && (
                        <span className="inline-flex items-center gap-1">
                          <GitCommit className="w-3 h-3 text-gray-300 shrink-0" />
                          {build.github_repo_url ? (
                            <a
                              href={`${build.github_repo_url}/commit/${build.commit_sha}`}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="hover:underline hover:text-[#00C2D1] font-mono"
                            >
                              {build.commit_sha.slice(0, 7)}
                            </a>
                          ) : <span className="font-mono">{build.commit_sha.slice(0, 7)}</span>}
                        </span>
                      )}
                      {directory && (
                        <span className={`inline-flex items-center gap-1 ${directory === 'プロジェクトルート' ? '' : 'font-mono'}`} title={`ディレクトリ: ${directory}`}>
                          <FolderOpen className="w-3 h-3 text-gray-300 shrink-0" />
                          {directory}
                        </span>
                      )}
                      {build?.author && (
                        <span className="text-gray-400">{build.author}</span>
                      )}
                    </div>
                  )}

                  {/* イメージURL: truncate + コピー */}
                  {image.image_url && (
                    <div className="flex items-center gap-1 px-3 pt-1.5">
                      <p className="flex-1 min-w-0 text-[10px] text-gray-400 font-mono truncate" title={image.image_url}>{image.image_url}</p>
                      <button
                        onClick={() => handleCopyImageUrl(image)}
                        className="p-0.5 rounded hover:bg-gray-100 text-gray-300 hover:text-gray-500 transition-colors shrink-0"
                        title="URLをコピー"
                      >
                        {copiedImageId === image.id ? <Check className="w-3 h-3 text-emerald-500" /> : <Copy className="w-3 h-3" />}
                      </button>
                    </div>
                  )}

                  {/* アクション行: デプロイ・ログ */}
                  <div className="flex items-stretch gap-1.5 px-3 py-2.5 mt-1">
                    <button
                      onClick={() => handleDeployFromImage(image)}
                      className="flex-1 flex items-center justify-center gap-1.5 bg-emerald-600 hover:bg-emerald-700 text-white text-xs font-medium py-1.5 rounded-md transition-colors"
                    >
                      <Play className="w-3 h-3" />
                      このイメージでデプロイ
                    </button>
                    {image.build_id && (
                      <button
                        onClick={() => navigate(`/builds/${image.build_id}/logs`)}
                        className="flex items-center justify-center gap-1 px-2.5 rounded-md border border-gray-200 text-gray-500 hover:bg-gray-50 hover:text-gray-700 text-xs transition-colors"
                        title="ビルドログを見る"
                      >
                        <ScrollText className="w-3 h-3" />
                        ログ
                      </button>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* イメージ削除確認ダイアログ */}
      <ConfirmDialog
        open={deleteImageConfirmId !== null}
        onOpenChange={open => { if (!open) setDeleteImageConfirmId(null) }} // ダイアログを閉じる
        title="イメージを削除"
        description={
          isExternalImage
            ? 'このイメージを削除しますか？\nこのイメージはアプリ上の登録のみ削除されます。外部レジストリ上の実イメージは削除されません。'
            : 'このイメージを削除しますか？\nHarbor 上のイメージも削除されます。'
        }
        confirmLabel="削除"
        variant="destructive"
        onConfirm={async () => {
          setDeleteImageConfirmId(null) // ダイアログを閉じる
          if (targetImage) await handleDeleteImage(targetImage) // イメージを削除する
        }}
      />
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
