package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// generateFiles создает набор файлов для теста
func generateFiles(t *testing.B, dir string, count int, size int64) {
	os.MkdirAll(dir, 0755)
	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file_%d.bin", i))
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		// Заполняем случайными данными, чтобы сжатие реально работало
		buf := make([]byte, 4096)
		for written := int64(0); written < size; {
			todo := size - written
			if todo > 4096 {
				todo = 4096
			}
			rand.Read(buf[:todo])
			f.Write(buf[:todo])
			written += todo
		}
		f.Close()
	}
}

func BenchmarkCompareWith7z(b *testing.B) {
	p7z, err := exec.LookPath("7z")
	if err != nil {
		b.Skip("7z not found in PATH, skipping comparison")
	}

	tmp := b.TempDir()
	smallFilesDir := filepath.Join(tmp, "small_files")
	generateFiles(b, smallFilesDir, 1000, 4096)
	largeFileDir := filepath.Join(tmp, "large_file")
	generateFiles(b, largeFileDir, 1, 100*1024*1024)

	// --- 1. ZIP (Deflate) - Прямое сравнение ---
	b.Run("ZIP_Deflate_Create_Zipper", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			arc := filepath.Join(tmp, "perf_zipper.zip")
			os.Remove(arc)
			runZipper([]string{"zipper", "c", "-m", "deflate", "-C", largeFileDir, arc, "."})
		}
	})
	b.Run("ZIP_Deflate_Create_7z", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			arc := filepath.Join(tmp, "perf_7z.zip")
			os.Remove(arc)
			exec.Command(p7z, "a", "-tzip", "-mm=Deflate", arc, largeFileDir+string(filepath.Separator)+"*").Run()
		}
	})

	zipArc := filepath.Join(tmp, "ref.zip")
	runZipper([]string{"zipper", "c", "-m", "deflate", "-C", largeFileDir, zipArc, "."})

	b.Run("ZIP_Deflate_Extract_Zipper", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			out := filepath.Join(tmp, "out_zip_zipper")
			os.RemoveAll(out)
			runZipper([]string{"zipper", "x", "-C", out, zipArc})
		}
	})
	b.Run("ZIP_Deflate_Extract_7z", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			out := filepath.Join(tmp, "out_zip_7z")
			os.RemoveAll(out)
			exec.Command(p7z, "x", zipArc, "-o"+out, "-y").Run()
		}
	})

	// --- 2. TAR.ZST (Zipper) - Наш стандарт ---
	b.Run("TAR_ZSTD_Create_Zipper", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			arc := filepath.Join(tmp, "perf.tar.zst")
			os.Remove(arc)
			runZipper([]string{"zipper", "c", "-m", "zstd", "-C", largeFileDir, arc, "."})
		}
	})
	tarZstArc := filepath.Join(tmp, "ref.tar.zst")
	runZipper([]string{"zipper", "c", "-m", "zstd", "-C", largeFileDir, tarZstArc, "."})

	b.Run("TAR_ZSTD_Extract_Zipper", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			out := filepath.Join(tmp, "out_zst_zipper")
			os.RemoveAll(out)
			runZipper([]string{"zipper", "x", "-C", out, tarZstArc})
		}
	})

	// --- 3. 7Z (LZMA2) - Флагман 7-zip ---
	b.Run("7Z_LZMA2_Create_7z", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			arc := filepath.Join(tmp, "perf.7z")
			os.Remove(arc)
			exec.Command(p7z, "a", "-t7z", "-m0=lzma2", arc, largeFileDir+string(filepath.Separator)+"*").Run()
		}
	})
	sevenZipArc := filepath.Join(tmp, "ref.7z")
	exec.Command(p7z, "a", "-t7z", "-m0=lzma2", sevenZipArc, largeFileDir+string(filepath.Separator)+"*").Run()

	b.Run("7Z_LZMA2_Extract_7z", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			out := filepath.Join(tmp, "out_7z_7z")
			os.RemoveAll(out)
			exec.Command(p7z, "x", sevenZipArc, "-o"+out, "-y").Run()
		}
	})

	// --- 4. Мелкие файлы (ZIP) ---
	b.Run("SmallFiles_ZIP_Create_Zipper", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			arc := filepath.Join(tmp, "small_zipper.zip")
			os.Remove(arc)
			runZipper([]string{"zipper", "c", "-C", smallFilesDir, arc, "."})
		}
	})
	b.Run("SmallFiles_ZIP_Create_7z", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			arc := filepath.Join(tmp, "small_7z.zip")
			os.Remove(arc)
			exec.Command(p7z, "a", "-tzip", arc, smallFilesDir+string(filepath.Separator)+"*").Run()
		}
	})
}