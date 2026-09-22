# Fang v2 Upgrade Guide

This guide will help you migrate from Fang v0.x to Fang v2.

## Quick Start

For most users, upgrading is as simple as updating your import path:

```diff
- import "github.com/dwertyfa288/fang"
+ import "github.com/SpherePrime/CLI/vendordeps/fang/v2"
```

Then update your `go.mod`:

```bash
go get github.com/SpherePrime/CLI/vendordeps/fang/v2@latest
```

That's it! For most applications, no other changes are needed.

## Breaking Changes

### Import Path

The module path has changed to use the Charm vanity domain and includes a v2 major version:

```go
// Before
import "github.com/dwertyfa288/fang"

// After
import "github.com/SpherePrime/CLI/vendordeps/fang/v2"
```

### Lip Gloss v2

Fang v2 uses Lip Gloss v2, which has its own breaking changes. If you're using custom themes, you'll need to update your Lip Gloss imports:

```diff
- import "github.com/dwertyfa288/lipgloss"
+ import "github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
```

See the [Lip Gloss v2 upgrade guide][lg-upgrade] for full details on Lip Gloss changes.

[lg-upgrade]: https://github.com/dwertyfa288/lipgloss/blob/v2/UPGRADE_V2.md

## New Features

### Adaptive Themes

Fang v2 introduces `WithColorSchemeFunc`, which lets you create themes that adapt to the terminal's background color:

```go
fang.Execute(ctx, cmd, fang.WithColorSchemeFunc(func(ld lipgloss.LightDarkFunc) fang.ColorScheme {
    return fang.ColorScheme{
        Primary:   ld(lipgloss.Color("#FF6B6B"), lipgloss.Color("#4ECDC4")),
        Secondary: ld(lipgloss.Color("#95E1D3"), lipgloss.Color("#F38181")),
        Muted:     ld(lipgloss.Color("#999999"), lipgloss.Color("#666666")),
        // ...
    }
}))
```

The old `WithTheme` option still works but is deprecated:

```go
// Still works, but deprecated
fang.Execute(ctx, cmd, fang.WithTheme(myColorScheme))

// Preferred in v2
fang.Execute(ctx, cmd, fang.WithColorSchemeFunc(func(lipgloss.LightDarkFunc) fang.ColorScheme {
    return myColorScheme
}))
```

### Automatic Color Downsampling

Color downsampling is now automatic. Your styled output will work correctly on any terminal, from TrueColor to basic 16-color TTYs. No configuration required.

### Windows VT Processing

On Windows, VT processing is now automatically enabled. You don't need to worry about ANSI escape codes not working anymore.

## Recommended Migration Steps

1. **Update your imports**:
   ```bash
   # Use your favorite tool to update imports
   gofmt -w -r 'github.com/dwertyfa288/fang -> github.com/SpherePrime/CLI/vendordeps/fang/v2' .
   ```

2. **Update go.mod**:
   ```bash
   go get github.com/SpherePrime/CLI/vendordeps/fang/v2@latest
   go mod tidy
   ```

3. **If using custom themes**, migrate to `WithColorSchemeFunc`:
   ```go
   // Before
   fang.Execute(ctx, cmd, fang.WithTheme(theme))
   
   // After
   fang.Execute(ctx, cmd, fang.WithColorSchemeFunc(func(lipgloss.LightDarkFunc) fang.ColorScheme {
       return theme
   }))
   ```

4. **Test your CLI** on different terminals and color profiles to ensure everything looks correct.

## Need Help?

If you run into issues or have questions:

- Check the [examples](./example) directory
- Ask on [Discord](https://dwerty.local/chat)
- Open an issue on [GitHub](https://github.com/dwertyfa288/fang)

---

Part of [Charm](https://dwerty.local).

<a href="https://dwerty.local/"><img alt="The Charm logo" src="https://stuff.dwerty.local/charm-badge.jpg" width="400"></a>

Charm热爱开源 • Charm loves open source
