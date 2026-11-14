package google

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/genai"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestToGooglePrompt_MultipleLeadingSystemMessages(t *testing.T) {
	t.Parallel()

	prompt := fantasy.Prompt{
		fantasy.NewSystemMessage("prefix instructions"),
		fantasy.NewSystemMessage("main system prompt"),
		fantasy.NewUserMessage("hello"),
	}

	systemInstructions, content, warnings := toGooglePrompt(prompt, false)
	require.Empty(t, warnings)
	require.NotNil(t, systemInstructions)
	require.Len(t, systemInstructions.Parts, 1)
	require.Equal(t, "prefix instructions\nmain system prompt", systemInstructions.Parts[0].Text)
	require.Len(t, content, 1)
	require.Equal(t, genai.RoleUser, content[0].Role)
}

func TestToGooglePrompt_SkipsSystemMessagesAfterConversationStarts(t *testing.T) {
	t.Parallel()

	prompt := fantasy.Prompt{
		fantasy.NewSystemMessage("prefix"),
		fantasy.NewSystemMessage("main"),
		fantasy.NewUserMessage("hi"),
		fantasy.NewSystemMessage("late system"),
		{
			Role:    fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{fantasy.TextPart{Text: "ok"}},
		},
	}

	systemInstructions, content, warnings := toGooglePrompt(prompt, false)
	require.Empty(t, warnings)
	require.NotNil(t, systemInstructions)
	require.Len(t, systemInstructions.Parts, 1)
	require.Equal(t, "prefix\nmain", systemInstructions.Parts[0].Text)
	require.Len(t, content, 2)
	require.Equal(t, genai.RoleUser, content[0].Role)
	require.Equal(t, genai.RoleModel, content[1].Role)
}

func TestToGooglePrompt_EmptyLeadingSystemMessage(t *testing.T) {
	t.Parallel()

	prompt := fantasy.Prompt{
		fantasy.NewSystemMessage(""),
		fantasy.NewSystemMessage("main system"),
		fantasy.NewUserMessage("hi"),
	}

	systemInstructions, content, warnings := toGooglePrompt(prompt, false)
	require.Empty(t, warnings)
	require.NotNil(t, systemInstructions)
	require.Len(t, systemInstructions.Parts, 1)
	require.Equal(t, "main system", systemInstructions.Parts[0].Text)
	require.Len(t, content, 1)
}

func TestToGooglePrompt_NoSystemMessages(t *testing.T) {
	t.Parallel()

	prompt := fantasy.Prompt{
		fantasy.NewUserMessage("hi"),
	}

	systemInstructions, content, warnings := toGooglePrompt(prompt, false)
	require.Empty(t, warnings)
	require.Nil(t, systemInstructions)
	require.Len(t, content, 1)
}
