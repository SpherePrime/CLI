const { chmod, rename, writeFile } = require('node:fs/promises')
const { join } = require('node:path')

exports.default = async function wrapLinuxExecutable(context) {
  if (context.electronPlatformName !== 'linux') return
  const name = context.packager.executableName
  const binary = join(context.appOutDir, name)
  await rename(binary, `${binary}.bin`)
  await writeFile(
    binary,
    `#!/bin/sh\nexec "$(dirname "$(readlink -f "$0")")/${name}.bin" --no-sandbox "$@"\n`
  )
  await chmod(binary, 0o755)
}
