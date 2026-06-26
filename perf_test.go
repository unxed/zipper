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
	if s == 0 {
		s = 1 // Xorshift32 не должен инициализироваться нулем
	}
	for i := 0; i < len(buf); i++ {
		s ^= s << 13
		s ^= s >> 17
		s ^= s << 5
		buf[i] = byte(s)
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
					clear(c) // Идеально сжимаемые нули
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

func cleanupLeftoverTempFiles() {
	files, err := filepath.Glob(filepath.Join(os.TempDir(), "zipper-stdin-*.tmp"))
	if err == nil {
		for _, f := range files {
			_ = os.Remove(f)
		}
	}
	files, err = filepath.Glob(filepath.Join(os.TempDir(), "f4crypt-zip-*.tmp"))
	if err == nil {
		for _, f := range files {
			_ = os.Remove(f)
		}
	}
}

func BenchmarkPerformance(b *testing.B) {
	cleanupLeftoverTempFiles()

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
		{Name: "100_Files_x_512KB_Mixed", FileCount: 100, TotalSize: 50 * 1024 * 1024, DataProfile: ProfileMixed},
	}

	if isFull {
		datasets = append(datasets,
			DatasetDef{Name: "1_File_x_50MB_Mixed", FileCount: 1, TotalSize: 50 * 1024 * 1024, DataProfile: ProfileMixed},
			DatasetDef{Name: "100_Files_x_512KB_Rand", FileCount: 100, TotalSize: 50 * 1024 * 1024, DataProfile: ProfileRand},
			DatasetDef{Name: "10000_Files_x_5KB_Text", FileCount: 10000, TotalSize: 50 * 1024 * 1024, DataProfile: ProfileText},
		)
	}

	tmpBase := b.TempDir()

	for _, ds := range datasets {
		b.Run(ds.Name, func(b *testing.B) {
			srcDir := filepath.Join(tmpBase, ds.Name+"_src")
			generateDataset(b, srcDir, ds)

			type ToolDef struct {
				Name       string
				IsInternal bool
				PackArgs   []string
				UnpackArgs []string
			}

			tools := []ToolDef{}

			// 1. ZIP (Deflate) - Native vs Zipper
			if pZip != "" && pUnzip != "" {
				tools = append(tools, ToolDef{
					Name:       "ZIP_Deflate_Native",
					IsInternal: false,
					PackArgs:   []string{pZip, "-q", "-r", "-1"},
					UnpackArgs: []string{pUnzip},
				})
			}
			tools = append(tools, ToolDef{
				Name:       "ZIP_Deflate_Zipper",
				IsInternal: true,
				PackArgs:   []string{"zip", "-1"},
				UnpackArgs: []string{"unzip"},
			})

			// 3. TAR (Zstd and Gzip) - Native vs Zipper
			if pTar != "" {
				tools = append(tools, ToolDef{
					Name:       "TAR_ZSTD_Native",
					IsInternal: false,
					PackArgs:   []string{pTar, "--zstd", "-cf"},
					UnpackArgs: []string{pTar},
				})
				tools = append(tools, ToolDef{
					Name:       "TAR_ZSTD_Zipper",
					IsInternal: true,
					PackArgs:   []string{"tar", "--zstd", "-cf"},
					UnpackArgs: []string{"tar"},
				})
				tools = append(tools, ToolDef{
					Name:       "TAR_GZIP_Native",
					IsInternal: false,
					PackArgs:   []string{pTar, "-czf"},
					UnpackArgs: []string{pTar},
				})
				tools = append(tools, ToolDef{
					Name:       "TAR_GZIP_Zipper",
					IsInternal: true,
					PackArgs:   []string{"tar", "-czf"},
					UnpackArgs: []string{"tar"},
				})
			}

			// 4. 7Z (LZMA) - Native vs Zipper (Solid & Non-Solid)
			if p7z != "" {
				tools = append(tools, ToolDef{
					Name:       "7Z_LZMA_Native_Solid",
					IsInternal: false,
					PackArgs:   []string{p7z, "a", "-t7z", "-ms=on", "-bso0"},
					UnpackArgs: []string{p7z},
				})
				tools = append(tools, ToolDef{
					Name:       "7Z_LZMA_Zipper_Solid",
					IsInternal: true,
					PackArgs:   []string{"zipper", "c", "-solid"},
					UnpackArgs: []string{"zipper", "x"},
				})
				tools = append(tools, ToolDef{
					Name:       "7Z_LZMA_Native_Files",
					IsInternal: false,
					PackArgs:   []string{p7z, "a", "-t7z", "-ms=off", "-bso0"},
					UnpackArgs: []string{p7z},
				})
				tools = append(tools, ToolDef{
					Name:       "7Z_LZMA_Zipper_Files",
					IsInternal: true,
					PackArgs:   []string{"zipper", "c", "-non-solid"},
					UnpackArgs: []string{"zipper", "x"},
				})
			}

			// 5. ZIP Advanced (Zstd, Solid/Chunked)
			tools = append(tools, ToolDef{
				Name:       "ZIP_ZSTD_Zipper_Solid",
				IsInternal: true,
				PackArgs:   []string{"zipper", "c", "-solid", "-l", "1", "-m", "zstd"},
				UnpackArgs: []string{"zipper", "x"},
			})
			tools = append(tools, ToolDef{
				Name:       "ZIP_ZSTD_Zipper_Chunked",
				IsInternal: true,
				PackArgs:   []string{"zipper", "c", "-solid", "-l", "1", "-seek-chunk", "1048576", "-m", "zstd"},
				UnpackArgs: []string{"zipper", "x"},
			})

			// Run Pack benchmarks for all tools
			b.Run("Pack", func(b *testing.B) {
				for _, tdef := range tools {
					b.Run(tdef.Name, func(b *testing.B) {
						ext := ".zip"
						if strings.Contains(tdef.Name, "TAR_GZIP") {
							ext = ".tar.gz"
						} else if strings.Contains(tdef.Name, "TAR_ZSTD") {
							ext = ".tar.zst"
						} else if strings.Contains(strings.ToLower(tdef.Name), "7z") {
							ext = ".7z"
						}
						arcPath := filepath.Join(tmpBase, ds.Name+"_"+tdef.Name+ext)

						b.SetBytes(ds.TotalSize)
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							b.StopTimer()
							os.Remove(arcPath)
							b.StartTimer()
							fullArgs := append([]string{}, tdef.PackArgs...)
							fullArgs = append(fullArgs, arcPath, ".")

							if tdef.IsInternal {
								oldWd, _ := os.Getwd()
								os.Chdir(srcDir)
								if err := runInternal(fullArgs); err != nil {
									os.Chdir(oldWd)
									b.Fatalf("internal pack failed: %v", err)
								}
								os.Chdir(oldWd)
							} else {
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
				}
			})

			// Run Unpack benchmarks for all tools
			b.Run("Unpack", func(b *testing.B) {
				for _, tdef := range tools {
					ext := ".zip"
					if strings.Contains(tdef.Name, "TAR_GZIP") {
						ext = ".tar.gz"
					} else if strings.Contains(tdef.Name, "TAR_ZSTD") {
						ext = ".tar.zst"
					} else if strings.Contains(strings.ToLower(tdef.Name), "7z") {
						ext = ".7z"
					}
					arcPath := filepath.Join(tmpBase, ds.Name+"_"+tdef.Name+ext)
					outDir := filepath.Join(tmpBase, ds.Name+"_"+tdef.Name+"_out")

					// Ensure the archive is pre-generated (in case Pack step didn't run or was skipped)
					if _, err := os.Stat(arcPath); os.IsNotExist(err) {
						fullArgs := append([]string{}, tdef.PackArgs...)
						fullArgs = append(fullArgs, arcPath, ".")
						var genErr error
						if tdef.IsInternal {
							oldWd, _ := os.Getwd()
							os.Chdir(srcDir)
							genErr = runInternal(fullArgs)
							os.Chdir(oldWd)
						} else {
							cmd := exec.Command(fullArgs[0], fullArgs[1:]...)
							cmd.Dir = srcDir
							genErr = cmd.Run()
						}
						if genErr != nil {
							b.Fatalf("failed to pre-generate archive for unpack: %v", genErr)
						}
					}

					b.Run(tdef.Name, func(b *testing.B) {
						b.SetBytes(ds.TotalSize)
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							b.StopTimer()
							if err := os.RemoveAll(outDir); err != nil {
								b.Fatalf("cleanup failed: %v", err)
							}
							if err := os.MkdirAll(outDir, 0755); err != nil {
								b.Fatalf("mkdir failed: %v", err)
							}
							b.StartTimer()

							if tdef.IsInternal {
								fullArgs := append([]string{}, tdef.UnpackArgs...)
								oldWd, _ := os.Getwd()
								if tdef.UnpackArgs[0] == "unzip" {
									fullArgs = append(fullArgs, arcPath, "-d", outDir)
								} else if tdef.UnpackArgs[0] == "tar" {
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
								prog := tdef.UnpackArgs[0]
								var cmd *exec.Cmd
								if filepath.Base(prog) == "unzip" {
									cmd = exec.Command(prog, "-q", arcPath, "-d", outDir)
								} else if filepath.Base(prog) == "7z" {
									cmd = exec.Command(prog, "x", arcPath, "-o"+outDir, "-y")
								} else if filepath.Base(prog) == "tar" {
									if strings.Contains(tdef.Name, "GZIP") {
										cmd = exec.Command(prog, "-zxf", arcPath, "-C", outDir)
									} else {
										cmd = exec.Command(prog, "--zstd", "-xf", arcPath, "-C", outDir)
									}
								}
								if err := cmd.Run(); err != nil {
									b.Fatalf("external unpack failed: %v", err)
								}
							}
						}
					})
				}
			})
		})
	}
}
