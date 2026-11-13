package agent

import (
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"testing"
)

func TestProgressContinuation(t *testing.T) {
	require.True(t, unfinishedProgress("Да, дальше анализирую сам, без агентов. Проверю, как подключить вход.", true))
	require.True(t, unfinishedProgress("Сейчас разберусь.", false))
	require.False(t, unfinishedProgress("Исправлено. Тесты прошли.", true))
	require.False(t, unfinishedProgress("Проверю после получения доступа. Нужен пароль.", false))
	require.False(t, unfinishedProgress("", true))
}
