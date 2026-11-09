// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import { Call as $Call, CancellablePromise as $CancellablePromise } from '@wailsio/runtime'

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as ai$0 from '../ai/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as applock$0 from '../applock/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as credential$0 from '../credential/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as fonts$0 from '../fonts/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as hostimport$0 from '../hostimport/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as model$0 from '../model/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as session$0 from '../session/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as sftp$0 from '../sftp/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as translate$0 from '../translate/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as update$0 from '../update/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as x11$0 from '../x11/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as zmodem$0 from '../zmodem/models.js'

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as $models from './models.js'

export function AIAgentAudit(limit: number): $CancellablePromise<ai$0.AgentAuditEntry[] | null> {
  return $Call.ByID(3472399800, limit)
}

export function AIAgentCancel(runID: string): $CancellablePromise<ai$0.AgentSnapshot> {
  return $Call.ByID(1244378891, runID)
}

export function AIAgentCancelAll(): $CancellablePromise<number> {
  return $Call.ByID(3211239502)
}

export function AIAgentDecide(input: ai$0.DecideInput): $CancellablePromise<ai$0.AgentSnapshot> {
  return $Call.ByID(2664465425, input)
}

export function AIAgentRelease(runID: string): $CancellablePromise<void> {
  return $Call.ByID(2911744206, runID)
}

export function AIAgentSnapshot(runID: string, since: number): $CancellablePromise<ai$0.AgentSnapshot> {
  return $Call.ByID(3167678717, runID, since)
}

export function AIAgentStart(input: ai$0.StartInput): $CancellablePromise<ai$0.AgentSnapshot> {
  return $Call.ByID(55687791, input)
}

export function AIChat(input: $models.AIChatInput): $CancellablePromise<$models.AIChatResult> {
  return $Call.ByID(3734108874, input)
}

export function AIChatStream(input: $models.AIChatInput): $CancellablePromise<$models.AIChatStreamResult> {
  return $Call.ByID(3924191282, input)
}

export function AIChatStreamCancel(streamID: string): $CancellablePromise<void> {
  return $Call.ByID(2128265056, streamID)
}

export function AIChatStreamPoll(streamID: string): $CancellablePromise<$models.AIChatStreamPollResult> {
  return $Call.ByID(2039935851, streamID)
}

export function AIChatStreamRelease(streamID: string): $CancellablePromise<void> {
  return $Call.ByID(1375990547, streamID)
}

export function AIProviderKeyConfigured(providerID: string): $CancellablePromise<boolean> {
  return $Call.ByID(1242213856, providerID)
}

export function ApplySFTPSettings(settings: sftp$0.Settings): $CancellablePromise<void> {
  return $Call.ByID(4027991754, settings)
}

export function ArchiveSessionHistory(id: string, archived: boolean): $CancellablePromise<void> {
  return $Call.ByID(4170568592, id, archived)
}

export function CancelDroppedFiles(id: string): $CancellablePromise<void> {
  return $Call.ByID(3043275525, id)
}

export function CancelSFTPTransfer(id: string): $CancellablePromise<void> {
  return $Call.ByID(91425590, id)
}

export function ChangeAppLockPassword(current: string, next: string): $CancellablePromise<void> {
  return $Call.ByID(3277894939, current, next)
}

export function ChangeEncryptionKey(
  input: credential$0.ChangeEncryptionKeyInput,
): $CancellablePromise<credential$0.CredentialVaultStatus> {
  return $Call.ByID(2409049454, input)
}

export function ChangeMasterPassword(
  input: credential$0.ChangeMasterPasswordInput,
): $CancellablePromise<credential$0.CredentialVaultStatus> {
  return $Call.ByID(686754561, input)
}

export function ChmodLocalEntry(token: string, name: string, mode: string): $CancellablePromise<void> {
  return $Call.ByID(649698850, token, name, mode)
}

export function ChmodRemoteEntry(sessionID: string, remotePath: string, mode: string): $CancellablePromise<void> {
  return $Call.ByID(2173853807, sessionID, remotePath, mode)
}

export function ClaimQuitConfirm(requestID: number): $CancellablePromise<void> {
  return $Call.ByID(3612225691, requestID)
}

export function ClearAIProviderKey(providerID: string): $CancellablePromise<void> {
  return $Call.ByID(1826570639, providerID)
}

export function ClearCommandHistory(hostID: string): $CancellablePromise<number> {
  return $Call.ByID(2493567980, hostID)
}

export function ClearDebugLog(): $CancellablePromise<void> {
  return $Call.ByID(2289130260)
}

export function ClearUnarchivedSessionHistory(): $CancellablePromise<model$0.SessionHistoryClearResult> {
  return $Call.ByID(2415197634)
}

export function CloseForward(id: string): $CancellablePromise<void> {
  return $Call.ByID(1250373495, id)
}

export function CloseHostEditorWindow(): $CancellablePromise<void> {
  return $Call.ByID(1524381477)
}

export function CloseSession(id: string): $CancellablePromise<void> {
  return $Call.ByID(1658825694, id)
}

export function CloseToTray(): $CancellablePromise<boolean> {
  return $Call.ByID(411676227)
}

export function ConfirmSFTPTransferOverwrite(id: string): $CancellablePromise<model$0.SFTPTransfer> {
  return $Call.ByID(4063224497, id)
}

export function CreateCredential(input: credential$0.CreateCredentialInput): $CancellablePromise<model$0.Credential> {
  return $Call.ByID(238761463, input)
}

export function CreateCredentialPromptRule(
  input: model$0.CredentialPromptRuleInput,
): $CancellablePromise<model$0.CredentialPromptRule> {
  return $Call.ByID(3648680203, input)
}

export function CreateGroup(input: model$0.GroupInput): $CancellablePromise<model$0.Group> {
  return $Call.ByID(385879933, input)
}

export function CreateHost(input: model$0.HostInput): $CancellablePromise<model$0.Host> {
  return $Call.ByID(3009019220, input)
}

export function CreateLocalDirectory(token: string, name: string): $CancellablePromise<void> {
  return $Call.ByID(2037135932, token, name)
}

export function CreateLocalFile(token: string, name: string): $CancellablePromise<void> {
  return $Call.ByID(1816262823, token, name)
}

