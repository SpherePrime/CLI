package i18n

import "fmt"

const (
	En = "en"
	Ru = "ru"
)

// Catalog holds the translated strings for one locale.
type Catalog struct {
	Locale  string
	Title   string
	Strings map[string]string
}

// Sprintf returns the catalog string for key and formats it with args.
func (c Catalog) Sprintf(key string, args ...any) string {
	s, ok := c.Strings[key]
	if !ok {
		s = key
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}
