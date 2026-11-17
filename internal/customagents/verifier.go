package customagents

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	minRepeatBlockRunes = 60
	minRepeatCount      = 4
	repeatAnchorRunes   = 30
)

func ValidateWorkerOutput(output string) error {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return fmt.Errorf("пустой вывод исполнителя")
	}

	runes := []rune(trimmed)
	if len(runes) < minRepeatBlockRunes*minRepeatCount {
		return nil
	}

	const rhBase uint64 = 131
	var pow uint64 = 1
	for i := 0; i < repeatAnchorRunes-1; i++ {
		pow *= rhBase
	}

	var h uint64
	for i := 0; i < repeatAnchorRunes; i++ {
		h = h*rhBase + uint64(runes[i])
	}

	anchors := make(map[uint64][]int, len(runes)/repeatAnchorRunes+1)
	limit := len(runes) - repeatAnchorRunes
	for i := 0; i <= limit; i++ {
		if prevs, ok := anchors[h]; ok {
			for _, prev := range prevs {
				period := i - prev
				if period < minRepeatBlockRunes {
					continue
				}
				if !runesEqual(runes[prev:prev+repeatAnchorRunes], runes[i:i+repeatAnchorRunes]) {
					continue
				}
				back := 0
				for prev-back-1 >= 0 && runes[prev-back-1] == runes[i-back-1] {
					back++
				}
				fwd := repeatAnchorRunes
				for i+fwd < len(runes) && runes[prev+fwd] == runes[i+fwd] {
					fwd++
				}
				matchLen := back + fwd
				if matchLen >= (minRepeatCount-1)*period-2 {
					start := prev - back
					if start+period <= len(runes) {
						block := strings.TrimSpace(string(runes[start : start+period]))
						if utf8.RuneCountInString(block) >= minRepeatBlockRunes {
							return fmt.Errorf("обнаружено циклическое повторение блока (%d символов, %d+ раз подряд)", utf8.RuneCountInString(block), minRepeatCount)
						}
					}
				}
			}
		}
		if i%repeatAnchorRunes == 0 {
			anchors[h] = append(anchors[h], i)
		}
		if i < limit {
			h = (h-uint64(runes[i])*pow)*rhBase + uint64(runes[i+repeatAnchorRunes])
		}
	}

	return nil
}

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func strictCompletenessChecklist() string {
	return `Критерии приёмки (строгий режим):
1. Задача выполнена полностью, без пропусков, заглушек и незавершённых разделов.
2. Все заявленные изменения, пути файлов, команды и результаты проверок подтверждены конкретными фактами в ответе.
3. Отсутствуют противоречия, выдуманные данные и нерелевантный или повторяющийся текст.
4. Ответ структурирован и напрямую отвечает на исходную задачу.`
}

func RunDeterministicChecks(ctx context.Context, checks []DeterministicCheck, outputText string, workDir string) (bool, string) {
	if len(checks) == 0 {
		return true, ""
	}

	allPassed := true
	var report strings.Builder

	for idx, check := range checks {
		label := check.ID
		if label == "" {
			label = fmt.Sprintf("check-%d", idx+1)
		}
		ok, detail := runSingleDeterministicCheck(ctx, check, outputText, workDir)
		if ok {
			fmt.Fprintf(&report, "[PASS] %s (%s): %s\n", label, check.Kind, detail)
		} else if check.Required {
			allPassed = false
			fmt.Fprintf(&report, "[FAIL] %s (%s): %s\n", label, check.Kind, detail)
		} else {
			fmt.Fprintf(&report, "[WARN] %s (%s): %s\n", label, check.Kind, detail)
		}
	}

	return allPassed, strings.TrimRight(report.String(), "\n")
}

func runSingleDeterministicCheck(ctx context.Context, check DeterministicCheck, outputText string, workDir string) (bool, string) {
	switch check.Kind {
	case CheckRegex:
		return checkRegex(check, outputText, workDir)
	case CheckJSONFields:
		return checkJSONFields(check, outputText)
	case CheckFileExists:
		return checkFileExists(check, workDir)
	case CheckCommand:
		return checkCommand(ctx, check, workDir)
	default:
		return false, fmt.Sprintf("неизвестный тип проверки %q", check.Kind)
	}
}

func checkRegex(check DeterministicCheck, outputText, workDir string) (bool, string) {
	if check.Pattern == "" {
		return false, "пустое регулярное выражение"
	}
	re, err := regexp.Compile(check.Pattern)
	if err != nil {
		return false, fmt.Sprintf("ошибка компиляции regex: %v", err)
	}

	content := outputText
	target := strings.TrimSpace(check.Target)
	if target != "" && !strings.EqualFold(target, "output") {
		path := resolveCheckPath(target, workDir)
		f, err := os.Open(path)
		if err != nil {
			return false, fmt.Sprintf("не удалось открыть файл %s: %v", path, err)
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, maxMonitorBytes))
		if err != nil {
			return false, fmt.Sprintf("ошибка чтения файла %s: %v", path, err)
		}
		content = string(data)
	}

	if re.MatchString(content) {
		return true, fmt.Sprintf("совпадение с шаблоном %q найдено", check.Pattern)
	}
	return false, fmt.Sprintf("шаблон %q не найден", check.Pattern)
}

