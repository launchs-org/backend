import { useState, useRef, useLayoutEffect, useEffect } from 'react' // React フックをインポートする
import { createPortal } from 'react-dom' // Portal で body にマウントするためインポートする
import { X, ChevronRight, BookOpen, HelpCircle } from 'lucide-react' // アイコンをインポートする
import { useTutorialContext } from './TutorialContext' // チュートリアル Context をインポートする
import { GlossaryModal } from './GlossaryModal' // 用語解説モーダルをインポートする
import { getTrackProgress } from './steps' // トラック内進捗を計算する関数をインポートする
import type { PopupPlacement, TutorialTrack } from './types' // 型をインポートする

// ポップアップの表示位置を計算する
function calcPopupPosition(
  targetRect: DOMRect | null,
  placement: PopupPlacement,
  popupWidth: number,
  popupHeight: number,
): { top: number; left: number } {
  const MARGIN = 14 // ハイライト要素との間隔（px）
  const EDGE_MARGIN = 8 // ビューポート端からの最小余白（px）
  const vw = window.innerWidth // ビューポート幅
  const vh = window.innerHeight // ビューポート高さ

  // ターゲットなし（center）またはターゲット要素が取得できない場合は中央に配置する
  if (placement === 'center' || !targetRect) {
    return {
      top: Math.max(EDGE_MARGIN, vh / 2 - popupHeight / 2), // 縦中央
      left: Math.max(EDGE_MARGIN, vw / 2 - popupWidth / 2), // 横中央
    }
  }

  // clamp 関数：値を min〜max の範囲に収める
  const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max)

  const centerX = targetRect.left + targetRect.width / 2 // ターゲットの水平中心
  const centerY = targetRect.top + targetRect.height / 2 // ターゲットの垂直中心

  switch (placement) {
    case 'bottom':
      return {
        top: clamp(targetRect.bottom + MARGIN, EDGE_MARGIN, vh - popupHeight - EDGE_MARGIN),
        left: clamp(centerX - popupWidth / 2, EDGE_MARGIN, vw - popupWidth - EDGE_MARGIN),
      }
    case 'top':
      return {
        top: clamp(targetRect.top - MARGIN - popupHeight, EDGE_MARGIN, vh - popupHeight - EDGE_MARGIN),
        left: clamp(centerX - popupWidth / 2, EDGE_MARGIN, vw - popupWidth - EDGE_MARGIN),
      }
    case 'right':
      return {
        top: clamp(centerY - popupHeight / 2, EDGE_MARGIN, vh - popupHeight - EDGE_MARGIN),
        left: clamp(targetRect.right + MARGIN, EDGE_MARGIN, vw - popupWidth - EDGE_MARGIN),
      }
    case 'left':
      return {
        top: clamp(centerY - popupHeight / 2, EDGE_MARGIN, vh - popupHeight - EDGE_MARGIN),
        left: clamp(targetRect.left - MARGIN - popupWidth, EDGE_MARGIN, vw - popupWidth - EDGE_MARGIN),
      }
    default:
      return { top: EDGE_MARGIN, left: EDGE_MARGIN }
  }
}

// トラック一覧の定義
const TRACK_MENU: { track: TutorialTrack; label: string; description: string; emoji: string }[] = [
  { track: 'basic', label: '基本チュートリアル', description: 'プロジェクト・デプロイ・IngressRoute作成の基本的な流れを学ぶ', emoji: '🚀' },
  { track: 'adv-storage', label: 'ストレージ', description: 'ボリュームの作成とデプロイメントへのマウント方法を学ぶ', emoji: '💾' },
  { track: 'adv-envvar', label: '環境変数', description: '環境変数の作成とデプロイメントへの適用方法を学ぶ', emoji: '🔑' },
  { track: 'adv-quota', label: 'リソースQuota', description: 'プロジェクトのリソース使用量の確認方法を学ぶ', emoji: '📊' },
]

