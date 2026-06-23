package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// generateFiles создает реалистичный набор данных для теста (без повторяющихся блоков)
func generateFiles(b *testing.B, dir string, count int, size int64) {
	os.MkdirAll(dir, 0755)

	chunk := make([]byte, 64*1024)

	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file_%d.bin", i))
		f, err := os.Create(path)
		if err != nil {
			b.Fatal(err)
		}

		var written int64
		var seed byte = 0
		for written < size {
			todo := size - written
			if todo > int64(len(chunk)) {
				todo = int64(len(chunk))
			}

			// 30% Имитация текста / кода с динамическим сдвигом символов
			textLen := int(float64(todo) * 0.3)
			for j := 0; j < textLen; j++ {
				if j%8 == 0 {
					chunk[j] = '\n'
				} else if j%2 == 0 {
					chunk[j] = ' '
				} else {
					chunk[j] = 'a' + byte((j+int(seed))%26)
				}
			}

			// 30% Нулевые байты
			zeroStart := textLen
			zeroEnd := zeroStart + int(float64(todo)*0.3)
			for j := zeroStart; j < zeroEnd; j++ {
				chunk[j] = 0x00
			}

			// 40% Уникальный рандом для каждого блока
			rand.Read(chunk[zeroEnd:todo])

			f.Write(chunk[:todo])
			written += todo
			seed++
		}
		f.Close()
	}
}

// reportStats считает и выводит размер архива и степень сжатия
func reportStats(b *testing.B, archivePath string, originalSize int64) {
	fi, err := os.Stat(archivePath)
	if err == nil {
		ratio := (float64(fi.Size()) / float64(originalSize)) * 100
		b.ReportMetric(float64(fi.Size())/(1024*1024), "MB_arc")
		b.ReportMetric(ratio, "%_ratio")
	}
}

