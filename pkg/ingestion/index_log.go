// Copyright 2025 KrakLabs
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package ingestion

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var indexLogMu sync.Mutex

// AppendIndexLog дописывает строку в <project_folder>/.cie/index.log для диагностики индексации.
// dotCieDir — путь к каталогу .cie (например filepath.Join(repoPath, ".cie")).
// Формат строки: ISO8601 + " " + message. Удобно искать по имени файла: grep "pkg/foo.go" .cie/index.log
// Важные события (reindex started/completed, watch) дублируются в stderr, чтобы были видны в логе Kilo/IDE.
func AppendIndexLog(dotCieDir, message string) {
	if dotCieDir == "" {
		return
	}
	indexLogMu.Lock()
	defer indexLogMu.Unlock()
	if err := os.MkdirAll(dotCieDir, 0750); err != nil {
		return
	}
	logPath := filepath.Join(dotCieDir, "index.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return
	}
	line := fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), message)
	_, _ = f.WriteString(line)
	_ = f.Close()
	// Дублируем в stderr только события реиндекса/watch, чтобы видеть в Kilo (без перечисления файлов)
	if isReindexOrWatchEvent(message) {
		_, _ = os.Stderr.WriteString("[CIE index.log] " + message + "\n")
	}
}

func isReindexOrWatchEvent(message string) bool {
	return message == "mcp server started" || (len(message) >= 7 && message[:7] == "reindex ")
}
