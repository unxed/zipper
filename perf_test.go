package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type DataProfile int

const (
	ProfileMixed DataProfile = iota
	ProfileText
	ProfileRand
)

type DatasetDef struct {
	Name        string
	FileCount   int
	TotalSize   int64
	DataProfile DataProfile
}

// Вспомогательные генераторы данных (очень быстрый LCG алгоритм)
func fastRandBytes(seed *uint32, buf []byte) {
	s := *seed
	for i := 0; i < len(buf); i++ {
		s = s*1664525 + 1013904223
		buf[i] = byte(s >> 24)
	}
	*seed = s
}

func fastTextBytes(seed *uint32, buf []byte) {
	s := *seed
	const alphabet = "abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOPQRSTUVWXYZ 0123456789 \n\t" // 64 chars = 6 bits entropy
	for i := 0; i < len(buf); i++ {
		s = s*1664525 + 1013904223
		buf[i] = alphabet[(s>>24)&63]
	}
	*seed = s
}

func generateDataset(b *testing.B, dir string, def DatasetDef) {
	os.MkdirAll(dir, 0755)
	fileSize := def.TotalSize / int64(def.FileCount)
	if fileSize == 0 {
		fileSize = 1
	}

	// Общая часть для симуляции Solid-сжатия (общие заголовки, импорты в исходниках)
	commonSize := fileSize / 2
	if commonSize > 64*1024 {
		commonSize = 64 * 1024 // Максимум 64KB общего префикса на файл
	}
	commonText := make([]byte, commonSize)
	seed := uint32(42)
	fastTextBytes(&seed, commonText)

	chunkSize := 1024 * 1024 // 1MB буфер для записи
	chunk := make([]byte, chunkSize)

	for i := 0; i < def.FileCount; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file_%05d.dat", i))
		f, err := os.Create(path)
		if err != nil {
			b.Fatal(err)
		}

		fileSeed := uint32(1337 + i*7919) // Уникальный seed для каждого файла

		var written int64
		for written < fileSize {
			todo := fileSize - written
			if todo > int64(chunkSize) {
				todo = int64(chunkSize)
			}
			c := chunk[:todo]

			switch def.DataProfile {
			case ProfileRand:
				// 100% непредсказуемые несжимаемые данные
				fastRandBytes(&fileSeed, c)
			case ProfileText:
				// Уникальный текст + общая вставка для тестирования Solid-режима
				fastTextBytes(&fileSeed, c)
				if written == 0 && len(c) >= len(commonText) {
					copy(c, commonText)
				}
			case ProfileMixed:
				rem := i % 3
				if rem == 0 {
					fastTextBytes(&fileSeed, c)
					if written == 0 && len(c) >= len(commonText) {
						copy(c, commonText)
					}
				} else if rem == 1 {
					for j := range c {
						c[j] = 0 // Идеально сжимаемые нули
					}
				} else {
					fastRandBytes(&fileSeed, c) // Абсолютно несжимаемый мусор
				}
			}

			f.Write(c)
			written += int64(len(c))
		}
		f.Close()
	}
}

func reportStats(b *testing.B, archivePath string, originalSize int64) {
	fi, err := os.Stat(archivePath)
	if err == nil {
		ratio := (float64(fi.Size()) / float64(originalSize)) * 100
		b.ReportMetric(ratio, "%_ratio")
	}
}

