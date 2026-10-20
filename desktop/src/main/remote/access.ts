import type { RemoteManager } from './manager'
import type { RemoteConnection } from './connection'
import type { WorkbenchTool } from '../workbench/tools'
import { schema, string, stringProperty, text } from './executionSupport'

type Options = {
  manager(): RemoteManager
  byWorkspace(workspaceId: string): RemoteConnection | undefined
}

export class SshAccess {
  private pending = new Map<string, Promise<RemoteConnection>>()
  constructor(private options: Options) {}
  profiles() {
    return this.options
      .manager()
      .list()
      .profiles.map((profile) => ({
        id: profile.id,
        name: profile.name,
        host: profile.host,
        port: profile.port,
        user: profile.user,
        root: profile.root,
        auth: profile.auth,
        status: profile.status,
        hasSavedCredentials: Boolean(profile.hasSavedCredentials),
        ...(profile.error ? { error: profile.error } : {}),
        ...(profile.challenge ? { requiresHostVerification: true } : {})
      }))
  }
  async resolve(workspaceId: string, profileId?: string): Promise<RemoteConnection> {
    if (!profileId) {
      const connection = this.options.byWorkspace(workspaceId)
      if (!connection?.root)
        throw new Error('SSH workspace is disconnected; select profileId using ssh_profiles')
      return connection
    }
    if (!this.profiles().some((profile) => profile.id === profileId))
      throw new Error('Unknown SSH profile')
    const manager = this.options.manager()
    try {
      return manager.connection(profileId)
    } catch {}
    const pending = this.pending.get(profileId)
    if (pending) return pending
    const operation = (async () => {
      await manager.connect(profileId, {}, { prepareWorkspace: false })
      const profile = this.profiles().find((profile) => profile.id === profileId)
      if (profile?.requiresHostVerification)
        throw new Error('Verify the SSH host fingerprint in the SSH panel before connecting')
      return manager.connection(profileId)
    })()
    this.pending.set(profileId, operation)
    try {
      return await operation
    } finally {
      this.pending.delete(profileId)
    }
  }
  async disconnect(profileId: string) {
    await this.options.manager().disconnect(profileId)
    return this.profiles().find((profile) => profile.id === profileId)
  }
}

export function sshProfileTools(access: SshAccess): WorkbenchTool[] {
  return [
    {
      name: 'ssh_profiles',
      description:
        'List saved SSH servers and their connection status from any chat. No project or separate SSH chat is required. Credentials and private keys are never returned. Use the profile id in SSH command, file and upload tools.',
      inputSchema: schema({}, []),
      async run() {
        return text({ profiles: access.profiles() })
      }
    },
    {
      name: 'ssh_connect',
      description:
        'Connect a saved SSH profile from the current chat using securely stored credentials or SSH agent. Does not create or switch projects or chats. Unknown host fingerprints must be confirmed by the user in the SSH panel; never ask for passwords in chat.',
      inputSchema: schema({ profileId: stringProperty }, ['profileId']),
      async run(args, context) {
        if (!context.workspaceId) throw new Error('Workspace binding is required')
        const id = string(args, 'profileId')
        await access.resolve(context.workspaceId, id)
        return text(access.profiles().find((profile) => profile.id === id))
      }
    },
    {
      name: 'ssh_disconnect',
      description:
        'Disconnect a saved SSH server. This closes its active connection, including any remote workspace using it. Use only when requested.',
      inputSchema: schema({ profileId: stringProperty }, ['profileId']),
      async run(args) {
        return text(await access.disconnect(string(args, 'profileId')))
      }
    }
  ]
}
