import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { zip } from 'fflate'
import { Upload, Loader2 } from 'lucide-react'
import { get, post, del, postMultipart, ApiError, QuotaExceededApiError } from '@/lib/api'
import type {
  Deployment,
  Project,
  Build,
  BuildLogsResponse,
  UploadBuildArchiveResponse,
  IngressRoute,
  ApplyProjectResult,
} from '@/lib/types'

type Phase = 'landing' | 'wizard' | 'building' | 'applying' | 'done' | 'failed'

type WizardStep = 1 | 2 | 3 | 4

type EnvVarRow = { key: string; value: string }

type BuilderType = 'railpack' | 'dockerfile'

const VOLUME_SIZE_OPTIONS_GB = [1, 2, 3, 5] // 保存容量の選択肢（GB、最大5GB）

const BUILD_LOG_POLL_INTERVAL = 1500 // ビルドログのポーリング間隔（ms）
const APPLY_POLL_INTERVAL = 3000 // apply完了待機のポーリング間隔（ms）
const APPLY_POLL_TIMEOUT = 180000 // apply完了待機のタイムアウト（ms）

const STORAGE_KEY = 'deploy-wizard-build-id' // ビルド開始後にリロードされた場合、ビルドログ画面へ戻すためのsessionStorageキー

function loadPersistedBuildId(): string | null {
  try {
    return sessionStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

function savePersistedBuildId(buildId: string): void {
  try {
    sessionStorage.setItem(STORAGE_KEY, buildId)
  } catch {
    // sessionStorageが使えない環境では状態復元機能を諦める
  }
}

function clearPersistedBuildId(): void {
  sessionStorage.removeItem(STORAGE_KEY)
}

// アプリ名からk8sリソース名相当のスラッグを生成する（DNS-1123ラベル準拠：英数字・ハイフンのみ、小文字、先頭末尾ハイフン禁止）
function slugify(input: string, maxLength = 20): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, maxLength)
    .replace(/-$/, '') // 切り詰めで末尾がハイフンになった場合を除去する
}

// プロジェクト名の一意制約に対応するため、末尾にランダムな8文字を付与する
function generateUniqueSuffix(): string {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  const bytes = crypto.getRandomValues(new Uint8Array(8))
  return Array.from(bytes).map((byte) => chars[byte % chars.length]).join('')
}

// キャンセル要求時にポーリングループを中断するために投げる専用エラー
class CancelledError extends Error {}

// エラー内容から、ユーザーが取れる対処法のヒントを判定する
function resolveFailureHint(deployError: unknown): string | null {
  if (deployError instanceof QuotaExceededApiError) {
    return 'プロジェクトやリソースの上限に達しています。ダッシュボードで不要なプロジェクト・デプロイメント・ボリュームなどを削除するか、プランのアップグレードをご検討ください。'
  }
  if (deployError instanceof ApiError && deployError.status === 409) {
    return '同じ名前のリソースが既に存在する可能性があります。しばらく待ってから再試行してください。'
  }
  if (deployError instanceof ApiError && deployError.status >= 500) {
    return 'サーバー側で問題が発生しました。時間をおいて再試行するか、解消しない場合はサポートへお問い合わせください。'
  }
  return null
}

