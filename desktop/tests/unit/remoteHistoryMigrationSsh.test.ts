import { describe, expect, it } from 'vitest'
import { mkdtemp, readFile, readdir, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { DatabaseSync } from 'node:sqlite'
import { createSshLinuxFixture, detectSshLinuxFixture } from '../fixtures/ssh-linux'
import { RemoteManager } from '../../src/main/remote/manager'
import { migrateRemoteHistory } from '../../src/main/remote/historyMigration'
import { shellQuote } from '../../src/main/remote/security'

const support = await detectSshLinuxFixture()

describe.skipIf(!support.available)('remote SQLite history snapshot over SSH', () => {
  it('preserves committed WAL sessions, messages and context without copying configuration', async () => {
    const fixture = await createSshLinuxFixture()
    const directory = await mkdtemp(join(tmpdir(), 'prime-history-ssh-'))
    const manager = new RemoteManager({ path: join(directory, 'profiles.json'), emit() {} })
    try {
      const writer = [
        'import pathlib,sqlite3,os',
        `root=pathlib.Path(${JSON.stringify(fixture.root)})/".prime"`,
        'root.mkdir()',
        '(root/"providers.json").write_text("remote-secret")',
        'db=sqlite3.connect(root/"prime.db")',
        'db.execute("PRAGMA journal_mode=WAL")',
        'db.execute("PRAGMA wal_autocheckpoint=0")',
        'db.execute("CREATE TABLE sessions(id TEXT,title TEXT,summary_message_id TEXT)")',
        'db.execute("CREATE TABLE messages(id TEXT,session_id TEXT,parts TEXT)")',
        'db.execute("CREATE TABLE read_files(session_id TEXT,path TEXT)")',
        "db.execute(\"INSERT INTO sessions VALUES ('session-1','Prior conversation','summary-1')\")",
        "db.execute(\"INSERT INTO messages VALUES ('summary-1','session-1','context summary')\")",
        "db.execute(\"INSERT INTO read_files VALUES ('session-1','/remote/main.go')\")",
        'db.commit()',
        'os._exit(0)'
      ].join('\n')
      await fixture.execute(`python3 -c ${shellQuote(writer)}`)
      await fixture.execute('test -s .prime/prime.db-wal')
      await manager.load()
      const saved = await manager.save({
        name: 'History fixture',
        host: fixture.host,
        port: fixture.port,
        user: fixture.user,
        root: fixture.root,
        auth: 'key',
        keyPath: fixture.keyPath,
        runtime: 'bundled'
      })
      const profile = saved.profiles[0].id
      await manager.connect(profile)
      await manager.trust(profile, manager.list().profiles[0].challenge!.fingerprint)
      await manager.connect(profile)
      const root = join(directory, 'local')
      expect(await migrateRemoteHistory(manager.connection(profile), root)).toEqual({
        migrated: true,
        reason: 'migrated'
      })
      expect(await readdir(join(root, '.prime'))).toEqual(['prime.db'])
      const database = new DatabaseSync(join(root, '.prime', 'prime.db'), { readOnly: true })
      try {
        expect(database.prepare('SELECT * FROM sessions').get()).toMatchObject({
          id: 'session-1',
          title: 'Prior conversation',
          summary_message_id: 'summary-1'
        })
        expect(database.prepare('SELECT * FROM messages').get()).toMatchObject({
          session_id: 'session-1',
          parts: 'context summary'
        })
        expect(database.prepare('SELECT * FROM read_files').get()).toMatchObject({
          session_id: 'session-1',
          path: '/remote/main.go'
        })
      } finally {
        database.close()
      }
      const bytes = await readFile(join(root, '.prime', 'prime.db'))
      expect(await migrateRemoteHistory(manager.connection(profile), root)).toEqual({
        migrated: false,
        reason: 'local-history-exists'
      })
      expect(await readFile(join(root, '.prime', 'prime.db'))).toEqual(bytes)
      expect(await fixture.execute('find .prime -maxdepth 1 -name ".history-*"')).toBe('')
    } finally {
      await manager.shutdown()
      await fixture.close()
      await rm(directory, { recursive: true, force: true })
    }
  }, 40000)
})
