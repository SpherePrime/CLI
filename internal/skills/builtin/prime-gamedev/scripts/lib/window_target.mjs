import { spawnSync } from 'node:child_process'

function runPowerShell(script, envExtra = {}) {
  const encoded = Buffer.from(script, 'utf16le').toString('base64')
  return spawnSync(
    'powershell',
    ['-NoProfile', '-NonInteractive', '-EncodedCommand', encoded],
    {
      encoding: 'utf8',
      windowsHide: true,
      env: { ...process.env, ...envExtra }
    }
  )
}

function getWindowsDescendantPids(rootPid) {
  if (!rootPid || rootPid <= 0) {
    return []
  }
  const script = `
$root = [int]$env:PRIME_ROOT_PID
$procs = Get-CimInstance Win32_Process | Select-Object ProcessId, ParentProcessId
$byParent = @{}
foreach ($p in $procs) {
  $ppid = [int]$p.ParentProcessId
  if (-not $byParent.ContainsKey($ppid)) { $byParent[$ppid] = New-Object System.Collections.Generic.List[int] }
  $byParent[$ppid].Add([int]$p.ProcessId)
}
$seen = New-Object System.Collections.Generic.HashSet[int]
$queue = New-Object System.Collections.Generic.Queue[int]
[void]$seen.Add($root)
$queue.Enqueue($root)
while ($queue.Count -gt 0) {
  $curr = $queue.Dequeue()
  if ($byParent.ContainsKey($curr)) {
    foreach ($child in $byParent[$curr]) {
      if ($seen.Add($child)) { $queue.Enqueue($child) }
    }
  }
}
$seen | ConvertTo-Json -Compress
`
  const res = runPowerShell(script, { PRIME_ROOT_PID: String(rootPid) })
  if (res.error || res.status !== 0 || !res.stdout.trim()) {
    return [rootPid]
  }
  try {
    const parsed = JSON.parse(res.stdout.trim())
    const list = Array.isArray(parsed) ? parsed : [parsed]
    return list.map((n) => Number(n)).filter((n) => Number.isFinite(n) && n > 0)
  } catch {
    return [rootPid]
  }
}

