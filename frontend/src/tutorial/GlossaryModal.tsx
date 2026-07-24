import * as Dialog from '@radix-ui/react-dialog' // Radix UI Dialog をインポートする
import { X, BookOpen } from 'lucide-react' // アイコンをインポートする
import { GLOSSARY_TERMS } from './glossary' // 用語解説データをインポートする

type GlossaryModalProps = {
  open: boolean // モーダルの開閉状態
  onOpenChange: (open: boolean) => void // 開閉状態変更コールバック
}

// 用語解説モーダルコンポーネント
// TutorialOverlay から showGlossary ステップのときに表示される
export function GlossaryModal({ open, onOpenChange }: GlossaryModalProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        {/* オーバーレイ（チュートリアルのz-indexより上に表示） */}
        <Dialog.Overlay
          style={{ zIndex: 10000 }}
          className="fixed inset-0 bg-black/60 backdrop-blur-sm"
        />
        {/* モーダルコンテンツ */}
        <Dialog.Content
          style={{ zIndex: 10001 }}
          className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full max-w-lg max-h-[80vh] bg-white rounded-xl shadow-2xl flex flex-col overflow-hidden"
        >
          {/* ヘッダー */}
          <div className="flex items-center justify-between px-5 py-4 border-b border-gray-100">
            <div className="flex items-center gap-2">
              <BookOpen className="w-4 h-4 text-[#00C2D1]" /> {/* アイコン */}
              <Dialog.Title className="text-sm font-semibold text-gray-800">
                用語解説
              </Dialog.Title>
            </div>
            <Dialog.Close className="p-1 rounded hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors">
              <X className="w-4 h-4" /> {/* 閉じるボタン */}
            </Dialog.Close>
          </div>

          {/* 用語一覧（スクロール可能） */}
          <div className="overflow-y-auto flex-1 px-5 py-4 space-y-4">
            {GLOSSARY_TERMS.map(glossaryTerm => (
              <div key={glossaryTerm.term} className="border border-gray-100 rounded-lg p-4">
                <h3 className="text-sm font-semibold text-gray-800 mb-1.5">
                  {glossaryTerm.term} {/* 用語名 */}
                </h3>
                <p className="text-sm text-gray-600 leading-relaxed">
                  {glossaryTerm.description} {/* 用語の説明 */}
                </p>
              </div>
            ))}
          </div>

          {/* フッター */}
          <div className="px-5 py-3 border-t border-gray-100 bg-gray-50">
            <Dialog.Close className="w-full text-center text-sm text-[#00C2D1] hover:underline font-medium py-1">
              閉じる
            </Dialog.Close>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
