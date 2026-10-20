package chat

import "testing"

func TestCappedMessageWidthNeverBecomesNegative(t *testing.T) {
	for _, width := range []int{0, 1, MessageLeftPaddingTotal - 1} {
		if got := cappedMessageWidth(width); got != 1 {
			t.Fatalf("cappedMessageWidth(%d) = %d, want 1", width, got)
		}
	}
}

func TestCappedMessageWidthStillHonorsContentCap(t *testing.T) {
	if got := cappedMessageWidth(maxTextWidth + MessageLeftPaddingTotal + 100); got != maxTextWidth {
		t.Fatalf("cappedMessageWidth() = %d, want %d", got, maxTextWidth)
	}
}