export function DeployWizardPage() {
  const navigate = useNavigate()

  const cancelledRef = useRef(false) // キャンセル要求フラグ（各ポーリングループがこれを見て中断する）

  const [phaseState, setPhaseState] = useState<Phase>('landing') // 常にランディング（フォルダドロップ）から開始する
  const phase = phaseState
  // done/failedに到達したらビルド開始の永続化情報をクリアする（以降のリロードはランディングから始めればよいため）
  const setPhase = useCallback((nextPhase: Phase) => {
    if (nextPhase === 'done' || nextPhase === 'failed') {
      clearPersistedBuildId()
    }
    setPhaseState(nextPhase)
  }, [])
  const [dragOver, setDragOver] = useState(false) // ドラッグオーバー中の見た目切り替え用フラグ
  const fileInputRef = useRef<HTMLInputElement>(null) // フォルダ選択ダイアログを開くための隠しinput

  // 選択されたフォルダの情報
  const [folderName, setFolderName] = useState('') // 選択したフォルダ名
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]) // 選択したファイル一覧（zip化前の元データ）
  const [buildDirOptions, setBuildDirOptions] = useState<string[]>([]) // ビルドディレクトリのサジェスト一覧

  // ウィザード入力
  const [wizardStep, setWizardStep] = useState<WizardStep>(1) // 現在のウィザードステップ
  const [builder, setBuilder] = useState<BuilderType>('railpack') // ビルダー（archiveタイプは常にRailpackのみ対応）
  const [appName, setAppName] = useState('') // アプリの名前
  const [buildDirectory, setBuildDirectory] = useState('./') // ビルドディレクトリ
  const [portEnabled, setPortEnabled] = useState(false) // インターネット公開トグル
  const [port, setPort] = useState('3000') // ポート番号
  const [ingressName, setIngressName] = useState('') // 公開URLの名前（空ならアプリ名から自動生成）
  const [envRows, setEnvRows] = useState<EnvVarRow[]>([]) // 環境変数の行一覧
  const [volumeEnabled, setVolumeEnabled] = useState(false) // データ保存トグル
  const [volumeSizeGb, setVolumeSizeGb] = useState(1) // 保存容量（GB）
  const [mountPath, setMountPath] = useState('/data') // 保存先フォルダ

  const [wizardError, setWizardError] = useState<string | null>(null) // ウィザード入力・送信時のエラーメッセージ
  const [cancelling, setCancelling] = useState(false) // キャンセル処理中フラグ

  // デプロイ進行状態
  const [deploymentId, setDeploymentId] = useState<string | null>(null) // 作成済みデプロイメントID
  const [projectId, setProjectId] = useState<string | null>(null) // 作成済みプロジェクトID
  const [buildId, setBuildId] = useState<string | null>(null) // 開始したビルドID
  const [buildLogs, setBuildLogs] = useState('') // ビルドログ全文
  const [applyChecklist, setApplyChecklist] = useState<{ label: string; done: boolean }[]>([]) // 反映フェーズのチェックリスト
  const [resultHost, setResultHost] = useState<string | null>(null) // 完了後の稼働URL（ホスト名）
  const [failureMessage, setFailureMessage] = useState<string | null>(null) // 失敗時のエラーメッセージ
  const [failureHint, setFailureHint] = useState<string | null>(null) // 失敗時の対処ヒント（原因が特定できる場合のみ）
  const [failureStage, setFailureStage] = useState<string | null>(null) // 失敗したフェーズ名（ビルド中／反映中など）

  const logPanelRef = useRef<HTMLDivElement>(null) // ログパネルの自動スクロール用

  useEffect(() => {
    if (logPanelRef.current) {
      logPanelRef.current.scrollTop = logPanelRef.current.scrollHeight // 常に最下部へスクロールする
    }
  }, [buildLogs])

  // ビルド開始後（k8s Jobが走った後）にリロードされた場合、ビルドログ画面へ遷移する。
  // それ以前（landing/wizard）のリロードは常にランディング画面から始めればよいので何もしない。
  useEffect(() => {
    const persistedBuildId = loadPersistedBuildId()
    if (persistedBuildId) {
      navigate(`/builds/${persistedBuildId}/logs`, { state: { returnToWizard: true } })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // 選択されたフォルダからフォルダ名・トップレベルサブフォルダ一覧を読み取る
  const handleFilesSelected = useCallback((fileList: FileList | null) => {
    if (!fileList || fileList.length === 0) return
    const files = Array.from(fileList)
    const firstPath = files[0].webkitRelativePath || files[0].name
    const rootName = firstPath.split('/')[0] // フォルダ名（先頭ディレクトリ）を抽出する

    const topLevelDirs = new Set<string>()
    for (const file of files) {
      const relPath = file.webkitRelativePath || file.name
      const parts = relPath.split('/') // rootName/sub/.../file.ext の形式
      if (parts.length > 2) {
        topLevelDirs.add(`./${parts[1]}`) // トップレベルのサブフォルダのみを候補にする
      }
    }

    setFolderName(rootName)
    setSelectedFiles(files)
    setBuildDirOptions(['./', ...Array.from(topLevelDirs).sort()])
    setAppName((prev) => prev || rootName)
    setBuildDirectory('./')
    setWizardStep(1)
    setWizardError(null)
    setPhase('wizard')
  }, [setPhase])

  const handleLandingClick = () => fileInputRef.current?.click()

  const handleDragOver = (event: React.DragEvent) => {
    event.preventDefault()
    setDragOver(true)
  }

  const handleDragLeave = () => setDragOver(false)

  // ドロップされたディレクトリエントリを再帰的に走査してFileの配列を得る
  const readEntry = async (entry: FileSystemEntry, path: string): Promise<File[]> => {
    if (entry.isFile) {
      const fileEntry = entry as FileSystemFileEntry
      const file: File = await new Promise((resolve, reject) => fileEntry.file(resolve, reject))
      Object.defineProperty(file, 'webkitRelativePath', { value: `${path}${entry.name}` }) // 相対パスを付与する
      return [file]
    }
    const dirEntry = entry as FileSystemDirectoryEntry
    const reader = dirEntry.createReader()
    const entries: FileSystemEntry[] = await new Promise((resolve, reject) => reader.readEntries(resolve, reject))
    const nested = await Promise.all(entries.map((child) => readEntry(child, `${path}${entry.name}/`)))
    return nested.flat()
  }

  const handleDrop = async (event: React.DragEvent) => {
    event.preventDefault()
    setDragOver(false)
    const items = event.dataTransfer.items
    if (!items || items.length === 0) return
    const entries = Array.from(items)
      .map((item) => item.webkitGetAsEntry())
      .filter((entry): entry is FileSystemEntry => entry !== null)
    const nestedFiles = await Promise.all(entries.map((entry) => readEntry(entry, '')))
    const files = nestedFiles.flat()
    if (files.length === 0) return
    // FileList相当のダミーを構築するためDataTransferを使う
    const dt = new DataTransfer()
    files.forEach((file) => dt.items.add(file))
    handleFilesSelected(dt.files)
  }

  // 環境変数の行操作
  const addEnvRow = () => setEnvRows((prev) => [...prev, { key: '', value: '' }])
  const removeEnvRow = (index: number) => setEnvRows((prev) => prev.filter((_, i) => i !== index))
  const updateEnvRow = (index: number, field: 'key' | 'value', value: string) => {
    setEnvRows((prev) => prev.map((row, i) => (i === index ? { ...row, [field]: value } : row)))
  }

  const ingressNamePlaceholder = slugify(appName) || 'app'

  const goNextStep = () => {
    if (wizardStep === 1 && !appName.trim()) {
      setWizardError('アプリの名前を入力してください')
      return
    }
    setWizardError(null)
    setWizardStep((prev) => (Math.min(prev + 1, 4) as WizardStep))
  }
  const goPrevStep = () => setWizardStep((prev) => (Math.max(prev - 1, 1) as WizardStep))

  // フォルダをzip圧縮してUint8Arrayを得る
  const buildZipArchive = async (files: File[]): Promise<Uint8Array> => {
    const entries: Record<string, Uint8Array> = {}
    for (const file of files) {
      const relPath = file.webkitRelativePath || file.name
      // 先頭のフォルダ名を除いた相対パスをzip内のパスにする
      const zipPath = relPath.split('/').slice(1).join('/')
      if (!zipPath) continue
      const buffer = await file.arrayBuffer()
      entries[zipPath] = new Uint8Array(buffer)
    }
    return new Promise((resolve, reject) => {
      zip(entries, { level: 6 }, (err, data) => {
        if (err) reject(err)
        else resolve(data)
      })
    })
  }

  // ビルドログをポーリングで差分取得しつつステータス変化を監視する
  const pollBuildUntilDone = async (targetBuildId: string): Promise<Build> => {
    let sinceCursor: string | null = null
    while (true) {
      if (cancelledRef.current) throw new CancelledError() // キャンセル要求があればループを中断する
      const logsResponse: BuildLogsResponse = await get<BuildLogsResponse>(`/builds/${targetBuildId}/logs`, sinceCursor ? { since: sinceCursor } : undefined)
      if (logsResponse.logs) {
        setBuildLogs((prev) => prev + logsResponse.logs)
      }
      if (logsResponse.last_timestamp) {
        sinceCursor = logsResponse.last_timestamp
      }
      const buildData = await get<Build>(`/builds/${targetBuildId}`)
      if (buildData.status === 'succeeded' || buildData.status === 'failed' || buildData.status === 'cancelled') {
        return buildData
      }
      await new Promise((resolve) => setTimeout(resolve, BUILD_LOG_POLL_INTERVAL))
    }
  }

  // 指定した全デプロイメントのapply完了（app_status !== 'deploying'）を待ち、各デプロイメントの最終状態をIDごとに返す
  const waitForDeploymentsCompletion = async (targetDeploymentIdList: string[]): Promise<Map<string, Deployment>> => {
    const pendingIdSet = new Set(targetDeploymentIdList) // 未完了のdeployment IDを管理する
    const startedAt = Date.now()
    const deploymentDataMap = new Map<string, Deployment>()

    while (pendingIdSet.size > 0) {
      if (cancelledRef.current) throw new CancelledError() // キャンセル要求があればループを中断する
      if (Date.now() - startedAt > APPLY_POLL_TIMEOUT) {
        throw new Error('反映の完了確認がタイムアウトしました。時間をおいて画面を確認してください')
      }
      await new Promise((resolve) => setTimeout(resolve, APPLY_POLL_INTERVAL))

      await Promise.all(
        Array.from(pendingIdSet).map(async (targetDeploymentId) => {
          const deploymentData = await get<Deployment>(`/deployments/${targetDeploymentId}`)
          deploymentDataMap.set(targetDeploymentId, deploymentData)
          if (deploymentData.app_status !== 'deploying') { // deployingでなくなったら完了扱いにする
            pendingIdSet.delete(targetDeploymentId)
          }
        })
      )
    }
    return deploymentDataMap
  }

  // ビルド成功後：公開設定・環境変数・ボリュームを保存し、プロジェクト一括applyを実行して反映完了まで待つ
  const runApplyPhase = async (targetProjectId: string, targetDeploymentId: string) => {
    const checklist: { label: string; done: boolean }[] = []
    if (portEnabled) checklist.push({ label: 'ポート公開設定を保存', done: false })
    checklist.push({ label: '公開URLを設定', done: false })
    if (envRows.some((row) => row.key.trim())) checklist.push({ label: '環境変数を設定', done: false })
    if (volumeEnabled) checklist.push({ label: 'データの保存先を設定', done: false })
    checklist.push({ label: 'アプリの起動を待っています', done: false })
    setApplyChecklist(checklist)
    setPhase('applying')

    const markDone = (label: string) => setApplyChecklist((prev) => prev.map((item) => (item.label === label ? { ...item, done: true } : item)))

    if (portEnabled) {
      await post(`/deployments/${targetDeploymentId}/service`, {
        port: parseInt(port, 10),
        target_port: parseInt(port, 10),
      })
      markDone('ポート公開設定を保存')
    }

    const resolvedIngressName = ingressName.trim() || slugify(appName) || undefined
    const createdIngress = await post<IngressRoute>(`/projects/${targetProjectId}/ingress-routes`, resolvedIngressName ? { name: resolvedIngressName } : undefined)
    if (portEnabled) {
      const serviceData = await get<{ id: string }>(`/deployments/${targetDeploymentId}/service`)
      await post(`/ingress-routes/${createdIngress.id}/path-rules`, {
        path_prefix: '/',
        service_id: serviceData.id,
        strip_prefix: false,
      })
    }
    markDone('公開URLを設定')

    for (const row of envRows) {
      if (!row.key.trim()) continue
      const createdEnvVar = await post<{ id: string }>(`/projects/${targetProjectId}/env-vars`, {
        key: row.key.trim(),
        value: row.value,
        is_secret: false,
      })
      await post(`/deployments/${targetDeploymentId}/env-var-mounts`, { env_var_id: createdEnvVar.id })
    }
    if (envRows.some((row) => row.key.trim())) markDone('環境変数を設定')

    if (volumeEnabled) {
      const createdVolume = await post<{ id: string }>(`/projects/${targetProjectId}/volumes`, {
        name: `${slugify(appName) || 'app'}-data`,
        size_mb: volumeSizeGb * 1024,
      })
      await post(`/deployments/${targetDeploymentId}/volume-mounts`, {
        volume_id: createdVolume.id,
        mount_path: mountPath || '/data',
      })
      markDone('データの保存先を設定')
    }

    await runApplyAndWait(targetProjectId, targetDeploymentId, createdIngress.id, markDone)
  }

  // プロジェクト一括applyを実行し、反映完了・IngressRouteのactive化まで確認してから完了/失敗フェーズへ遷移する
  const runApplyAndWait = async (
    targetProjectId: string,
    targetDeploymentId: string,
    ingressRouteId: string,
    markDone: (label: string) => void
  ) => {
    // プロジェクト単位で一括applyする（IngressRoute/PathRuleの反映にはdeployment単体のapplyでは不十分なため）
    const applyResult = await post<ApplyProjectResult>(`/projects/${targetProjectId}/apply`)

    if (applyResult.failed_deployment_list.some((failure) => failure.deployment_id === targetDeploymentId)) {
      const failure = applyResult.failed_deployment_list.find((item) => item.deployment_id === targetDeploymentId)
      setFailureStage('反映')
      setFailureMessage(failure?.error ?? '反映処理の起動に失敗しました')
      setFailureHint('デプロイメント詳細画面でリソースの状態を確認してください。上限超過やイメージの設定ミスが原因の場合があります。')
      setPhase('failed')
      return
    }
    if (!applyResult.applied_deployment_id_list.includes(targetDeploymentId)) {
      setFailureStage('反映')
      setFailureMessage('反映処理の起動に失敗しました。詳細はダッシュボードで確認してください')
      setPhase('failed')
      return
    }

    // 対象デプロイメントの反映完了（app_status !== 'deploying'）を待つ
    const finalDeploymentMap = await waitForDeploymentsCompletion(applyResult.applied_deployment_id_list)
    const finalDeployment = finalDeploymentMap.get(targetDeploymentId)!
    markDone('アプリの起動を待っています')

    if (finalDeployment.status === 'failed' || finalDeployment.app_status === 'error') {
      setFailureStage('反映')
      setFailureMessage('反映処理中にエラーが発生しました。詳細はダッシュボードで確認してください')
      setFailureHint('ポート番号やビルドディレクトリの設定、環境変数の値が正しいか確認してください。コンテナがクラッシュしている可能性があります。')
      setPhase('failed')
      return
    }

    // IngressRouteが実際にactiveへ反映されたことを確認してから稼働URLを表示する
    if (portEnabled) {
      const ingressPollStartedAt = Date.now()
      let activeIngress: IngressRoute | undefined
      while (Date.now() - ingressPollStartedAt <= APPLY_POLL_TIMEOUT) {
        if (cancelledRef.current) throw new CancelledError() // キャンセル要求があればループを中断する
        const ingressRoutes = await get<IngressRoute[]>(`/projects/${targetProjectId}/ingress-routes`)
        const matchedIngress = ingressRoutes.find((route) => route.id === ingressRouteId)
        if (matchedIngress && matchedIngress.status === 'active') {
          activeIngress = matchedIngress
          break
        }
        if (matchedIngress && matchedIngress.status === 'deleting') {
          break // 想定外の状態になった場合はポーリングを打ち切る
        }
        await new Promise((resolve) => setTimeout(resolve, APPLY_POLL_INTERVAL))
      }
      if (!activeIngress) {
        setFailureStage('反映')
        setFailureMessage('公開URLの反映確認がタイムアウトしました。時間をおいてダッシュボードで確認してください')
        setPhase('failed')
        return
      }
      setResultHost(activeIngress.host)
    } else {
      setResultHost(null)
    }
    setPhase('done')
  }

  const handleDeploy = async () => {
    setWizardError(null)
    setBuildLogs('')
    setFailureMessage(null)
    setFailureHint(null)
    setFailureStage(null)
    setPhase('building')

    try {
      // 1. プロジェクトとarchiveタイプのデプロイメントを作成する
      const projectNameBase = (slugify(appName) || 'app').slice(0, 40)
      const project = await post<Project>('/projects', { name: `deploy-${projectNameBase}-${generateUniqueSuffix()}` })
      setProjectId(project.id)
      const deploymentName = slugify(appName, 63) || 'app' // k8sリソース名の制約（DNS-1123ラベル、最大63文字）に合わせて変換する
      const deployment = await post<Deployment>(`/projects/${project.id}/deployments`, {
        name: deploymentName,
        type: 'archive',
      })
      setDeploymentId(deployment.id)

      // 2. フォルダをzip圧縮する
      const archiveData = await buildZipArchive(selectedFiles)
      const archiveBlob = new Blob([archiveData.buffer as ArrayBuffer], { type: 'application/zip' })

      // 3. アーカイブをアップロードしてアップロードトークンを取得する
      const formData = new FormData()
      formData.append('archive', archiveBlob, `${folderName || 'app'}.zip`)
      const uploadResult = await postMultipart<UploadBuildArchiveResponse>(`/deployments/${deployment.id}/build/upload`, formData)

      // 4. ビルドを開始する
      const build = await post<Build>(`/deployments/${deployment.id}/build`, {
        archive_upload_token: uploadResult.upload_token,
        build_directory: buildDirectory || './',
      })
      setBuildId(build.id)
      savePersistedBuildId(build.id) // ビルド開始後にリロードされたらこのビルドのログ画面へ戻すために保存する

      // 5. ビルド完了までポーリングする
      const finishedBuild = await pollBuildUntilDone(build.id)
      if (finishedBuild.status !== 'succeeded') {
        setFailureStage('ビルド')
        setFailureMessage(`ビルドに失敗しました（status: ${finishedBuild.status}）`)
        setFailureHint('ビルドログを確認し、依存関係のインストールエラーや起動コマンドの設定を見直してください。')
        setPhase('failed')
        return
      }

      // 6. 公開設定・環境変数・ボリュームの保存とプロジェクト一括applyを実行する
      await runApplyPhase(project.id, deployment.id)
    } catch (deployError) {
      if (deployError instanceof CancelledError) return // キャンセル処理側で既に画面遷移済みのため何もしない
      console.error(deployError)
      setFailureStage(phase === 'building' ? 'ビルド' : phase === 'applying' ? '反映' : 'デプロイ準備')
      setFailureHint(resolveFailureHint(deployError))
      if (deployError instanceof QuotaExceededApiError) {
        setFailureMessage(deployError.message)
      } else if (deployError instanceof ApiError) {
        setFailureMessage(deployError.message)
      } else {
        setFailureMessage(deployError instanceof Error ? deployError.message : 'デプロイに失敗しました')
      }
      setPhase('failed')
    }
  }

  // 進行中の状態を全てリセットしてランディング（フォルダドロップ）画面へ戻す
  const resetToLanding = () => {
    cancelledRef.current = false
    clearPersistedBuildId()
    setPhase('landing')
    setFolderName('')
    setSelectedFiles([])
    setBuildDirOptions([])
    setWizardStep(1)
    setAppName('')
    setBuildDirectory('./')
    setPortEnabled(false)
    setPort('3000')
    setIngressName('')
    setEnvRows([])
    setVolumeEnabled(false)
    setVolumeSizeGb(1)
    setMountPath('/data')
    setWizardError(null)
    setDeploymentId(null)
    setProjectId(null)
    setBuildId(null)
    setBuildLogs('')
    setApplyChecklist([])
    setResultHost(null)
    setFailureMessage(null)
    setFailureHint(null)
    setFailureStage(null)
  }

  // デプロイをキャンセルする：ビルド中なら実行中のビルドを止め、作成済みのプロジェクトを削除してランディングへ戻る
  const handleCancel = async () => {
    cancelledRef.current = true // 実行中のポーリングループを中断させる
    setCancelling(true)
    try {
      if (buildId) {
        await del(`/builds/${buildId}`).catch(() => {}) // pending/building以外だと失敗するが無視してよい（既に完了しているため）
      }
      if (projectId) {
        await del(`/projects/${projectId}`).catch(() => {}) // 関連するdeployment・k8sリソースも非同期に削除される
      }
    } finally {
      setCancelling(false)
      resetToLanding()
    }
  }

  // デプロイに失敗した後、同じ入力内容で再試行する（失敗した古いプロジェクトは削除してから新規に作り直す）
  const handleRetry = async () => {
    if (projectId) {
      await del(`/projects/${projectId}`).catch(() => {})
    }
    setDeploymentId(null)
    setProjectId(null)
    setBuildId(null)
    setResultHost(null)
    void handleDeploy()
  }

  // フォルダ選択ダイアログを開くための隠しinput（landing/wizard両フェーズから参照するため共通化する）
  const folderInputElement = (
    <input
      ref={fileInputRef}
      type="file"
      // @ts-expect-error webkitdirectory is a non-standard attribute not in the DOM lib types
      webkitdirectory="true"
      directory="true"
      multiple
      onChange={(event) => handleFilesSelected(event.target.files)}
      className="hidden"
    />
  )

  if (phase === 'landing') {
    return (
      <div className="fixed inset-0 bg-white flex flex-col items-center justify-center px-10">
        {folderInputElement}
        <Link to="/" className="absolute top-8 right-10 text-[13px] text-[#1a73e8] font-medium hover:underline">
          既存のプロジェクトを見る →
        </Link>
        <h1 className="text-[32px] font-bold text-[#202124] mb-8 text-center">プロジェクトフォルダをドロップ</h1>
        <div
          onClick={handleLandingClick}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={(event) => void handleDrop(event)}
          className={`w-full max-w-[1180px] h-[560px] rounded-2xl border-2 border-dashed flex flex-col items-center justify-center cursor-pointer transition-colors ${
            dragOver ? 'border-[#1a73e8] bg-[#e8f0fe]' : 'border-[#dadce0] bg-[#f8f9fa]'
          }`}
        >
          <div className="w-[88px] h-[88px] rounded-full bg-[#e8f0fe] text-[#1a73e8] flex items-center justify-center mb-7">
            <Upload className="w-9 h-9" />
          </div>
          <div className="text-xl font-semibold text-[#202124] mb-2">フォルダをドラッグ&ドロップ</div>
          <div className="text-[15px] text-[#5f6368]">またはクリックして選択</div>
        </div>
        <div className="mt-5 text-[13px] text-[#9aa0a6]">自動でzip形式に圧縮してアップロードされます</div>
      </div>
    )
  }

  if (phase === 'wizard') {
    return (
      <div className="fixed inset-0 bg-[#f8f9fa] z-50 flex flex-col items-start overflow-y-auto py-10 px-6">
        {folderInputElement}
        <div className="w-full max-w-[1400px] mx-auto flex items-start justify-between mb-8 px-2">
          {[1, 2, 3, 4].map((step) => (
            <div key={step} className="flex flex-col items-center gap-2">
              <div
                className={`w-7 h-7 rounded-full flex items-center justify-center text-sm font-semibold ${
                  step === wizardStep ? 'bg-[#1a73e8] text-white' : step < wizardStep ? 'bg-[#e8f0fe] text-[#1a73e8]' : 'bg-[#dadce0] text-[#5f6368]'
                }`}
              >
                {step}
              </div>
              <span className={`text-sm ${step === wizardStep ? 'text-[#1a73e8] font-medium' : 'text-[#5f6368]'}`}>
                {['ビルドとポート', '環境変数', 'データの保存', '確認'][step - 1]}
              </span>
            </div>
          ))}
        </div>

        <div className="w-full max-w-[1400px] mx-auto bg-white rounded-xl border border-[#dadce0] p-8">
          {wizardStep === 1 && (
            <div className="space-y-5">
              <div>
                <label className="block text-sm font-medium text-[#202124] mb-1.5">アプリの名前</label>
                <input
                  type="text"
                  value={appName}
                  onChange={(event) => setAppName(event.target.value)}
                  placeholder="例：my-first-app"
                  maxLength={63}
                  className="w-full rounded-md border border-[#dadce0] px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#1a73e8]/30 focus:border-[#1a73e8]"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-[#202124] mb-1.5">ビルダー</label>
                <div className="space-y-2">
                  <button
                    type="button"
                    onClick={() => setBuilder('railpack')}
                    className={`w-full text-left rounded-lg border px-4 py-3 transition-colors ${
                      builder === 'railpack' ? 'border-[#1a73e8] bg-[#e8f0fe]' : 'border-[#dadce0] bg-white'
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      <span
                        className={`w-4 h-4 rounded-full border-2 flex items-center justify-center flex-shrink-0 ${
                          builder === 'railpack' ? 'border-[#1a73e8]' : 'border-[#dadce0]'
                        }`}
                      >
                        {builder === 'railpack' && <span className="w-2 h-2 rounded-full bg-[#1a73e8]" />}
                      </span>
                      <div>
                        <div className="text-sm font-semibold text-[#202124]">Railpack</div>
                        <div className="text-xs text-[#5f6368]">言語を自動検出してビルドします（おすすめ）</div>
                      </div>
                    </div>
                  </button>
                  <div className="w-full text-left rounded-lg border border-[#dadce0] bg-[#f8f9fa] px-4 py-3 opacity-60 cursor-not-allowed">
                    <div className="flex items-center gap-3">
                      <span className="w-4 h-4 rounded-full border-2 border-[#dadce0] flex-shrink-0" />
                      <div>
                        <div className="text-sm font-semibold text-[#5f6368]">Dockerfile</div>
                        <div className="text-xs text-[#9aa0a6]">フォルダアップロードでは現在利用できません</div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-[#202124] mb-1.5">ビルドディレクトリ</label>
                <input
                  type="text"
                  list="build-dir-options"
                  value={buildDirectory}
                  onChange={(event) => setBuildDirectory(event.target.value)}
                  placeholder="./"
                  className="w-full rounded-md border border-[#dadce0] px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#1a73e8]/30 focus:border-[#1a73e8]"
                />
                <datalist id="build-dir-options">
                  {buildDirOptions.map((dir) => (
                    <option key={dir} value={dir} />
                  ))}
                </datalist>
                <p className="text-xs text-[#9aa0a6] mt-1">アプリのコードがあるフォルダです</p>
              </div>
              <div className="flex items-center justify-between py-1">
                <div>
                  <label className="text-sm font-medium text-[#202124]">アプリをインターネットに公開する</label>
                  <p className="text-xs text-[#9aa0a6]">オンにすると、他の人もアプリを開けるようになります</p>
                </div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={portEnabled}
                  onClick={() => setPortEnabled((prev) => !prev)}
                  className={`relative w-11 h-6 rounded-full transition-colors flex-shrink-0 ${portEnabled ? 'bg-[#1a73e8]' : 'bg-[#dadce0]'}`}
                >
                  <span className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white transition-transform ${portEnabled ? 'translate-x-5' : ''}`} />
                </button>
              </div>
              {portEnabled && (
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-[#202124] mb-1.5">ポート番号</label>
                    <input
                      type="number"
                      value={port}
                      onChange={(event) => setPort(event.target.value)}
                      className="w-full rounded-md border border-[#dadce0] px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#1a73e8]/30 focus:border-[#1a73e8]"
                    />
                    <p className="text-xs text-[#9aa0a6] mt-1">
                      アプリが待ち受けている番号です（例：静的サイト（HTML/CSS/JS）は80）
                    </p>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-[#202124] mb-1.5">公開URLの名前（任意）</label>
                    <input
                      type="text"
                      value={ingressName}
                      onChange={(event) => setIngressName(event.target.value)}
                      placeholder={ingressNamePlaceholder}
                      maxLength={20}
                      className="w-full rounded-md border border-[#dadce0] px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#1a73e8]/30 focus:border-[#1a73e8]"
                    />
                    <p className="text-xs text-[#9aa0a6] mt-1">未入力の場合はアプリ名から自動で決まります</p>
                  </div>
                </div>
              )}
            </div>
          )}

          {wizardStep === 2 && (
            <div className="space-y-3">
              <div className="text-sm font-medium text-[#202124] mb-2">環境変数（任意）</div>
              {envRows.map((row, index) => (
                <div key={index} className="flex items-center gap-2">
                  <input
                    type="text"
                    value={row.key}
                    onChange={(event) => updateEnvRow(index, 'key', event.target.value)}
                    placeholder="KEY"
                    className="flex-1 rounded-md border border-[#dadce0] px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-[#1a73e8]/30 focus:border-[#1a73e8]"
                  />
                  <input
                    type="text"
                    value={row.value}
                    onChange={(event) => updateEnvRow(index, 'value', event.target.value)}
                    placeholder="value"
                    className="flex-1 rounded-md border border-[#dadce0] px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-[#1a73e8]/30 focus:border-[#1a73e8]"
                  />
                  <button onClick={() => removeEnvRow(index)} className="text-[#d93025] text-sm px-2 hover:underline">
                    削除
                  </button>
                </div>
              ))}
              <button onClick={addEnvRow} className="text-sm text-[#1a73e8] hover:underline">
                + 環境変数を追加
              </button>
            </div>
          )}

          {wizardStep === 3 && (
            <div className="space-y-5">
              <div className="flex items-center justify-between">
                <div>
                  <label className="text-sm font-medium text-[#202124]">データを保存する場所を使う</label>
                  <p className="text-xs text-[#9aa0a6]">オンにすると、アプリを再起動してもデータが消えません</p>
                </div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={volumeEnabled}
                  onClick={() => setVolumeEnabled((prev) => !prev)}
                  className={`relative w-11 h-6 rounded-full transition-colors flex-shrink-0 ${volumeEnabled ? 'bg-[#1a73e8]' : 'bg-[#dadce0]'}`}
                >
                  <span className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white transition-transform ${volumeEnabled ? 'translate-x-5' : ''}`} />
                </button>
              </div>
              {volumeEnabled && (
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-[#202124] mb-1.5">保存する容量（最大5GB）</label>
                    <div className="flex gap-2">
                      {VOLUME_SIZE_OPTIONS_GB.map((sizeGb) => (
                        <button
                          key={sizeGb}
                          onClick={() => setVolumeSizeGb(sizeGb)}
                          className={`px-4 py-2 rounded-md border text-sm font-medium ${
                            volumeSizeGb === sizeGb ? 'border-[#1a73e8] text-[#1a73e8]' : 'border-[#dadce0] text-[#5f6368]'
                          }`}
                        >
                          {sizeGb}GB
                        </button>
                      ))}
                    </div>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-[#202124] mb-1.5">保存先のフォルダ</label>
                    <div className="flex rounded-md border border-[#dadce0] overflow-hidden focus-within:ring-2 focus-within:ring-[#1a73e8]/30 focus-within:border-[#1a73e8]">
                      <span className="bg-[#f8f9fa] text-[#9aa0a6] text-xs font-medium px-3 flex items-center border-r border-[#dadce0]">PATH</span>
                      <input
                        type="text"
                        value={mountPath}
                        onChange={(event) => setMountPath(event.target.value)}
                        placeholder="/data"
                        className="flex-1 px-3 py-2.5 text-sm font-mono focus:outline-none"
                      />
                    </div>
                    <p className="text-xs text-[#9aa0a6] mt-1">アプリがデータを書き込むフォルダです</p>
                  </div>
                </div>
              )}
            </div>
          )}

          {wizardStep === 4 && (
            <div className="space-y-3 text-sm">
              <div className="flex justify-between py-2 border-b border-[#dadce0]">
                <span className="text-[#5f6368]">アプリの名前</span>
                <span className="text-[#202124] font-medium">{appName}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-[#dadce0]">
                <span className="text-[#5f6368]">ビルドディレクトリ</span>
                <span className="text-[#202124] font-mono">{buildDirectory || './'}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-[#dadce0]">
                <span className="text-[#5f6368]">インターネット公開</span>
                <span className="text-[#202124]">{portEnabled ? `ポート ${port} / ${ingressName.trim() || ingressNamePlaceholder}` : '無効'}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-[#dadce0]">
                <span className="text-[#5f6368]">環境変数</span>
                <span className="text-[#202124]">{envRows.filter((row) => row.key.trim()).length}件</span>
              </div>
              <div className="flex justify-between py-2">
                <span className="text-[#5f6368]">データの保存</span>
                <span className="text-[#202124]">{volumeEnabled ? `${volumeSizeGb}GB / ${mountPath || '/data'}` : '無効'}</span>
              </div>
            </div>
          )}

          {wizardError && <div className="mt-4 bg-red-50 border border-red-200 rounded-lg px-3 py-2 text-sm text-red-700">{wizardError}</div>}

          <div className="flex items-center justify-between mt-6">
            {wizardStep > 1 ? (
              <button
                onClick={goPrevStep}
                className="border border-[#dadce0] text-sm text-[#202124] px-5 py-2 rounded-md hover:bg-[#f8f9fa] transition-colors"
              >
                戻る
              </button>
            ) : (
              <span />
            )}
            {wizardStep < 4 ? (
              <button onClick={goNextStep} className="bg-[#1a73e8] text-white text-sm px-6 py-2 rounded-md hover:bg-[#1557b0] transition-colors">
                次へ
              </button>
            ) : (
              <button onClick={() => void handleDeploy()} className="bg-[#1a73e8] text-white text-sm px-6 py-2 rounded-md hover:bg-[#1557b0] transition-colors">
                デプロイする
              </button>
            )}
          </div>
        </div>
      </div>
    )
  }

  // ビルド中・反映中・完了・失敗のフルスクリーンオーバーレイ
  return (
    <div className="fixed inset-0 bg-[#f8f9fa] z-50 flex flex-col items-center justify-center px-10">
      <div className="w-full max-w-[1400px] bg-white rounded-xl border border-[#dadce0] p-8">
        {phase === 'building' && (
          <>
            <h2 className="text-lg font-semibold text-[#202124] mb-4">アプリをビルドしています…</h2>
            <div
              ref={logPanelRef}
              className="bg-[#202124] text-[#e8eaed] rounded-lg p-4 h-[560px] overflow-y-auto font-mono text-xs whitespace-pre-wrap"
            >
              {buildLogs || 'ログを待機中...'}
            </div>
            <div className="mt-5 flex justify-end">
              <button
                onClick={() => void handleCancel()}
                disabled={cancelling}
                className="border border-[#dadce0] text-sm text-[#d93025] px-5 py-2 rounded-md hover:bg-red-50 transition-colors disabled:opacity-50"
              >
                {cancelling ? 'キャンセル中...' : 'デプロイをキャンセル'}
              </button>
            </div>
          </>
        )}

        {phase === 'applying' && (
          <>
            <h2 className="text-lg font-semibold text-[#202124] mb-4">アプリを反映しています…</h2>
            <div className="space-y-2">
              {applyChecklist.map((item, index) => {
                const isInProgress = !item.done && applyChecklist.slice(0, index).every((prev) => prev.done) // 未完了項目のうち最初の1件だけを実行中として扱う
                return (
                  <div key={item.label} className="flex items-center gap-3 py-2 border-b border-[#dadce0] last:border-b-0">
                    <div
                      className={`w-5 h-5 rounded-full flex items-center justify-center text-xs ${
                        item.done ? 'bg-[#188038] text-white' : isInProgress ? 'bg-transparent text-[#1a73e8]' : 'bg-[#dadce0] text-transparent'
                      }`}
                    >
                      {item.done ? '✓' : isInProgress ? <Loader2 className="w-4 h-4 animate-spin" /> : '✓'}
                    </div>
                    <span className={`text-sm ${item.done || isInProgress ? 'text-[#202124]' : 'text-[#9aa0a6]'}`}>{item.label}</span>
                  </div>
                )
              })}
            </div>
            <div className="mt-5 flex justify-end">
              <button
                onClick={() => void handleCancel()}
                disabled={cancelling}
                className="border border-[#dadce0] text-sm text-[#d93025] px-5 py-2 rounded-md hover:bg-red-50 transition-colors disabled:opacity-50"
              >
                {cancelling ? 'キャンセル中...' : 'デプロイをキャンセル'}
              </button>
            </div>
          </>
        )}

        {phase === 'done' && (
          <div className="text-center py-6">
            <div className="w-16 h-16 rounded-full bg-[#e6f4ea] text-[#188038] flex items-center justify-center text-3xl mx-auto mb-5">✓</div>
            <h2 className="text-lg font-semibold text-[#202124] mb-2">デプロイが完了しました</h2>
            {resultHost ? (
              <a href={`http://${resultHost}`} target="_blank" rel="noreferrer" className="text-[#1a73e8] hover:underline font-mono text-sm">
                {resultHost}
              </a>
            ) : (
              <p className="text-sm text-[#5f6368]">公開URLの設定はありませんでした</p>
            )}
            <div className="mt-6 flex items-center justify-center gap-3">
              <button
                onClick={resetToLanding}
                className="border border-[#dadce0] text-sm text-[#202124] px-5 py-2 rounded-md hover:bg-[#f8f9fa] transition-colors"
              >
                最初の画面に戻る
              </button>
              <button
                onClick={() => projectId && deploymentId && navigate(`/projects/${projectId}/deployments/${deploymentId}`)}
                className="bg-[#111827] text-white text-sm px-6 py-2 rounded-md hover:bg-gray-800 transition-colors"
              >
                デプロイメントを開く
              </button>
            </div>
          </div>
        )}

        {phase === 'failed' && (
          <div className="text-center py-6">
            <div className="w-16 h-16 rounded-full bg-[#fce8e6] text-[#d93025] flex items-center justify-center text-3xl mx-auto mb-5">✕</div>
            <h2 className="text-lg font-semibold text-[#202124] mb-2">
              {failureStage ? `${failureStage}フェーズで失敗しました` : 'デプロイに失敗しました'}
            </h2>
            <p className="text-sm text-[#5f6368] max-w-[560px] mx-auto">{failureMessage}</p>
            {failureHint && (
              <div className="mt-4 mx-auto max-w-[560px] bg-[#fef7e0] border border-[#fdd663] rounded-lg px-4 py-3 text-left text-sm text-[#202124]">
                <span className="font-medium">対処方法：</span>
                {failureHint}
              </div>
            )}
            <div className="mt-6 flex items-center justify-center gap-3">
              <button onClick={resetToLanding} className="text-sm text-[#5f6368] hover:text-[#202124] transition-colors">
                最初からやり直す
              </button>
              {buildId && (
                <button
                  onClick={() => navigate(`/builds/${buildId}/logs`)}
                  className="text-sm text-[#1a73e8] hover:underline"
                >
                  ビルドログを見る
                </button>
              )}
              {selectedFiles.length > 0 && (
                <button
                  onClick={() => void handleRetry()}
                  className="border border-[#1a73e8] text-sm text-[#1a73e8] px-5 py-2 rounded-md hover:bg-[#e8f0fe] transition-colors"
                >
                  再試行
                </button>
              )}
              {(projectId && deploymentId) && (
                <button
                  onClick={() => navigate(`/projects/${projectId}/deployments/${deploymentId}`)}
                  className="bg-[#111827] text-white text-sm px-6 py-2 rounded-md hover:bg-gray-800 transition-colors"
                >
                  デプロイメントを開く
                </button>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
