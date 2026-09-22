package i18n

// Translator renders UI strings for a locale.
type Translator struct {
	catalog Catalog
}

// New returns a translator for the given locale, falling back to English.
func New(locale string) Translator {
	if cat, ok := Catalogs[locale]; ok {
		return Translator{catalog: cat}
	}
	return Translator{catalog: Catalogs[En]}
}

// Locale returns the locale this translator renders.
func (t Translator) Locale() string {
	return t.catalog.Locale
}

// Label returns the translated value for a key, or the key itself when
// missing.
func (t Translator) Label(key string) string {
	if v, ok := t.catalog.Strings[key]; ok {
		return v
	}
	return key
}

// Sprintf formats a translated string with args.
func (t Translator) Sprintf(key string, args ...any) string {
	return t.catalog.Sprintf(key, args...)
}

// IsSupported reports whether a locale has a catalog.
func IsSupported(locale string) bool {
	_, ok := Catalogs[locale]
	return ok
}

// Locales lists available locales, English first.
func Locales() []string {
	locales := []string{En}
	for l := range Catalogs {
		if l != En {
			locales = append(locales, l)
		}
	}
	return locales
}

// Title returns the display name for a locale ("English", "Русский", ...).
func Title(locale string) string {
	for _, c := range Catalogs {
		if c.Locale == locale {
			return c.Title
		}
	}
	return locale
}