export function CreateProxy(input: model$0.ProxyInput): $CancellablePromise<model$0.ProxyRecord> {
  return $Call.ByID(313684390, input)
}

export function CreateRemoteDirectory(sessionID: string, directory: string, name: string): $CancellablePromise<void> {
  return $Call.ByID(4278613987, sessionID, directory, name)
}

export function CreateRemoteFile(sessionID: string, directory: string, name: string): $CancellablePromise<void> {
  return $Call.ByID(897607862, sessionID, directory, name)
}

export function CreateSnippet(input: model$0.SnippetInput): $CancellablePromise<model$0.Snippet> {
  return $Call.ByID(4079013443, input)
}

export function CreateSnippetGroup(name: string): $CancellablePromise<model$0.Group> {
  return $Call.ByID(1404906746, name)
}

export function CredentialStatus(): $CancellablePromise<credential$0.CredentialVaultStatus> {
  return $Call.ByID(3923529053)
}

export function DebugLoggingEnabled(): $CancellablePromise<boolean> {
  return $Call.ByID(205497095)
}

export function DeleteCommandHistory(id: string): $CancellablePromise<void> {
  return $Call.ByID(592556222, id)
}

export function DeleteCredential(id: string): $CancellablePromise<void> {
  return $Call.ByID(2957472444, id)
}

export function DeleteCredentialPromptRule(id: string): $CancellablePromise<void> {
  return $Call.ByID(2021379484, id)
}

export function DeleteGroup(id: string): $CancellablePromise<void> {
  return $Call.ByID(535515504, id)
}

export function DeleteHost(id: string): $CancellablePromise<void> {
  return $Call.ByID(1962312371, id)
}

export function DeleteKnownHost(line: number): $CancellablePromise<void> {
  return $Call.ByID(1353670362, line)
}

export function DeleteLocalEntry(token: string, name: string): $CancellablePromise<void> {
  return $Call.ByID(2606584800, token, name)
}

export function DeleteOTPEntry(id: string): $CancellablePromise<void> {
  return $Call.ByID(860336590, id)
}

export function DeleteProxy(id: string): $CancellablePromise<number> {
  return $Call.ByID(1520664207, id)
}

export function DeleteRemoteEntry(sessionID: string, remotePath: string): $CancellablePromise<void> {
  return $Call.ByID(2713071257, sessionID, remotePath)
}

export function DeleteSessionHistory(id: string): $CancellablePromise<void> {
  return $Call.ByID(1060799205, id)
}

export function DeleteSnippet(id: string): $CancellablePromise<void> {
  return $Call.ByID(909599298, id)
}

export function DeleteSnippetGroup(id: string): $CancellablePromise<void> {
  return $Call.ByID(1066458933, id)
}

export function DeleteTerminalRecording(id: string): $CancellablePromise<void> {
  return $Call.ByID(988509556, id)
}

export function DeleteTerminalRecordings(
  ids: string[] | null,
): $CancellablePromise<model$0.TerminalRecordingClearResult> {
  return $Call.ByID(3349756421, ids)
}

export function DeleteTranslationProviderCredential(provider: string): $CancellablePromise<void> {
  return $Call.ByID(3511792168, provider)
}

export function DetectHostIcon(input: session$0.DetectIconInput): $CancellablePromise<string> {
  return $Call.ByID(1653377954, input)
}

export function DetectX11Server(settings: x11$0.Settings): $CancellablePromise<x11$0.Result> {
  return $Call.ByID(2850121394, settings)
}

export function DirectoryFollow(sessionID: string, configure: boolean): $CancellablePromise<string> {
  return $Call.ByID(813059872, sessionID, configure)
}

export function DisableAppLock(password: string): $CancellablePromise<void> {
  return $Call.ByID(3730357018, password)
}

export function DiscardLocalEdit(editID: string): $CancellablePromise<void> {
  return $Call.ByID(1354879359, editID)
}

export function DockerComposeAction(sessionID: string, containerID: string, action: string): $CancellablePromise<void> {
  return $Call.ByID(3442158080, sessionID, containerID, action)
}

export function DockerComposeFilePath(sessionID: string, containerID: string): $CancellablePromise<string> {
  return $Call.ByID(1113758595, sessionID, containerID)
}

export function DockerContainerAction(
  sessionID: string,
  containerID: string,
  action: string,
  force: boolean,
): $CancellablePromise<void> {
  return $Call.ByID(371654235, sessionID, containerID, action, force)
}

export function DockerContainerDetail(
  sessionID: string,
  containerID: string,
): $CancellablePromise<model$0.DockerContainerDetail> {
  return $Call.ByID(3213822248, sessionID, containerID)
}

export function DockerContainers(sessionID: string): $CancellablePromise<model$0.DockerContainer[] | null> {
  return $Call.ByID(2277021438, sessionID)
}

export function DockerEnvironment(sessionID: string): $CancellablePromise<model$0.DockerEnvironment> {
  return $Call.ByID(3841656887, sessionID)
}

export function DockerImageDetail(sessionID: string, imageRef: string): $CancellablePromise<model$0.DockerImageDetail> {
  return $Call.ByID(1381256830, sessionID, imageRef)
}

export function DockerImages(sessionID: string): $CancellablePromise<model$0.DockerImage[] | null> {
  return $Call.ByID(2872934372, sessionID)
}

export function DockerNetworkDetail(
  sessionID: string,
  networkID: string,
): $CancellablePromise<model$0.DockerNetworkDetail> {
  return $Call.ByID(3564475945, sessionID, networkID)
}

export function DockerNetworks(sessionID: string): $CancellablePromise<model$0.DockerNetwork[] | null> {
  return $Call.ByID(26725793, sessionID)
}

export function DockerOpenComposeLogs(sessionID: string, containerID: string): $CancellablePromise<void> {
  return $Call.ByID(798311577, sessionID, containerID)
}

export function DockerOpenComposeStats(sessionID: string, containerID: string): $CancellablePromise<void> {
  return $Call.ByID(1359725305, sessionID, containerID)
}

export function DockerOpenContainerLogs(sessionID: string, containerID: string): $CancellablePromise<void> {
  return $Call.ByID(336482906, sessionID, containerID)
}

export function DockerOpenContainerStats(sessionID: string, containerID: string): $CancellablePromise<void> {
  return $Call.ByID(3423687392, sessionID, containerID)
}

