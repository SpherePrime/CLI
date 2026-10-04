package model

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
)

// Keybinding names are the keymap's own field paths in lower snake case:
// Chat.NewSession is "chat.new_session", Editor.SendMessage is
// "editor.send_message", and Quit is "quit".
//
// Deriving the name from the field rather than maintaining a hand-written list
// means a new binding is remappable the moment it is added, instead of after
// somebody remembers to add it to a table that then quietly rots.
var bindingType = reflect.TypeOf(key.Binding{})

// walkBindings calls fn for every binding reachable from the struct value v,
// keyed by its dotted name.
func walkBindings(v reflect.Value, prefix string, fn func(name string, binding *key.Binding)) {
	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		name := joinKeyName(prefix, snakeCase(field.Name))

		// Sub-structs contribute their name as a prefix. Every one of them here
		// is anonymous, so field.Name is the only thing to go on.
		if field.Type.Kind() == reflect.Struct && field.Type != bindingType {
			walkBindings(v.Field(i), name, fn)
			continue
		}

		if field.Type != bindingType {
			continue
		}

		f := v.Field(i)
		if !f.CanAddr() {
			continue
		}
		fn(name, f.Addr().Interface().(*key.Binding))
	}
}

// joinKeyName joins a prefix and a name with the separator used in keybinding
// names, keeping "quit" rather than ".quit".
func joinKeyName(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// snakeCase converts a Go field name to the lower snake case used in keybinding
// names: NewSession becomes new_session, PasteImage becomes paste_image.
func snakeCase(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)

	runes := []rune(s)
	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'
		if !isUpper {
			b.WriteRune(r)
			continue
		}

		// Start a new word at the boundary: either the previous rune was
		// lower case ("NewSession"), or this is the start of an acronym
		// followed by a lower case rune ("HTTPServer").
		prevLower := i > 0 && runes[i-1] >= 'a' && runes[i-1] <= 'z'
		nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
		if i > 0 && (prevLower || (isUpperAt(runes, i-1) && nextLower)) && b.Len() > 0 {
			b.WriteRune('_')
		}
		b.WriteRune(r - 'A' + 'a')
	}
	return b.String()
}

func isUpperAt(runes []rune, i int) bool {
	return runes[i] >= 'A' && runes[i] <= 'Z'
}

// ParseKeybinding splits a configured value into the keys it binds.
//
// A value may hold several keys separated by a comma, so one action can answer
// to both its default and a remapped key. An empty value means unbind, which
// is how an action is disabled without removing the name.
func ParseKeybinding(value string) []string {
	var keys []string
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	return keys
}

// ApplyKeyBindings remaps keys on km according to overrides, which maps a
// keybinding name to one or more comma-separated keys.
//
// It returns a problem per entry it could not apply. Anything not named in
// overrides keeps its default, because a remap that silently does nothing is
// the worst outcome: the user believes a key was moved and it was not.
//
// A name is only accepted if it matches a binding in the keymap, so a typo is
// reported rather than ignored.
func ApplyKeyBindings(km *KeyMap, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return nil
	}

	// The stock keymap shares keys across contexts that are never on screen at
	// the same time: esc is both editor.escape and chat.cancel, enter is both
	// editor.send_message and initialize.enter. Those are not bugs, they are how
	// the same key means different things in different states, so the baseline is
	// recorded and only conflicts the remap actually introduces are reported.
	baseline := map[string]bool{}
	for _, conflict := range findKeyConflicts(km) {
		baseline[conflict] = true
	}

	var problems []string

	names := make(map[string]bool, len(overrides))
	for name := range overrides {
		names[name] = true
	}

	walkBindings(reflect.ValueOf(km).Elem(), "", func(name string, binding *key.Binding) {
		value, ok := overrides[name]
		if !ok {
			return
		}
		delete(names, name)

		keys := ParseKeybinding(value)
		if len(keys) == 0 {
			// An empty value unbinds: SetEnabled(false) leaves the name
			// known but nothing triggers it, which is what someone disabling
			// an action expects.
			binding.SetEnabled(false)
			binding.Unbind()
			return
		}

		binding.SetKeys(keys...)
		// The help line still showed the old key, so the one thing that
		// tells the user what to press would have been wrong.
		help := binding.Help()
		binding.SetHelp(strings.Join(keys, ", "), help.Desc)
	})

	// Report names that match nothing, sorted so the message is stable.
	missed := make([]string, 0, len(names))
	for name := range names {
		missed = append(missed, name)
	}
	sort.Strings(missed)
	for _, name := range missed {
		problems = append(problems, fmt.Sprintf("unknown keybinding %q", name))
	}

	for _, conflict := range findKeyConflicts(km) {
		if !baseline[conflict] {
			problems = append(problems, conflict)
		}
	}

	return problems
}

// findKeyConflicts reports keys bound to more than one action.
//
// Bubbles does not resolve this: both bindings match the same message and
// whichever case the switch reaches first wins, so the other action silently
// stops responding. The user gets no error at all, just a key that does
// something else than they set, so it is worth saying so out loud.
func findKeyConflicts(km *KeyMap) []string {
	owner := map[string]string{}
	var duplicates []string

	walkBindings(reflect.ValueOf(km).Elem(), "", func(name string, binding *key.Binding) {
		if !binding.Enabled() {
			return
		}
		for _, k := range binding.Keys() {
			if previous, taken := owner[k]; taken && previous != name {
				duplicates = append(duplicates, fmt.Sprintf("%s is bound to both %s and %s", k, previous, name))
				continue
			}
			owner[k] = name
		}
	})

	sort.Strings(duplicates)
	return slices.Compact(dedupe(duplicates))
}

func dedupe(in []string) []string {
	out := in[:0]
	for i, s := range in {
		if i == 0 || in[i-1] != s {
			out = append(out, s)
		}
	}
	return out
}

// KeyBindingNames returns every keybinding name in km with the keys it is
// currently bound to, sorted by name.
//
// This is what "prime keys" prints: the set of names is not something to guess
// at, and a remap feature nobody can discover the names for is a remap feature
// nobody uses.
func KeyBindingNames(km KeyMap) []string {
	type entry struct {
		name string
		keys string
	}

	var entries []entry
	walkBindings(reflect.ValueOf(&km).Elem(), "", func(name string, binding *key.Binding) {
		keys := strings.Join(binding.Keys(), ", ")
		if !binding.Enabled() && keys == "" {
			keys = "(unbound)"
		}
		entries = append(entries, entry{name: name, keys: keys})
	})

	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, fmt.Sprintf("%-40s %s", e.name, e.keys))
	}
	return out
}