import { RememberHostPassword } from '../../bindings/changeme/internal/app/service'

export async function rememberHostPassword(hostId: string, password: string): Promise<void> {
  if (!hostId || !password) {
    return
  }
  await RememberHostPassword(hostId, password)
}