export function DockerOpenContainerTerminal(sessionID: string, containerID: string): $CancellablePromise<void> {
  return $Call.ByID(2658458565, sessionID, containerID)
}

export function DockerPanelEnabled(sessionID: string): $CancellablePromise<boolean> {
  return $Call.ByID(4087747145, sessionID)
}

export function DockerPruneImages(sessionID: string, all: boolean): $CancellablePromise<void> {
  return $Call.ByID(3354105416, sessionID, all)
}

export function DockerPruneNetworks(sessionID: string): $CancellablePromise<void> {
  return $Call.ByID(3209510357, sessionID)
}

export function DockerPruneVolumes(sessionID: string): $CancellablePromise<void> {
  return $Call.ByID(1873055537, sessionID)
}

export function DockerRemoveImage(sessionID: string, imageID: string): $CancellablePromise<void> {
  return $Call.ByID(1687826179, sessionID, imageID)
}

export function DockerRemoveNetwork(sessionID: string, networkID: string): $CancellablePromise<void> {
  return $Call.ByID(1098092300, sessionID, networkID)
}

export function DockerRemoveVolume(sessionID: string, volumeName: string): $CancellablePromise<void> {
  return $Call.ByID(1541434352, sessionID, volumeName)
}

export function DockerVolumeDetail(
  sessionID: string,
  volumeName: string,
): $CancellablePromise<model$0.DockerVolumeDetail> {
  return $Call.ByID(3526412141, sessionID, volumeName)
}

export function DockerVolumes(sessionID: string): $CancellablePromise<model$0.DockerVolume[] | null> {
  return $Call.ByID(2740883837, sessionID)
}

export function DuplicateHost(id: string, name: string): $CancellablePromise<model$0.Host> {
  return $Call.ByID(2519449577, id, name)
}

export function EnableAppLock(
  password: string,
  autoLockMinutes: number,
  question: string,
  answer: string,
): $CancellablePromise<void> {
  return $Call.ByID(2556552779, password, autoLockMinutes, question, answer)
}

export function ExportCredentialVaultKey(input: credential$0.ExportCredentialVaultKeyInput): $CancellablePromise<void> {
  return $Call.ByID(637797398, input)
}

export function ExportDebugLog(): $CancellablePromise<boolean> {
  return $Call.ByID(2369405043)
}

export function ExportResourceBundle(password: string): $CancellablePromise<boolean> {
  return $Call.ByID(2972663712, password)
}

export function ExportSessionHistoryText(historyID: string): $CancellablePromise<boolean> {
  return $Call.ByID(2463108303, historyID)
}

export function ExportTerminalRecordings(ids: string[] | null, format: string): $CancellablePromise<boolean> {
  return $Call.ByID(1679106884, ids, format)
}

export function ExportTerminalRecordingsText(ids: string[] | null, filename: string): $CancellablePromise<boolean> {
  return $Call.ByID(3001363379, ids, filename)
}

export function ExportTerminalRecordingsWithOptions(
  ids: string[] | null,
  format: string,
  includeOutput: boolean,
): $CancellablePromise<boolean> {
  return $Call.ByID(3410843522, ids, format, includeOutput)
}

export function FinishZmodemFile(token: string): $CancellablePromise<void> {
  return $Call.ByID(1970632839, token)
}

export function FocusAuxiliaryWindow(): $CancellablePromise<void> {
  return $Call.ByID(2944042250)
}

export function ForgetAdHocSession(hostID: string): $CancellablePromise<void> {
  return $Call.ByID(1812198524, hostID)
}

export function ForwardStatus(id: string): $CancellablePromise<model$0.ForwardStatus> {
  return $Call.ByID(1063434213, id)
}

export function GenerateOTPCode(entry: model$0.OTPEntry): $CancellablePromise<model$0.OTPCode> {
  return $Call.ByID(1152370611, entry)
}

export function GeneratePrivateKey(
  input: credential$0.GeneratePrivateKeyInput,
): $CancellablePromise<credential$0.GeneratedPrivateKey> {
  return $Call.ByID(1580133415, input)
}

export function GetAppLockStatus(): $CancellablePromise<applock$0.Status> {
  return $Call.ByID(3987772584)
}

export function GetPreference(key: string): $CancellablePromise<string> {
  return $Call.ByID(3832334539, key)
}

export function GetRecordingSettings(): $CancellablePromise<$models.RecordingSettings> {
  return $Call.ByID(2270951508)
}

export function GetSFTPSettings(): $CancellablePromise<sftp$0.Settings> {
  return $Call.ByID(2069614902)
}

export function GetSession(id: string): $CancellablePromise<model$0.Session> {
  return $Call.ByID(3221111070, id)
}

export function GetSessionAuthenticationChallenge(id: string): $CancellablePromise<session$0.AuthenticationChallenge> {
  return $Call.ByID(3989211779, id)
}

export function GetSessionFailure(id: string): $CancellablePromise<string> {
  return $Call.ByID(2487557140, id)
}

export function GetSessionHostKeyChallenge(id: string): $CancellablePromise<model$0.HostKeyChallenge> {
  return $Call.ByID(3295975960, id)
}

export function GetSessionHostKeyPending(id: string): $CancellablePromise<model$0.HostKeyPending> {
  return $Call.ByID(3017114806, id)
}

export function GetSessionWarnings(id: string): $CancellablePromise<string[] | null> {
  return $Call.ByID(2986836979, id)
}

export function HideSettingsWindow(): $CancellablePromise<void> {
  return $Call.ByID(2087786787)
}

export function ImportCredentialVaultKey(
  input: credential$0.ImportCredentialVaultKeyInput,
): $CancellablePromise<credential$0.CredentialVaultStatus> {
  return $Call.ByID(4239103037, input)
}

export function ImportHosts(input: $models.ImportHostsInput): $CancellablePromise<$models.ImportHostsResult> {
  return $Call.ByID(2579302042, input)
}

export function ImportLocalPrivateKey(
  input: credential$0.ImportLocalPrivateKeyInput,
): $CancellablePromise<model$0.Credential> {
  return $Call.ByID(1212757688, input)
}

export function ImportResourceBundle(password: string): $CancellablePromise<boolean> {
  return $Call.ByID(4092480087, password)
}

