# mango-cobra

[![Latest Release](https://img.shields.io/github/release/muesli/mango-cobra.svg)](https://github.com/dwertyfa288/CLI/vendordeps/muesli/mango-cobra/releases)
[![Build Status](https://github.com/dwertyfa288/CLI/vendordeps/muesli/mango-cobra/workflows/build/badge.svg)](https://github.com/dwertyfa288/CLI/vendordeps/muesli/mango-cobra/actions)
[![Go ReportCard](https://goreportcard.com/badge/muesli/mango-cobra)](https://goreportcard.com/report/muesli/mango-cobra)
[![GoDoc](https://godoc.org/github.com/golang/gddo?status.svg)](https://pkg.go.dev/github.com/dwertyfa288/CLI/vendordeps/muesli/mango-cobra)

cobra adapter for [mango](https://github.com/dwertyfa288/CLI/vendordeps/muesli/mango).

## Example

```go
import (
	"fmt"

	mcobra "github.com/dwertyfa288/CLI/vendordeps/muesli/mango-cobra"
	"github.com/dwertyfa288/CLI/vendordeps/muesli/roff"
	"github.com/dwertyfa288/CLI/vendordeps/spf13/cobra"
)

var (
    rootCmd = &cobra.Command{
        Use:   "mango",
        Short: "A man-page generator",
    }
)

func main() {
    manPage, err := mcobra.NewManPage(1, rootCmd)
    if err != nil {
        panic(err)
    }

    manPage = manPage.WithSection("Copyright", "(C) 2022 Christian Muehlhaeuser.\n"+
        "Released under MIT license.")

    fmt.Println(manPage.Build(roff.NewDocument()))
}
```
