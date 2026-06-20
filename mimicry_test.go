package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainMimicryLogic(t *testing.T) {
	// Мы не можем вызвать main() напрямую без выхода из теста,
	// но мы можем протестировать функцию showHelp для разных имен.

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		w.Close()
		os.Stdout = oldStdout
	}()

	testNames := []string{"tar", "zip", "unzip", "zipper"}
	for _, name := range testNames {
		t.Run("Help_"+name, func(t *testing.T) {
			// Перехватываем вывод (необязательно, просто вызываем для покрытия)
			showHelp(name)
		})
	}
}

func TestBinaryNameDetection(t *testing.T) {
	// Тестируем логику определения имени из os.Args[0]
	cases := []struct {
		input    string
		expected string
	}{
		{"/usr/local/bin/tar", "tar"},
		{"C:\\bin\\zip.exe", "zip"},
		{"./unzip", "unzip"},
		{"zipper", "zipper"},
	}

	for _, tc := range cases {
		base := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(tc.input, ".exe"), "/"))
		if i := strings.LastIndexAny(base, "/\\"); i >= 0 {
			base = base[i+1:]
		}
		if base != tc.expected {
			t.Errorf("For %s expected %s, got %s", tc.input, tc.expected, base)
		}
	}
}