export function ImportResourceBundleData(data: string | null, password: string): $CancellablePromise<boolean> {
  return $Call.ByID(2260233215, data, password)
}

export function KnownHostsPath(): $CancellablePromise<string> {
  return $Call.ByID(2598525091)
}

export function ListAIModels(input: $models.AIModelListInput): $CancellablePromise<string[] | null> {
  return $Call.ByID(479945634, input)
}

export function ListAIModelsPreview(input: $models.AIProviderPreviewInput): $CancellablePromise<string[] | null> {
  return $Call.ByID(2378449630, input)
}

export function ListCommandHistory(hostID: string): $CancellablePromise<model$0.CommandHistory[] | null> {
  return $Call.ByID(2871219925, hostID)
}

export function ListCredentialPromptRules(): $CancellablePromise<model$0.CredentialPromptRule[] | null> {
  return $Call.ByID(1857505944)
}

export function ListCredentials(): $CancellablePromise<model$0.Credential[] | null> {
  return $Call.ByID(3341414780)
}

export function ListGroups(): $CancellablePromise<model$0.Group[] | null> {
  return $Call.ByID(3978759226)
}

export function ListHostImportSources(): $CancellablePromise<$models.HostImportSourceInfo[] | null> {
  return $Call.ByID(2576679269)
}

export function ListHosts(): $CancellablePromise<model$0.Host[] | null> {
  return $Call.ByID(3228783509)
}

export function ListKnownHosts(): $CancellablePromise<model$0.KnownHostEntry[] | null> {
  return $Call.ByID(1060404672)
}

export function ListLocalEdits(): $CancellablePromise<model$0.LocalEdit[] | null> {
  return $Call.ByID(4155875522)
}

export function ListLocalPrivateKeys(): $CancellablePromise<credential$0.LocalPrivateKey[] | null> {
  return $Call.ByID(1468199356)
}

export function ListProxies(): $CancellablePromise<model$0.ProxyRecord[] | null> {
  return $Call.ByID(2894752158)
}

export function ListRemoteDirectory(
  sessionID: string,
  directory: string,
): $CancellablePromise<model$0.RemoteFileEntry[] | null> {
  return $Call.ByID(314599347, sessionID, directory)
}

export function ListRemotePrincipals(sessionID: string): $CancellablePromise<model$0.RemotePrincipals> {
  return $Call.ByID(3699595763, sessionID)
}

export function ListSFTPTransfers(): $CancellablePromise<model$0.SFTPTransfer[] | null> {
  return $Call.ByID(2142389289)
}

export function ListSerialPorts(): $CancellablePromise<$models.SerialPortInfo[] | null> {
  return $Call.ByID(2185579694)
}

export function ListServices(sessionID: string): $CancellablePromise<model$0.ServiceSnapshot> {
  return $Call.ByID(901846326, sessionID)
}

export function ListSessionHistory(): $CancellablePromise<model$0.SessionHistory[] | null> {
  return $Call.ByID(1272692246)
}

export function ListSessions(): $CancellablePromise<model$0.Session[] | null> {
  return $Call.ByID(1258374775)
}

export function ListSnippetGroups(): $CancellablePromise<model$0.Group[] | null> {
  return $Call.ByID(3413215995)
}

export function ListSnippets(): $CancellablePromise<model$0.Snippet[] | null> {
  return $Call.ByID(342156576)
}

export function ListTerminalFonts(refresh: boolean): $CancellablePromise<fonts$0.Family[] | null> {
  return $Call.ByID(3445732048, refresh)
}

export function ListTerminalRecordings(): $CancellablePromise<model$0.TerminalRecording[] | null> {
  return $Call.ByID(3740319254)
}

export function ListTerminalRecordingsPage(
  page: number,
  pageSize: number,
  query: string,
): $CancellablePromise<model$0.TerminalRecordingPage> {
  return $Call.ByID(535855101, page, pageSize, query)
}

export function ListTranslationProviderCredentials(): $CancellablePromise<translate$0.CredentialStatus[] | null> {
  return $Call.ByID(3803565930)
}

export function LockCredentialVault(): $CancellablePromise<void> {
  return $Call.ByID(760453592)
}

export function MoveGroup(id: string, parentID: string): $CancellablePromise<void> {
  return $Call.ByID(635579154, id, parentID)
}

export function MoveHost(id: string, groupID: string): $CancellablePromise<void> {
  return $Call.ByID(1505326085, id, groupID)
}

export function NotifyAppLockState(locked: boolean): $CancellablePromise<void> {
  return $Call.ByID(571225882, locked)
}

export function OTPProvisioningURI(entry: model$0.OTPEntry): $CancellablePromise<string> {
  return $Call.ByID(363929716, entry)
}

export function OpenAdHocSession(input: $models.AdHocSessionInput): $CancellablePromise<session$0.OpenSessionResult> {
  return $Call.ByID(583057837, input)
}

export function OpenForward(config: model$0.ForwardConfig): $CancellablePromise<void> {
  return $Call.ByID(3611018899, config)
}

export function OpenHostEditorWindow(hostID: string): $CancellablePromise<void> {
  return $Call.ByID(2176633297, hostID)
}

export function OpenLocalEdit(sessionID: string, remotePath: string): $CancellablePromise<model$0.LocalEdit> {
  return $Call.ByID(27047233, sessionID, remotePath)
}

export function OpenLocalSession(
  shellPath: string,
  workingDirectory: string,
): $CancellablePromise<session$0.OpenSessionResult> {
  return $Call.ByID(2050395015, shellPath, workingDirectory)
}

export function OpenNewWindow(): $CancellablePromise<void> {
  return $Call.ByID(1683412772)
}

export function OpenRdpWindow(hostID: string, title: string): $CancellablePromise<void> {
  return $Call.ByID(1914972316, hostID, title)
}

export function OpenRecordingDirectory(): $CancellablePromise<void> {
  return $Call.ByID(1274047008)
}

export function OpenSFTPLocalDirectory(token: string, child: string): $CancellablePromise<sftp$0.LocalDirectory> {
  return $Call.ByID(3529304341, token, child)
}

export function OpenSerialSession(
  input: $models.OpenSerialSessionInput,
): $CancellablePromise<session$0.OpenSessionResult> {
  return $Call.ByID(2132867350, input)
}

