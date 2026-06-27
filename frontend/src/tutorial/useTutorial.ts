import { useState, useEffect, useCallback } from 'react' // React フックをインポートする
import { useLocation } from 'react-router-dom' // 現在のパスを取得するフック
import { TUTORIAL_STEPS, getTrackStartIndex } from './steps' // ステップ定義をインポートする
import type { TutorialStep, TutorialContextValue, TutorialTrack } from './types' // 型をインポートする

const STORAGE_STEP = 'tutorial:step' // 現在ステップを保存するキー
const STORAGE_DONE = 'tutorial:completed' // 完了フラグを保存するキー

// ステップの page フィールド（:id などワイルドカードあり）と現在パスを照合する
function matchPage(stepPage: string, currentPath: string): boolean {
  if (stepPage === '/') return currentPath === '/' // ルートは完全一致のみ
  // :xxx セグメントを「スラッシュを含まない1文字以上の文字列」にマッチするパターンに変換する
  // ただし固定セグメント（new など）と衝突しないよう、固定パスを先に優先マッチする
  const pattern = stepPage.replace(/:[^/]+/g, '[^/]+')
  const regex = new RegExp(`^${pattern}$`)
  if (!regex.test(currentPath)) return false // パターンに一致しない場合は即 false
  // ワイルドカードを含むパターンの場合、より具体的なステップ（固定パス）が存在するか確認して
  // 固定パスにも一致する場合はこのステップのマッチを無効にする
  if (stepPage.includes(':')) {
    const hasFixedMatch = TUTORIAL_STEPS.some(step => {
      if (step.page === stepPage) return false // 自分自身は除外する
      if (step.page.includes(':')) return false // 他のワイルドカードパスも除外する
      return step.page === currentPath // 固定パスが完全一致するか確認する
    })
    if (hasFixedMatch) return false // より具体的な固定パスが存在するなら負け
  }
  return true
}

