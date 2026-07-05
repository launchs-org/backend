import { useState, useEffect } from 'react'
import { Layout } from '@/components/Layout'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { get, post, del } from '@/lib/api'
import { toast } from 'sonner'
import { Copy, Check, Trash2, KeyRound } from 'lucide-react'
import type { Quota, CliToken, CreateCliTokenResponse } from '@/lib/types'

export function SettingsPage() {
  const [quota, setQuota] = useState<Quota | null>(null) // クォータ情報を管理する
  const [loading, setLoading] = useState(true) // ローディング状態を管理する

  useEffect(() => {
    get<Quota>('/users/quota')
      .then(setQuota)
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  return (
    <Layout breadcrumbs={[{ label: 'Settings' }]}>
      <div className="max-w-2xl space-y-6">
        <h1 className="text-xl font-semibold text-[#111827]">設定</h1>

        {/* クォータ情報 */}
        <div className="bg-white rounded-lg border border-gray-200 p-5">
          <h2 className="text-sm font-semibold text-[#111827] mb-4">リソースクォータ</h2>

          {loading ? (
            <div className="space-y-3 animate-pulse">
              {[...Array(4)].map((_, skeletonIndex) => (
                <div key={skeletonIndex} className="h-8 bg-gray-100 rounded" />
              ))}
            </div>
          ) : quota ? (
            <div className="space-y-4">
              <QuotaRow
                label="プロジェクト"
                current={quota.current_projects}
                max={quota.max_projects}
              />
              <QuotaRow
                label="デプロイメント"
                current={quota.current_deployments}
                max={quota.max_deployments}
              />
              <QuotaRow
                label="最大レプリカ数 / デプロイ"
                current={0}
                max={quota.max_replicas_per_deployment}
              />
              <QuotaRow
                label="ボリューム数"
                current={quota.current_volumes}
                max={quota.max_volumes}
              />
              <QuotaRow
                label="ボリューム総容量"
                current={quota.current_total_volume_mb}
                max={quota.max_total_volume_mb}
                unit="MB"
              />
              <QuotaRow
                label="1ボリューム最大サイズ"
                current={0}
                max={quota.max_volume_size_mb}
                unit="MB"
              />
            </div>
          ) : (
            <p className="text-sm text-gray-400">クォータ情報を取得できませんでした</p>
          )}
        </div>

        {/* CLIトークン管理 */}
        <CliTokenSection />
      </div>
    </Layout>
  )
}

function QuotaRow({
  label,
  current,
  max,
  unit = '',
}: {
  label: string
  current: number
  max: number
  unit?: string
}) {
  const pct = max > 0 ? Math.min((current / max) * 100, 100) : 0 // 使用率を計算する
  const isWarning = pct >= 80 // 80%以上で警告色にする

  return (
    <div>
      <div className="flex justify-between text-sm mb-1.5">
        <span className="text-gray-600">{label}</span>
        <span className={isWarning ? 'text-amber-600 font-medium' : 'text-gray-400'}>
          {current}{unit} / {max}{unit}
        </span>
      </div>
      <div className="h-2 bg-gray-100 rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full transition-all ${isWarning ? 'bg-amber-400' : 'bg-[#00C2D1]'}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  )
}

function CliTokenSection() {
  const [tokenList, setTokenList] = useState<CliToken[]>([]) // 発行済みCLIトークン一覧
  const [loading, setLoading] = useState(true) // 一覧取得中フラグ
  const [issuing, setIssuing] = useState(false) // 発行中フラグ
  const [newName, setNewName] = useState('') // 新規トークンの用途ラベル
  const [newExpiresInDays, setNewExpiresInDays] = useState('') // 新規トークンの有効期限（日数、空欄は無期限）
  const [issuedToken, setIssuedToken] = useState<CreateCliTokenResponse | null>(null) // 発行直後の平文トークン
  const [copied, setCopied] = useState(false) // 発行直後トークンのコピー完了フィードバック
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null) // 失効確認ダイアログ対象ID
  const [revokingId, setRevokingId] = useState<string | null>(null) // 失効処理中ID

  const fetchTokenList = () => {
    setLoading(true)
    get<CliToken[]>('/cli-tokens')
      .then(setTokenList)
      .catch(console.error)
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    fetchTokenList()
  }, [])

  const handleIssue = async () => {
    if (!newName) return
    setIssuing(true) // 発行中フラグを立てる
    try {
      const expiresInDays = newExpiresInDays ? Number(newExpiresInDays) : 0 // 空欄は0（無期限）扱いにする
      const created = await post<CreateCliTokenResponse>('/cli-tokens', {
        name: newName,
        expires_in_days: expiresInDays,
      })
      setIssuedToken(created) // 平文トークンを一度だけ表示するためstateに保持する
      setNewName('') // フォームをリセットする
      setNewExpiresInDays('') // 有効期限入力をリセットする
      fetchTokenList() // 一覧を再取得する
    } catch (issueError) {
      console.error(issueError)
      toast.error(issueError instanceof Error ? issueError.message : 'CLIトークンの発行に失敗しました') // エラートーストを表示する
    } finally {
      setIssuing(false) // 発行中フラグを下げる
    }
  }

  const handleCopyIssuedToken = async () => {
    if (!issuedToken) return
    await navigator.clipboard.writeText(issuedToken.token) // クリップボードに書き込む
    setCopied(true) // コピー完了フィードバックを表示する
    setTimeout(() => setCopied(false), 1500) // 1.5秒後にフィードバックをリセットする
  }

  const handleRevoke = async (cliTokenId: string) => {
    setRevokingId(cliTokenId) // 失効処理中IDを設定する
    try {
      await del(`/cli-tokens/${cliTokenId}`) // トークンを失効させる
      fetchTokenList() // 一覧を再取得する
      toast.success('CLIトークンを失効しました') // 成功トーストを表示する
    } catch (revokeError) {
      console.error(revokeError)
      toast.error(revokeError instanceof Error ? revokeError.message : 'CLIトークンの失効に失敗しました') // エラートーストを表示する
    } finally {
      setRevokingId(null) // 失効処理中フラグを下げる
    }
  }

  const isRevoked = (cliTokenData: CliToken) => cliTokenData.revoked_at !== null // 失効済みかどうかを判定する
  const isExpired = (cliTokenData: CliToken) =>
    cliTokenData.expires_at !== null && new Date(cliTokenData.expires_at) < new Date() // 期限切れかどうかを判定する

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-5">
      <div className="flex items-center gap-2 mb-1">
        <KeyRound className="w-4 h-4 text-gray-400" />
        <h2 className="text-sm font-semibold text-[#111827]">CLIトークン</h2>
      </div>
      <p className="text-xs text-gray-400 mb-4">
        CLI から API を利用するための長期トークンを発行・管理します。トークンは発行時にのみ表示され、以後は再取得できません。
      </p>

      {/* 発行直後の平文トークン表示 */}
      {issuedToken && (
        <div className="mb-4 bg-amber-50 border border-amber-200 rounded-md p-3 space-y-2">
          <p className="text-xs font-medium text-amber-800">
            発行されたトークン（このタイミングでのみ表示されます。必ず控えてください）
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 min-w-0 truncate text-xs font-mono bg-white border border-amber-200 rounded px-2 py-1.5">
              {issuedToken.token}
            </code>
            <button
              onClick={() => void handleCopyIssuedToken()}
              className="p-1.5 rounded text-amber-600 hover:bg-amber-100 transition-colors shrink-0"
              title="コピー"
            >
              {copied ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4" />}
            </button>
          </div>
          <button
            onClick={() => setIssuedToken(null)}
            className="text-xs text-amber-700 hover:text-amber-900 underline"
          >
            閉じる
          </button>
        </div>
      )}

      {/* トークン一覧 */}
      <div className="space-y-1.5 mb-4">
        {loading ? (
          <div className="space-y-2 animate-pulse">
            {[...Array(2)].map((_, skeletonIndex) => (
              <div key={skeletonIndex} className="h-10 bg-gray-100 rounded" />
            ))}
          </div>
        ) : tokenList.length === 0 ? (
          <p className="text-xs text-gray-400 py-2">発行済みのCLIトークンはありません</p>
        ) : (
          tokenList.map(cliTokenData => (
            <div key={cliTokenData.id} className="bg-gray-50 rounded-md px-3 py-2 border border-gray-100 flex items-center justify-between gap-2">
              <div className="min-w-0 flex-1 space-y-0.5">
                <div className="flex items-center gap-1.5">
                  <span className="text-sm font-medium text-[#111827] truncate">{cliTokenData.name}</span>
                  {isRevoked(cliTokenData) && (
                    <span className="text-[10px] bg-red-50 text-red-500 px-1.5 py-0.5 rounded shrink-0">失効済み</span>
                  )}
                  {!isRevoked(cliTokenData) && isExpired(cliTokenData) && (
                    <span className="text-[10px] bg-gray-100 text-gray-500 px-1.5 py-0.5 rounded shrink-0">期限切れ</span>
                  )}
                </div>
                <p className="text-[11px] text-gray-400">
                  {cliTokenData.expires_at
                    ? `有効期限: ${new Date(cliTokenData.expires_at).toLocaleDateString()}`
                    : '無期限'}
                </p>
              </div>
              {!isRevoked(cliTokenData) && (
                <button
                  onClick={() => setDeleteConfirmId(cliTokenData.id)}
                  disabled={revokingId === cliTokenData.id}
                  className="p-1.5 rounded text-gray-300 hover:text-red-400 hover:bg-red-50 transition-colors disabled:opacity-50 shrink-0"
                  title="失効"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              )}
            </div>
          ))
        )}
      </div>

      {/* 発行フォーム */}
      <div className="border-t border-gray-100 pt-3 space-y-2.5">
        <p className="text-xs font-semibold text-gray-500">新しいCLIトークンを発行</p>
        <div className="flex items-center gap-2">
          <Input
            type="text"
            value={newName}
            onChange={ev => setNewName(ev.target.value)}
            placeholder="用途（例: my-laptop）"
            className="flex-1 text-sm"
          />
          <Input
            type="number"
            min={0}
            value={newExpiresInDays}
            onChange={ev => setNewExpiresInDays(ev.target.value)}
            placeholder="有効日数（空欄で無期限）"
            className="w-40 text-sm"
          />
          <Button
            onClick={() => void handleIssue()}
            disabled={!newName || issuing}
            size="sm"
          >
            {issuing ? '発行中...' : '発行'}
          </Button>
        </div>
      </div>

      {/* 失効確認ダイアログ */}
      <ConfirmDialog
        open={deleteConfirmId !== null}
        onOpenChange={open => { if (!open) setDeleteConfirmId(null) }}
        title="CLIトークンを失効"
        description="このCLIトークンを失効しますか？失効後はこのトークンでの認証ができなくなります。この操作は取り消せません。"
        confirmLabel="失効"
        variant="destructive"
        onConfirm={async () => {
          const targetId = deleteConfirmId // 失効対象IDを保持する
          setDeleteConfirmId(null) // ダイアログを閉じる
          if (targetId) await handleRevoke(targetId) // トークンを失効させる
        }}
      />
    </div>
  )
}