export function OpenSession(hostID: string): $CancellablePromise<session$0.OpenSessionResult> {
  return $Call.ByID(778782082, hostID)
}

export function OpenSessionWindow(
  kind: string,
  hostID: string,
  sessionID: string,
  title: string,
): $CancellablePromise<void> {
  return $Call.ByID(590323982, kind, hostID, sessionID, title)
}

export function OpenSettingsWindow(
  kind: string,
  title: string,
  category: string,
  search: string,
  resizable: boolean,
): $CancellablePromise<void> {
  return $Call.ByID(1755290403, kind, title, category, search, resizable)
}

export function OpenSftpConnection(hostID: string): $CancellablePromise<model$0.Session> {
  return $Call.ByID(1735042375, hostID)
}

export function OpenSftpWindow(sessionID: string, title: string): $CancellablePromise<void> {
  return $Call.ByID(1576087119, sessionID, title)
}

export function OpenVncWindow(hostID: string, title: string): $CancellablePromise<void> {
  return $Call.ByID(4255285435, hostID, title)
}

export function PauseRecording(id: string): $CancellablePromise<model$0.TerminalRecording> {
  return $Call.ByID(2287420803, id)
}

export function PauseSFTPTransfer(id: string): $CancellablePromise<model$0.SFTPTransfer> {
  return $Call.ByID(1270514538, id)
}

export function PickHostImportPath(source: string): $CancellablePromise<string> {
  return $Call.ByID(2211582407, source)
}

export function PrepareSessionHandover(id: string): $CancellablePromise<void> {
  return $Call.ByID(4031088092, id)
}

export function PreviewHostImport(source: string, path: string): $CancellablePromise<hostimport$0.Result> {
  return $Call.ByID(488612513, source, path)
}

export function PreviewHostImportWithSecret(
  source: string,
  path: string,
  secret: string,
): $CancellablePromise<hostimport$0.Result> {
  return $Call.ByID(2050037089, source, path, secret)
}

export function QueueSFTPRemoteTransfer(
  sourceSessionID: string,
  destSessionID: string,
  remotePath: string,
  destDirectory: string,
): $CancellablePromise<model$0.SFTPTransfer> {
  return $Call.ByID(3907434849, sourceSessionID, destSessionID, remotePath, destDirectory)
}

export function QueueSFTPRemoteTransferDirectory(
  sourceSessionID: string,
  destSessionID: string,
  remotePath: string,
  destDirectory: string,
): $CancellablePromise<sftp$0.RemoteDirectoryTransferResult> {
  return $Call.ByID(3121391068, sourceSessionID, destSessionID, remotePath, destDirectory)
}

export function QueueSFTPWorkspaceDownload(
  sessionID: string,
  remotePath: string,
  directoryToken: string,
): $CancellablePromise<model$0.SFTPTransfer> {
  return $Call.ByID(1746730837, sessionID, remotePath, directoryToken)
}

export function QueueSFTPWorkspaceDownloadDirectory(
  sessionID: string,
  remotePath: string,
  directoryToken: string,
): $CancellablePromise<sftp$0.DirectoryDownloadResult> {
  return $Call.ByID(3732298776, sessionID, remotePath, directoryToken)
}

export function QueueSFTPWorkspaceUpload(
  sessionID: string,
  directoryToken: string,
  name: string,
  remoteDirectory: string,
): $CancellablePromise<model$0.SFTPTransfer> {
  return $Call.ByID(1184994768, sessionID, directoryToken, name, remoteDirectory)
}

export function QueueSFTPWorkspaceUploadDirectory(
  sessionID: string,
  directoryToken: string,
  name: string,
  remoteDirectory: string,
): $CancellablePromise<sftp$0.DirectoryUploadResult> {
  return $Call.ByID(2850876243, sessionID, directoryToken, name, remoteDirectory)
}

export function QuitConfirm(): $CancellablePromise<boolean> {
  return $Call.ByID(417972399)
}

export function RdpAdoptWindowSession(sessionID: string): $CancellablePromise<void> {
  return $Call.ByID(680844730, sessionID)
}

export function RdpCancelReturnToMain(sessionID: string): $CancellablePromise<void> {
  return $Call.ByID(242172752, sessionID)
}

export function RdpClipboardText(id: string): $CancellablePromise<string> {
  return $Call.ByID(514082589, id)
}

export function RdpClose(id: string): $CancellablePromise<void> {
  return $Call.ByID(2349233828, id)
}

export function RdpCompleteReturnToMain(sessionID: string): $CancellablePromise<void> {
  return $Call.ByID(620124961, sessionID)
}

export function RdpForgetCertificate(hostID: string): $CancellablePromise<void> {
  return $Call.ByID(2452186692, hostID)
}

export function RdpFrame(id: string): $CancellablePromise<model$0.RdpFrame> {
  return $Call.ByID(1601236493, id)
}

export function RdpHelperEngines(): $CancellablePromise<model$0.RdpHelperEngine[] | null> {
  return $Call.ByID(879266883)
}

export function RdpHostProfile(hostID: string): $CancellablePromise<model$0.RdpHostProfile> {
  return $Call.ByID(1654069615, hostID)
}

export function RdpListSessions(): $CancellablePromise<model$0.RdpSession[] | null> {
  return $Call.ByID(861141541)
}

export function RdpOpen(input: model$0.RdpOpenInput): $CancellablePromise<model$0.RdpSession> {
  return $Call.ByID(2988981768, input)
}

export function RdpOpenHost(
  hostID: string,
  width: number,
  height: number,
  password: string,
  engine: string,
): $CancellablePromise<model$0.RdpSession> {
  return $Call.ByID(983145818, hostID, width, height, password, engine)
}

export function RdpOpenSessionWindow(sessionID: string, hostID: string, title: string): $CancellablePromise<void> {
  return $Call.ByID(2007812296, sessionID, hostID, title)
}

export function RdpPrepareReturnToMain(sessionID: string): $CancellablePromise<void> {
  return $Call.ByID(1641277437, sessionID)
}

export function RdpRefreshClipboard(id: string): $CancellablePromise<void> {
  return $Call.ByID(250770387, id)
}

export function RdpRegisterWindowSession(sessionID: string): $CancellablePromise<void> {
  return $Call.ByID(3601401467, sessionID)
}

export function RdpResize(id: string, width: number, height: number): $CancellablePromise<void> {
  return $Call.ByID(1079210936, id, width, height)
}