// チュートリアルのオーバーレイ UI コンポーネント
// ReactDOM.createPortal で document.body にマウントし、既存の z-index 階層を壊さない
export function TutorialOverlay() {
  const { currentStep, targetRect, isActive, stepIndex, advance, skip, startTrack } =
    useTutorialContext() // チュートリアルの状態を取得する

  const [helpPanelOpen, setHelpPanelOpen] = useState(false) // ? パネルの開閉状態
  const [glossaryOpen, setGlossaryOpen] = useState(false) // 用語解説モーダルの開閉状態
  const popupRef = useRef<HTMLDivElement>(null) // ポップアップ要素の参照
  const [popupPos, setPopupPos] = useState({ top: 0, left: 0 }) // ポップアップの表示位置

  // ハイライト対象要素を z-index で浮き上がらせてクリックを通す
  useEffect(() => {
    if (!currentStep?.targetId) return // ターゲットなし時は何もしない
    const element = currentStep.targetId
      ? document.querySelector<HTMLElement>(`[data-tutorial="${currentStep.targetId}"]`)
      : null
    if (!element) return

    const originalPosition = element.style.position // 元の position を保存する
    const originalZIndex = element.style.zIndex // 元の z-index を保存する

    element.style.position = 'relative' // z-index が効くよう position を設定する
    element.style.zIndex = '10001' // オーバーレイ（9998）より上に浮かせる

    return () => {
      element.style.position = originalPosition // 元に戻す
      element.style.zIndex = originalZIndex // 元に戻す
    }
  }, [currentStep?.targetId, targetRect]) // targetRect も依存に入れて要素出現後に再実行する

  // ポップアップの実サイズを取得して位置を計算する
  useLayoutEffect(() => {
    if (!currentStep) return // ステップがない場合は何もしない
    const popupWidth = popupRef.current?.offsetWidth ?? 320 // ポップアップの実幅を取得する
    const popupHeight = popupRef.current?.offsetHeight ?? 200 // ポップアップの実高さを取得する
    const newPos = calcPopupPosition(targetRect, currentStep.placement, popupWidth, popupHeight) // 位置を計算する
    setPopupPos(newPos) // 位置を更新する
  }, [currentStep, targetRect]) // ステップまたはターゲット位置が変わったら再計算する

  // ? パネルの外クリックで閉じる
  useEffect(() => {
    if (!helpPanelOpen) return // パネルが開いていない場合は何もしない
    const handleOutsideClick = (event: MouseEvent) => {
      const target = event.target as HTMLElement
      if (!target.closest('[data-help-panel]')) {
        setHelpPanelOpen(false) // パネル外クリックで閉じる
      }
    }
    document.addEventListener('mousedown', handleOutsideClick)
    return () => document.removeEventListener('mousedown', handleOutsideClick)
  }, [helpPanelOpen])

  // チュートリアルが非アクティブ、またはステップがない場合も ? ボタンだけは表示する
  const trackProgress = isActive && currentStep ? getTrackProgress(stepIndex) : { current: 1, total: 1 } // トラック内進捗を計算する
  const isLastStep = isActive && currentStep ? trackProgress.current === trackProgress.total : false // トラック内最後のステップかどうかを判定する

  // ? ボタン + パネル（常時表示）
  const helpButton = createPortal(
    <div data-help-panel style={{ position: 'fixed', bottom: 24, right: 24, zIndex: 10003 }}>
      {/* トラック選択パネル */}
      {helpPanelOpen && (
        <div
          className="absolute bottom-14 right-0 bg-white rounded-xl shadow-2xl border border-gray-100 overflow-hidden"
          style={{ width: 280 }}
        >
          {/* パネルヘッダー */}
          <div className="flex items-center justify-between px-4 py-3 border-b border-gray-100">
            <span className="text-sm font-semibold text-gray-800">チュートリアルを選ぶ</span>
            <button
              onClick={() => setHelpPanelOpen(false)} // パネルを閉じる
              className="p-1 rounded hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
              aria-label="閉じる"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
          {/* トラック一覧 */}
          <div className="py-2">
            {TRACK_MENU.map(({ track, label, description, emoji }) => (
              <button
                key={track}
                onClick={() => {
                  startTrack(track) // 選択したトラックを開始する
                  setHelpPanelOpen(false) // パネルを閉じる
                }}
                className="w-full flex items-start gap-3 px-4 py-3 hover:bg-gray-50 transition-colors text-left"
              >
                <span className="text-xl leading-none mt-0.5">{emoji}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium text-gray-800">{label}</div>
                  <div className="text-xs text-gray-500 mt-0.5 leading-relaxed">{description}</div>
                </div>
              </button>
            ))}
          </div>
        </div>
      )}

      {/* ? フローティングボタン */}
      <button
        onClick={() => setHelpPanelOpen(prev => !prev)} // パネルの開閉を切り替える
        className="w-12 h-12 rounded-full bg-[#00C2D1] text-white shadow-lg hover:bg-[#00A8B5] transition-colors flex items-center justify-center"
        aria-label="チュートリアルメニューを開く"
      >
        <HelpCircle className="w-5 h-5" />
      </button>
    </div>,
    document.body,
  )

  // チュートリアルが非アクティブ、またはステップがない場合はオーバーレイなしで ? ボタンのみ表示する
  if (!isActive || !currentStep) return <>{helpButton}</>

  return (
    <>
      {helpButton}
      {createPortal(
    <>
      {/* スポットライトオーバーレイ */}
      {targetRect ? (
        // ターゲット要素がある場合：clip-path で対象矩形を切り抜いた暗幕
        // clip-path の外側（切り抜き部分）は表示されないため、穴の部分はクリックも透過する
        <div
          aria-hidden="true"
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0, 0, 0, 0.55)',
            zIndex: 9998,
            // polygon で「画面全体 - 対象矩形の穴」を定義する
            // 外周（時計回り）→ 内側の穴（反時計回り）で穴あき形状を作る
            clipPath: `polygon(
              0% 0%, 100% 0%, 100% 100%, 0% 100%, 0% 0%,
              ${targetRect.left - 4}px ${targetRect.top - 4}px,
              ${targetRect.left - 4}px ${targetRect.bottom + 4}px,
              ${targetRect.right + 4}px ${targetRect.bottom + 4}px,
              ${targetRect.right + 4}px ${targetRect.top - 4}px,
              ${targetRect.left - 4}px ${targetRect.top - 4}px
            )`,
            pointerEvents: 'none', // 暗幕自体はクリック透過（穴の外もユーザー操作を止めない）
            transition: 'clip-path 0.2s ease',
          }}
        />
      ) : (
        // ターゲット要素がない場合：フルスクリーン半透明オーバーレイ
        <div
          aria-hidden="true"
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0, 0, 0, 0.55)',
            zIndex: 9998,
            pointerEvents: 'none',
          }}
        />
      )}

      {/* ポップアップバブル */}
      {/* ポップアップ全体は pointerEvents: none にして、内側のボタン等だけ auto に戻す */}
      {/* これにより、ポップアップの背後にある対象要素のクリックがブロックされない */}
      <div
        ref={popupRef}
        role="dialog"
        aria-modal="false"
        aria-label={currentStep.title}
        style={{
          position: 'fixed',
          top: popupPos.top,
          left: popupPos.left,
          width: 320,
          zIndex: 10002,
          pointerEvents: 'none', // ポップアップ全体はクリック透過にする
          transition: 'top 0.2s ease, left 0.2s ease', // アニメーション
        }}
        className="bg-white rounded-xl shadow-2xl border border-gray-100 overflow-hidden"
      >
        {/* ポップアップヘッダー（クリック透過を解除してボタンを操作可能にする） */}
        <div style={{ pointerEvents: 'auto' }} className="flex items-start justify-between px-4 pt-4 pb-2">
          <div className="flex-1 pr-2">
            {/* ステップ進捗インジケーター（トラック内の位置を表示） */}
            <div className="flex items-center gap-2 mb-2">
              <span className="text-xs font-medium text-[#00C2D1] tabular-nums">
                {trackProgress.current} / {trackProgress.total}
              </span>
              {/* トラック内の進捗バー */}
              <div className="flex-1 h-1 bg-gray-200 rounded-full overflow-hidden">
                <div
                  className="h-full bg-[#00C2D1] rounded-full transition-all"
                  style={{ width: `${(trackProgress.current / trackProgress.total) * 100}%` }}
                />
              </div>
            </div>
            <h2 className="text-sm font-semibold text-gray-800 leading-tight">
              {currentStep.title} {/* ステップタイトル */}
            </h2>
          </div>
          {/* スキップボタン */}
          <button
            onClick={skip}
            className="p-1 rounded hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors shrink-0"
            aria-label="チュートリアルをスキップ"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>

        {/* ポップアップ本文（クリック透過を解除してボタンを操作可能にする） */}
        <div style={{ pointerEvents: 'auto' }} className="px-4 pb-4">
          <p className="text-xs text-gray-600 leading-relaxed mb-3">
            {currentStep.body} {/* ステップの説明文 */}
          </p>

          {/* ボタン群 */}
          <div className="flex items-center gap-2">
            {/* 用語解説ボタン（showGlossary ステップのみ表示） */}
            {currentStep.showGlossary && (
              <button
                onClick={() => setGlossaryOpen(true)} // 用語解説モーダルを開く
                className="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-700 border border-gray-200 rounded-md px-2.5 py-1.5 hover:bg-gray-50 transition-colors"
              >
                <BookOpen className="w-3 h-3" />
                用語解説
              </button>
            )}

            {/* スペーサー */}
            <div className="flex-1" />

            {/* 次へ / 完了ボタン */}
            <button
              onClick={advance} // 次のステップへ進む
              className="flex items-center gap-1 text-xs font-medium bg-[#00C2D1] text-white rounded-md px-3 py-1.5 hover:bg-[#00A8B5] transition-colors"
            >
              {isLastStep ? '完了' : '次へ'}
              {!isLastStep && <ChevronRight className="w-3 h-3" />} {/* 最後以外は矢印を表示する */}
            </button>
          </div>
        </div>
      </div>

      {/* 用語解説モーダル */}
      <GlossaryModal open={glossaryOpen} onOpenChange={setGlossaryOpen} />
    </>,
    document.body, // body に直接マウントして z-index 問題を回避する
  )}
    </>
  )
}
