package main

import (
	"os"
	"testing"
)

func TestCli_ParserEdges(t *testing.T) {
	// Тестирование краевых случаев zipper
	if err := runZipper([]string{"zipper"}); err == nil {
		t.Error("expected error for missing args")
	}
	if err := runZipper([]string{"zipper", "unknown_cmd", "archive.zip"}); err == nil {
		t.Error("expected error for unknown command")
	}
	if err := runZipper([]string{"zipper", "c"}); err == nil {
		t.Error("expected error for missing archive name")
	}
	if err := runZipper([]string{"zipper", "c", "archive.zip"}); err == nil {
		t.Error("expected error for missing files to archive")
	}
	if err := runZipper([]string{"zipper", "a", "archive.zip"}); err == nil {
		t.Error("expected error for missing files to append")
	}
	if err := runZipper([]string{"zipper", "d", "archive.zip"}); err == nil {
		t.Error("expected error for missing files to delete")
	}
	if err := runZipper([]string{"zipper", "repair"}); err == nil {
		t.Error("expected error for missing repair file")
	}

	// Тестирование краевых случаев tar
	if err := runTar([]string{"tar"}); err == nil {
		t.Error("expected error for missing args")
	}
	if err := runTar([]string{"tar", "cf"}); err == nil {
		t.Error("expected error for missing archive path")
	}
	if err := runTar([]string{"tar", "-c"}); err == nil {
		t.Error("expected error for missing archive path")
	}

	// Тестирование краевых случаев unzip
	if err := runUnzip([]string{"unzip"}); err == nil {
		t.Error("expected error for missing zip")
	}

	// Тестирование краевых случаев zip
	if err := runZip([]string{"zip"}); err == nil {
		t.Error("expected error for missing zip")
	}

	// Тестирование парсера размеров
	if _, err := parseSize("invalid"); err == nil {
		t.Error("expected error for invalid size")
	}
	if v, _ := parseSize("10K"); v != 10240 {
		t.Errorf("got %d, want 10240", v)
	}
	if v, _ := parseSize("1M"); v != 1024*1024 {
		t.Errorf("got %d, want 1MB", v)
	}
	if v, _ := parseSize("1G"); v != 1024*1024*1024 {
		t.Errorf("got %d, want 1GB", v)
	}

	// Проверяем, что хелпы не паникуют, перехватывая их вывод для чистоты логов
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	showHelp("tar")
	showHelp("zip")
	showHelp("unzip")
	showHelp("zipper")

	w.Close()
	os.Stdout = oldStdout
}