export function RdpSendKey(
  id: string,
  scanCode: number,
  pressed: boolean,
  extended: boolean,
): $CancellablePromise<void> {
  return $Call.ByID(2564926069, id, scanCode, pressed, extended)
}

export function RdpSendPointerButton(
  id: string,
  x: number,
  y: number,
  button: number,
  pressed: boolean,
): $CancellablePromise<void> {
  return $Call.ByID(543870439, id, x, y, button, pressed)
}

export function RdpSendPointerMove(id: string, x: number, y: number): $CancellablePromise<void> {
  return $Call.ByID(4080260718, id, x, y)
}

export function RdpSendText(id: string, text: string): $CancellablePromise<void> {
  return $Call.ByID(2782297549, id, text)
}

export function RdpSendWheel(
  id: string,
  x: number,
  y: number,
  notches: number,
  horizontal: boolean,
): $CancellablePromise<void> {
  return $Call.ByID(1210127533, id, x, y, notches, horizontal)
}

export function RdpSetClipboardText(id: string, text: string): $CancellablePromise<void> {
  return $Call.ByID(2469722509, id, text)
}

export function RdpStats(id: string): $CancellablePromise<model$0.RdpStats> {
  return $Call.ByID(2764884627, id)
}

export function RdpStreamTicket(sessionID: string): $CancellablePromise<model$0.RdpStreamTicket> {
  return $Call.ByID(1008595764, sessionID)
}

export function RdpTrustCertificate(hostID: string, fingerprint: string): $CancellablePromise<void> {
  return $Call.ByID(1412544901, hostID, fingerprint)
}

export function RdpTrustedCertificate(hostID: string): $CancellablePromise<model$0.RdpTrustedCertificate> {
  return $Call.ByID(2328963594, hostID)
}

export function ReadAccelerators(sessionID: string): $CancellablePromise<model$0.AcceleratorSnapshot> {
  return $Call.ByID(1050106490, sessionID)
}

export function ReadActiveRecordingEvents(
  id: string,
  cursor: number,
): $CancellablePromise<model$0.TerminalRecordingEventBatch> {
  return $Call.ByID(2854179134, id, cursor)
}

export function ReadDebugLog(): $CancellablePromise<string> {
  return $Call.ByID(2682561501)
}

export function ReadProcesses(sessionID: string): $CancellablePromise<model$0.ProcessSnapshot> {
  return $Call.ByID(1144941899, sessionID)
}

export function ReadRemoteFileText(
  sessionID: string,
  remotePath: string,
): $CancellablePromise<model$0.RemoteFileContent> {
  return $Call.ByID(321626153, sessionID, remotePath)
}

export function ReadRemoteImage(
  sessionID: string,
  remotePath: string,
): $CancellablePromise<model$0.RemoteImageContent> {
  return $Call.ByID(194925249, sessionID, remotePath)
}

export function ReadResources(id: string): $CancellablePromise<model$0.ResourceSnapshot> {
  return $Call.ByID(3769369431, id)
}

export function ReadTerminalRecording(id: string): $CancellablePromise<model$0.TerminalRecordingEvent[] | null> {
  return $Call.ByID(2644352693, id)
}

export function ReadZmodemFile(token: string, offset: number, length: number): $CancellablePromise<string | null> {
  return $Call.ByID(3997625896, token, offset, length)
}

export function RebindSession(id: string): $CancellablePromise<session$0.OpenSessionResult> {
  return $Call.ByID(2173601416, id)
}

export function ReleaseZmodemFile(token: string): $CancellablePromise<void> {
  return $Call.ByID(1943757433, token)
}

export function RenameCredential(
  id: string,
  input: credential$0.RenameCredentialInput,
): $CancellablePromise<model$0.Credential> {
  return $Call.ByID(838624425, id, input)
}

export function RenameLocalEntry(token: string, name: string, newName: string): $CancellablePromise<void> {
  return $Call.ByID(510542333, token, name, newName)
}

export function RenameRemoteEntry(sessionID: string, remotePath: string, newName: string): $CancellablePromise<void> {
  return $Call.ByID(753175270, sessionID, remotePath, newName)
}

export function RenameSnippetGroup(id: string, name: string): $CancellablePromise<model$0.Group> {
  return $Call.ByID(32802628, id, name)
}

export function ReorderGroup(
  id: string,
  parentID: string,
  targetID: string,
  position: string,
): $CancellablePromise<void> {
  return $Call.ByID(1820518348, id, parentID, targetID, position)
}

export function ReorderHost(
  id: string,
  groupID: string,
  targetID: string,
  position: string,
): $CancellablePromise<void> {
  return $Call.ByID(3512032687, id, groupID, targetID, position)
}

export function ReorderSnippet(
  id: string,
  groupID: string,
  targetID: string,
  position: string,
): $CancellablePromise<void> {
  return $Call.ByID(1317123486, id, groupID, targetID, position)
}

export function ReplaceCredentialSecret(
  id: string,
  input: credential$0.ReplaceCredentialSecretInput,
): $CancellablePromise<model$0.Credential> {
  return $Call.ByID(3756570335, id, input)
}

export function ResetAppLockPassword(answer: string, next: string): $CancellablePromise<void> {
  return $Call.ByID(3191838286, answer, next)
}

export function ResetCredentialVault(
  input: credential$0.ResetCredentialVaultInput,
): $CancellablePromise<credential$0.CredentialVaultStatus> {
  return $Call.ByID(570355072, input)
}

export function ResolveQuitConfirm(requestID: number, confirmed: boolean): $CancellablePromise<void> {
  return $Call.ByID(1801040047, requestID, confirmed)
}

export function ResolveSFTPTransferConflict(id: string, action: string): $CancellablePromise<void> {
  return $Call.ByID(1189283678, id, action)
}

export function ResolveZmodemDownload(token: string, action: string): $CancellablePromise<zmodem$0.LocalFile> {
  return $Call.ByID(1293642878, token, action)
}

export function ResumeRecording(id: string): $CancellablePromise<model$0.TerminalRecording> {
  return $Call.ByID(2403291472, id)
}

export function ResumeSFTPTransfer(id: string): $CancellablePromise<model$0.SFTPTransfer> {
  return $Call.ByID(2169212063, id)
}

