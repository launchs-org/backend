import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Layout } from '@/components/Layout'
import { post, QuotaExceededApiError } from '@/lib/api'
import type { Project } from '@/lib/types'

export function ProjectNewPage() {
  const navigate = useNavigate()
  const [name, setName] = useState('') // プロジェクト名を管理する
  const [creating, setCreating] = useState(false) // 作成中フラグ
  const [error, setError] = useState<string | null>(null) // エラーメッセージを管理する

  const handleCreate = async () => {
    if (!name.trim()) {
      setError('プロジェクト名を入力してください')
      return
    }

    setCreating(true)
    setError(null)

    try {
      const project = await post<Project>('/projects', { name: name.trim() }) // プロジェクトを作成する
      navigate(`/projects/${project.id}`) // 作成したプロジェクトの詳細ページへ遷移する
    } catch (createError) {
      console.error(createError)
      if (createError instanceof QuotaExceededApiError) {
        setError(createError.message)
      } else {
        setError('プロジェクトの作成に失敗しました')
      }
    } finally {
      setCreating(false)
    }
  }

  return (
    <Layout breadcrumbs={[{ label: 'New Project' }]}>
      <div className="max-w-md mx-auto pt-8">
        <h1 className="text-xl font-semibold text-[#111827] mb-6">新しいプロジェクト</h1>

        <div className="bg-white rounded-lg border border-gray-200 p-5 space-y-4">
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">プロジェクト名 *</label>
            <input
              type="text"
              value={name}
              onChange={(event) => setName(event.target.value)}
              onKeyDown={(event) => { if (event.key === 'Enter') void handleCreate() }}
              placeholder="my-project"
              maxLength={63}
              className="w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#00C2D1]/50 focus:border-[#00C2D1] transition-colors"
              autoFocus
            />
            <p className="text-xs text-gray-400 mt-1">英小文字・数字・ハイフンのみ、最大63文字</p>
          </div>

          {error && (
            <div className="bg-red-50 border border-red-200 rounded-lg px-3 py-2 text-sm text-red-700">
              {error}
            </div>
          )}
        </div>

        <div className="flex items-center justify-between mt-4">
          <button
            onClick={() => navigate('/')}
            className="text-sm text-gray-500 hover:text-gray-700 transition-colors"
          >
            ← キャンセル
          </button>
          <button
            onClick={() => void handleCreate()}
            disabled={creating || !name.trim()}
            className="bg-[#111827] text-white text-sm px-6 py-2 rounded-md hover:bg-gray-800 transition-colors disabled:opacity-50"
          >
            {creating ? '作成中...' : 'Create Project'}
          </button>
        </div>
      </div>
    </Layout>
  )
}
