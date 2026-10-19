import React from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './app/App'
import '@fontsource/inter/400.css'
import '@fontsource/inter/500.css'
import '@fontsource/inter/600.css'
import '@fontsource/inter/700.css'
import '@fontsource/ibm-plex-sans/400.css'
import '@fontsource/ibm-plex-sans/500.css'
import '@fontsource/ibm-plex-sans/600.css'
import '@fontsource/ibm-plex-sans/cyrillic-400.css'
import '@fontsource/ibm-plex-sans/cyrillic-500.css'
import '@fontsource/jetbrains-mono/400.css'
import './styles/global.css'
import './styles/brand.css'
import './styles/composer.css'
import './styles/files.css'
import './styles/tasks.css'
import './styles/browser.css'
import './styles/motion.css'
import './styles/welcome.css'
import './styles/navigation.css'
import './styles/workbench.css'
import './styles/context.css'
import './styles/polish.css'

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
