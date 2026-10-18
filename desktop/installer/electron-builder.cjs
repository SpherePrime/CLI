const { version } = require('./package.json')
module.exports = {
  appId: 'com.sphereprime.desktop.setup',
  productName: 'Prime Desktop Installer',
  electronVersion: require('../package.json').devDependencies.electron,
  npmRebuild: false,
  directories: { output: `../release/${version}`, buildResources: '../build' },
  files: ['out/**/*'],
  extraResources: [
    { from: 'payload', to: 'payload' },
    { from: '../build/icon.png', to: 'icon.png' }
  ],
  win: {
    executableName: 'PrimeSetup',
    icon: '../build/icon.ico',
    target: [{ target: 'portable', arch: ['x64'] }]
  },
  portable: {
    artifactName: 'PrimeSetup-${version}.${ext}',
    splashImage: null,
    requestExecutionLevel: 'user'
  }
}
