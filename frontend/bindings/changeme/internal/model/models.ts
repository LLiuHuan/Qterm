export interface AcceleratorInfo {
  index: number
  name: string
  kind: AcceleratorKind

  utilization: number

  memoryUsedMb: number
  memoryTotalMb: number

  temperatureC: number
}

export enum AcceleratorKind {
  $zero = '',

  AcceleratorGPU = 'gpu',
  AcceleratorNPU = 'npu',
}

export interface AcceleratorSnapshot {
  devices: AcceleratorInfo[] | null
  updatedAt: number
}

export interface CommandHistory {
  id: string
  hostId: string
  hostName: string
  sessionId: string
  command: string
  executedAt: string
  suppressed: boolean
}

export interface Credential {
  id: string
  name: string
  kind: CredentialKind
  username?: string
  publicKey?: string
  configured: boolean
  createdAt: string
  updatedAt: string
}

export enum CredentialKind {
  $zero = '',

  CredentialKindPassword = 'password',
  CredentialKindPrivateKey = 'privateKey',
  CredentialKindPrivateKeyPassphrase = 'privateKeyPassphrase',
  CredentialKindSSHAgent = 'sshAgent',
}

export interface CredentialPromptRule {
  id: string
  kind: string
  pattern: string
  enabled: boolean
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export interface CredentialPromptRuleInput {
  kind: string
  pattern: string
  enabled?: boolean | null
}

export interface DiskResource {
  mount: string
  fileSystemType: string
  usedPercent: number
  usedBytes: number
  totalBytes: number
  usedText: string
  totalText: string
}

export interface DockerContainer {
  id: string
  name: string
  image: string

  state: string

  status: string

  ports: string

  project: string

  projectDir: string

  service: string

  createdAt: string
}

export interface DockerContainerDetail {
  id: string

  name: string

  image: string

  entrypoint: string

  command: string

  created: string

  state: string

  status: string

  ports: DockerPortMapping[] | null
  mounts: DockerMount[] | null
  environment: string[] | null
  labels: DockerLabel[] | null
}

export interface DockerEnvironment {
  available: boolean

  dockerVersion: string

  apiVersion: string

  composeVersion: string

  message: string
}

export interface DockerImage {
  id: string

  repository: string

  tag: string

  digest: string

  size: string

  createdAt: string

  inUse: boolean
}

export interface DockerImageDetail {
  id: string

  reference: string

  repoTags: string[] | null
  repoDigests: string[] | null

  digest: string

  created: string

  sizeBytes: number
  size: string

  architecture: string
  os: string
  author: string

  entrypoint: string
  command: string

  exposedPorts: string[] | null

  environment: string[] | null

  labels: DockerLabel[] | null

  layers: number
}

export interface DockerLabel {
  key: string
  value: string
}

export interface DockerMount {
  type: string
  name: string
  source: string
  destination: string
  readOnly: boolean
}

export interface DockerNetwork {
  id: string
  name: string
  driver: string
  scope: string

  inUse: boolean

  builtIn: boolean
}

export interface DockerNetworkDetail {
  id: string
  name: string
  driver: string
  scope: string
  created: string

  internal: boolean
  attachable: boolean
  ingress: boolean
  enableIpv6: boolean

  subnets: DockerSubnet[] | null

  containers: DockerNetworkMember[] | null

  labels: DockerLabel[] | null
  options: DockerLabel[] | null
}

export interface DockerNetworkMember {
  name: string
  ipv4: string
  ipv6: string
  mac: string
}

export interface DockerPortMapping {
  privatePort: number

  publicPort: number

  type: string
}

export interface DockerSubnet {
  subnet: string
  gateway: string
}

export interface DockerVolume {
  name: string

  driver: string

  scope: string

  mountpoint: string

  inUse: boolean
}

export interface DockerVolumeDetail {
  name: string

  driver: string
  scope: string

  mountpoint: string

  createdAt: string

  labels: DockerLabel[] | null
  options: DockerLabel[] | null
}

export interface ForwardConfig {
  id: string
  name: string
  kind: ForwardKind

  hostId: string
  listenHost: string
  listenPort: number
  targetHost: string
  targetPort: number

  proxy?: string

  autoOpen: boolean
}

export enum ForwardKind {
  $zero = '',

