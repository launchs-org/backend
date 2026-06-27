// チュートリアル機能全体の型定義

export type TutorialStepId =
  | 'welcome'
  | 'new-project-button'
  | 'project-name-input'
  | 'project-create-button'
  | 'add-deployment-button'
  | 'add-deployment-menu'
  | 'deployment-type-select'
  | 'deployment-name-input'
  | 'deployment-image-input'
  | 'deployment-create-button'
  | 'deployment-open-card'
  | 'deployment-networking-tab'
  | 'deployment-port-input'
  | 'deployment-target-port-input'
  | 'deployment-service-save'
  | 'deployment-envvars-tab'
  | 'deployment-volumes-tab'
  | 'deployment-history-tab'
  | 'deployment-apply-button'
  | 'deployment-close-sidebar'
  | 'ingress-creation'
  | 'ingress-menu'
  | 'ingress-overview'
  | 'ingress-paths-tab'
  | 'ingress-service-select'
  | 'ingress-add-path-rule'
  | 'ingress-apply'
  | 'complete'
  // ── アドバンス：ストレージ ──
  | 'adv-storage-intro'
  | 'adv-storage-add-button'
  | 'adv-storage-menu'
  | 'adv-storage-form'
  | 'adv-storage-size'
  | 'adv-storage-create'
  | 'adv-storage-open-card'
  | 'adv-storage-mount-tab'
  | 'adv-storage-mount-select'
  | 'adv-storage-mount-path'
  | 'adv-storage-mount-add'
  | 'adv-storage-apply'
  | 'adv-storage-complete'
  // ── アドバンス：環境変数 ──
  | 'adv-envvar-intro'
  | 'adv-envvar-add-button'
  | 'adv-envvar-menu'
  | 'adv-envvar-form'
  | 'adv-envvar-value'
  | 'adv-envvar-secret'
  | 'adv-envvar-create'
  | 'adv-envvar-open-card'
  | 'adv-envvar-mount-tab'
  | 'adv-envvar-mount-select'
  | 'adv-envvar-mount-add'
  | 'adv-envvar-apply'
  | 'adv-envvar-complete'
  // ── アドバンス：リソースQuota ──
  | 'adv-quota-intro'
  | 'adv-quota-footer'
  | 'adv-quota-complete'

export type PopupPlacement = 'top' | 'bottom' | 'left' | 'right' | 'center'

// チュートリアルの種別
export type TutorialTrack = 'basic' | 'adv-storage' | 'adv-envvar' | 'adv-quota'

// 各ステップの設定オブジェクト
export type TutorialStep = {
  id: TutorialStepId
  targetId: string | null // data-tutorial属性の値。nullはオーバーレイなし（フルスクリーン）
  placement: PopupPlacement // ポップアップの表示位置
  title: string // ポップアップのタイトル
  body: string // ポップアップの本文
  page: string // 対象ページのパス（:id はワイルドカード）
  track: TutorialTrack // どのトラックに属するか
  showGlossary?: boolean // trueのとき「用語解説を見る」ボタンを表示する
  autoAdvanceOnAppear?: boolean // trueのとき、このステップのターゲット要素が DOM に出現したら自動でこのステップに進む
}

// 用語解説エントリ
export type GlossaryTerm = {
  term: string
  description: string
}

// Context に公開する値
export type TutorialContextValue = {
  currentStep: TutorialStep | null // 現在のステップ（非表示時は null）
  actualStep: TutorialStep | null // pause中でも実際のステップを返す（ロジック判定用）
  targetRect: DOMRect | null // ハイライト対象要素の DOMRect
  isActive: boolean // チュートリアルが進行中かどうか
  stepIndex: number // 現在のステップインデックス
  totalSteps: number // 全ステップ数
  advance: () => void // 次のステップへ進む
  skip: () => void // チュートリアルをスキップ（完了扱い）
  reset: () => void // チュートリアルをリセット（ステップ0に戻す）
  startTrack: (track: TutorialTrack) => void // 指定トラックをリセットして開始する
  pause: () => void // チュートリアル表示を一時停止する（サイドバー開中など）
  resume: () => void // 一時停止を解除する
}
