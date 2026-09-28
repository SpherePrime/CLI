package model

import (
	"strconv"
	"testing"

	"github.com/SpherePrime/CLI/internal/ui/chat"
)

func BenchmarkUIViewIdle(b *testing.B) {
	u := newFrameTestUI(b)
	items := make([]chat.MessageItem, 0, 60)
	for i := range 60 {
		items = append(items, &focusableTestItem{testMessageItem: testMessageItem{id: strconv.Itoa(i), text: "line " + strconv.Itoa(i)}})
	}
	u.chat.SetMessages(items...)
	u.chat.ScrollToBottom()
	u.shimmerEnabled = true
	for b.Loop() {
		u.View()
	}
}
