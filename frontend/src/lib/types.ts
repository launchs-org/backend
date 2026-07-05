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

export type DeploymentStatus = 'not_init' | 'pending' | 'running' | 'failed' | 'deleting'

export type AppStatus = 'pending' | 'building' | 'deploying' | 'running' | 'error'

export type Deployment = {
  id: string
  project_id: string
  name: string
  type: DeploymentType
  image_id: string | null
  image: Image | null
  pending_image_id: string | null
  pending_image: Image | null
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
  delete_progress: string
  applied_at: string | null
  created_at: string
  updated_at: string
}

// ── Build ─────────────────────────────────────────────────────

export type BuildStatus = 'pending' | 'building' | 'succeeded' | 'failed' | 'cancelled'

export type Build = {
  id: string
  project_id: string
  deployment_id: string | null
  build_type: 'dockerfile' | 'railpack'
  status: BuildStatus
  k8s_job_name: string
  github_repo_url: string
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

// ── Image ─────────────────────────────────────────────────────

export type Image = {
  id: string
  project_id: string
  build_id: string | null
  build?: Build
  image_url: string
  size_bytes: number
  created_at: string
}

// ── ProjectQuota ──────────────────────────────────────────────

export type ProjectQuota = {
  used_bytes: number
  limit_bytes: number
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
  cluster_ip: string
  k8s_status: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

// ── IngressRoute ──────────────────────────────────────────────

export type IngressStatus = 'pending' | 'active' | 'deleting'

export type IngressRoute = {
  id: string
  project_id: string
  name: string
  pending_name: string
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
  strip_prefix: boolean
  status: PathRuleStatus
  created_at: string
  updated_at: string
}

// ── ApplyHistory ──────────────────────────────────────────────

export type ApplyHistory = {
  id: string
  deployment_id: string
  manifests: Record<string, unknown> | null
  status: 'applied' | 'failed'
  error_message: string
  applied_at: string
}

export type ProjectPendingSummary = {
  has_pending: boolean
  pending_deployment_count: number
  pending_ingress_route_count: number
}

export type ApplyProjectFailure = {
  deployment_id: string
  error: string
}

export type ApplyProjectResult = {
  applied_deployment_count: number
  applied_deployment_id_list: string[]
  failed_deployment_list: ApplyProjectFailure[]
  ingress_route_applied: boolean
}

// ── Logs ──────────────────────────────────────────────────────

export type PodLogEntry = {
  pod_name: string
  logs: string
  last_timestamp: string | null
}

export type PodLogsResponse = {
  active_pod_names: string[]
  pods: PodLogEntry[]
}

export type BuildLogsResponse = {
  logs: string
  last_timestamp: string | null
}

// ── Quota ─────────────────────────────────────────────────────

export type Quota = {
  user_id: string
  plan_id: string
  max_projects: number
  max_deployments: number
  max_replicas_per_deployment: number
  max_volumes: number
  max_volume_size_mb: number
  max_total_volume_mb: number
  instance_limits: Record<string, number>
  current_instances: Record<string, number>
  current_projects: number
  current_deployments: number
  current_volumes: number
  current_total_volume_mb: number
}

export type QuotaExceededError = {
  error: 'quota_exceeded'
  resource: string
  current: number
  limit: number
}

// ── EnvVar ────────────────────────────────────────────────────

export type EnvVar = {
  id: string
  project_id: string
  key: string
  value: string
  is_secret: boolean
  status: 'active' | 'deleting'
  created_at: string
  updated_at: string
}

// ── EnvVarMount ───────────────────────────────────────────────

export type EnvVarMountStatus = 'pending' | 'applied' | 'deleting'

export type EnvVarMount = {
  id: string
  env_var_id: string
  deployment_id: string
  override_key: string
  pending_override_key: string
  status: EnvVarMountStatus
  created_at: string
  updated_at: string
}

// ── Volume ────────────────────────────────────────────────────

export type Volume = {
  id: string
  project_id: string
  name: string
  size_mb: number
  status: 'pending' | 'bound' | 'deleting'
  created_at: string
  updated_at: string
}

// ── VolumeMount ───────────────────────────────────────────────

export type VolumeMountStatus = 'pending' | 'mounted' | 'deleting'

export type VolumeMount = {
  id: string
  volume_id: string
  deployment_id: string
  mount_path: string
  pending_mount_path: string
  status: VolumeMountStatus
  created_at: string
  updated_at: string
}

// ── DeploymentTemplate ────────────────────────────────────────

export type TemplateEnvVar = {
  key: string
  value: string
  is_secret: boolean
  auto_generate: boolean
  length: number
}

export type TemplateVolume = {
  name: string
  size_mb: number
  mount_path: string
}

export type DeploymentTemplate = {
  id: string
  name: string
  description: string
  type: DeploymentType
  image_url: string
  instance_size: string
  replicas: number
  command: string[]
  args: string[]
  service_port: number
  service_target_port: number
  service_type: string
  env_vars: TemplateEnvVar[] | null
  volumes: TemplateVolume[] | null
  created_by: string
  created_at: string
  updated_at: string
}

// ── Webhook ───────────────────────────────────────────────────

export type Webhook = {
  id: string
  deployment_id: string
  secret: string
  github_repo_url: string
  is_active: boolean
  created_at: string
  updated_at: string
}

// ── DeploymentMetrics ─────────────────────────────────────────

export type DeploymentMetrics = {
  id: string
  deployment_id: string
  pod_name: string
  cpu_millicores: number
  memory_bytes: number
  ready_replicas: number
  total_replicas: number
  recorded_at: string
  created_at: string
}

export type DeploymentMetricsResponse = {
  metrics: DeploymentMetrics[]
}

// ── CliToken ──────────────────────────────────────────────────

export type CliToken = {
  id: string
  user_id: string
  name: string
  expires_at: string | null
  created_at: string
}

export type CreateCliTokenRequest = {
  name: string
  expires_in_days: number // 0または未指定の場合は無期限
}

export type CreateCliTokenResponse = {
  id: string
  name: string
  token: string // 発行時のみ返却される平文トークン
  expires_at: string | null
  created_at: string
}