function probeWindowWindows({ pids, titleQuery, focus }) {
  const script = `
Add-Type -TypeDefinition @"
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;

public static class PrimeWinProbe {
  [StructLayout(LayoutKind.Sequential)]
  public struct RECT { public int Left; public int Top; public int Right; public int Bottom; }

  public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);

  [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
  [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool ShowWindowAsync(IntPtr hWnd, int nCmdShow);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint lpdwProcessId);
  [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetWindowTextLength(IntPtr hWnd);
  [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetWindowText(IntPtr hWnd, StringBuilder lpString, int nMaxCount);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
  [DllImport("user32.dll")] public static extern int GetSystemMetrics(int nIndex);
  [DllImport("dwmapi.dll")] public static extern int DwmGetWindowAttribute(IntPtr hwnd, int dwAttribute, out RECT pvAttribute, int cbAttribute);

  public static RECT BestRect(IntPtr hWnd) {
    RECT r;
    if (DwmGetWindowAttribute(hWnd, 9, out r, Marshal.SizeOf(typeof(RECT))) == 0) {
      if (r.Right > r.Left && r.Bottom > r.Top) return r;
    }
    GetWindowRect(hWnd, out r);
    return r;
  }
}
"@
[void][PrimeWinProbe]::SetProcessDPIAware()
$pidSet = New-Object System.Collections.Generic.HashSet[uint32]
if ($env:PRIME_TARGET_PIDS) {
  foreach ($part in $env:PRIME_TARGET_PIDS.Split(',')) {
    $v = 0
    if ([uint32]::TryParse($part.Trim(), [ref]$v)) { [void]$pidSet.Add($v) }
  }
}
$titleNeedle = if ($env:PRIME_TARGET_TITLE) { $env:PRIME_TARGET_TITLE.Trim().ToLowerInvariant() } else { "" }
$shouldFocus = $env:PRIME_FOCUS_WINDOW -eq "1"
$matches = New-Object System.Collections.ArrayList

$cb = [PrimeWinProbe+EnumWindowsProc]{
  param($hWnd, $lParam)
  if (-not [PrimeWinProbe]::IsWindowVisible($hWnd)) { return $true }
  if ([PrimeWinProbe]::IsIconic($hWnd)) { return $true }
  $len = [PrimeWinProbe]::GetWindowTextLength($hWnd)
  $sb = New-Object System.Text.StringBuilder ($len + 2)
  [void][PrimeWinProbe]::GetWindowText($hWnd, $sb, $sb.Capacity)
  $title = $sb.ToString()
  $wpid = [uint32]0
  [void][PrimeWinProbe]::GetWindowThreadProcessId($hWnd, [ref]$wpid)

  $pidHit = $pidSet.Count -gt 0 -and $pidSet.Contains($wpid)
  $titleHit = $titleNeedle.Length -gt 0 -and $title.ToLowerInvariant().Contains($titleNeedle)
  if (-not $pidHit -and -not $titleHit) { return $true }

  $rect = [PrimeWinProbe]::BestRect($hWnd)
  $w = $rect.Right - $rect.Left
  $h = $rect.Bottom - $rect.Top
  if ($w -lt 80 -or $h -lt 80) { return $true }

  $score = ($w * $h)
  if ($pidHit) { $score += 100000000 }
  if ($titleHit) { $score += 50000000 }

  [void]$matches.Add([pscustomobject]@{
    hwnd = ("0x{0:X}" -f $hWnd.ToInt64())
    hwndDec = $hWnd.ToInt64()
    pid = [int]$wpid
    title = $title
    x = $rect.Left
    y = $rect.Top
    width = $w
    height = $h
    score = $score
  })
  return $true
}

[void][PrimeWinProbe]::EnumWindows($cb, [IntPtr]::Zero)
if ($matches.Count -eq 0) { exit 0 }
$best = $matches | Sort-Object -Property score -Descending | Select-Object -First 1
if ($shouldFocus) {
  $ptr = [IntPtr]::new([int64]$best.hwndDec)
  [void][PrimeWinProbe]::ShowWindowAsync($ptr, 5)
  [void][PrimeWinProbe]::SetForegroundWindow($ptr)
  Start-Sleep -Milliseconds 120
  $rect2 = [PrimeWinProbe]::BestRect($ptr)
  $best.x = $rect2.Left
  $best.y = $rect2.Top
  $best.width = $rect2.Right - $rect2.Left
  $best.height = $rect2.Bottom - $rect2.Top
}
$vLeft = [PrimeWinProbe]::GetSystemMetrics(76)
$vTop = [PrimeWinProbe]::GetSystemMetrics(77)
$vWidth = [PrimeWinProbe]::GetSystemMetrics(78)
$vHeight = [PrimeWinProbe]::GetSystemMetrics(79)
[pscustomobject]@{
  hwnd = $best.hwnd
  pid = $best.pid
  title = $best.title
  x = $best.x
  y = $best.y
  width = $best.width
  height = $best.height
  screen = @{ left = $vLeft; top = $vTop; width = $vWidth; height = $vHeight }
} | ConvertTo-Json -Compress
`
  const res = runPowerShell(script, {
    PRIME_TARGET_PIDS: (pids || []).join(','),
    PRIME_TARGET_TITLE: titleQuery || '',
    PRIME_FOCUS_WINDOW: focus ? '1' : '0'
  })
  if (res.error || res.status !== 0 || !res.stdout.trim()) {
    return null
  }
  try {
    return JSON.parse(res.stdout.trim())
  } catch {
    return null
  }
}