  ForwardLocal = 'local',
  ForwardRemote = 'remote',
  ForwardDynamic = 'dynamic',
}

export enum ForwardState {
  $zero = '',

  ForwardStopped = 'stopped',
  ForwardStarting = 'starting',
  ForwardRunning = 'running',
  ForwardReconnecting = 'reconnecting',
  ForwardDisconnected = 'disconnected',
  ForwardError = 'error',
}

export interface ForwardStatus {
  id: string

  state: ForwardState

  error?: string

  listenPort: number

  retryAttempt: number

  updatedAt: number
}

export interface Group {
  id: string
  name: string
  parentId?: string
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export interface GroupInput {
  name: string
  parentId?: string
}

export interface Host {
  id: string
  icon: string

  color?: string
  name: string

  alias?: string

  customId?: string
  protocol: string
  address: string
  port: number
  username: string
  groupId: string
  sortOrder: number
  group: string
  credentialId: string
  passwordSource: string
  passwordConfigured: boolean

  sshTwoFactorEnabled: boolean

  sshOtpEntryId?: string

  sshOtpAutoFill: boolean

  proxyPasswordConfigured: boolean
  notes: string
  createdAt: string
  updatedAt: string
  sshKeepAlive: boolean
  latencyDetection: boolean
  autoDetectOSIcon: boolean
  directoryFollow: string

  sftpShowHiddenFiles: string

  sftpDefaultDirectory: string

  serialBaud?: number

  serialDataBits?: number

  serialParity?: string

  serialStopBits?: string

  backspaceMode: string
  encoding: string
  connectTimeoutSeconds: number
  postLoginCommand: string
  postLoginCommandDelaySeconds: number

  environmentVariables: HostEnvironmentVariable[] | null

  x11Forwarding: boolean
  x11Trusted: boolean

  rdpDisplayMode: string
  rdpWidth: number
  rdpHeight: number

  rdpClipboardMode: string
  rdpClipboardContent: string

  vncViewOnly: boolean
  vncShared: boolean
  vncPixelDepth: number
  vncScale: string
  vncAutoReconnect: boolean
  vncAutoReconnectAttempts: number
  vncTransport: string
  vncWsPath: string
  vncAllowInsecureTls: boolean

  vncRfbPasswordMode: string
  tags: string[] | null

  connectionType: string
  terminalType: string

  jumpHostId?: string | null

  proxyId?: string
  proxyMode?: string | null
  proxyHost?: string | null
  proxyPort?: number | null
  proxyUsername?: string | null
}

export interface HostEnvironmentVariable {
  name: string
  value: string
}

export interface HostInput {
  color?: string
  icon: string
  name: string

  alias?: string

  customId?: string
  protocol: string
  address: string
  port: number
  username: string
  group: string
  credentialId: string
  passwordSource?: string
  notes: string

  sshTwoFactorEnabled?: boolean | null

  sshOtpEntryId?: string | null

  sshOtpAutoFill?: boolean | null
  password?: string
  clearPassword?: boolean
  sshKeepAlive?: boolean | null
  latencyDetection?: boolean | null
  autoDetectOSIcon?: boolean | null
  directoryFollow?: string

  sftpShowHiddenFiles?: string | null

  sftpDefaultDirectory?: string | null

  backspaceMode?: string | null

  encoding?: string | null

  connectTimeoutSeconds?: number | null

  postLoginCommand?: string | null

  postLoginCommandDelaySeconds?: number | null

  environmentVariables?: HostEnvironmentVariable[] | null

  x11Forwarding?: boolean | null

  x11Trusted?: boolean | null

  serialBaud?: number | null

  serialDataBits?: number | null

  serialParity?: string | null

  serialStopBits?: string | null

  rdpDisplayMode?: string | null

  rdpWidth?: number | null
  rdpHeight?: number | null

  rdpClipboardMode?: string | null

  rdpClipboardContent?: string | null

  vncViewOnly?: boolean | null

  vncShared?: boolean | null

  vncPixelDepth?: number | null

  vncScale?: string | null

  vncAutoReconnect?: boolean | null

  vncAutoReconnectAttempts?: number | null

  vncTransport?: string | null

  vncWsPath?: string | null

