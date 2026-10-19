package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const fileName = "goarchivist.log"

// Init направляет стандартный пакет log в файл внутри logsDir.
// Возвращённый файл нужно закрыть после завершения приложения.
func Init(logsDir string) (*os.File, error) {
	if logsDir == "" {
		return nil, fmt.Errorf("путь к папке журналов не задан")
	}

	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("не удалось создать папку журналов: %w", err)
	}

	path := filepath.Join(logsDir, fileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл журнала: %w", err)
	}

	log.SetOutput(file)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return file, nil
}
