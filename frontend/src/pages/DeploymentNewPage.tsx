import { useState, useCallback, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Container, GitBranch, Package, LayoutTemplate, Plus, Trash2 } from 'lucide-react'
import { Layout } from '@/components/Layout'
import { get, post, QuotaExceededApiError } from '@/lib/api'
import type { Deployment, DeploymentType, DeploymentTemplate, EnvVar } from '@/lib/types'

type Step = 'type' | 'form' | 'template'

const DEPLOYMENT_TYPES: { type: DeploymentType; label: string; description: string; Icon: React.ElementType }[] = [
  {
    type: 'image_url',
    label: 'Container Image',
    description: '既存の Docker イメージを直接デプロイします。Docker Hub や GHCR などのレジストリから指定できます。',
    Icon: Container,
  },
  {
    type: 'dockerfile',
    label: 'Dockerfile',
    description: 'GitHub リポジトリの Dockerfile を使ってイメージをビルドしてデプロイします。',
    Icon: GitBranch,
  },
  {
    type: 'railpack',
    label: 'Railpack',
    description: 'GitHub リポジトリを自動解析してビルドします。設定不要で多くのフレームワークに対応します。',
    Icon: Package,
  },
]

// GitHub URL から owner/repo を抽出する
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
type GitHubCommit = { sha: string; commit: { message: string } }
type GitHubTree   = { path: string; type: string }

async function fetchGitHubBranches(repo: string): Promise<GitHubBranch[]> {
  const res = await fetch(`https://api.github.com/repos/${repo}/branches?per_page=100`) // ブランチ一覧を取得する
  if (!res.ok) throw new Error(`branches fetch failed: ${res.status}`)
  return res.json() as Promise<GitHubBranch[]>
}

async function fetchGitHubCommits(repo: string, branch: string): Promise<GitHubCommit[]> {
  const res = await fetch(`https://api.github.com/repos/${repo}/commits?sha=${branch}&per_page=30`) // コミット一覧を取得する
  if (!res.ok) throw new Error(`commits fetch failed: ${res.status}`)
  return res.json() as Promise<GitHubCommit[]>
}

async function fetchGitHubDirs(repo: string, branch: string): Promise<string[]> {
  const res = await fetch(`https://api.github.com/repos/${repo}/git/trees/${branch}?recursive=0`) // ルートツリーを取得する
  if (!res.ok) throw new Error(`tree fetch failed: ${res.status}`)
  const data = await res.json() as { tree: GitHubTree[] }
  return data.tree.filter(item => item.type === 'tree').map(item => `./${item.path}`) // ディレクトリのみ ./ スタートで返す
}