  vncAllowInsecureTls?: boolean | null

  vncRfbPasswordMode?: string | null

  tags?: string[] | null

  connectionType?: string | null

  terminalType?: string | null

  jumpHostId?: string | null

  proxyId?: string

  proxyMode?: string | null

  proxyHost?: string | null

  proxyPort?: number | null

  proxyUsername?: string | null

  proxyPassword?: string
  clearProxyPassword?: boolean
}

export interface HostKeyChallenge {
  address: string

  hopIndex: number

  hopLabel: string
  algorithm: string
  sha256Fingerprint: string
  issue: HostKeyIssue
  trustedFingerprints: string[] | null
}

export enum HostKeyIssue {
  $zero = '',

  HostKeyIssueUnknown = 'unknown',
  HostKeyIssueChanged = 'changed',
}

export interface HostKeyPending {
  sessionId: string
  challenge: HostKeyChallenge
  challenges: HostKeyChallenge[] | null
}

export interface KnownHostEntry {
  host: string

  keyType: string

  sha256Fingerprint: string

  comment: string

  marker: string

  source: string

  line: number

  editable: boolean
}

export interface LocalEdit {
  id: string
  sessionId: string
  remotePath: string
  localPath: string
  state: LocalEditState

  baselineHash: string

  lastError: string

  hasUnsyncedChanges: boolean
}

export enum LocalEditState {
  $zero = '',

  LocalEditStateWatching = 'watching',

  LocalEditStateSyncing = 'syncing',

  LocalEditStateSynced = 'synced',

  LocalEditStateError = 'error',

  LocalEditStateStopped = 'stopped',
}

export interface NetworkResource {
  name: string
  receiveBytes: number
  transmitBytes: number
  receiveText: string
  transmitText: string
  receiveRateBytes: number
  transmitRateBytes: number
}

export interface OTPCode {
  code: string

  remainingSeconds: number

  counter: number
}

export interface OTPEntry {
  id: string

  name: string

  username?: string

  hostId?: string

  type: string

  algorithm: string
  digits: number

  period: number

  counter: number
}

export interface OTPSaveInput {
  entry: OTPEntry
  secret: string
}

export interface ProcessInfo {
  pid: number
  ppid: number
  user: string
  cpu: number
  memory: number
  rssKb: number

  vszKb: number

  tty: string
  state: string

  priority: number

  nice: number

  elapsed: string

  cpuTime: string
  command: string

  startTime: string

  metricsAvailable: boolean

  cpuAvailable: boolean
}

export interface ProcessSnapshot {
  processes: ProcessInfo[] | null

  updatedAt: number
}

export interface ProxyInput {
  name: string
  mode: string
  host: string
  port: number
  username: string
  password?: string
  clearPassword?: boolean
}

export interface ProxyRecord {
  id: string
  name: string
  mode: string
  host: string
  port: number
  username: string

  passwordConfigured: boolean
  createdAt: string
  updatedAt: string
}

export interface RdpFrame {
  sessionId: string
  sequence: number
  width: number
  height: number
  changed: boolean
  png: string | null
  cursorVisible: boolean
  cursorX: number
  cursorY: number
  cursorWidth: number
  cursorHeight: number
  cursorPng: string | null
}

export interface RdpHelperEngine {
  engine: string
  ready: boolean
  path?: string
  message?: string
}

export interface RdpHostProfile {
  hostId: string
  name: string
  address: string
  port: number
  username: string

  domain?: string

  passwordAvailable: boolean

  passwordSource?: string

  displayMode?: string

  width?: number
  height?: number

  clipboardMode?: string

  clipboardContent?: string
}

export interface RdpOpenInput {
  host: string
  port: number
  username: string
  password: string
  domain: string
  width: number
  height: number

  engine?: string
}

export interface RdpSession {
  id: string
  host: string
  width: number
  height: number
  connected: boolean
  error?: string
  securityLabel: string
  channels?: { [_ in string]?: number } | null

  engine?: string
}

export interface RdpStats {
  sessionId: string
  connected: boolean
  closed: boolean
  error?: string
  width: number
  height: number
  packets: number
  channelPackets: number
  bitmapUpdates: number
  orderUpdates: number
  pointerUpdates: number
  ordersApplied: number
  clipboardText?: string

