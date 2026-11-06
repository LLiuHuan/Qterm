import {
  ReadAccelerators as readAcceleratorsBinding,
  ReadProcesses as readProcessesBinding,
  RunProcessAction as runProcessActionBinding,
} from '../../bindings/changeme/internal/app/service'
import { createRuntimeGateway } from './runtime'

export interface ProcessInfo {
  pid: number
  ppid: number
  user: string
  cpu: number
  memory: number
  rssKb: number
  state: string
  command: string
  startTime: string
  tty: string
  priority: number
  nice: number
  elapsed: string
  cpuTime: string
  vszKb: number
  metricsAvailable: boolean
}

export type ProcessAction = 'terminate' | 'kill'

export interface ProcessSnapshot {
  processes: ProcessInfo[]
  updatedAt: number
}

export type AcceleratorKind = 'gpu' | 'npu'

export interface AcceleratorInfo {
  index: number
  name: string
  kind: AcceleratorKind
  utilization: number
  memoryUsedMb: number
  memoryTotalMb: number
  temperatureC: number
}

export interface AcceleratorSnapshot {
  devices: AcceleratorInfo[]
  updatedAt: number
}

export interface MonitorGateway {
  readonly persistent: boolean
  processes(sessionId: string): Promise<ProcessSnapshot>
  accelerators(sessionId: string): Promise<AcceleratorSnapshot>
  processAction(sessionId: string, pid: number, expectedStartTime: string, action: ProcessAction): Promise<void>
}

class WailsMonitorGateway implements MonitorGateway {
  readonly persistent = true

  async processes(sessionId: string): Promise<ProcessSnapshot> {
    const snapshot = await readProcessesBinding(sessionId)
    return { processes: snapshot.processes ?? [], updatedAt: snapshot.updatedAt ?? Date.now() }
  }

  async accelerators(sessionId: string): Promise<AcceleratorSnapshot> {
    const snapshot = await readAcceleratorsBinding(sessionId)
    return { devices: (snapshot.devices ?? []) as AcceleratorInfo[], updatedAt: snapshot.updatedAt ?? Date.now() }
  }

  async processAction(sessionId: string, pid: number, expectedStartTime: string, action: ProcessAction): Promise<void> {
    await runProcessActionBinding(sessionId, pid, expectedStartTime, action)
  }
}

class MemoryMonitorGateway implements MonitorGateway {
  readonly persistent = false

  async processes(): Promise<ProcessSnapshot> {
    return {
      updatedAt: Date.now(),
      processes: [
        {
          pid: 1,
          ppid: 0,
          user: 'root',
          cpu: 0.1,
          memory: 0.2,
          rssKb: 12_000,
          state: 'Ss',
          command: '/sbin/init',
          startTime: '',
          tty: '?',
          priority: 20,
          nice: 0,
          elapsed: '00:12:08',
          cpuTime: '00:00:01',
          vszKb: 120_000,
          metricsAvailable: true,
        },
        {
          pid: 4242,
          ppid: 1,
          user: 'me',
          cpu: 12.5,
          memory: 3.4,
          rssKb: 220_000,
          state: 'Sl',
          command: 'node vite.mjs',
          startTime: '',
          tty: 'pts/0',
          priority: 20,
          nice: 0,
          elapsed: '00:04:15',
          cpuTime: '00:00:32',
          vszKb: 480_000,
          metricsAvailable: true,
        },
        {
          pid: 5150,
          ppid: 4242,
          user: 'me',
          cpu: 3.1,
          memory: 1.2,
          rssKb: 88_000,
          state: 'S',
          command: 'go run ./cmd',
          startTime: '',
          tty: 'pts/0',
          priority: 20,
          nice: 0,
          elapsed: '00:02:03',
          cpuTime: '00:00:12',
          vszKb: 260_000,
          metricsAvailable: true,
        },
      ],
    }
  }

  async accelerators(): Promise<AcceleratorSnapshot> {
    return {
      updatedAt: Date.now(),
      devices: [
        {
          index: 0,
          name: 'Preview GPU',
          kind: 'gpu',
          utilization: 42,
          memoryUsedMb: 2048,
          memoryTotalMb: 8192,
          temperatureC: 61,
        },
      ],
    }
  }

  async processAction(): Promise<void> {}
}

export function createMonitorGateway(): MonitorGateway {
  return createRuntimeGateway<MonitorGateway>(
    () => new WailsMonitorGateway(),
    () => new MemoryMonitorGateway(),
  )
}