export function DeploymentNewPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const navigate = useNavigate()

  const [step, setStep] = useState<Step>('type') // 現在のステップを管理する
  const [selectedType, setSelectedType] = useState<DeploymentType | null>(null) // 選択したタイプを管理する

  // テンプレート関連状態
  const [templates, setTemplates] = useState<DeploymentTemplate[]>([]) // テンプレート一覧
  const [templatesLoading, setTemplatesLoading] = useState(false) // テンプレート取得中フラグ
  const [selectedTemplate, setSelectedTemplate] = useState<DeploymentTemplate | null>(null) // 選択中のテンプレート
  const [templateName, setTemplateName] = useState('') // テンプレートから作成するデプロイメント名
  const [templateCreating, setTemplateCreating] = useState(false) // テンプレート作成中フラグ
  const [templateError, setTemplateError] = useState<string | null>(null) // テンプレートエラー
  const [enabledVolumeNames, setEnabledVolumeNames] = useState<Set<string>>(new Set()) // 有効化するボリューム名のセット
  const [tmplEnvVarOverrides, setTmplEnvVarOverrides] = useState<Record<string, string>>({}) // テンプレート定義のenv_varの値の上書き（キー → 値）
  const [tmplExtraEnvVars, setTmplExtraEnvVars] = useState<{ key: string; value: string; is_secret: boolean }[]>([]) // 追加環境変数一覧
  const [tmplNewEnvKey, setTmplNewEnvKey] = useState('') // 追加環境変数キー入力
  const [tmplNewEnvValue, setTmplNewEnvValue] = useState('') // 追加環境変数値入力
  const [tmplNewEnvSecret, setTmplNewEnvSecret] = useState(false) // 追加環境変数シークレットフラグ

  const [formData, setFormData] = useState({
    name: '',
    image_url: '',
    github_repo_url: '',
    github_branch: '',
    github_commit_sha: '',
    github_repo_directory: './',
    dockerfile_path: './Dockerfile',
    replicas: '1',
    instance_size: 'small',
  }) // フォームデータを管理する
  const [creating, setCreating] = useState(false) // 作成中フラグ
  const [error, setError] = useState<string | null>(null) // エラーメッセージを管理する

  // 環境変数（新規作成時に設定する）
  type EnvVarEntry = { key: string; value: string; is_secret: boolean }
  const [envVarEntries, setEnvVarEntries] = useState<EnvVarEntry[]>([]) // 環境変数エントリ一覧
  const [newEnvKey, setNewEnvKey] = useState('') // 追加する環境変数キー
  const [newEnvValue, setNewEnvValue] = useState('') // 追加する環境変数値
  const [newEnvSecret, setNewEnvSecret] = useState(false) // シークレットフラグ

  // GitHub API から取得したデータ
  const [ghBranches, setGhBranches] = useState<GitHubBranch[]>([]) // ブランチ一覧
  const [ghCommits, setGhCommits]   = useState<GitHubCommit[]>([]) // コミット一覧
  const [ghDirs, setGhDirs]         = useState<string[]>([])       // ディレクトリ一覧
  const [ghLoading, setGhLoading]   = useState<'branches' | 'commits' | null>(null) // ローディング中の対象
  const [ghError, setGhError]       = useState<string | null>(null) // GitHub API エラー

  // リポジトリURLからブランチ一覧を取得する
  const loadBranches = useCallback(async (repoUrl: string) => {
    const repo = extractGitHubRepo(repoUrl) // owner/repo を抽出する
    if (!repo) {
      setGhError('有効な GitHub リポジトリ URL を入力してください') // 無効な URL の場合はエラーを表示する
      return
    }
    setGhLoading('branches')
    setGhError(null)
    setGhBranches([])
    setGhCommits([])
    setGhDirs([])
    setFormData(prev => ({ ...prev, github_branch: '', github_commit_sha: '', github_repo_directory: './' })) // 選択をリセットする
    try {
      const branches = await fetchGitHubBranches(repo) // ブランチ一覧を取得する
      setGhBranches(branches)
      if (branches.length === 0) setGhError('ブランチが見つかりませんでした') // ブランチが空の場合はエラーを表示する
    } catch {
      setGhError('ブランチの取得に失敗しました。リポジトリURLを確認してください。') // エラーを表示する
    } finally {
      setGhLoading(null)
    }
  }, [])

  // ブランチ選択時にコミット・ディレクトリを取得する
  const handleBranchSelect = async (branch: string) => {
    setFormData(prev => ({ ...prev, github_branch: branch, github_commit_sha: '', github_repo_directory: './' })) // ブランチをセットしコミット・ディレクトリをリセットする
    const repo = extractGitHubRepo(formData.github_repo_url)
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
  }

  // テンプレート一覧を取得する（templateステップ表示時）
  useEffect(() => {
    if (step !== 'template') return
    setTemplatesLoading(true)
    void get<DeploymentTemplate[]>('/deployment-templates')
      .then(setTemplates)
      .catch(() => setTemplates([]))
      .finally(() => setTemplatesLoading(false))
  }, [step])

  const handleTypeSelect = (type: DeploymentType) => {
    setSelectedType(type) // タイプを選択する
    setStep('form') // フォームステップへ進む
  }

  // テンプレートを選択する
  const handleTemplateSelect = (template: DeploymentTemplate) => {
    setSelectedTemplate(template) // テンプレートを選択する
    setTemplateName('') // 名前をリセットする
    setTemplateError(null)
    // ボリュームをすべてデフォルト有効にする
    setEnabledVolumeNames(new Set((template.volumes ?? []).map(vol => vol.name)))
    // テンプレートのenv_varのデフォルト値をオーバーライドマップに初期設定する
    const overridesInit: Record<string, string> = {}
    for (const envVar of template.env_vars ?? []) {
      if (!envVar.auto_generate) {
        overridesInit[envVar.key] = envVar.value // デフォルト値を設定する
      }
    }
    setTmplEnvVarOverrides(overridesInit)
    setTmplExtraEnvVars([]) // 追加環境変数をリセットする
    setTmplNewEnvKey('')
    setTmplNewEnvValue('')
    setTmplNewEnvSecret(false)
  }

  // テンプレートからデプロイメントを作成する
  const handleCreateFromTemplate = async () => {
    if (!selectedTemplate || !projectId) return
    if (!templateName.trim()) {
      setTemplateError('デプロイメント名を入力してください')
      return
    }
    setTemplateCreating(true)
    setTemplateError(null)
    try {
      const allVolumeNames = (selectedTemplate.volumes ?? []).map(vol => vol.name)
      const skipVolumeNames = allVolumeNames.filter(name => !enabledVolumeNames.has(name)) // 無効化されたボリュームをスキップ対象にする
      // テンプレートのenv_varの上書き値をリストに変換する（auto_generateでないもののみ）
      const overrideEnvVars = (selectedTemplate.env_vars ?? [])
        .filter(envVar => !envVar.auto_generate)
        .map(envVar => ({
          key: envVar.key,
          value: tmplEnvVarOverrides[envVar.key] ?? envVar.value, // 上書き値があれば使用する
          is_secret: envVar.is_secret,
        }))
      await post<Deployment>(`/projects/${projectId}/deployments/from-template`, {
        template_id: selectedTemplate.id,                                              // テンプレート ID を設定する
        name: templateName.trim(),                                                     // デプロイメント名を設定する
        skip_volume_names: skipVolumeNames,                                            // スキップするボリューム名を設定する
        override_env_vars: overrideEnvVars,                                            // テンプレートenv_varの上書き値を設定する
        extra_env_vars: tmplExtraEnvVars,                                              // 追加環境変数を設定する
      })
      // iframe 内で表示されている場合は親ウィンドウに完了を通知する
      if (window.parent !== window) {
        window.parent.postMessage({ type: 'deployment-created', projectId }, '*')
      } else {
        navigate(`/projects/${projectId}`) // 通常遷移（単体ページとして開かれた場合）
      }
    } catch (createError) {
      if (createError instanceof QuotaExceededApiError) {
        setTemplateError(createError.message)
      } else {
        setTemplateError('デプロイメントの作成に失敗しました')
      }
    } finally {
      setTemplateCreating(false)
    }
  }

  const handleCreate = async () => {
    if (!selectedType || !projectId) return
    if (!formData.name.trim()) {
      setError('デプロイメント名を入力してください')
      return
    }

    setCreating(true)
    setError(null)

    try {
      const body: Record<string, unknown> = {
        name: formData.name.trim(),
        type: selectedType,
        replicas: parseInt(formData.replicas, 10), // レプリカ数を数値に変換する
        instance_size: formData.instance_size,
      }

      if (selectedType === 'image_url') {
        body.image_url = formData.image_url // image_url タイプのフォームデータを設定する
      } else {
        body.github_repo_url = formData.github_repo_url // GitHub URL を設定する
        body.github_branch = formData.github_branch || 'main' // ブランチを設定する（未選択は main）
        body.github_commit_sha = formData.github_commit_sha // コミット SHA を設定する
        body.github_repo_directory = formData.github_repo_directory // ディレクトリを設定する
        if (selectedType === 'dockerfile') {
          body.dockerfile_path = formData.dockerfile_path // Dockerfile パスを設定する
        }
      }

      const deployment = await post<Deployment>(`/projects/${projectId}/deployments`, body) // デプロイメントを作成する
      if (deployment && envVarEntries.length > 0) {
        for (const entry of envVarEntries) {
          const created = await post<EnvVar>(`/projects/${projectId}/env-vars`, { // プロジェクトに環境変数を作成する
            key: entry.key,
            value: entry.value,
            is_secret: entry.is_secret,
          })
          if (created) {
            await post(`/deployments/${deployment.id}/env-var-mounts`, { // 環境変数をデプロイメントにマウントする
              env_var_id: created.id,
            })
          }
        }
      }
      // iframe 内で表示されている場合は親ウィンドウに完了を通知する
      if (window.parent !== window) {
        window.parent.postMessage({ type: 'deployment-created', projectId }, '*')
      } else {
        navigate(`/projects/${projectId}`) // 通常遷移（単体ページとして開かれた場合）
      }
    } catch (createError) {
      console.error(createError)
      if (createError instanceof QuotaExceededApiError) {
        setError(createError.message)
      } else {
        setError('デプロイメントの作成に失敗しました')
      }
    } finally {
      setCreating(false)
    }
  }

  const inputClass = 'w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#00C2D1]/50 focus:border-[#00C2D1] transition-colors'
  const labelClass = 'block text-xs font-medium text-gray-500 mb-1'

  return (
    <Layout
      breadcrumbs={[
        { label: 'プロジェクト', href: `/projects/${projectId}` },
        { label: '新規デプロイメント' },
      ]}
    >
      <div className={step === 'template' ? 'w-full' : 'max-w-2xl mx-auto'}>
        {/* ステップインジケーター */}
        <div className="flex items-center gap-2 mb-8 text-sm">
          <span className={`flex items-center gap-1.5 ${step === 'type' ? 'text-[#00C2D1] font-medium' : 'text-gray-400'}`}>
            <span className={`w-5 h-5 rounded-full text-xs flex items-center justify-center ${step === 'type' ? 'bg-[#00C2D1] text-white' : 'bg-gray-200 text-gray-500'}`}>1</span>
            タイプ選択
          </span>
          <span className="text-gray-300">→</span>
          <span className={`flex items-center gap-1.5 ${step === 'form' || step === 'template' ? 'text-[#00C2D1] font-medium' : 'text-gray-400'}`}>
            <span className={`w-5 h-5 rounded-full text-xs flex items-center justify-center ${step === 'form' || step === 'template' ? 'bg-[#00C2D1] text-white' : 'bg-gray-200 text-gray-500'}`}>2</span>
            設定
          </span>
        </div>

        {/* ステップ1: タイプ選択 */}
        {step === 'type' && (
          <div className="space-y-4">
            <h1 className="text-xl font-semibold text-[#111827]">デプロイメントタイプを選択</h1>
            <div className="grid grid-cols-1 gap-3">
              {/* テンプレートから作成 */}
              <button
                onClick={() => setStep('template')}
                className="flex items-start gap-4 p-4 bg-white rounded-lg border border-[#00C2D1]/40 text-left hover:border-[#00C2D1] hover:shadow-sm transition-all group"
              >
                <span className="p-2.5 rounded-lg bg-[#00C2D1]/10 text-[#00C2D1] group-hover:bg-[#00C2D1]/20 transition-colors shrink-0">
                  <LayoutTemplate className="w-5 h-5" />
                </span>
                <div>
                  <p className="font-medium text-[#111827] mb-1">テンプレートから作成</p>
                  <p className="text-sm text-gray-500 leading-relaxed">MySQL・PostgreSQL・Redis などのプリセット構成から素早くデプロイできます。</p>
                </div>
              </button>
              {DEPLOYMENT_TYPES.map(({ type, label, description, Icon }) => (
                <button
                  key={type}
                  onClick={() => handleTypeSelect(type)}
                  className="flex items-start gap-4 p-4 bg-white rounded-lg border border-gray-200 text-left hover:border-[#00C2D1] hover:shadow-sm transition-all group"
                >
                  <span className="p-2.5 rounded-lg bg-gray-50 text-gray-500 group-hover:bg-[#00C2D1]/10 group-hover:text-[#00C2D1] transition-colors shrink-0">
                    <Icon className="w-5 h-5" />
                  </span>
                  <div>
                    <p className="font-medium text-[#111827] mb-1">{label}</p>
                    <p className="text-sm text-gray-500 leading-relaxed">{description}</p>
                  </div>
                </button>
              ))}
            </div>
          </div>
        )}

        {/* テンプレートステップ */}
        {step === 'template' && (
          <div className="flex gap-6 h-[calc(100vh-180px)] min-h-0">
            {/* 左カラム：テンプレート一覧 */}
            <div className="w-72 shrink-0 flex flex-col min-h-0">
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-sm font-semibold text-[#111827]">テンプレート</h2>
                <button
                  onClick={() => { setStep('type'); setSelectedTemplate(null); setTemplateError(null) }}
                  className="text-xs text-gray-400 hover:text-gray-600 transition-colors"
                >
                  ← 戻る
                </button>
              </div>
              <div className="flex-1 overflow-y-auto space-y-2 pr-1">
                {templatesLoading && (
                  <p className="text-sm text-gray-400">読み込み中...</p>
                )}
                {!templatesLoading && templates.length === 0 && (
                  <p className="text-sm text-gray-400">テンプレートがありません</p>
                )}
                {!templatesLoading && templates.map((template) => (
                  <button
                    key={template.id}
                    onClick={() => handleTemplateSelect(template)}
                    className={`w-full flex items-start gap-3 p-3 bg-white rounded-lg border text-left hover:shadow-sm transition-all ${selectedTemplate?.id === template.id ? 'border-[#00C2D1] ring-2 ring-[#00C2D1]/20' : 'border-gray-200 hover:border-[#00C2D1]'}`}
                  >
                    <span className={`p-2 rounded-lg shrink-0 mt-0.5 ${selectedTemplate?.id === template.id ? 'bg-[#00C2D1]/10 text-[#00C2D1]' : 'bg-gray-50 text-gray-400'}`}>
                      <LayoutTemplate className="w-4 h-4" />
                    </span>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-[#111827]">{template.name}</p>
                      {template.description && (
                        <p className="text-xs text-gray-400 mt-0.5 leading-relaxed">{template.description}</p>
                      )}
                      <div className="flex flex-wrap gap-1 mt-1.5">
                        <span className="text-[10px] bg-gray-100 text-gray-500 px-1.5 py-0.5 rounded font-mono">{template.image_url}</span>
                      </div>
                    </div>
                  </button>
                ))}
              </div>
            </div>

            {/* 右カラム：設定フォーム */}
            <div className="flex-1 min-w-0 flex flex-col min-h-0">
              {!selectedTemplate ? (
                <div className="flex-1 flex items-center justify-center">
                  <p className="text-sm text-gray-400">左からテンプレートを選択してください</p>
                </div>
              ) : (
                <div className="flex-1 flex flex-col min-h-0">
                  <div className="flex items-center justify-between mb-4">
                    <h2 className="text-sm font-semibold text-[#111827]">{selectedTemplate.name} の設定</h2>
                    <div className="flex items-center gap-2 text-xs text-gray-400">
                      <span className="font-mono">{selectedTemplate.image_url}</span>
                      <span>·</span>
                      <span>{selectedTemplate.instance_size}</span>
                      <span>·</span>
                      <span>{selectedTemplate.replicas} レプリカ</span>
                      {selectedTemplate.service_port > 0 && (
                        <>
                          <span>·</span>
                          <span>:{selectedTemplate.service_port}</span>
                        </>
                      )}
                    </div>
                  </div>

                  <div className="flex-1 overflow-y-auto space-y-5 pr-1">
                    {/* デプロイメント名 */}
                    <div className="bg-white rounded-lg border border-gray-200 p-4">
                      <label className={labelClass}>デプロイメント名 *</label>
                      <input
                        type="text"
                        value={templateName}
                        onChange={(event) => setTemplateName(event.target.value)}
                        placeholder="my-app"
                        maxLength={63}
                        className={inputClass}
                        autoFocus
                      />
                      <p className="text-xs text-gray-400 mt-1">英小文字・数字・ハイフンのみ、最大63文字</p>
                    </div>

                    {/* 環境変数 */}
                    {(selectedTemplate.env_vars ?? []).length > 0 && (
                      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
                        <p className="text-xs font-semibold text-[#111827]">環境変数</p>
                        <div className="space-y-2">
                          {(selectedTemplate.env_vars ?? []).map((envVar) => (
                            <div key={envVar.key} className="flex items-center gap-3">
                              <div className="w-40 shrink-0 flex items-center gap-1.5">
                                <span className="font-mono text-xs text-[#111827] truncate">{envVar.key}</span>
                                {envVar.is_secret && <span className="text-[10px] bg-purple-50 text-purple-500 px-1 py-0.5 rounded shrink-0">secret</span>}
                              </div>
                              {envVar.auto_generate ? (
                                <span className="text-xs bg-amber-50 text-amber-600 px-2 py-1 rounded border border-amber-100">自動生成</span>
                              ) : (
                                <input
                                  type={envVar.is_secret ? 'password' : 'text'}
                                  value={tmplEnvVarOverrides[envVar.key] ?? envVar.value}
                                  onChange={overrideEv => setTmplEnvVarOverrides(prev => ({ ...prev, [envVar.key]: overrideEv.target.value }))}
                                  className={`flex-1 ${inputClass} font-mono text-xs`}
                                  placeholder={envVar.value || '(空)'}
                                />
                              )}
                            </div>
                          ))}
                        </div>

                        {/* 追加の環境変数 */}
                        {tmplExtraEnvVars.length > 0 && (
                          <div className="pt-2 border-t border-gray-100 space-y-1.5">
                            <p className="text-[10px] text-gray-400 font-medium">追加の環境変数</p>
                            {tmplExtraEnvVars.map((entry, entryIndex) => (
                              <div key={entryIndex} className="flex items-center gap-2 bg-blue-50 rounded px-2.5 py-1.5 border border-blue-100">
                                <span className="font-mono text-xs text-[#111827]">{entry.key}</span>
                                {entry.is_secret ? (
                                  <span className="text-[10px] bg-purple-50 text-purple-500 px-1.5 py-0.5 rounded shrink-0">secret</span>
                                ) : (
                                  <span className="font-mono text-xs text-gray-400 truncate">{entry.value || '(空)'}</span>
                                )}
                                <button
                                  onClick={() => setTmplExtraEnvVars(prev => prev.filter((_, idx) => idx !== entryIndex))}
                                  className="ml-auto p-0.5 rounded hover:bg-red-50 text-gray-300 hover:text-red-400 transition-colors shrink-0"
                                >
                                  <Trash2 className="w-3 h-3" />
                                </button>
                              </div>
                            ))}
                          </div>
                        )}

                        {/* 新規追加フォーム */}
                        <div className="pt-2 border-t border-gray-100 space-y-1.5">
                          <p className="text-[10px] text-gray-400 font-medium">新しい変数を追加</p>
                          <div className="flex gap-2">
                            <input
                              type="text"
                              value={tmplNewEnvKey}
                              onChange={ev => setTmplNewEnvKey(ev.target.value)}
                              placeholder="KEY"
                              className={`flex-1 ${inputClass} font-mono text-xs`}
                            />
                            <input
                              type={tmplNewEnvSecret ? 'password' : 'text'}
                              value={tmplNewEnvValue}
                              onChange={ev => setTmplNewEnvValue(ev.target.value)}
                              placeholder="VALUE"
                              className={`flex-1 ${inputClass} font-mono text-xs`}
                            />
                            <label className="flex items-center gap-1 text-xs text-gray-500 cursor-pointer select-none shrink-0">
                              <input
                                type="checkbox"
                                checked={tmplNewEnvSecret}
                                onChange={ev => setTmplNewEnvSecret(ev.target.checked)}
                                className="rounded border-gray-300"
                              />
                              secret
                            </label>
                            <button
                              onClick={() => {
                                if (!tmplNewEnvKey.trim()) return
                                setTmplExtraEnvVars(prev => [...prev, { key: tmplNewEnvKey.trim(), value: tmplNewEnvValue, is_secret: tmplNewEnvSecret }])
                                setTmplNewEnvKey('')
                                setTmplNewEnvValue('')
                                setTmplNewEnvSecret(false)
                              }}
                              disabled={!tmplNewEnvKey.trim()}
                              className="flex items-center gap-1 text-xs bg-[#111827] text-white px-3 py-1.5 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50 shrink-0"
                            >
                              <Plus className="w-3 h-3" />
                              追加
                            </button>
                          </div>
                        </div>
                      </div>
                    )}

                    {/* ボリューム */}
                    {(selectedTemplate.volumes ?? []).length > 0 && (
                      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-2">
                        <p className="text-xs font-semibold text-[#111827]">ボリューム</p>
                        <div className="space-y-1.5">
                          {(selectedTemplate.volumes ?? []).map((vol) => (
                            <label key={vol.name} className="flex items-center gap-3 rounded px-2.5 py-2 border border-gray-100 cursor-pointer hover:bg-gray-50 transition-colors">
                              <input
                                type="checkbox"
                                checked={enabledVolumeNames.has(vol.name)}
                                onChange={volEv => {
                                  setEnabledVolumeNames(prev => {
                                    const next = new Set(prev)
                                    if (volEv.target.checked) {
                                      next.add(vol.name)
                                    } else {
                                      next.delete(vol.name)
                                    }
                                    return next
                                  })
                                }}
                                className="rounded border-gray-300"
                              />
                              <span className="text-xs font-medium text-[#111827]">{vol.name}</span>
                              <span className="text-xs text-gray-400">{vol.size_mb >= 1024 ? `${vol.size_mb / 1024}GB` : `${vol.size_mb}MB`}</span>
                              <span className="font-mono text-xs text-gray-400 ml-auto">→ {vol.mount_path}</span>
                            </label>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>

                  {/* エラー + ボタン */}
                  <div className="mt-4 pt-4 border-t border-gray-100">
                    {templateError && (
                      <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-3 text-sm text-red-700 mb-3">
                        {templateError}
                      </div>
                    )}
                    <div className="flex items-center justify-end">
                      <button
                        onClick={() => void handleCreateFromTemplate()}
                        disabled={templateCreating}
                        className="bg-[#111827] text-white text-sm px-6 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
                      >
                        {templateCreating ? '作成中...' : 'デプロイメントを作成 →'}
                      </button>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {/* ステップ2: フォーム */}
        {step === 'form' && selectedType && (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h1 className="text-xl font-semibold text-[#111827]">デプロイメント設定</h1>
              <button
                onClick={() => setStep('type')}
                className="text-sm text-gray-400 hover:text-gray-600 transition-colors"
              >
                ← タイプを変更
              </button>
            </div>

            <div className="bg-white rounded-lg border border-gray-200 p-5 space-y-4">
              {/* 基本設定 */}
              <div>
                <label className={labelClass}>デプロイメント名 *</label>
                <input
                  type="text"
                  value={formData.name}
                  onChange={(event) => setFormData((prev) => ({ ...prev, name: event.target.value }))}
                  placeholder="my-app"
                  maxLength={63}
                  className={inputClass}
                  autoFocus
                />
                <p className="text-xs text-gray-400 mt-1">英小文字・数字・ハイフンのみ、最大63文字</p>
              </div>

              {/* image_url タイプ */}
              {selectedType === 'image_url' && (
                <div>
                  <label className={labelClass}>イメージURL *</label>
                  <input
                    type="text"
                    value={formData.image_url}
                    onChange={(event) => setFormData((prev) => ({ ...prev, image_url: event.target.value }))}
                    placeholder="nginx:latest"
                    className={inputClass}
                  />
                </div>
              )}

              {/* dockerfile / railpack タイプ */}
              {selectedType !== 'image_url' && (
                <>
                  {/* リポジトリURL入力 */}
                  <div>
                    <label className={labelClass}>GitHubリポジトリURL *</label>
                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={formData.github_repo_url}
                        onChange={(event) => setFormData((prev) => ({ ...prev, github_repo_url: event.target.value }))}
                        onKeyDown={(event) => { if (event.key === 'Enter') void loadBranches(formData.github_repo_url) }} // Enter でも読み込む
                        placeholder="https://github.com/org/repo"
                        className={inputClass}
                      />
                      <button
                        type="button"
                        onClick={() => void loadBranches(formData.github_repo_url)}
                        disabled={ghLoading === 'branches' || !formData.github_repo_url}
                        className="shrink-0 px-3 py-2 text-xs rounded-md bg-[#111827] text-white hover:bg-gray-800 transition-colors disabled:opacity-50"
                      >
                        {ghLoading === 'branches' ? '取得中...' : '読み込む'}
                      </button>
                    </div>
                    {ghError && <p className="text-xs text-red-500 mt-1">{ghError}</p>}
                  </div>

                  {/* ブランチ */}
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
                        <option value="">— 一覧から選択 —</option>
                        {ghBranches.map(branchItem => (
                          <option key={branchItem.name} value={branchItem.name}>{branchItem.name}</option>
                        ))}
                      </select>
                    )}
                  </div>

                  {/* コミットSHA */}
                  <div>
                    <label className={labelClass}>コミットSHA（空欄で最新）</label>
                    <input
                      type="text"
                      value={formData.github_commit_sha}
                      onChange={(event) => setFormData(prev => ({ ...prev, github_commit_sha: event.target.value }))}
                      placeholder="例: abc1234（空欄で最新）"
                      className={`${inputClass} font-mono`}
                    />
                    {ghLoading === 'commits' && (
                      <p className="text-xs text-gray-400 mt-1">コミット・ディレクトリを取得中...</p>
                    )}
                    {ghCommits.length > 0 && (
                      <select
                        value={formData.github_commit_sha}
                        onChange={(event) => setFormData(prev => ({ ...prev, github_commit_sha: event.target.value }))}
                        className={`${inputClass} font-mono mt-1`}
                      >
                        <option value="">最新のコミット（HEAD）</option>
                        {ghCommits.map(commitItem => (
                          <option key={commitItem.sha} value={commitItem.sha}>
                            {commitItem.sha.slice(0, 7)} — {commitItem.commit.message.split('\n')[0].slice(0, 55)}
                          </option>
                        ))}
                      </select>
                    )}
                  </div>

                  {/* ビルドディレクトリ */}
                  <div>
                    <label className={labelClass}>ビルドディレクトリ</label>
                    <input
                      type="text"
                      value={formData.github_repo_directory}
                      onChange={(event) => setFormData(prev => ({ ...prev, github_repo_directory: event.target.value }))}
                      placeholder="./"
                      className={`${inputClass} font-mono`}
                    />
                    {ghDirs.length > 0 && (
                      <select
                        value={formData.github_repo_directory}
                        onChange={(event) => setFormData(prev => ({ ...prev, github_repo_directory: event.target.value }))}
                        className={`${inputClass} font-mono mt-1`}
                      >
                        <option value="./">./（ルート）</option>
                        {ghDirs.map(dirPath => (
                          <option key={dirPath} value={dirPath}>{dirPath}</option>
                        ))}
                      </select>
                    )}
                  </div>

                  {/* Dockerfile パス */}
                  {selectedType === 'dockerfile' && (
                    <div>
                      <label className={labelClass}>Dockerfileのパス</label>
                      <input
                        type="text"
                        value={formData.dockerfile_path}
                        onChange={(event) => setFormData((prev) => ({ ...prev, dockerfile_path: event.target.value }))}
                        placeholder="./Dockerfile"
                        className={`${inputClass} font-mono`}
                      />
                    </div>
                  )}
                </>
              )}

              {/* 共通設定 */}
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

            {/* 環境変数 */}
            <div className="bg-white rounded-lg border border-gray-200 p-5 space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold text-[#111827]">環境変数（任意）</h3>
                <span className="text-xs text-gray-400">作成後にも追加できます</span>
              </div>

              {/* 登録済みエントリ一覧 */}
              {envVarEntries.length > 0 && (
                <div className="space-y-1.5">
                  {envVarEntries.map((entry, entryIndex) => (
                    <div key={entryIndex} className="flex items-center gap-2 bg-gray-50 rounded-md px-3 py-2 border border-gray-100">
                      <span className="font-mono text-xs font-medium text-[#111827] truncate">{entry.key}</span>
                      {entry.is_secret ? (
                        <span className="text-[10px] bg-purple-50 text-purple-500 px-1.5 py-0.5 rounded shrink-0">secret</span>
                      ) : (
                        <span className="font-mono text-xs text-gray-400 truncate">{entry.value || '(空)'}</span>
                      )}
                      <button
                        onClick={() => setEnvVarEntries(prev => prev.filter((_, idx) => idx !== entryIndex))}
                        className="ml-auto p-1 rounded hover:bg-red-50 text-gray-300 hover:text-red-400 transition-colors shrink-0"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  ))}
                </div>
              )}

              {/* 追加フォーム */}
              <div className="space-y-2">
                <div className="grid grid-cols-2 gap-2">
                  <input
                    type="text"
                    value={newEnvKey}
                    onChange={ev => setNewEnvKey(ev.target.value)}
                    placeholder="KEY"
                    className={`${inputClass} font-mono`}
                  />
                  <input
                    type={newEnvSecret ? 'password' : 'text'}
                    value={newEnvValue}
                    onChange={ev => setNewEnvValue(ev.target.value)}
                    placeholder="VALUE"
                    className={`${inputClass} font-mono`}
                  />
                </div>
                <div className="flex items-center justify-between">
                  <label className="flex items-center gap-2 text-xs text-gray-500 cursor-pointer select-none">
                    <input
                      type="checkbox"
                      checked={newEnvSecret}
                      onChange={ev => setNewEnvSecret(ev.target.checked)}
                      className="rounded border-gray-300"
                    />
                    シークレット
                  </label>
                  <button
                    onClick={() => {
                      if (!newEnvKey.trim()) return
                      setEnvVarEntries(prev => [...prev, { key: newEnvKey.trim(), value: newEnvValue, is_secret: newEnvSecret }])
                      setNewEnvKey('')
                      setNewEnvValue('')
                      setNewEnvSecret(false)
                    }}
                    disabled={!newEnvKey.trim()}
                    className="flex items-center gap-1 text-xs bg-[#111827] text-white px-3 py-1.5 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
                  >
                    <Plus className="w-3 h-3" />
                    追加
                  </button>
                </div>
              </div>
            </div>

            {/* エラーメッセージ */}
            {error && (
              <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-3 text-sm text-red-700">
                {error}
              </div>
            )}

            {/* アクションボタン */}
            <div className="flex items-center justify-between">
              <button
                onClick={() => setStep('type')}
                className="text-sm text-gray-500 hover:text-gray-700 transition-colors"
              >
                ← 戻る
              </button>
              <button
                onClick={() => void handleCreate()}
                disabled={creating}
                className="bg-[#111827] text-white text-sm px-6 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
              >
                {creating ? '作成中...' : 'デプロイメントを作成 →'}
              </button>
            </div>
          </div>
        )}
      </div>
    </Layout>
  )
}