func BenchmarkPerformance(b *testing.B) {
	p7z, _ := exec.LookPath("7z")
	pTar, _ := exec.LookPath("tar")

	tmp := b.TempDir()
	largeFileDir := filepath.Join(tmp, "large_data")
	const totalSize = 100 * 1024 * 1024 // 100 MB total
	const fileCount = 5
	const fileSize = totalSize / fileCount
	generateFiles(b, largeFileDir, fileCount, fileSize)

	// Предварительно создаем эталонные архивы для тестов распаковки
	zipArc := filepath.Join(tmp, "ref.zip")
	runZipper([]string{"zipper", "c", "-m", "deflate", "-C", largeFileDir, zipArc, "."})

	tgzArc := filepath.Join(tmp, "ref.tar.gz")
	runZipper([]string{"zipper", "c", "-m", "gzip", "-C", largeFileDir, tgzArc, "."})

	zstArc := filepath.Join(tmp, "ref.tar.zst")
	runZipper([]string{"zipper", "c", "-m", "zstd", "-C", largeFileDir, zstArc, "."})

	var sevenZipArc string
	if p7z != "" {
		sevenZipArc = filepath.Join(tmp, "ref.7z")
		os.Remove(sevenZipArc)
		// Создаем многопоточный не-солид архив (-ms=off) из 5 независимых файлов
		exec.Command(p7z, "a", "-t7z", "-m0=lzma2", "-ms=off", sevenZipArc, filepath.Join(largeFileDir, "*")).Run()
	}

	// ==========================================
	// --- 1. ZIP (Deflate) ---
	// ==========================================
	b.Run("ZIP_Pack_Zipper", func(b *testing.B) {
		b.SetBytes(totalSize)
		for i := 0; i < b.N; i++ {
			arc := filepath.Join(tmp, "perf.zip")
			os.Remove(arc)
			runZipper([]string{"zipper", "c", "-m", "deflate", "-C", largeFileDir, arc, "."})
			reportStats(b, arc, totalSize)
		}
	})

	if p7z != "" {
		b.Run("ZIP_Pack_Native_7z", func(b *testing.B) {
			b.SetBytes(totalSize)
			for i := 0; i < b.N; i++ {
				arc := filepath.Join(tmp, "perf_7z.zip")
				os.Remove(arc)
				exec.Command(p7z, "a", "-tzip", "-mm=Deflate", arc, filepath.Join(largeFileDir, "*")).Run()
				reportStats(b, arc, totalSize)
			}
		})
	}

	b.Run("ZIP_Unpack_Zipper", func(b *testing.B) {
		b.SetBytes(totalSize)
		for i := 0; i < b.N; i++ {
			out := filepath.Join(tmp, "out_zip_zpr")
			os.RemoveAll(out)
			runZipper([]string{"zipper", "x", "-C", out, zipArc})
		}
	})

	if p7z != "" {
		b.Run("ZIP_Unpack_Native_7z", func(b *testing.B) {
			b.SetBytes(totalSize)
			for i := 0; i < b.N; i++ {
				out := filepath.Join(tmp, "out_zip_7z")
				os.RemoveAll(out)
				exec.Command(p7z, "x", zipArc, "-o"+out, "-y").Run()
			}
		})
	}

	// ==========================================
	// --- 2. TAR.GZ (Gzip) ---
	// ==========================================
	b.Run("TGZ_Pack_Zipper", func(b *testing.B) {
		b.SetBytes(totalSize)
		for i := 0; i < b.N; i++ {
			arc := filepath.Join(tmp, "perf.tar.gz")
			os.Remove(arc)
			runZipper([]string{"zipper", "c", "-m", "gzip", "-C", largeFileDir, arc, "."})
			reportStats(b, arc, totalSize)
		}
	})

	if pTar != "" {
		b.Run("TGZ_Pack_Native_Tar", func(b *testing.B) {
			b.SetBytes(totalSize)
			for i := 0; i < b.N; i++ {
				arc := filepath.Join(tmp, "perf_tar.tar.gz")
				os.Remove(arc)
				exec.Command(pTar, "-czf", arc, "-C", largeFileDir, ".").Run()
				reportStats(b, arc, totalSize)
			}
		})
	}

	b.Run("TGZ_Unpack_Zipper", func(b *testing.B) {
		b.SetBytes(totalSize)
		for i := 0; i < b.N; i++ {
			out := filepath.Join(tmp, "out_tgz_zpr")
			os.RemoveAll(out)
			runZipper([]string{"zipper", "x", "-C", out, tgzArc})
		}
	})

	if pTar != "" {
		b.Run("TGZ_Unpack_Native_Tar", func(b *testing.B) {
			b.SetBytes(totalSize)
			for i := 0; i < b.N; i++ {
				out := filepath.Join(tmp, "out_tgz_tar")
				os.RemoveAll(out)
				os.MkdirAll(out, 0755)
				exec.Command(pTar, "-xzf", tgzArc, "-C", out).Run()
			}
		})
	}

	// ==========================================
	// --- 3. TAR.ZST (Zstandard) ---
	// ==========================================
	b.Run("ZST_Pack_Zipper", func(b *testing.B) {
		b.SetBytes(totalSize)
		for i := 0; i < b.N; i++ {
			arc := filepath.Join(tmp, "perf.tar.zst")
			os.Remove(arc)
			runZipper([]string{"zipper", "c", "-m", "zstd", "-C", largeFileDir, arc, "."})
			reportStats(b, arc, totalSize)
		}
	})

	if pTar != "" {
		b.Run("ZST_Pack_Native_Tar", func(b *testing.B) {
			// Проверяем, поддерживает ли системный tar упаковку в zstd
			testArc := filepath.Join(tmp, "test_zst.tar.zst")
			err := exec.Command(pTar, "--zstd", "-cf", testArc, "-C", largeFileDir, ".").Run()
			os.Remove(testArc)
			if err != nil {
				b.Skip("system tar does not support --zstd option")
			}

			b.SetBytes(totalSize)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				arc := filepath.Join(tmp, "perf_tar.tar.zst")
				os.Remove(arc)
				exec.Command(pTar, "--zstd", "-cf", arc, "-C", largeFileDir, ".").Run()
				reportStats(b, arc, totalSize)
			}
		})
	}

	b.Run("ZST_Unpack_Zipper", func(b *testing.B) {
		b.SetBytes(totalSize)
		for i := 0; i < b.N; i++ {
			out := filepath.Join(tmp, "out_zst_zpr")
			os.RemoveAll(out)
			runZipper([]string{"zipper", "x", "-C", out, zstArc})
		}
	})

	if pTar != "" {
		b.Run("ZST_Unpack_Native_Tar", func(b *testing.B) {
			// Проверяем, поддерживает ли системный tar распаковку zstd
			outTest := filepath.Join(tmp, "test_out_zst_tar")
			os.MkdirAll(outTest, 0755)
			err := exec.Command(pTar, "--zstd", "-xf", zstArc, "-C", outTest).Run()
			os.RemoveAll(outTest)
			if err != nil {
				b.Skip("system tar does not support --zstd option")
			}

			b.SetBytes(totalSize)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out := filepath.Join(tmp, "out_zst_tar")
				os.RemoveAll(out)
				os.MkdirAll(out, 0755)
				exec.Command(pTar, "--zstd", "-xf", zstArc, "-C", out).Run()
			}
		})
	}

	// ==========================================
	// --- 4. 7Z (LZMA2) ---
	// ==========================================
	b.Run("7Z_Pack_Zipper", func(b *testing.B) {
		b.SetBytes(totalSize)
		for i := 0; i < b.N; i++ {
			arc := filepath.Join(tmp, "perf.7z")
			os.Remove(arc)
			// Упаковываем в 7z силами Zipper (через fallback-архиватор mholt/archives)
			runZipper([]string{"zipper", "c", "-C", largeFileDir, arc, "."})
			reportStats(b, arc, totalSize)
		}
	})

	if p7z != "" {
		b.Run("7Z_Pack_Native_7z", func(b *testing.B) {
			b.SetBytes(totalSize)
			for i := 0; i < b.N; i++ {
				arc := filepath.Join(tmp, "perf_7z.7z")
				os.Remove(arc)
				// Для честного сравнения пакуем без солид-блоков (-ms=off)
				exec.Command(p7z, "a", "-t7z", "-m0=lzma2", "-ms=off", arc, filepath.Join(largeFileDir, "*")).Run()
				reportStats(b, arc, totalSize)
			}
		})
	}

	b.Run("7Z_Unpack_Zipper", func(b *testing.B) {
		b.SetBytes(totalSize)
		for i := 0; i < b.N; i++ {
			out := filepath.Join(tmp, "out_7z_zpr")
			os.RemoveAll(out)
			runZipper([]string{"zipper", "x", "-C", out, sevenZipArc})
		}
	})

	if p7z != "" {
		b.Run("7Z_Unpack_Native_7z", func(b *testing.B) {
			b.SetBytes(totalSize)
			for i := 0; i < b.N; i++ {
				out := filepath.Join(tmp, "out_7z_7z")
				os.RemoveAll(out)
				exec.Command(p7z, "x", sevenZipArc, "-o"+out, "-y").Run()
			}
		})
	}
}
