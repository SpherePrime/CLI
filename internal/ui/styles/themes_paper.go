package styles

import "github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"

func applyPaperTheme(s Styles) Styles {
	colors := []string{"#282E38", "#AE2942", "#20643C", "#805E17", "#365E8D", "#8B4C5D", "#22626C", "#4B505A", "#555B65", "#B22D3D", "#216738", "#735614", "#315AA8", "#84519C", "#225E74", "#282E38"}
	for index, value := range colors {
		s.ANSI[index] = lipgloss.Color(value)
	}
	primary, secondary, foreground, muted, success := s.ANSI[4], s.ANSI[5], s.ANSI[0], s.ANSI[8], s.ANSI[2]
	s.Markdown.Link.Color = hex(primary)
	s.Markdown.LinkText.Color = hex(primary)
	s.Markdown.Image.Color = hex(primary)
	s.Markdown.Code.Color = hex(primary)
	chroma := s.Markdown.CodeBlock.Chroma
	chroma.Text.Color = hex(foreground)
	chroma.Comment.Color = hex(muted)
	chroma.CommentPreproc.Color = hex(muted)
	chroma.Keyword.Color = hex(secondary)
	chroma.KeywordReserved.Color = hex(secondary)
	chroma.KeywordNamespace.Color = hex(secondary)
	chroma.KeywordType.Color = hex(primary)
	chroma.Operator.Color = hex(primary)
	chroma.Punctuation.Color = hex(foreground)
	chroma.Name.Color = hex(foreground)
	chroma.NameBuiltin.Color = hex(primary)
	chroma.NameTag.Color = hex(primary)
	chroma.NameAttribute.Color = hex(secondary)
	chroma.NameClass.Color = hex(primary)
	chroma.NameConstant.Color = hex(s.ANSI[3])
	chroma.NameDecorator.Color = hex(secondary)
	chroma.NameFunction.Color = hex(success)
	chroma.LiteralNumber.Color = hex(s.ANSI[3])
	chroma.LiteralString.Color = hex(success)
	chroma.LiteralStringEscape.Color = hex(secondary)
	chroma.GenericDeleted.Color = hex(s.ANSI[1])
	chroma.GenericInserted.Color = hex(success)
	return s
}
