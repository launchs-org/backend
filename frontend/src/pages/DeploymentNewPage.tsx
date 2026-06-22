import { useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Container, GitBranch, Package } from 'lucide-react'
import { Layout } from '@/components/Layout'
import { post, QuotaExceededApiError } from '@/lib/api'
import type { Deployment, DeploymentType } from '@/lib/types'

type Step = 'type' | 'form'

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

  const handleTypeSelect = (type: DeploymentType) => {
    setSelectedType(type) // タイプを選択する
    setStep('form') // フォームステップへ進む
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

      await post<Deployment>(`/projects/${projectId}/deployments`, body) // デプロイメントを作成する
      navigate(`/projects/${projectId}`) // プロジェクト画面へ遷移する
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
      <div className="max-w-2xl mx-auto">
        {/* ステップインジケーター */}
        <div className="flex items-center gap-2 mb-8 text-sm">
          <span className={`flex items-center gap-1.5 ${step === 'type' ? 'text-[#00C2D1] font-medium' : 'text-gray-400'}`}>
            <span className={`w-5 h-5 rounded-full text-xs flex items-center justify-center ${step === 'type' ? 'bg-[#00C2D1] text-white' : 'bg-gray-200 text-gray-500'}`}>1</span>
            タイプ選択
          </span>
          <span className="text-gray-300">→</span>
          <span className={`flex items-center gap-1.5 ${step === 'form' ? 'text-[#00C2D1] font-medium' : 'text-gray-400'}`}>
            <span className={`w-5 h-5 rounded-full text-xs flex items-center justify-center ${step === 'form' ? 'bg-[#00C2D1] text-white' : 'bg-gray-200 text-gray-500'}`}>2</span>
            設定
          </span>
        </div>

        {/* ステップ1: タイプ選択 */}
        {step === 'type' && (
          <div className="space-y-4">
            <h1 className="text-xl font-semibold text-[#111827]">デプロイメントタイプを選択</h1>
            <div className="grid grid-cols-1 gap-3">
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

                  {/* ブランチ選択 */}
                  {ghBranches.length > 0 && (
                    <div>
                      <label className={labelClass}>ブランチ *</label>
                      <select
                        value={formData.github_branch}
                        onChange={(event) => void handleBranchSelect(event.target.value)}
                        className={inputClass}
                      >
                        <option value="">ブランチを選択してください</option>
                        {ghBranches.map(branchItem => (
                          <option key={branchItem.name} value={branchItem.name}>{branchItem.name}</option>
                        ))}
                      </select>
                    </div>
                  )}

                  {/* コミット選択 */}
                  {ghLoading === 'commits' && (
                    <p className="text-xs text-gray-400">コミット・ディレクトリを取得中...</p>
                  )}
                  {ghCommits.length > 0 && (
                    <div>
                      <label className={labelClass}>コミット</label>
                      <select
                        value={formData.github_commit_sha}
                        onChange={(event) => setFormData(prev => ({ ...prev, github_commit_sha: event.target.value }))}
                        className={`${inputClass} font-mono`}
                      >
                        <option value="">最新のコミット（HEAD）</option>
                        {ghCommits.map(commitItem => (
                          <option key={commitItem.sha} value={commitItem.sha}>
                            {commitItem.sha.slice(0, 7)} — {commitItem.commit.message.split('\n')[0].slice(0, 55)}
                          </option>
                        ))}
                      </select>
                    </div>
                  )}

                  {/* ディレクトリ選択 */}
                  {ghCommits.length > 0 && ghLoading === null && (
                    <div>
                      <label className={labelClass}>ビルドディレクトリ</label>
                      {ghDirs.length > 0 ? (
                        <select
                          value={formData.github_repo_directory}
                          onChange={(event) => setFormData(prev => ({ ...prev, github_repo_directory: event.target.value }))}
                          className={`${inputClass} font-mono`}
                        >
                          <option value="./">./（ルート）</option>
                          {ghDirs.map(dirPath => (
                            <option key={dirPath} value={dirPath}>{dirPath}</option>
                          ))}
                        </select>
                      ) : (
                        <p className="text-xs text-gray-400 py-2">サブディレクトリなし — ./（ルート）でビルドします</p>
                      )}
                    </div>
                  )}

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
                disabled={creating || (selectedType !== 'image_url' && !formData.github_branch)}
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