  clipboardSequence?: number

  engine?: string
  helperStderr?: string

  resizeUnsupported?: boolean

  certificateState: string

  certificateFingerprint?: string

  certificateSubject?: string
  certificateIssuer?: string

  serverFPS?: number
}

export interface RdpStreamTicket {
  streamName: string
  token: string

  protocolVersion: number
}

export interface RdpTrustedCertificate {
  hostId: string
  address: string
  port: number

  fingerprint?: string
  subject?: string
  issuer?: string

  trustedAt?: string
}

export interface RemoteFileContent {
  contentBase64: string
  size: number
  binary: boolean
  truncated: boolean
  hash: string
  modTime: string
}

export interface RemoteFileEntry {
  name: string
  path: string
  directory: string
  type: RemoteFileType

  targetType?: RemoteFileType
  size: number
  sizeText: string
  mode: string
  uid: number
  gid: number
  modTime: string
}

export enum RemoteFileType {
  $zero = '',

  RemoteFileTypeFile = 'file',
  RemoteFileTypeDirectory = 'directory',
  RemoteFileTypeSymlink = 'symlink',
  RemoteFileTypeOther = 'other',
}

export interface RemoteFileWriteResult {
  saved: boolean
  conflict: boolean
  currentHash: string
}

export interface RemoteImageContent {
  contentBase64: string
  size: number
  mimeType: string
  truncated: boolean
  modTime: string
}

export interface RemotePrincipal {
  id: number
  name: string
}

export interface RemotePrincipals {
  users: RemotePrincipal[] | null
  groups: RemotePrincipal[] | null
}

export interface ResourceSnapshot {
  metricsSupported: boolean

  loadAverage1: number | null

  loadAverage5: number | null
  loadAverage15: number | null

  sampleWindowMs: number
  hostname: string
  os: string
  architecture: string
  cpuCores: number
  memoryTotalBytes: number
  memoryAvailableBytes: number

  memoryCachedBytes: number
  uptimeSeconds: number
  cpuUsagePercent: number
  memoryUsagePercent: number
  memoryUsedText: string

  memoryAvailableText: string
  memoryCachedText: string
  memoryTotalText: string
  cpuPerCore: number[] | null
  networks: NetworkResource[] | null
  disks: DiskResource[] | null
}

export interface RouteDraftInput {
  hostId?: string
  address: string
  port: number
  username: string
  jumpHostId?: string

  proxyId?: string
  proxyMode?: string
  proxyHost?: string
  proxyPort?: number
  proxyUsername?: string
  proxyPassword?: string
  clearProxyPassword?: boolean
  credentialId?: string

  protocol?: string

  kind?: string
  secret?: string
}

export interface RouteTestHop {
  kind: string

  label: string

  address: string

  reachable: boolean

  stage: string

  errorKey: string

  latencyMs: number

  unverifiedHostKey: boolean
}

export interface RouteTestResult {
  mode: string
  hops: RouteTestHop[] | null
  ok: boolean
}

export interface SFTPFavoritePath {
  path: string

  name?: string
}

export interface SFTPTransfer {
  id: string
  sessionId: string
  direction: SFTPTransferDirection
  localName: string
  remotePath: string
  state: SFTPTransferState
  transferred: number
  total: number
  createdAt: string
  updatedAt: string

  rateBytesPerSecond: number

  stage?: string
}

export enum SFTPTransferDirection {
  $zero = '',

  SFTPTransferUpload = 'upload',
  SFTPTransferDownload = 'download',
}

export enum SFTPTransferState {
  $zero = '',

  SFTPTransferQueued = 'queued',
  SFTPTransferAwaitingOverwrite = 'awaitingOverwrite',
  SFTPTransferRunning = 'running',

  SFTPTransferPaused = 'paused',
  SFTPTransferCompleted = 'completed',
  SFTPTransferFailed = 'failed',
  SFTPTransferCanceled = 'canceled',
}

export interface ServiceDetail {
  name: string
  fields: { [_ in string]?: string } | null
}

export interface ServiceInfo {
  name: string

  description: string

  loadState: string
  activeState: string
  subState: string

  enabledState: string

  custom: boolean

