import { useState, useEffect } from 'react'
import { checkAuth } from '@/lib/api'

type AuthState = 'loading' | 'authenticated' | 'unauthenticated'

export function useAuth() {
  const [authState, setAuthState] = useState<AuthState>('loading') // 認証状態を管理する

  useEffect(() => {
    let cancelled = false // コンポーネントアンマウント時のstate更新を防ぐ

    checkAuth().then((ok) => { // 認証状態を確認する
      if (!cancelled) {
        setAuthState(ok ? 'authenticated' : 'unauthenticated') // 認証結果を設定する
      }
    })

    return () => { cancelled = true } // クリーンアップ
  }, [])

  return { authState } // 認証状態を返す
}