// チュートリアルのロジックを担うカスタムフック
export function useTutorial(): TutorialContextValue {
  const location = useLocation() // 現在のパスを取得する

  // localStorage から初期ステップインデックスを読み込む
  const [stepIndex, setStepIndex] = useState<number>(() => {
    if (localStorage.getItem(STORAGE_DONE) === 'true') return -1 // 完了済みなら非アクティブにする
    const savedIndex = localStorage.getItem(STORAGE_STEP) // 保存済みインデックスを取得する
    const parsedIndex = savedIndex !== null ? parseInt(savedIndex, 10) : 0 // 数値に変換する
    if (isNaN(parsedIndex) || parsedIndex < 0 || parsedIndex >= TUTORIAL_STEPS.length) return 0 // 不正値はリセットする
    return parsedIndex
  })

  const [targetRect, setTargetRect] = useState<DOMRect | null>(null) // ハイライト対象の位置を保持する
  const [paused, setPaused] = useState(false) // 一時停止フラグ（サイドバー開中などに使う）

  // iframe など別フレームが localStorage を書き換えたとき（storage イベント）に stepIndex を同期する
  useEffect(() => {
    const handleStorage = (event: StorageEvent) => {
      if (event.key === STORAGE_DONE && event.newValue === 'true') {
        setStepIndex(-1) // 完了フラグが立ったら非アクティブにする
        return
      }
      if (event.key === STORAGE_STEP && event.newValue !== null) {
        const parsedIndex = parseInt(event.newValue, 10) // 新しいインデックスを取得する
        if (!isNaN(parsedIndex) && parsedIndex >= 0 && parsedIndex < TUTORIAL_STEPS.length) {
          setStepIndex(parsedIndex) // 他フレームの進行を反映する
        }
      }
    }
    window.addEventListener('storage', handleStorage) // storage イベントを購読する
    return () => window.removeEventListener('storage', handleStorage)
  }, [])

  const isActive = stepIndex >= 0 && stepIndex < TUTORIAL_STEPS.length // チュートリアルが進行中かどうか
  const currentStep: TutorialStep | null = isActive ? (TUTORIAL_STEPS[stepIndex] ?? null) : null // 現在のステップオブジェクト

  // ページ遷移を検知して、次のステップのページに一致したら自動的にステップを進める
  useEffect(() => {
    if (!isActive) return // 非アクティブなら何もしない

    const currentPath = location.pathname // 現在のパスを取得する

    // 現在のステップがこのページ対象なら何もしない（そのまま表示する）
    if (currentStep && matchPage(currentStep.page, currentPath)) return

    // 現在ステップより先のステップで、このページに対応するものを探す
    for (let searchIndex = stepIndex + 1; searchIndex < TUTORIAL_STEPS.length; searchIndex++) {
      const nextStep = TUTORIAL_STEPS[searchIndex] // 次の候補ステップを取得する
      if (nextStep && matchPage(nextStep.page, currentPath)) {
        // このページに対応するステップに自動ジャンプする
        localStorage.setItem(STORAGE_STEP, String(searchIndex)) // 進捗を保存する
        setStepIndex(searchIndex) // ステップを更新する
        return
      }
    }
  }, [location.pathname]) // パスが変わるたびに確認する（isActive/currentStep/stepIndex は意図的に除外）

  // 現在のパスがステップの対象ページかどうかを確認する
  const isOnCorrectPage = currentStep !== null && matchPage(currentStep.page, location.pathname)

  // ハイライト対象要素の DOMRect をリアクティブに追跡する
  useEffect(() => {
    if (!currentStep?.targetId || !isOnCorrectPage) { // ターゲットなし or ページ不一致のときはクリアする
      setTargetRect(null)
      return
    }

    const targetId = currentStep.targetId // クロージャ用にコピーする

    const updateRect = () => {
      const element = document.querySelector(`[data-tutorial="${targetId}"]`) // data-tutorial 属性で要素を取得する
      if (element) {
        setTargetRect(element.getBoundingClientRect()) // 要素の位置・サイズを取得する
      } else {
        setTargetRect(null) // 要素が見つからない場合はクリアする
      }
    }

    updateRect() // 初回実行する

    const resizeObserver = new ResizeObserver(updateRect) // 要素サイズ変更を監視する
    const attachResizeObserver = () => {
      const element = document.querySelector(`[data-tutorial="${targetId}"]`)
      if (element) resizeObserver.observe(element) // 要素を監視対象に追加する
    }
    attachResizeObserver()

    // 要素がまだ DOM にない場合（非同期レンダリング）、MutationObserver で出現を待つ
    const mutationObserver = new MutationObserver(() => {
      const element = document.querySelector(`[data-tutorial="${targetId}"]`)
      if (element) {
        updateRect() // 要素が追加されたら位置を取得する
        attachResizeObserver() // ResizeObserver にも登録する
      }

      // 次のステップが autoAdvanceOnAppear の場合、そのターゲット要素が出現したら自動進行する
      // targetId は複数ステップで共有される場合があるため、stepIndex を直接使って現在位置を特定する
      const currentStepIndex = parseInt(localStorage.getItem(STORAGE_STEP) ?? '-1', 10)
      if (currentStepIndex < 0) return // 無効なインデックスは無視する
      const nextStep = TUTORIAL_STEPS[currentStepIndex + 1]
      if (nextStep?.autoAdvanceOnAppear && nextStep.targetId) {
        const nextElement = document.querySelector(`[data-tutorial="${nextStep.targetId}"]`)
        if (nextElement) {
          const nextIndex = currentStepIndex + 1 // 次のステップインデックスを計算する
          localStorage.setItem(STORAGE_STEP, String(nextIndex)) // 進捗を保存する
          setStepIndex(nextIndex) // 次のステップに自動進行する
        }
      }
    })
    mutationObserver.observe(document.body, { childList: true, subtree: true }) // DOM 変更を監視する

    window.addEventListener('scroll', updateRect, true) // スクロール時に位置を再計算する
    window.addEventListener('resize', updateRect) // ウィンドウリサイズ時に位置を再計算する

    return () => {
      mutationObserver.disconnect() // DOM 変更の監視を解除する
      resizeObserver.disconnect() // サイズ変更の監視を解除する
      window.removeEventListener('scroll', updateRect, true)
      window.removeEventListener('resize', updateRect)
    }
  }, [currentStep?.targetId, isOnCorrectPage]) // ステップまたはページが変わったら再実行する

  // 次のステップへ進む
  const advance = useCallback(() => {
    setStepIndex(prevIndex => {
      const nextIndex = prevIndex + 1 // 次のインデックスを計算する
      if (nextIndex >= TUTORIAL_STEPS.length) {
        // 全ステップ完了時
        localStorage.setItem(STORAGE_DONE, 'true') // 完了フラグを保存する
        localStorage.removeItem(STORAGE_STEP) // ステップキーを削除する
        return -1 // 非アクティブにする
      }
      localStorage.setItem(STORAGE_STEP, String(nextIndex)) // 進捗を保存する
      return nextIndex
    })
  }, [])

  // チュートリアルをスキップ（完了扱いにする）
  const skip = useCallback(() => {
    localStorage.setItem(STORAGE_DONE, 'true') // 完了フラグを保存する
    localStorage.removeItem(STORAGE_STEP) // ステップキーを削除する
    setStepIndex(-1) // 非アクティブにする
  }, [])

  // チュートリアルをリセット（最初から始める）
  const reset = useCallback(() => {
    localStorage.removeItem(STORAGE_DONE) // 完了フラグを削除する
    localStorage.setItem(STORAGE_STEP, '0') // ステップを0にリセットする
    setStepIndex(0) // ステップ0から再開する
  }, [])

  // 指定したトラックの先頭から開始する
  const startTrack = useCallback((track: TutorialTrack) => {
    const startIndex = getTrackStartIndex(track) // トラックの開始インデックスを取得する
    localStorage.removeItem(STORAGE_DONE) // 完了フラグを削除する
    localStorage.setItem(STORAGE_STEP, String(startIndex)) // 開始インデックスを保存する
    setStepIndex(startIndex) // 指定トラックから開始する
  }, [])

  // チュートリアル表示を一時停止する（サイドバー開中など）
  const pause = useCallback(() => setPaused(true), [])

  // 一時停止を解除する
  const resume = useCallback(() => setPaused(false), [])

  // ページが対象外またはポーズ中はオーバーレイを表示しない
  const visibleStep = isOnCorrectPage && !paused ? currentStep : null // ページ一致かつ非ポーズのときのみ公開する

  return {
    currentStep: visibleStep,
    actualStep: currentStep, // pause中でも実際のステップを返す（ロジック判定用）
    targetRect: isOnCorrectPage && !paused ? targetRect : null,
    isActive,
    stepIndex,
    totalSteps: TUTORIAL_STEPS.length,
    advance,
    skip,
    reset,
    startTrack,
    pause,
    resume,
  }
}