export function RetrySFTPTransfer(id: string): $CancellablePromise<model$0.SFTPTransfer> {
  return $Call.ByID(869503398, id)
}

export function RevealCredential(id: string): $CancellablePromise<credential$0.RevealedCredential> {
  return $Call.ByID(4105731082, id)
}

export function RunAIBackgroundCommand(
  sessionID: string,
  command: string,
): $CancellablePromise<$models.AIBackgroundCommandResult> {
  return $Call.ByID(4293626990, sessionID, command)
}

export function RunProcessAction(
  sessionID: string,
  pid: number,
  expectedStartTime: string,
  action: string,
): $CancellablePromise<void> {
  return $Call.ByID(1545066326, sessionID, pid, expectedStartTime, action)
}

export function RunServiceAction(sessionID: string, unit: string, action: string): $CancellablePromise<void> {
  return $Call.ByID(2335157384, sessionID, unit, action)
}

export function SaveAIProviderKey(providerID: string, key: string): $CancellablePromise<void> {
  return $Call.ByID(3082854875, providerID, key)
}

export function SaveCommandHistory(item: model$0.CommandHistory): $CancellablePromise<void> {
  return $Call.ByID(2010441328, item)
}

export function SaveOTPEntry(input: model$0.OTPSaveInput): $CancellablePromise<model$0.OTPEntry> {
  return $Call.ByID(4230147212, input)
}

export function SaveSessionHistory(item: model$0.SessionHistory): $CancellablePromise<void> {
  return $Call.ByID(2644172975, item)
}

export function SaveTranslationProviderCredential(
  provider: string,
  input: translate$0.CredentialInput,
): $CancellablePromise<void> {
  return $Call.ByID(3594716366, provider, input)
}

export function SelectRecordingDirectory(): $CancellablePromise<string> {
  return $Call.ByID(2207868616)
}

export function SelectSFTPDownloadFile(
  sessionID: string,
  remotePath: string,
): $CancellablePromise<sftp$0.SFTPTransferSelectionResult> {
  return $Call.ByID(3871848317, sessionID, remotePath)
}

export function SelectSFTPEditor(): $CancellablePromise<string> {
  return $Call.ByID(4272653162)
}

export function SelectSFTPLocalDirectory(): $CancellablePromise<sftp$0.SFTPLocalDirectorySelectionResult> {
  return $Call.ByID(3894810621)
}

export function SelectSFTPLocalDirectorySetting(): $CancellablePromise<string> {
  return $Call.ByID(1312289779)
}

export function SelectSFTPUploadDirectory(
  sessionID: string,
  remoteDirectory: string,
): $CancellablePromise<sftp$0.DirectoryUploadResult> {
  return $Call.ByID(907766137, sessionID, remoteDirectory)
}

export function SelectSFTPUploadFile(
  sessionID: string,
  remoteDirectory: string,
): $CancellablePromise<sftp$0.SFTPTransferSelectionResult> {
  return $Call.ByID(3089984444, sessionID, remoteDirectory)
}

export function SelectZmodemDownload(name: string, size: number): $CancellablePromise<zmodem$0.LocalFile> {
  return $Call.ByID(2462619430, name, size)
}

export function SelectZmodemUpload(): $CancellablePromise<zmodem$0.LocalFile> {
  return $Call.ByID(1264418739)
}

export function ServiceDetails(sessionID: string, unit: string): $CancellablePromise<model$0.ServiceDetail> {
  return $Call.ByID(2532336165, sessionID, unit)
}

export function ServiceLogs(sessionID: string, unit: string, lines: number): $CancellablePromise<model$0.ServiceLog> {
  return $Call.ByID(1699247110, sessionID, unit, lines)
}

export function SetAppLockAutoLockMinutes(minutes: number): $CancellablePromise<void> {
  return $Call.ByID(2378997311, minutes)
}

export function SetAppLockTouchIDEnabled(enabled: boolean): $CancellablePromise<void> {
  return $Call.ByID(3693842441, enabled)
}

export function SetAutoUnlock(
  input: credential$0.SetAutoUnlockInput,
): $CancellablePromise<credential$0.CredentialVaultStatus> {
  return $Call.ByID(1335587145, input)
}

export function SetCloseToTray(enabled: boolean): $CancellablePromise<void> {
  return $Call.ByID(980049083, enabled)
}

export function SetDebugLoggingEnabled(enabled: boolean): $CancellablePromise<void> {
  return $Call.ByID(828941439, enabled)
}

export function SetDockerPanelEnabled(hostID: string, enabled: boolean): $CancellablePromise<void> {
  return $Call.ByID(3318709425, hostID, enabled)
}

export function SetPreference(key: string, value: string): $CancellablePromise<void> {
  return $Call.ByID(3964293223, key, value)
}

export function SetQuitConfirm(enabled: boolean): $CancellablePromise<void> {
  return $Call.ByID(131531863, enabled)
}

export function SetQuitConfirmLabels(
  title: string,
  message: string,
  confirmLabel: string,
  cancelLabel: string,
): $CancellablePromise<void> {
  return $Call.ByID(2786196596, title, message, confirmLabel, cancelLabel)
}

export function SetRemoteEntryPermissions(
  sessionID: string,
  remotePath: string,
  mode: string,
  uid: number,
  gid: number,
): $CancellablePromise<void> {
  return $Call.ByID(869198628, sessionID, remotePath, mode, uid, gid)
}

export function SetSessionHistoryArchived(
  ids: string[] | null,
  archived: boolean,
): $CancellablePromise<model$0.SessionHistoryBatchArchiveResult> {
  return $Call.ByID(2010384268, ids, archived)
}

export function SetSessionHistoryLimit(limit: number): $CancellablePromise<void> {
  return $Call.ByID(590286313, limit)
}

export function SetSettingsWindowEnabled(enabled: boolean): $CancellablePromise<void> {
  return $Call.ByID(2251456864, enabled)
}

export function SetTrayMenuState(state: $models.TrayMenuState): $CancellablePromise<void> {
  return $Call.ByID(304239774, state)
}

export function SettingsWindowVisible(): $CancellablePromise<boolean> {
  return $Call.ByID(575904647)
}

export function SetupMasterPassword(
  input: credential$0.SetupMasterPasswordInput,
): $CancellablePromise<credential$0.CredentialVaultStatus> {
  return $Call.ByID(2744218308, input)
}

