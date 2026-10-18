import React, { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { StartupScreen } from '../../src/renderer/src/components/StartupScreen'
import '../../src/renderer/src/styles/global.css'
import '../../src/renderer/src/styles/brand.css'
function Intro() {
  const [ready, setReady] = useState(false)
  useEffect(() => {
    const done = () => setReady(true)
    window.addEventListener('fixture:ready', done)
    return () => window.removeEventListener('fixture:ready', done)
  }, [])
  return (
    <>
      <StartupScreen ready={ready} failed={false} />
      {ready && <p>Chat ready</p>}
    </>
  )
}
createRoot(document.getElementById('root')!).render(<Intro />)
