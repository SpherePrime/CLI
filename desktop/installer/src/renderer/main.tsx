import React from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource/inter/400.css'
import '@fontsource/inter/500.css'
import '@fontsource/inter/600.css'
import '@fontsource/inter/700.css'
import './setupBridge'
import { SetupApp } from './App'
import './style.css'

createRoot(document.getElementById('root')!).render(<SetupApp />)
