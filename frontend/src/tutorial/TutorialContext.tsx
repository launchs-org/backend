import { createContext, useContext } from 'react' // React Context API をインポートする
import type { ReactNode } from 'react'
import { useTutorial } from './useTutorial' // チュートリアルロジックフックをインポートする
import type { TutorialContextValue } from './types' // Context 型をインポートする

// チュートリアルの Context（初期値は undefined）
const TutorialContext = createContext<TutorialContextValue | undefined>(undefined)

// チュートリアル Context プロバイダー
// App.tsx の BrowserRouter 直内側でラップして使用する
export function TutorialProvider({ children }: { children: ReactNode }) {
  const tutorialValue = useTutorial() // チュートリアルの状態とロジックを取得する

  return (
    <TutorialContext.Provider value={tutorialValue}>
      {children}
    </TutorialContext.Provider>
  )
}

// チュートリアル Context を使うカスタムフック
export function useTutorialContext(): TutorialContextValue {
  const contextValue = useContext(TutorialContext) // Context から値を取得する
  if (contextValue === undefined) {
    throw new Error('useTutorialContext must be used within TutorialProvider') // プロバイダー外での使用を防ぐ
  }
  return contextValue
}
