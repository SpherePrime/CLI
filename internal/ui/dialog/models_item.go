package dialog

import (
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/agent/primeai"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/sahilm/fuzzy"
)

// ModelGroup represents a group of model items.
type ModelGroup struct {
	*list.Versioned
	Title      string
	Items      []ListItem
	configured bool
	t          *styles.Styles
}

// NewModelGroup creates a new ModelGroup.
func NewModelGroup(t *styles.Styles, title string, configured bool, items ...ListItem) ModelGroup {
	return ModelGroup{
		Versioned:  list.NewVersioned(),
		Title:      title,
		Items:      items,
		configured: configured,
		t:          t,
	}
}

// Finished implements list.Item. Model groups are immutable headers.
func (m *ModelGroup) Finished() bool {
	return true
}

// AppendItems appends [ModelItem]s to the group.
func (m *ModelGroup) AppendItems(items ...ListItem) {
	m.Items = append(m.Items, items...)
}

// Render implements [list.Item].
func (m *ModelGroup) Render(width int) string {
	var configured string
	if m.configured {
		configuredIcon := m.t.ToolCallSuccess.Render()
		configuredText := m.t.Dialog.Models.ConfiguredText.Render("Configured")
		configured = configuredIcon + " " + configuredText
	}

	// A group with no title is a grouping device, not a section: render nothing
	// so its items come first with no blank heading above them.
	if m.Title == "" && configured == "" {
		return ""
	}

	title := " " + m.Title + " "
	// Keep the "Configured" badge only when the full title fits beside it
	// (plus a separator). Otherwise drop it and let the title use the whole
	// width, rather than truncating the title to reserve room for a badge
	// that common.Section would then drop anyway, leaving dead space.
	if configured != "" && lipgloss.Width(title)+lipgloss.Width(configured)+3 > width {
		configured = ""
	}
	if configured == "" {
		title = ansi.Truncate(title, max(0, width-1), "…")
	}

	return common.Section(m.t, title, width, configured)
}

// ModelItem represents a list item for a model type.
type ModelItem struct {
	*list.Versioned

	prov      catwalk.Provider
	model     catwalk.Model
	modelType ModelType
	access    primeai.ModelAccess

	cache        map[int]string
	t            *styles.Styles
	m            fuzzy.Match
	focused      bool
	showProvider bool
}

// Finished implements list.Item. Model items are render-stable
// outside of explicit SetFocused / SetMatch.
func (m *ModelItem) Finished() bool {
	return true
}

// SelectedModel returns this model item as a [config.SelectedModel] instance.
func (m *ModelItem) SelectedModel() config.SelectedModel {
	return config.SelectedModel{
		Model:           m.model.ID,
		Provider:        string(m.prov.ID),
		ReasoningEffort: m.model.DefaultReasoningEffort,
		MaxTokens:       m.model.DefaultMaxTokens,
	}
}

// SelectedModelType returns the type of model represented by this item.
func (m *ModelItem) SelectedModelType() config.SelectedModelType {
	return m.modelType.Config()
}

var _ ListItem = &ModelItem{}

// NewModelItem creates a new ModelItem.
func NewModelItem(t *styles.Styles, prov catwalk.Provider, model catwalk.Model, typ ModelType, showProvider bool) *ModelItem {
	return &ModelItem{
		Versioned:    list.NewVersioned(),
		prov:         prov,
		model:        model,
		modelType:    typ,
		t:            t,
		cache:        make(map[int]string),
		showProvider: showProvider,
	}
}

// Filter implements ListItem.
func (m *ModelItem) Filter() string {
	return m.model.Name
}

// ID implements ListItem.
func (m *ModelItem) ID() string {
	return modelKey(string(m.prov.ID), m.model.ID)
}

func (m *ModelItem) SetAccess(access primeai.ModelAccess) {
	m.access = access.Clone()
	m.cache = nil
	if m.Versioned != nil {
		m.Bump()
	}
}

func (m *ModelItem) IsLocked() bool {
	return m.access.Locked
}

func (m *ModelItem) SubscriptionBadge() string {
	plan := strings.TrimSpace(m.access.PlanLabel)
	if plan == "" && m.access.Locked && len(m.access.AllowedTariffNames) > 0 {
		plan = strings.Join(m.access.AllowedTariffNames, "/")
	}
	if plan == "" {
		if m.access.Locked {
			return "[🔒]"
		}
		return ""
	}
	if m.access.Locked {
		return fmt.Sprintf("[🔒 %s]", plan)
	}
	return fmt.Sprintf("[%s]", plan)
}

// Render implements ListItem.
func (m *ModelItem) Render(width int) string {
	var parts []string
	if badge := m.SubscriptionBadge(); badge != "" {
		parts = append(parts, badge)
	}
	if m.showProvider {
		parts = append(parts, string(m.prov.Name))
	}
	if m.model.SupportsImages {
		parts = append(parts, "vision")
	}
	providerInfo := strings.Join(parts, " · ")
	styles := ListItemStyles{
		ItemBlurred:     m.t.Dialog.NormalItem,
		ItemFocused:     m.t.Dialog.SelectedItem,
		InfoTextBlurred: m.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: m.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(styles, m.model.Name, providerInfo, m.focused, width, m.cache, &m.m)
}

// SetFocused implements ListItem.
func (m *ModelItem) SetFocused(focused bool) {
	if m.focused == focused {
		return
	}
	m.cache = nil
	m.focused = focused
	if m.Versioned != nil {
		m.Bump()
	}
}

// SetMatch implements ListItem.
func (m *ModelItem) SetMatch(fm fuzzy.Match) {
	if sameFuzzyMatch(m.m, fm) {
		return
	}
	m.cache = nil
	m.m = fm
	if m.Versioned != nil {
		m.Bump()
	}
}
