import { useState, useEffect, useRef, useCallback } from 'react'
import { Search, X, Copy, Check, ChevronUp, ChevronDown, RefreshCw, Pause } from 'lucide-react'
import AnsiToHtml from 'ansi-to-html'

const ansiConverter = new AnsiToHtml({
  fg: '#E6EDF3',
  bg: '#0D1117',
  newline: true,
  escapeXML: true,
}) // ANSIエスケープコードをHTMLに変換するコンバーターを初期化する

type LogViewerProps = {
  fetchLogs: (since?: string) => Promise<{ logs: string; lastTimestamp?: string | null }>
  pollInterval?: number // ポーリング間隔（ms）
  title?: string
  initialLive?: boolean
  autoStopLive?: boolean // trueの場合、ログ取得が空になったら自動でLive OFFにする
}

type LogLine = {
  id: number
  raw: string
  html: string
}

let lineIdCounter = 0 // ログ行のIDカウンター

function parseLines(text: string): LogLine[] {
  return text
    .split('\n')
    .filter((rawLine) => rawLine.trim() !== '')
    .map((rawLine) => ({
      id: lineIdCounter++,
      raw: rawLine,
      html: ansiConverter.toHtml(rawLine), // ANSIエスケープコードをHTMLに変換する
    }))
}

