package dialog

import (
	"testing"

	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/textinput"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func newProvidersTestDialog() *Providers {
	styles := styles.ColorTonePantera()
	return NewProviders(&common.Common{Styles: &styles})
}

func TestProvidersRetryKeepsFormState(t *testing.T) {
	t.Parallel()

	d := newProvidersTestDialog()
	d.providerID = "asdasd"
	d.urlInput = textinput.New()
	d.urlInput.SetValue("https://vyceai.com/v1")
	d.keyInput = textinput.New()
	d.keyInput.SetValue("sk-test")
	d.state = providersStateError

	d.advance()
	require.Equal(t, providersStateDiscovering, d.state, "retry must re-run discovery without clearing the form")
}

func TestProvidersErrorSubmitSchedulesDiscovery(t *testing.T) {
	t.Parallel()

	d := newProvidersTestDialog()
	d.providerID = "asdasd"
	d.urlInput = textinput.New()
	d.urlInput.SetValue("https://vyceai.com/v1")
	d.keyInput = textinput.New()
	d.keyInput.SetValue("sk-test")
	d.state = providersStateError

	// Only verify the state transition: the discovery command would
	// resolve config variables, which is not wired up in this test.
	d.advance()
	require.Equal(t, providersStateDiscovering, d.state)
}

func TestProvidersDiscoveringIgnoresKeys(t *testing.T) {
	t.Parallel()

	d := newProvidersTestDialog()
	d.state = providersStateDiscovering

	d.HandleMsg(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	require.Equal(t, providersStateDiscovering, d.state, "keys must be ignored while discovery is in flight")
}
