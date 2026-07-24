import * as DialogPrimitive from '@radix-ui/react-dialog'
import { AlertTriangle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

interface ConfirmDialogProps {
  open: boolean // ダイアログの表示状態
  onOpenChange: (open: boolean) => void // 表示状態の変更ハンドラー
  title: string // ダイアログのタイトル
  description: string // 確認メッセージ
  confirmLabel?: string // 確認ボタンのラベル（デフォルト: 削除）
  cancelLabel?: string // キャンセルボタンのラベル（デフォルト: キャンセル）
  variant?: 'destructive' | 'default' // 確認ボタンのスタイル
  onConfirm: () => void | Promise<void> // 確認時のコールバック
  loading?: boolean // ローディング状態
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = '削除',
  cancelLabel = 'キャンセル',
  variant = 'destructive',
  onConfirm,
  loading = false,
}: ConfirmDialogProps) {
  const handleConfirm = async () => { // 確認ボタンが押された時の処理
    await onConfirm()
  }

  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay
          className={cn(
            'fixed inset-0 z-50 bg-black/40 backdrop-blur-sm',
            'data-[state=open]:animate-in data-[state=closed]:animate-out',
            'data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
          )}
        />
        <DialogPrimitive.Content
          className={cn(
            'fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2',
            'w-full max-w-md rounded-xl bg-white p-6 shadow-xl',
            'data-[state=open]:animate-in data-[state=closed]:animate-out',
            'data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
            'data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
            'data-[state=closed]:slide-out-to-left-1/2 data-[state=open]:slide-in-from-left-1/2',
            'data-[state=closed]:slide-out-to-top-[48%] data-[state=open]:slide-in-from-top-[48%]',
          )}
        >
          <div className="flex items-start gap-4">
            {variant === 'destructive' && ( // 削除系は警告アイコンを表示する
              <div className="flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-full bg-red-100">
                <AlertTriangle className="w-5 h-5 text-red-600" />
              </div>
            )}
            <div className="flex-1 min-w-0">
              <DialogPrimitive.Title className="text-base font-semibold text-zinc-900">
                {title}
              </DialogPrimitive.Title>
              <DialogPrimitive.Description className="mt-1 text-sm text-zinc-500 whitespace-pre-wrap">
                {description}
              </DialogPrimitive.Description>
            </div>
          </div>

          <div className="flex justify-end gap-2 mt-6">
            <DialogPrimitive.Close asChild>
              <Button variant="outline" size="sm" disabled={loading}>
                {cancelLabel}
              </Button>
            </DialogPrimitive.Close>
            <Button
              variant={variant}
              size="sm"
              onClick={handleConfirm}
              disabled={loading}
            >
              {loading ? '処理中...' : confirmLabel}
            </Button>
          </div>
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  )
}