function probeWindowLinux({ pids, titleQuery, focus }) {
  const candidates = []
  for (const pid of pids || []) {
    const res = spawnSync('xdotool', ['search', '--onlyvisible', '--pid', String(pid)], {
      encoding: 'utf8'
    })
    if (!res.error && res.status === 0 && res.stdout.trim()) {
      for (const line of res.stdout.trim().split(/\s+/)) {
        candidates.push(line)
      }
    }
  }
  if (candidates.length === 0 && titleQuery) {
    const res = spawnSync('xdotool', ['search', '--onlyvisible', '--name', titleQuery], {
      encoding: 'utf8'
    })
    if (!res.error && res.status === 0 && res.stdout.trim()) {
      for (const line of res.stdout.trim().split(/\s+/)) {
        candidates.push(line)
      }
    }
  }
  let best = null
  for (const wid of candidates) {
    const info = spawnSync('xwininfo', ['-id', wid], { encoding: 'utf8' })
    if (info.error || info.status !== 0) {
      continue
    }
    const text = info.stdout
    const xMatch = text.match(/Absolute upper-left X:\s+(-?\d+)/)
    const yMatch = text.match(/Absolute upper-left Y:\s+(-?\d+)/)
    const wMatch = text.match(/Width:\s+(\d+)/)
    const hMatch = text.match(/Height:\s+(\d+)/)
    const titleMatch = text.match(/xwininfo:\s+Window id:\s+\S+\s+"([^"]*)"/)
    if (!xMatch || !yMatch || !wMatch || !hMatch) {
      continue
    }
    const width = Number(wMatch[1])
    const height = Number(hMatch[1])
    if (width < 80 || height < 80) {
      continue
    }
    const candidate = {
      wid,
      title: titleMatch?.[1] || titleQuery || '',
      x: Number(xMatch[1]),
      y: Number(yMatch[1]),
      width,
      height,
      area: width * height
    }
    if (!best || candidate.area > best.area) {
      best = candidate
    }
  }
  if (!best) {
    return null
  }
  if (focus) {
    spawnSync('xdotool', ['windowactivate', '--sync', best.wid], { encoding: 'utf8' })
  }
  return best
}

function probeWindowMac({ pids, titleQuery, focus }) {
  const jxa = `
ObjC.import('CoreGraphics');
ObjC.import('AppKit');
const pidStr = $.NSProcessInfo.processInfo.environment.objectForKey('PRIME_TARGET_PIDS').js || '';
const pidSet = new Set(pidStr.split(',').map(s => Number(s.trim())).filter(n => n > 0));
const titleNeedle = ($.NSProcessInfo.processInfo.environment.objectForKey('PRIME_TARGET_TITLE').js || '').trim().toLowerCase();
const list = ObjC.castRefToObject($.CGWindowListCopyWindowInfo($.kCGWindowListOptionOnScreenOnly | $.kCGWindowListExcludeDesktopElements, 0));
let best = null;
for (let i = 0; i < list.count; i++) {
  const w = list.objectAtIndex(i);
  const pid = Number(w.objectForKey('kCGWindowOwnerPID').js || 0);
  const layer = Number(w.objectForKey('kCGWindowLayer').js || 0);
  if (layer !== 0) continue;
  const owner = String(w.objectForKey('kCGWindowOwnerName') ? w.objectForKey('kCGWindowOwnerName').js : '');
  const title = String(w.objectForKey('kCGWindowName') ? w.objectForKey('kCGWindowName').js : '');
  const bounds = w.objectForKey('kCGWindowBounds');
  if (!bounds) continue;
  const x = Number(bounds.objectForKey('X').js || 0);
  const y = Number(bounds.objectForKey('Y').js || 0);
  const width = Number(bounds.objectForKey('Width').js || 0);
  const height = Number(bounds.objectForKey('Height').js || 0);
  if (width < 80 || height < 80) continue;
  const pidHit = pidSet.size > 0 && pidSet.has(pid);
  const titleHit = titleNeedle.length > 0 && (title.toLowerCase().includes(titleNeedle) || owner.toLowerCase().includes(titleNeedle));
  if (!pidHit && !titleHit) continue;
  const score = width * height + (pidHit ? 100000000 : 0) + (titleHit ? 50000000 : 0);
  if (!best || score > best.score) {
    best = { pid, title: title || owner, x, y, width, height, score };
  }
}
if (best) {
  if ($.NSProcessInfo.processInfo.environment.objectForKey('PRIME_FOCUS_WINDOW').js === '1') {
    const app = $.NSRunningApplication.runningApplicationWithProcessIdentifier(best.pid);
    if (app && app.js) app.activateWithOptions($.NSApplicationActivateIgnoringOtherApps);
  }
  JSON.stringify(best);
}
`
  const res = spawnSync('osascript', ['-l', 'JavaScript', '-e', jxa], {
    encoding: 'utf8',
    env: {
      ...process.env,
      PRIME_TARGET_PIDS: (pids || []).join(','),
      PRIME_TARGET_TITLE: titleQuery || '',
      PRIME_FOCUS_WINDOW: focus ? '1' : '0'
    }
  })
  if (res.error || res.status !== 0 || !res.stdout.trim()) {
    return null
  }
  try {
    return JSON.parse(res.stdout.trim())
  } catch {
    return null
  }
}

