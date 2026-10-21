package archive

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ValidateDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("укажите папку для хранения архива")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("укажите абсолютный путь к папке")
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("не удалось определить абсолютный путь: %w", err)
	}
	absolutePath = filepath.Clean(absolutePath)

	info, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("не удалось открыть папку: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("указанный путь не является папкой")
	}

	file, err := os.CreateTemp(absolutePath, ".goarchivist-write-test-*")
	if err != nil {
		return "", fmt.Errorf("не удалось создать временный файл: %w", err)
	}

	tempPath := file.Name()
	defer os.Remove(tempPath)

	expected := []byte("GoArchivist archive directory test")
	if _, err := file.Write(expected); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("не удалось записать временный файл: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("не удалось закрыть временный файл после записи: %w", err)
	}

	actual, err := os.ReadFile(tempPath)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать временный файл: %w", err)
	}
	if !bytes.Equal(actual, expected) {
		return "", errors.New("содержимое временного файла не совпало с записанными данными")
	}

	if err := os.Remove(tempPath); err != nil {
		return "", fmt.Errorf("не удалось удалить временный файл: %w", err)
	}

	return absolutePath, nil
}
