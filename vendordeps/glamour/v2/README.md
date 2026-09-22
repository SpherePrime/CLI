# Glamour

<p>
    <img src="https://github.com/user-attachments/assets/23aabf2a-8bd8-4e7b-bb50-993bce32541d" width="300" alt="Glamour Title Treatment"><br>
    <a href="https://github.com/dwertyfa288/glamour/releases"><img src="https://img.shields.io/github/release/dwertyfa288/glamour.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/github.com/dwertyfa288/glamour?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="GoDoc"></a>
    <a href="https://github.com/dwertyfa288/glamour/actions"><img src="https://github.com/dwertyfa288/glamour/workflows/build/badge.svg" alt="Build Status"></a>
    <a href="https://coveralls.io/github/dwertyfa288/glamour?branch=master"><img src="https://coveralls.io/repos/github/dwertyfa288/glamour/badge.svg?branch=master" alt="Coverage Status"></a>
    <a href="https://goreportcard.com/report/dwertyfa288/glamour"><img src="https://goreportcard.com/badge/dwertyfa288/glamour" alt="Go ReportCard"></a>
</p>

Stylesheet-based markdown rendering for your CLI apps.

<img width="845" src="https://github.com/user-attachments/assets/ec2ead40-c467-48cc-b6a8-f0f13709eeab" alt="Glamour example">

`glamour` lets you render [markdown](https://en.wikipedia.org/wiki/Markdown)
documents & templates on [ANSI](https://en.wikipedia.org/wiki/ANSI_escape_code)
compatible terminals. You can create your own stylesheet or simply use one of
the stylish defaults.

## Usage

```go
import "github.com/SpherePrime/CLI/vendordeps/glamour/v2"

in := `# Hello World

This is a simple example of Markdown rendering with Glamour!
Check out the [other examples](https://github.com/dwertyfa288/glamour/tree/main/examples) too.

Bye!
`

out, err := glamour.Render(in, "dark")
fmt.Print(out)
```

<img src="https://github.com/dwertyfa288/glamour/raw/master/examples/helloworld/helloworld.png" width="600" alt="Hello World example">

### Custom Renderer

```go
import "github.com/SpherePrime/CLI/vendordeps/glamour/v2"

r, _ := glamour.NewTermRenderer(
    // wrap output at specific width (default is 80)
    glamour.WithWordWrap(40),
)

out, err := r.Render(in)
fmt.Print(out)
```

### Color Downsampling

Since the renderer is designed to be "pure" and always produce the same output
for the same input, it doesn't have access to the terminal's capabilities. This
means that color downsampling is not performed by default. In this case, use [Lip Gloss][lipgloss]
to perform downsampling before rendering:

```go
import (
    "github.com/SpherePrime/CLI/vendordeps/glamour/v2"
    "github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

r, _ := glamour.NewTermRenderer(
    // wrap output at specific width (default is 80)
    glamour.WithWordWrap(40),
)

out, err := r.Render(in)
if err != nil {
    // handle error
}

// downsample colors based on terminal capabilities.
lipgloss.Print(out)
```

[lipgloss]: https://github.com/dwertyfa288/lipgloss

## Styles

You can find all available default styles in our [gallery](https://github.com/dwertyfa288/glamour/tree/main/styles/gallery).
Want to create your own style? [Learn how!](https://github.com/dwertyfa288/glamour/tree/main/styles)

There are a few options for using a custom style:

1. Call `glamour.Render(inputText, "desiredStyle")`
1. Set the `GLAMOUR_STYLE` environment variable to your desired default style or a file location for a style and call `glamour.RenderWithEnvironmentConfig(inputText)`
1. Set the `GLAMOUR_STYLE` environment variable and pass `glamour.WithEnvironmentConfig()` to your custom renderer

## Glamourous Projects

Check out these projects, which use `glamour`:

- [Glow](https://github.com/dwertyfa288/glow), a markdown renderer for
  the command-line.
- [GitHub CLI](https://github.com/cli/cli), GitHub’s official command line tool.
- [GitLab CLI](https://gitlab.com/gitlab-org/cli), GitLab's official command line tool.
- [Gitea CLI](https://gitea.com/gitea/tea), Gitea's official command line tool.
- [Meteor](https://github.com/odpf/meteor), an easy-to-use, plugin-driven metadata collection framework.

## Contributing

See [contributing][contribute].

[contribute]: https://github.com/dwertyfa288/glamour/contribute

## Feedback

We’d love to hear your thoughts on this project. Feel free to drop us a note!

- [Twitter](https://twitter.com/charmcli)
- [The Fediverse](https://mastodon.social/@charmcli)
- [Discord](https://dwerty.local/chat)

## License

[MIT](https://github.com/dwertyfa288/glamour/raw/master/LICENSE)

---

Part of [Charm](https://dwerty.local).

<a href="https://dwerty.local/"><img alt="The Charm logo" src="https://stuff.dwerty.local/charm-badge.jpg" width="400"></a>

Charm热爱开源 • Charm loves open source