export function normalizeCaptureRegion(target) {
  let x = Math.round(Number(target.x) || 0)
  let y = Math.round(Number(target.y) || 0)
  let width = Math.round(Number(target.width) || 0)
  let height = Math.round(Number(target.height) || 0)

  if (target.screen && Number(target.screen.width) > 0 && Number(target.screen.height) > 0) {
    const minX = Number(target.screen.left) || 0
    const minY = Number(target.screen.top) || 0
    const maxX = minX + Number(target.screen.width)
    const maxY = minY + Number(target.screen.height)
    const clampedRight = Math.min(maxX, x + width)
    const clampedBottom = Math.min(maxY, y + height)
    x = Math.max(minX, x)
    y = Math.max(minY, y)
    width = Math.max(2, clampedRight - x)
    height = Math.max(2, clampedBottom - y)
  }

  width = Math.max(2, width - (width % 2))
  height = Math.max(2, height - (height % 2))
  return { x, y, width, height }
}

export async function waitForTargetWindow({
  pid,
  titleQuery,
  timeoutMs = 15000,
  pollIntervalMs = 250,
  focus = true
}) {
  const deadline = Date.now() + Math.max(500, timeoutMs)
  while (Date.now() <= deadline) {
    const pids =
      process.platform === 'win32'
        ? getWindowsDescendantPids(pid)
        : pid
          ? [pid]
          : []

    let found = null
    if (process.platform === 'win32') {
      found = probeWindowWindows({ pids, titleQuery, focus })
    } else if (process.platform === 'darwin') {
      found = probeWindowMac({ pids, titleQuery, focus })
    } else {
      found = probeWindowLinux({ pids, titleQuery, focus })
    }

    if (found && found.width >= 80 && found.height >= 80) {
      return {
        ...found,
        region: normalizeCaptureRegion(found)
      }
    }
    await new Promise((resolvePromise) => setTimeout(resolvePromise, pollIntervalMs))
  }
  return null
}

export function buildWindowFfmpegInputArgs({
  windowTarget,
  recordFps = 30,
  grabMode = 'window',
  screen = '1:none'
}) {
  const fps = String(Number(recordFps) || 30)
  const region = windowTarget?.region || normalizeCaptureRegion(windowTarget || {})

  if (process.platform === 'win32') {
    if (grabMode !== 'region' && grabMode !== 'title' && windowTarget?.hwnd) {
      return {
        inputArgs: ['-f', 'gdigrab', '-framerate', fps, '-i', `hwnd=${windowTarget.hwnd}`],
        filterArgs: ['-vf', 'pad=ceil(iw/2)*2:ceil(ih/2)*2'],
        mode: 'gdigrab-hwnd'
      }
    }
    if (grabMode === 'title' && windowTarget?.title) {
      return {
        inputArgs: ['-f', 'gdigrab', '-framerate', fps, '-i', `title=${windowTarget.title}`],
        filterArgs: ['-vf', 'pad=ceil(iw/2)*2:ceil(ih/2)*2'],
        mode: 'gdigrab-title'
      }
    }
    return {
      inputArgs: [
        '-f',
        'gdigrab',
        '-framerate',
        fps,
        '-offset_x',
        String(region.x),
        '-offset_y',
        String(region.y),
        '-video_size',
        `${region.width}x${region.height}`,
        '-i',
        'desktop'
      ],
      filterArgs: [],
      mode: 'gdigrab-window-region'
    }
  }

  if (process.platform === 'darwin') {
    return {
      inputArgs: ['-f', 'avfoundation', '-framerate', fps, '-i', screen],
      filterArgs: ['-vf', `crop=${region.width}:${region.height}:${region.x}:${region.y}`],
      mode: 'avfoundation-window-crop'
    }
  }

  const display = process.env.DISPLAY || ':0.0'
  return {
    inputArgs: [
      '-f',
      'x11grab',
      '-framerate',
      fps,
      '-video_size',
      `${region.width}x${region.height}`,
      '-i',
      `${display}+${region.x},${region.y}`
    ],
    filterArgs: [],
    mode: 'x11grab-window-region'
  }
}