// runInternal вызывает функции проекта напрямую, подавляя вывод в консоль
func runInternal(args []string) error {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stdout = devNull
	os.Stderr = devNull
	defer func() {
		devNull.Close()
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	base := strings.ToLower(filepath.Base(args[0]))
	switch base {
	case "tar":
		return runTar(args)
	case "zip":
		return runZip(args)
	case "unzip":
		return runUnzip(args)
	default:
		return runZipper(args)
	}
}

func BenchmarkPerformance(b *testing.B) {
	pZip, _ := exec.LookPath("zip")
	pUnzip, _ := exec.LookPath("unzip")
	pTar, _ := exec.LookPath("tar")
	p7z, _ := exec.LookPath("7z")

	isFull := os.Getenv("ZIPPER_BENCH_FULL") == "1"
	warmupTmp := b.TempDir()
	warmupSrc := filepath.Join(warmupTmp, "warmup_src")
	_ = os.MkdirAll(warmupSrc, 0755)
	_ = os.WriteFile(filepath.Join(warmupSrc, "warmup.txt"), make([]byte, 1024*1024), 0644)
	warmupArc := filepath.Join(warmupTmp, "warmup.zip")
	_ = runInternal([]string{"zipper", "c", "-solid", "-C", warmupSrc, warmupArc, "warmup.txt"})
	_ = os.RemoveAll(warmupTmp)

	datasets := []DatasetDef{
		{Name: "10_Files_x_1MB_Mixed", FileCount: 10, TotalSize: 10 * 1024 * 1024, DataProfile: ProfileMixed},
	}

	if isFull {
		datasets = append(datasets,
    		DatasetDef{Name: "100_Files_x_1MB_Mixed", FileCount: 100, TotalSize: 100 * 1024 * 1024, DataProfile: ProfileMixed},
			DatasetDef{Name: "1_File_x_100MB_Mixed", FileCount: 1, TotalSize: 100 * 1024 * 1024, DataProfile: ProfileMixed},
			DatasetDef{Name: "100_Files_x_1MB_Rand", FileCount: 100, TotalSize: 100 * 1024 * 1024, DataProfile: ProfileRand},
			DatasetDef{Name: "10000_Files_x_10KB_Text", FileCount: 10000, TotalSize: 100 * 1024 * 1024, DataProfile: ProfileText},
		)
	}

	tmpBase := b.TempDir()

	for _, ds := range datasets {
		b.Run(ds.Name, func(b *testing.B) {
			srcDir := filepath.Join(tmpBase, ds.Name+"_src")
			generateDataset(b, srcDir, ds)

			// Helper для запуска тестов
			runTest := func(b *testing.B, name string, isInternal bool, packArgs []string, unpackArgs []string) {
				b.Run(name, func(b *testing.B) {
					ext := ".zip"
					if strings.Contains(strings.ToLower(name), "tar") {
						ext = ".tar.zst"
					} else if strings.Contains(strings.ToLower(name), "7z") {
						ext = ".7z"
					}
					arcPath := filepath.Join(tmpBase, ds.Name+"_"+name+ext)
					outDir := filepath.Join(tmpBase, ds.Name+"_"+name+"_out")

					b.Run("Pack", func(b *testing.B) {
						b.SetBytes(ds.TotalSize)
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							b.StopTimer()
							os.Remove(arcPath)
							b.StartTimer()
							fullArgs := append([]string{}, packArgs...)
							fullArgs = append(fullArgs, arcPath, ".")

							if isInternal {
								// Прямой вызов функции
								oldWd, _ := os.Getwd()
								os.Chdir(srcDir)
								if err := runInternal(fullArgs); err != nil {
									os.Chdir(oldWd)
									b.Fatalf("internal pack failed: %v", err)
								}
								os.Chdir(oldWd)
							} else {
								// Внешняя утилита
								cmd := exec.Command(fullArgs[0], fullArgs[1:]...)
								cmd.Dir = srcDir
								if err := cmd.Run(); err != nil {
									b.Fatalf("external pack failed: %v", err)
								}
							}
						}
						b.StopTimer()
						reportStats(b, arcPath, ds.TotalSize)
					})

					// Гарантируем наличие архива для распаковки
					if _, err := os.Stat(arcPath); os.IsNotExist(err) {
						oldWd, _ := os.Getwd()
						os.Chdir(srcDir)
						runInternal(append(append([]string{}, packArgs...), arcPath, "."))
						os.Chdir(oldWd)
					}

					b.Run("Unpack", func(b *testing.B) {
						b.SetBytes(ds.TotalSize)
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							b.StopTimer()
							os.RemoveAll(outDir)
							os.MkdirAll(outDir, 0755)
							b.StartTimer()


							if isInternal {
								fullArgs := append([]string{}, unpackArgs...)
								oldWd, _ := os.Getwd()
								if unpackArgs[0] == "unzip" {
									fullArgs = append(fullArgs, arcPath, "-d", outDir)
								} else if unpackArgs[0] == "tar" {
									// tar mimicry doesn't support -C, change dir manually
									os.MkdirAll(outDir, 0755)
									os.Chdir(outDir)
									fullArgs = append(fullArgs, "-xf", arcPath)
								} else {
									fullArgs = append(fullArgs, "-C", outDir, arcPath)
								}
								err := runInternal(fullArgs)
								os.Chdir(oldWd)
								if err != nil {
									b.Fatalf("internal unpack failed: %v", err)
								}
							} else {
								prog := unpackArgs[0]
								var cmd *exec.Cmd
								if filepath.Base(prog) == "unzip" {
									cmd = exec.Command(prog, "-q", arcPath, "-d", outDir)
								} else if filepath.Base(prog) == "7z" {
									cmd = exec.Command(prog, "x", arcPath, "-o"+outDir, "-y")
								} else if filepath.Base(prog) == "tar" {
									cmd = exec.Command(prog, "--zstd", "-xf", arcPath, "-C", outDir)
								}
								if err := cmd.Run(); err != nil {
									b.Fatalf("external unpack failed: %v", err)
								}
							}
						}
					})
				})
			}

			// 1. ZIP (Internal vs Native)
			runTest(b, "ZIP_Zipper", true, []string{"zip", "-1"}, []string{"unzip"})
			if pZip != "" && pUnzip != "" {
				runTest(b, "ZIP_Native", false, []string{pZip, "-q", "-r", "-1"}, []string{pUnzip})
			}

			// 2. Zipper Advanced (Internal)
			runTest(b, "ZSTD_Zipper_Solid", true, []string{"zipper", "c", "-solid", "-l", "1", "-m", "zstd"}, []string{"zipper", "x"})
			runTest(b, "ZSTD_Zipper_Chunked", true, []string{"zipper", "c", "-solid", "-l", "1", "-seek-chunk", "1048576", "-m", "zstd"}, []string{"zipper", "x"})

			// 3. TAR (Internal vs Native)
			runTest(b, "TAR_Internal", true, []string{"tar", "--zstd", "-cf"}, []string{"tar"})
			if pTar != "" {
				runTest(b, "TAR_Native", false, []string{pTar, "--zstd", "-cf"}, []string{pTar})
			}

			// 4. 7Z (External)
			if p7z != "" {
				runTest(b, "7Z_Native_Solid", false, []string{p7z, "a", "-t7z", "-ms=on", "-bso0"}, []string{p7z})
				runTest(b, "7Z_Native_Files", false, []string{p7z, "a", "-t7z", "-ms=off", "-bso0"}, []string{p7z})
			}
			runTest(b, "7Z_Zipper_Solid", true, []string{"zipper", "c", "-solid"}, []string{"zipper", "x"})
			runTest(b, "7Z_Zipper_Files", true, []string{"zipper", "c"}, []string{"zipper", "x"})
		})
	}
}
