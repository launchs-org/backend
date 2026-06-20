import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from '@/hooks/useAuth'
import { Loader2 } from 'lucide-react'
import { DashboardPage } from '@/pages/DashboardPage'
import { ProjectDetailPage } from '@/pages/ProjectDetailPage'
import { ProjectNewPage } from '@/pages/ProjectNewPage'
import { DeploymentDetailPage } from '@/pages/DeploymentDetailPage'
import { DeploymentNewPage } from '@/pages/DeploymentNewPage'
import { BuildLogPage } from '@/pages/BuildLogPage'
import { SettingsPage } from '@/pages/SettingsPage'

function AuthGate({ children }: { children: React.ReactNode }) {
  const { authState } = useAuth() // 認証状態を取得する

  if (authState === 'loading') {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#F0F2F5]">
        <Loader2 className="w-8 h-8 animate-spin text-[#00C2D1]" />
      </div>
    )
  }

  if (authState === 'unauthenticated') {
    window.location.href = '/auth/login' // 未認証の場合はログインページへリダイレクトする
    return null
  }

  return <>{children}</> // 認証済みの場合はコンテンツを表示する
}

export default function App() {
  return (
    <BrowserRouter basename="/ui">
      <Routes>
        {/* ログインリダイレクト */}
        <Route
          path="/login"
          element={<LoginRedirect />}
        />

        {/* 認証が必要なページ */}
        <Route
          path="/"
          element={
            <AuthGate>
              <DashboardPage />
            </AuthGate>
          }
        />
        <Route
          path="/projects/new"
          element={
            <AuthGate>
              <ProjectNewPage />
            </AuthGate>
          }
        />
        <Route
          path="/projects/:projectId"
          element={
            <AuthGate>
              <ProjectDetailPage />
            </AuthGate>
          }
        />
        <Route
          path="/projects/:projectId/deployments/new"
          element={
            <AuthGate>
              <DeploymentNewPage />
            </AuthGate>
          }
        />
        <Route
          path="/projects/:projectId/deployments/:deploymentId"
          element={
            <AuthGate>
              <DeploymentDetailPage />
            </AuthGate>
          }
        />
        <Route
          path="/builds/:buildId/logs"
          element={
            <AuthGate>
              <BuildLogPage />
            </AuthGate>
          }
        />
        <Route
          path="/settings"
          element={
            <AuthGate>
              <SettingsPage />
            </AuthGate>
          }
        />

        {/* 存在しないパスはダッシュボードへ */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}

function LoginRedirect() {
  // /auth/login へリダイレクトする
  window.location.href = '/auth/login'
  return (
    <div className="min-h-screen flex flex-col items-center justify-center gap-3 bg-[#F0F2F5]">
      <Loader2 className="w-8 h-8 animate-spin text-[#00C2D1]" />
      <p className="text-sm text-gray-500">認証画面へ移動中...</p>
    </div>
  )
}