  scope: string
}

export interface ServiceLog {
  name: string
  text: string

  truncated: boolean
}

export interface ServiceSnapshot {
  manager: string
  services: ServiceInfo[] | null
  updatedAt: number
}

export interface Session {
  id: string
  hostId: string
  kind: SessionKind
  state: SessionState
  purpose: SessionPurpose

  createdAt: string
  updatedAt: string
  latencyMs: number | null

  attachEpoch: number

  closeReason?: string

  exitCode?: number | null
}

export interface SessionHistory {
  id: string
  hostId: string
  hostName: string
  address: string
  startedAt: string
  endedAt: string
  status: string
  errorKey: string
  note: string
  archived: boolean
}

export interface SessionHistoryBatchArchiveResult {
  updated: number

  missing: number
}

export interface SessionHistoryClearResult {
  sessions: number

  recordings: number

  skipped: number

  failed: number
}

export enum SessionKind {
  $zero = '',

  SessionKindSSH = 'ssh',
  SessionKindTelnet = 'telnet',
  SessionKindLocal = 'local',
}

export enum SessionPurpose {
  $zero = '',

  SessionPurposeTerminal = 'terminal',

  SessionPurposeSftp = 'sftp',

  SessionPurposeAI = 'ai',
}

export enum SessionState {
  $zero = '',

  SessionStateConnecting = 'connecting',

  SessionStateAwaitingAuth = 'awaitingAuth',
  SessionStateHostKeyPending = 'hostKeyPending',
  SessionStateAuthenticating = 'authenticating',
  SessionStateConnected = 'connected',
  SessionStateClosing = 'closing',
  SessionStateClosed = 'closed',
  SessionStateFailed = 'failed',
}

export interface Snippet {
  icon?: string
  color?: string
  pinned?: boolean
  id: string
  name: string
  content: string
  groupId?: string
  group?: string
  hostIds?: string[] | null
  assetGroupId?: string
  assetGroup?: string
  autoExecute: boolean
  createdAt: string
  updatedAt: string
}

export interface SnippetInput {
  icon?: string
  color?: string
  pinned?: boolean
  name: string
  content: string
  groupId?: string
  hostIds?: string[] | null
  assetGroupId?: string
  autoExecute: boolean
}

export interface TerminalRecording {
  id: string
  sessionId: string
  hostId: string

  mode: string
  path: string
  startedAt: string
  endedAt: string
  status: string
  bytes: number
  eventCount: number
  hostName: string
  address: string
  note: string
}

export interface TerminalRecordingClearResult {
  deleted: number

  skipped: number

  missing: number

  failed: number
}

export interface TerminalRecordingEvent {
  atMs: number
  type: number
  columns: number
  rows: number
  data: string
}

export interface TerminalRecordingEventBatch {
  cursor: number
  truncated: boolean
  events: TerminalRecordingEvent[] | null
}

export interface TerminalRecordingPage {
  items: TerminalRecording[] | null
  page: number
  pageSize: number
  total: number
}

export interface VncHostProfile {
  hostId: string
  name: string
  address: string
  port: number

  username?: string

  passwordAvailable: boolean

  passwordSource?: string

  viewOnly: boolean
  shared: boolean
  pixelDepth: number

  transport: string
  wsPath: string
  allowInsecureTls: boolean

  basicAuthUsed: boolean

  rfbPasswordMode: string

  rfbPasswordRequired: boolean
}

export interface VncOpenResult {
  session: VncSession
  ticket: VncStreamTicket

  username?: string

  password?: string

  transport: string

  allowInsecureTls: boolean

  basicAuthUsed: boolean

  rfbPasswordMode: string

  serverCertFingerprint?: string

  serverSoftware?: string
  serverVersion?: string
  serverBanner?: string
  identitySource?: string
}

export interface VncSession {
  id: string
  hostId: string
  name: string
  address: string
  port: number

  viewOnly: boolean

  shared: boolean

  pixelDepth: number

  transport: string

  allowInsecureTls: boolean

  serverCertFingerprint?: string

  serverSoftware?: string
  serverVersion?: string

  serverBanner?: string

  identitySource?: string

  connected: boolean

  error?: string
}

export interface VncStreamTicket {
  streamName: string
  token: string

  protocolVersion: number
}