export function LogViewer({
  fetchLogs,
  pollInterval = 5_000,
  title,
  initialLive = true,
  autoStopLive = false,
}: LogViewerProps) {
  const [logLines, setLogLines] = useState<LogLine[]>([]) // ログ行を管理する
  const [isLive, setIsLive] = useState(initialLive) // リアルタイム更新状態を管理する
  const [searchQuery, setSearchQuery] = useState('') // 検索クエリを管理する
  const [showSearch, setShowSearch] = useState(false) // 検索バー表示状態を管理する
  const [matchIndices, setMatchIndices] = useState<number[]>([]) // 検索マッチインデックスを管理する
  const [currentMatchIndex, setCurrentMatchIndex] = useState(0) // 現在のマッチインデックスを管理する
  const [copied, setCopied] = useState(false) // コピー状態を管理する
  const [autoScroll, setAutoScroll] = useState(true) // 自動スクロール状態を管理する

  const containerRef = useRef<HTMLDivElement>(null) // コンテナのrefを定義する
  const lastTimestampRef = useRef<string | undefined>(undefined) // 最後のタイムスタンプのrefを定義する
  const isLiveRef = useRef(isLive) // Live状態のrefを定義する（クロージャ問題を回避する）

  useEffect(() => {
    isLiveRef.current = isLive // Live状態のrefを更新する
  }, [isLive])

  // 初回ログ取得
  useEffect(() => {
    const loadInitial = async () => {
      try {
        const result = await fetchLogs() // 初回ログを取得する
        setLogLines(parseLines(result.logs))
        if (result.lastTimestamp) {
          lastTimestampRef.current = result.lastTimestamp // 最後のタイムスタンプを保存する
        }
      } catch (loadError) {
        console.error('ログ取得エラー:', loadError)
      }
    }
    void loadInitial()
  }, [fetchLogs])

  // ポーリング
  useEffect(() => {
    if (!isLive) return // Live OFFの場合はポーリングしない

    const intervalId = setInterval(async () => {
      if (!isLiveRef.current) return // Live OFFになった場合はスキップする

      try {
        const result = await fetchLogs(lastTimestampRef.current) // 差分ログを取得する
        if (result.logs) {
          const newLines = parseLines(result.logs)
          setLogLines((prev) => [...prev, ...newLines]) // ログを追加する
        }
        if (result.lastTimestamp) {
          lastTimestampRef.current = result.lastTimestamp // タイムスタンプを更新する
        } else if (autoStopLive && !result.logs) {
          setIsLive(false) // ログが空になったらLive OFFにする
        }
      } catch (pollError) {
        console.error('ポーリングエラー:', pollError)
      }
    }, pollInterval)

    return () => clearInterval(intervalId) // クリーンアップ
  }, [isLive, fetchLogs, pollInterval, autoStopLive])

  // 自動スクロール（Live 中は常に最下部へ、Paused 中は最下部付近のときのみ）
  useEffect(() => {
    if ((isLive || autoScroll) && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight // 最下部へスクロールする
    }
  }, [logLines, isLive, autoScroll])

  // スクロールイベントで自動スクロール制御（Paused 中のみ有効）
  const handleScroll = useCallback(() => {
    if (isLiveRef.current) return // Live 中は手動スクロールで autoScroll を変えない
    const container = containerRef.current
    if (!container) return
    const isAtBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 50
    setAutoScroll(isAtBottom) // 最下部付近の場合のみ自動スクロールを有効にする
  }, [])

  // 検索処理
  useEffect(() => {
    if (!searchQuery.trim()) {
      setMatchIndices([]) // 検索クエリが空の場合はマッチをクリアする
      return
    }
    const query = searchQuery.toLowerCase()
    const matches = logLines
      .map((line, lineIndex) => (line.raw.toLowerCase().includes(query) ? lineIndex : -1))
      .filter((lineIndex) => lineIndex !== -1) // マッチしたインデックスを取得する
    setMatchIndices(matches)
    setCurrentMatchIndex(0) // 最初のマッチへ移動する
  }, [searchQuery, logLines])

  // マッチへスクロール
  useEffect(() => {
    if (matchIndices.length === 0 || !containerRef.current) return
    const matchedLineId = logLines[matchIndices[currentMatchIndex]]?.id
    if (matchedLineId === undefined) return
    const el = containerRef.current.querySelector(`[data-line-id="${matchedLineId}"]`)
    el?.scrollIntoView({ block: 'center', behavior: 'smooth' }) // マッチした行へスクロールする
  }, [currentMatchIndex, matchIndices, logLines])

  const navigateMatch = useCallback((direction: 'next' | 'prev') => {
    if (matchIndices.length === 0) return
    setCurrentMatchIndex((prev) => {
      if (direction === 'next') return (prev + 1) % matchIndices.length
      return (prev - 1 + matchIndices.length) % matchIndices.length
    })
  }, [matchIndices])

  const handleCopy = async () => {
    const text = logLines.map((line) => line.raw).join('\n') // 全ログをコピーする
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000) // 2秒後にコピー状態をリセットする
  }

  const highlightLine = (rawText: string): string => {
    if (!searchQuery.trim()) return ansiConverter.toHtml(rawText)
    const query = searchQuery.toLowerCase()
    const idx = rawText.toLowerCase().indexOf(query)
    if (idx === -1) return ansiConverter.toHtml(rawText)
    const before = rawText.slice(0, idx)
    const match = rawText.slice(idx, idx + searchQuery.length)
    const after = rawText.slice(idx + searchQuery.length)
    return (
      ansiConverter.toHtml(before) +
      `<mark style="background:#F59E0B;color:#111827;border-radius:2px">${match}</mark>` +
      ansiConverter.toHtml(after)
    ) // 検索マッチ箇所をハイライトする
  }

  return (
    <div className="flex flex-col overflow-hidden border border-[#30363D]" style={{ background: '#0D1117', height: '100%' }}>
      {/* ツールバー */}
      <div className="flex items-center gap-2 px-3 py-2 border-b border-[#30363D] shrink-0" style={{ background: '#161B22' }}>
        {title && (
          <span className="text-xs text-[#8B949E] font-mono flex-1">{title}</span>
        )}

        <div className="flex items-center gap-1 ml-auto">
          {/* Live トグル */}
          <button
            onClick={() => setIsLive((prev) => !prev)}
            className={`flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-md transition-colors font-medium ${
              isLive
                ? 'bg-[#00C2D1]/20 text-[#00C2D1] border border-[#00C2D1]/40'
                : 'bg-[#21262D] text-[#8B949E] border border-[#30363D] hover:text-[#E6EDF3]'
            }`}
            title={isLive ? 'リアルタイム更新中（クリックで停止）' : 'リアルタイム更新停止中（クリックで開始）'}
          >
            {isLive ? (
              <>
                <RefreshCw className="w-3 h-3 animate-spin" />
                Live
              </>
            ) : (
              <>
                <Pause className="w-3 h-3" />
                Paused
              </>
            )}
          </button>

          {/* 検索トグル */}
          <button
            onClick={() => setShowSearch((prev) => !prev)}
            className={`p-1.5 rounded-md transition-colors ${
              showSearch
                ? 'bg-[#21262D] text-[#E6EDF3]'
                : 'text-[#8B949E] hover:text-[#E6EDF3] hover:bg-[#21262D]'
            }`}
            title="検索（Ctrl+F）"
          >
            <Search className="w-3.5 h-3.5" />
          </button>

          {/* コピー */}
          <button
            onClick={() => void handleCopy()}
            className="p-1.5 rounded-md text-[#8B949E] hover:text-[#E6EDF3] hover:bg-[#21262D] transition-colors"
            title="全ログをコピー"
          >
            {copied ? <Check className="w-3.5 h-3.5 text-green-400" /> : <Copy className="w-3.5 h-3.5" />}
          </button>
        </div>
      </div>

      {/* 検索バー */}
      {showSearch && (
        <div className="flex items-center gap-2 px-3 py-1.5 border-b border-[#30363D]" style={{ background: '#161B22' }}>
          <Search className="w-3.5 h-3.5 text-[#8B949E] shrink-0" />
          <input
            type="text"
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder="検索..."
            className="flex-1 bg-transparent text-xs text-[#E6EDF3] placeholder-[#8B949E] outline-none font-mono"
            autoFocus
            onKeyDown={(event) => {
              if (event.key === 'Enter') navigateMatch(event.shiftKey ? 'prev' : 'next') // Enterキーで次のマッチへ移動する
              if (event.key === 'Escape') setShowSearch(false) // Escキーで検索を閉じる
            }}
          />
          {matchIndices.length > 0 && (
            <span className="text-xs text-[#8B949E] shrink-0">
              {currentMatchIndex + 1}/{matchIndices.length}
            </span>
          )}
          {searchQuery && matchIndices.length === 0 && (
            <span className="text-xs text-red-400 shrink-0">見つかりません</span>
          )}
          <button
            onClick={() => navigateMatch('prev')}
            disabled={matchIndices.length === 0}
            className="p-0.5 text-[#8B949E] hover:text-[#E6EDF3] disabled:opacity-30"
          >
            <ChevronUp className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={() => navigateMatch('next')}
            disabled={matchIndices.length === 0}
            className="p-0.5 text-[#8B949E] hover:text-[#E6EDF3] disabled:opacity-30"
          >
            <ChevronDown className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={() => { setShowSearch(false); setSearchQuery('') }}
            className="p-0.5 text-[#8B949E] hover:text-[#E6EDF3]"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* ログ本体 */}
      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="overflow-auto font-mono text-xs leading-5 select-text flex-1"
        style={{
          background: '#0D1117',
          color: '#E6EDF3',
        }}
      >
        {logLines.length === 0 ? (
          <div className="flex items-center justify-center h-32 text-[#8B949E] text-xs">
            {isLive ? 'ログ待機中...' : 'ログがありません'}
          </div>
        ) : (
          <table className="w-full border-collapse">
            <tbody>
              {logLines.map((line, lineIndex) => {
                const isMatch = matchIndices.includes(lineIndex) // 検索マッチ行かどうかを確認する
                const isCurrentMatch = isMatch && matchIndices[currentMatchIndex] === lineIndex // 現在のマッチ行かどうかを確認する

                return (
                  <tr
                    key={line.id}
                    data-line-id={line.id}
                    className={isCurrentMatch ? 'bg-[#F59E0B]/10' : isMatch ? 'bg-[#F59E0B]/5' : ''}
                  >
                    {/* 行番号 */}
                    <td
                      className="select-none text-right pr-4 pl-3 text-[#8B949E] w-12 align-top"
                      style={{ userSelect: 'none' }}
                    >
                      {lineIndex + 1}
                    </td>
                    {/* ログ内容 */}
                    <td
                      className="pr-4 py-0.5 whitespace-pre-wrap break-all align-top"
                      dangerouslySetInnerHTML={{ __html: highlightLine(line.raw) }} // HTMLとして安全にレンダリングする
                    />
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
