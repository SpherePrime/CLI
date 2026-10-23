$exts = @('.go','.ts','.tsx','.js','.jsx','.css','.html','.sh','.ps1','.bat','.sql','.yaml','.yml','.json','.md','.proto')
$res = @{}
$files = Get-ChildItem -Path .\ -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object { $exts -contains $_.Extension -and $_.FullName -notmatch '\\\.git\\' -and $_.FullName -notmatch '\\vendordeps\\' }
foreach ($f in $files) {
    $n = [System.IO.File]::ReadAllLines($f.FullName).Count
    if (-not $res.ContainsKey($f.Extension)) { $res[$f.Extension] = 0 }
    $res[$f.Extension] += $n
}
$res.GetEnumerator() | Sort-Object Value -Descending | ForEach-Object { '{0,-8} {1}' -f $_.Key, $_.Value }
'TOTAL: ' + (($res.Values | Measure-Object -Sum).Sum)
