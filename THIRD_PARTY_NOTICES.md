# Third-party notices

Frontend libraries are embedded in the executable. Go dependencies are resolved
from `go.mod`/`go.sum`; some entries below are used only in tests. Preserve these
notices with binary redistributions. Optional models and services are not bundled.

## Frontend

| Component | Version | License text |
| --- | --- | --- |
| [DOMPurify](https://github.com/cure53/DOMPurify) | 3.4.17 | [Apache-2.0 OR MPL-2.0](docs/licenses/DOMPurify.txt) |
| [marked](https://github.com/markedjs/marked) | 12.0.2 | [MIT](docs/licenses/marked.txt) |
| [highlight.js](https://github.com/highlightjs/highlight.js) | 11.9.0 | [BSD-3-Clause](docs/licenses/highlight.js.txt) |

DOMPurify comes from the tagged upstream `dist/purify.min.js`. Review upstream
security advisories when updating vendored assets; Go Dependabot does not track
these JavaScript files.

## Go modules

- `github.com/atotto/clipboard@v0.1.4`: [LICENSE](docs/licenses/go/github.com__atotto__clipboard__LICENSE.txt)
- `github.com/aymanbagabas/go-osc52/v2@v2.0.1`: [LICENSE](docs/licenses/go/github.com__aymanbagabas__go-osc52__v2__LICENSE.txt)
- `github.com/charmbracelet/bubbles@v1.0.0`: [LICENSE](docs/licenses/go/github.com__charmbracelet__bubbles__LICENSE.txt)
- `github.com/charmbracelet/bubbletea@v1.3.10`: [LICENSE](docs/licenses/go/github.com__charmbracelet__bubbletea__LICENSE.txt)
- `github.com/charmbracelet/colorprofile@v0.4.1`: [LICENSE](docs/licenses/go/github.com__charmbracelet__colorprofile__LICENSE.txt)
- `github.com/charmbracelet/lipgloss@v1.1.0`: [LICENSE](docs/licenses/go/github.com__charmbracelet__lipgloss__LICENSE.txt)
- `github.com/charmbracelet/x/ansi@v0.11.6`: [LICENSE](docs/licenses/go/github.com__charmbracelet__x__ansi__LICENSE.txt)
- `github.com/charmbracelet/x/cellbuf@v0.0.15`: [LICENSE](docs/licenses/go/github.com__charmbracelet__x__cellbuf__LICENSE.txt)
- `github.com/charmbracelet/x/term@v0.2.2`: [LICENSE](docs/licenses/go/github.com__charmbracelet__x__term__LICENSE.txt)
- `github.com/chromedp/cdproto@v0.157.4`: [LICENSE](docs/licenses/go/github.com__chromedp__cdproto__LICENSE.txt)
- `github.com/chromedp/chromedp@v0.19.1`: [LICENSE](docs/licenses/go/github.com__chromedp__chromedp__LICENSE.txt)
- `github.com/clipperhouse/displaywidth@v0.9.0`: [LICENSE](docs/licenses/go/github.com__clipperhouse__displaywidth__LICENSE.txt)
- `github.com/clipperhouse/stringish@v0.1.1`: [LICENSE](docs/licenses/go/github.com__clipperhouse__stringish__LICENSE.txt)
- `github.com/clipperhouse/uax29/v2@v2.5.0`: [LICENSE](docs/licenses/go/github.com__clipperhouse__uax29__v2__LICENSE.txt)
- `github.com/erikgeiser/coninput@v0.0.0-20211004153227-1c3628e74d0f`: [LICENSE](docs/licenses/go/github.com__erikgeiser__coninput__LICENSE.txt)
- `github.com/golang-jwt/jwt/v5@v5.3.1`: [LICENSE](docs/licenses/go/github.com__golang-jwt__jwt__v5__LICENSE.txt)
- `github.com/google/go-cmp@v0.7.0`: [LICENSE](docs/licenses/go/github.com__google__go-cmp__LICENSE.txt)
- `github.com/google/jsonschema-go@v0.4.3`: [LICENSE](docs/licenses/go/github.com__google__jsonschema-go__LICENSE.txt)
- `github.com/ledongthuc/pdf@v0.0.0-20260907135840-6c8c28e0e8a0`: [LICENSE](docs/licenses/go/github.com__ledongthuc__pdf__LICENSE.txt)
- `github.com/lucasb-eyer/go-colorful@v1.3.0`: [LICENSE](docs/licenses/go/github.com__lucasb-eyer__go-colorful__LICENSE.txt)
- `github.com/mattn/go-isatty@v0.0.20`: [LICENSE](docs/licenses/go/github.com__mattn__go-isatty__LICENSE.txt)
- `github.com/mattn/go-runewidth@v0.0.19`: [LICENSE](docs/licenses/go/github.com__mattn__go-runewidth__LICENSE.txt)
- `github.com/modelcontextprotocol/go-sdk@v1.8.0`: [LICENSE](docs/licenses/go/github.com__modelcontextprotocol__go-sdk__LICENSE.txt)
- `github.com/muesli/ansi@v0.0.0-20230316100256-276c6243b2f6`: [LICENSE](docs/licenses/go/github.com__muesli__ansi__LICENSE.txt)
- `github.com/muesli/cancelreader@v0.2.2`: [LICENSE](docs/licenses/go/github.com__muesli__cancelreader__LICENSE.txt)
- `github.com/muesli/termenv@v0.16.0`: [LICENSE](docs/licenses/go/github.com__muesli__termenv__LICENSE.txt)
- `github.com/rivo/uniseg@v0.4.7`: [LICENSE.txt](docs/licenses/go/github.com__rivo__uniseg__LICENSE.txt.txt)
- `github.com/segmentio/asm@v1.1.3`: [LICENSE](docs/licenses/go/github.com__segmentio__asm__LICENSE.txt)
- `github.com/segmentio/encoding@v0.5.4`: [LICENSE](docs/licenses/go/github.com__segmentio__encoding__LICENSE.txt)
- `github.com/xo/terminfo@v0.0.0-20220910002029-abceb7e1c41e`: [LICENSE](docs/licenses/go/github.com__xo__terminfo__LICENSE.txt)
- `github.com/yosida95/uritemplate/v3@v3.0.2`: [LICENSE](docs/licenses/go/github.com__yosida95__uritemplate__v3__LICENSE.txt)
- `golang.org/x/exp@v0.0.0-20231006140011-7918f672742d`: [LICENSE](docs/licenses/go/golang.org__x__exp__LICENSE.txt)
- `golang.org/x/oauth2@v0.35.0`: [LICENSE](docs/licenses/go/golang.org__x__oauth2__LICENSE.txt)
- `golang.org/x/sync@v0.20.0`: [LICENSE](docs/licenses/go/golang.org__x__sync__LICENSE.txt)
- `golang.org/x/sys@v0.44.0`: [LICENSE](docs/licenses/go/golang.org__x__sys__LICENSE.txt)
- `golang.org/x/text@v0.3.8`: [LICENSE](docs/licenses/go/golang.org__x__text__LICENSE.txt)
- `golang.org/x/time@v0.15.0`: [LICENSE](docs/licenses/go/golang.org__x__time__LICENSE.txt)
- `golang.org/x/tools@v0.42.0`: [LICENSE](docs/licenses/go/golang.org__x__tools__LICENSE.txt)

The Go runtime and standard library use the [Go BSD license](docs/licenses/Go.txt).

- `github.com/mattn/go-localereader@v0.0.1`: [upstream MIT declaration and author](docs/licenses/go/github.com__mattn__go-localereader__README.md.txt). This version includes the declaration in its README rather than a separate license file.