func checkJSONFields(check DeterministicCheck, outputText string) (bool, string) {
	doc, err := extractJSONDocument(outputText)
	if err != nil {
		return false, fmt.Sprintf("некорректный JSON: %v", err)
	}

	fieldsRaw := strings.TrimSpace(check.Pattern)
	if fieldsRaw == "" {
		return true, "валидный JSON"
	}

	fields := strings.Split(fieldsRaw, ",")
	var missing []string
	var lastVal any
	for _, rawField := range fields {
		fieldPath := strings.TrimSpace(rawField)
		if fieldPath == "" {
			continue
		}
		val, ok := lookupJSONPath(doc, fieldPath)
		if !ok {
			missing = append(missing, fieldPath)
			continue
		}
		lastVal = val
	}

	if len(missing) > 0 {
		return false, fmt.Sprintf("отсутствуют поля: %s", strings.Join(missing, ", "))
	}

	if check.Expected != "" && len(fields) == 1 {
		actual := fmt.Sprintf("%v", lastVal)
		if actual != check.Expected {
			return false, fmt.Sprintf("поле %s = %q, ожидалось %q", strings.TrimSpace(fields[0]), actual, check.Expected)
		}
	}

	return true, fmt.Sprintf("все обязательные JSON-поля присутствуют (%s)", fieldsRaw)
}

func extractJSONDocument(text string) (any, error) {
	trimmed := strings.TrimSpace(text)
	var doc any
	if err := json.Unmarshal([]byte(trimmed), &doc); err == nil {
		return doc, nil
	}

	if start := strings.Index(trimmed, "```json"); start >= 0 {
		rest := trimmed[start+len("```json"):]
		if end := strings.Index(rest, "```"); end >= 0 {
			candidate := strings.TrimSpace(rest[:end])
			if err := json.Unmarshal([]byte(candidate), &doc); err == nil {
				return doc, nil
			}
		}
	}

	if start := strings.IndexByte(trimmed, '{'); start >= 0 {
		if end := strings.LastIndexByte(trimmed, '}'); end > start {
			candidate := trimmed[start : end+1]
			if err := json.Unmarshal([]byte(candidate), &doc); err == nil {
				return doc, nil
			}
		}
	}

	return nil, fmt.Errorf("JSON не найден в выводе")
}

func lookupJSONPath(doc any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	cur := doc
	for _, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		val, exists := m[part]
		if !exists || val == nil {
			return nil, false
		}
		cur = val
	}
	return cur, true
}

func checkFileExists(check DeterministicCheck, workDir string) (bool, string) {
	target := strings.TrimSpace(check.Target)
	if target == "" {
		target = strings.TrimSpace(check.Pattern)
	}
	if target == "" {
		return false, "не указан путь к файлу"
	}

	path := resolveCheckPath(target, workDir)
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Sprintf("файл %s не найден: %v", path, err)
	}
	if strings.EqualFold(strings.TrimSpace(check.Expected), "nonempty") && info.Size() == 0 {
		return false, fmt.Sprintf("файл %s пуст", path)
	}
	return true, fmt.Sprintf("файл %s существует (%d байт)", path, info.Size())
}

func checkCommand(ctx context.Context, check DeterministicCheck, workDir string) (bool, string) {
	cmdStr := strings.TrimSpace(check.Target)
	if cmdStr == "" {
		cmdStr = strings.TrimSpace(check.Pattern)
	}
	if cmdStr == "" {
		return false, "не указана команда проверки"
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "cmd", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(cmdCtx, "sh", "-c", cmdStr)
	}
	if workDir != "" {
		cmd.Dir = workDir
	}

	var buf limitedBuffer
	buf.limit = maxMonitorBytes
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(buf.String())
		if out != "" {
			return false, fmt.Sprintf("команда %q завершилась с ошибкой (%v): %s", cmdStr, err, out)
		}
		return false, fmt.Sprintf("команда %q завершилась с ошибкой: %v", cmdStr, err)
	}

	out := buf.String()
	if check.Expected != "" && !strings.Contains(out, check.Expected) {
		return false, fmt.Sprintf("вывод команды не содержит %q", check.Expected)
	}
	return true, fmt.Sprintf("команда %q выполнена успешно", cmdStr)
}

func resolveCheckPath(target, workDir string) string {
	if filepath.IsAbs(target) || workDir == "" {
		return target
	}
	return filepath.Join(workDir, target)
}

func ParseVerifierVerdict(text string) (bool, string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false, "Верификатор вернул пустой ответ без явного VERDICT: PASS или VERDICT: FAIL."
	}

	sawPass := false
	sawFail := false
	var feedbackLines []string

	lines := strings.Split(strings.ReplaceAll(trimmed, "\r\n", "\n"), "\n")
	for _, rawLine := range lines {
		clean := strings.TrimSpace(rawLine)
		clean = strings.Trim(clean, "*`# ")
		upper := strings.ToUpper(clean)

		if strings.HasPrefix(upper, "VERDICT:") {
			val := strings.TrimSpace(strings.TrimPrefix(upper, "VERDICT:"))
			switch {
			case strings.HasPrefix(val, "PASS"):
				sawPass = true
			case strings.HasPrefix(val, "FAIL"):
				sawFail = true
			default:
				sawFail = true
			}
			continue
		}
		feedbackLines = append(feedbackLines, rawLine)
	}

	feedback := strings.TrimSpace(strings.Join(feedbackLines, "\n"))
	if sawPass && !sawFail {
		if feedback == "" {
			feedback = "Проверка пройдена."
		}
		return true, feedback
	}

	if !sawPass && !sawFail {
		return false, "Отсутствует явный вердикт VERDICT: PASS или VERDICT: FAIL.\n\nОтвет верификатора:\n" + trimmed
	}
	if sawPass && sawFail {
		return false, "Обнаружен противоречивый вердикт (одновременно PASS и FAIL).\n\n" + trimmed
	}
	if feedback == "" {
		feedback = trimmed
	}
	return false, feedback
}
