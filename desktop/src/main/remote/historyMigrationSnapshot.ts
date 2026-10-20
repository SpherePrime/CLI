import { shellQuote } from './security'

export function historySnapshotCommand(source: string, directory: string) {
  const python = [
    'import pathlib,sqlite3,sys',
    'source=sqlite3.connect(pathlib.Path(sys.argv[1]).as_uri()+"?mode=ro",uri=True,timeout=30)',
    'target=sqlite3.connect("prime.db",timeout=30)',
    'try:',
    ' source.backup(target,pages=256,sleep=0.05)',
    ' assert target.execute("PRAGMA quick_check").fetchall()==[("ok",)]',
    " assert target.execute(\"SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('sessions','messages')\").fetchone()[0]==2",
    'finally:',
    ' target.close()',
    ' source.close()'
  ].join('\n')
  return [
    'umask 077',
    `cd ${shellQuote(directory)} || exit 72`,
    'if command -v sqlite3 >/dev/null 2>&1; then',
    ` sqlite3 -readonly ${shellQuote(source)} '.timeout 30000' '.backup prime.db' >/dev/null 2>&1 || exit 72`,
    ` result=$(sqlite3 -readonly prime.db ${shellQuote("PRAGMA quick_check; SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('sessions','messages');")} 2>/dev/null) || exit 72`,
    ' [ "$result" = "$(printf \'ok\\n2\')" ] || exit 72',
    'elif command -v python3 >/dev/null 2>&1; then',
    ` python3 -c ${shellQuote(python)} ${shellQuote(source)} >/dev/null 2>&1 || exit 72`,
    'else',
    ' exit 71',
    'fi'
  ].join('\n')
}