export function ShowHostEditorWindow(hostID: string): $CancellablePromise<string> {
  return $Call.ByID(4271993420, hostID)
}

export function StartFullRecording(id: string): $CancellablePromise<model$0.TerminalRecording> {
  return $Call.ByID(2545149538, id)
}

export function StartRecording(id: string): $CancellablePromise<model$0.TerminalRecording> {
  return $Call.ByID(3896940013, id)
}

export function StopRecording(id: string): $CancellablePromise<model$0.TerminalRecording> {
  return $Call.ByID(430165493, id)
}

export function SubmitSessionAuthentication(
  id: string,
  input: session$0.AuthenticationInput,
): $CancellablePromise<void> {
  return $Call.ByID(3277168070, id, input)
}

export function SuppressCommandHistory(hostID: string, command: string): $CancellablePromise<number> {
  return $Call.ByID(796415126, hostID, command)
}

export function SyncLocalEdit(editID: string): $CancellablePromise<model$0.LocalEdit> {
  return $Call.ByID(3932343604, editID)
}

export function TestAIProvider(input: $models.AIProviderPreviewInput): $CancellablePromise<$models.AIChatResult> {
  return $Call.ByID(2276331201, input)
}

export function TestConnection(input: session$0.TestConnectionInput): $CancellablePromise<void> {
  return $Call.ByID(2652016690, input)
}

export function TestHostRoute(hostID: string): $CancellablePromise<model$0.RouteTestResult> {
  return $Call.ByID(2516836973, hostID)
}

export function TestHostRouteDraft(input: model$0.RouteDraftInput): $CancellablePromise<model$0.RouteTestResult> {
  return $Call.ByID(3825896580, input)
}

export function TranslateLanguages(): $CancellablePromise<string[] | null> {
  return $Call.ByID(3431599247)
}

export function TranslateText(
  text: string,
  target: string,
  provider: string,
  endpoint: string,
): $CancellablePromise<translate$0.Result> {
  return $Call.ByID(2416762651, text, target, provider, endpoint)
}

export function TrimSessionHistory(limit: number): $CancellablePromise<void> {
  return $Call.ByID(980861074, limit)
}

export function TrustSessionHostKey(id: string): $CancellablePromise<session$0.OpenSessionResult> {
  return $Call.ByID(3921634049, id)
}

export function TypeCredentialIntoSession(sessionID: string, credentialID: string): $CancellablePromise<void> {
  return $Call.ByID(1258321875, sessionID, credentialID)
}

export function UnlockAutomatically(): $CancellablePromise<credential$0.CredentialVaultStatus> {
  return $Call.ByID(3394488863)
}

export function UnlockWithCustomKey(key: string): $CancellablePromise<credential$0.CredentialVaultStatus> {
  return $Call.ByID(235759044, key)
}

export function UnlockWithPassword(password: string): $CancellablePromise<credential$0.CredentialVaultStatus> {
  return $Call.ByID(2221768555, password)
}

export function UpdateCheck(currentVersion: string): $CancellablePromise<update$0.Release> {
  return $Call.ByID(1240405081, currentVersion)
}

export function UpdateCredentialPromptRule(
  id: string,
  input: model$0.CredentialPromptRuleInput,
): $CancellablePromise<model$0.CredentialPromptRule> {
  return $Call.ByID(926788986, id, input)
}

export function UpdateDownload(): $CancellablePromise<void> {
  return $Call.ByID(751168547)
}

export function UpdateGroup(id: string, input: model$0.GroupInput): $CancellablePromise<model$0.Group> {
  return $Call.ByID(396655518, id, input)
}

export function UpdateHost(id: string, input: model$0.HostInput): $CancellablePromise<model$0.Host> {
  return $Call.ByID(30597537, id, input)
}

export function UpdateInstall(): $CancellablePromise<void> {
  return $Call.ByID(962770702)
}

export function UpdateProxy(id: string, input: model$0.ProxyInput): $CancellablePromise<model$0.ProxyRecord> {
  return $Call.ByID(2111554433, id, input)
}

export function UpdateSnippet(id: string, input: model$0.SnippetInput): $CancellablePromise<model$0.Snippet> {
  return $Call.ByID(1511989212, id, input)
}

export function UpdateStatus(): $CancellablePromise<update$0.Status> {
  return $Call.ByID(3158877089)
}

export function UpdateTerminalRecordingNote(id: string, note: string): $CancellablePromise<void> {
  return $Call.ByID(1569824722, id, note)
}

export function UploadDroppedFiles(id: string, directory: string): $CancellablePromise<sftp$0.DropUploadResult> {
  return $Call.ByID(4099579174, id, directory)
}

export function ValidateResourceBundleData(data: string | null, password: string): $CancellablePromise<boolean> {
  return $Call.ByID(3131037356, data, password)
}

export function VerifyAppLockPassword(password: string): $CancellablePromise<boolean> {
  return $Call.ByID(1772111560, password)
}

export function VerifyAppLockTouchID(): $CancellablePromise<boolean> {
  return $Call.ByID(2220322101)
}

export function VncCloseSession(sessionID: string): $CancellablePromise<void> {
  return $Call.ByID(945430683, sessionID)
}

export function VncHostProfile(hostID: string): $CancellablePromise<model$0.VncHostProfile> {
  return $Call.ByID(1291866134, hostID)
}

export function VncListSessions(): $CancellablePromise<model$0.VncSession[] | null> {
  return $Call.ByID(72056402)
}

export function VncOpenHost(hostID: string): $CancellablePromise<model$0.VncOpenResult> {
  return $Call.ByID(2704274809, hostID)
}

export function VncStreamTicket(sessionID: string): $CancellablePromise<model$0.VncStreamTicket> {
  return $Call.ByID(2982464867, sessionID)
}

export function WriteRemoteFileText(
  sessionID: string,
  remotePath: string,
  contentBase64: string,
  expectedHash: string,
): $CancellablePromise<model$0.RemoteFileWriteResult> {
  return $Call.ByID(1976634978, sessionID, remotePath, contentBase64, expectedHash)
}

export function WriteZmodemFile(token: string, offset: number, data: string | null): $CancellablePromise<void> {
  return $Call.ByID(602794903, token, offset, data)
}
