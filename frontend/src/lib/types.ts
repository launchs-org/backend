// ── Project ──────────────────────────────────────────────────

export type Project = {
  id: string
  user_id: string
  name: string
  namespace: string
  status: 'active' | 'deleting'
  created_at: string
  updated_at: string
}

// ── Deployment ────────────────────────────────────────────────

export type DeploymentType = 'image_url' | 'dockerfile' | 'railpack'

export type DeploymentStatus = 'pending' | 'running' | 'failed' | 'deleting'

export type AppStatus = 'pending' | 'building' | 'deploying' | 'running' | 'error'

export type Deployment = {
  id: string
  project_id: string
  name: string
  type: DeploymentType
  image_url: string
  pending_image_url: string
  github_repo_url: string
  pending_github_repo_url: string
  github_branch: string
  pending_github_branch: string
  github_commit_sha: string
  pending_github_commit_sha: string
  github_repo_directory: string
  pending_github_repo_directory: string
  dockerfile_path: string
  pending_dockerfile_path: string
  current_build_id: string | null
  instance_size: string
  pending_instance_size: string
  replicas: number
  pending_replicas: number
  command: string[]
  pending_command: string[]
  args: string[]
  pending_args: string[]
  status: DeploymentStatus
  app_status: AppStatus
  k8s_status: Record<string, unknown> | null
  applied_at: string | null
  created_at: string
  updated_at: string
}

// ── Build ─────────────────────────────────────────────────────

export type BuildStatus = 'pending' | 'building' | 'succeeded' | 'failed' | 'cancelled'

export type Build = {
  id: string
  deployment_id: string
  build_type: 'dockerfile' | 'railpack'
  status: BuildStatus
  k8s_job_name: string
  built_image_url: string
  commit_sha: string
  commit_message: string
  branch: string
  author: string
  directory: string
  dockerfile_path: string
  build_log: string
  started_at: string | null
  finished_at: string | null
  created_at: string
}

// ── Service ───────────────────────────────────────────────────

export type ServiceStatus = 'pending' | 'active' | 'deleting'

export type K8sService = {
  id: string
  deployment_id: string
  port: number
  target_port: number
  type: 'ClusterIP' | 'NodePort' | 'LoadBalancer'
  pending_port: number
  pending_target_port: number
  ports: Record<string, unknown> | null
  pending_ports: Record<string, unknown> | null
  status: ServiceStatus
  k8s_status: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

// ── IngressRoute ──────────────────────────────────────────────

export type IngressStatus = 'pending' | 'active' | 'deleting'

export type IngressRoute = {
  id: string
  project_id: string
  host: string
  status: IngressStatus
  k8s_status: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

export type PathRuleStatus = 'pending' | 'active' | 'deleting'

export type PathRule = {
  id: string
  ingress_route_id: string
  path_prefix: string
  service_id: string
  status: PathRuleStatus
  created_at: string
  updated_at: string
}

// ── ApplyHistory ──────────────────────────────────────────────

export type ApplyHistory = {
  id: string
  deployment_id: string
  manifests: unknown
  status: 'applied' | 'failed'
  error_message: string
  applied_at: string
}

// ── Logs ──────────────────────────────────────────────────────

export type PodLogsResponse = {
  logs: string
  last_timestamp: string | null
}

export type BuildLogsResponse = {
  logs: string
}

// ── Quota ─────────────────────────────────────────────────────

export type Quota = {
  user_id: string
  max_projects: number
  max_deployments: number
  max_replicas_per_deployment: number
  max_volume_mb: number
  current_projects: number
  current_deployments: number
  current_volume_mb: number
}

// ── EnvVar ────────────────────────────────────────────────────

export type EnvVar = {
  id: string
  project_id: string
  key: string
  value: string
  created_at: string
  updated_at: string
}

// ── Volume ────────────────────────────────────────────────────

export type Volume = {
  id: string
  project_id: string
  name: string
  size_gb: number
  status: 'pending' | 'bound' | 'deleting'
  created_at: string
  updated_at: string
}

// ── Webhook ───────────────────────────────────────────────────

export type Webhook = {
  id: string
  deployment_id: string
  secret: string
  created_at: string
  updated_at: string
}
