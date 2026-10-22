package archive

import (
	"context"
	"testing"
)

func TestExtractorsImplementCurrentFiler(t *testing.T) {
	for _, name := range []string{"fixture.zip", "fixture.tar"} {
		t.Run(name, func(t *testing.T) {
			archivePath := archiveForIncludeTest(t, name)
			extractor, err := NewExtractor(archivePath, t.TempDir(), Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer extractor.Close()

			current, ok := extractor.(CurrentFiler)
			if !ok {
				t.Fatalf("%T does not implement CurrentFiler", extractor)
			}
			if got := current.CurrentFile(); got != "" {
				t.Fatalf("CurrentFile() before extraction = %q, want empty", got)
			}
		})
	}
}

func TestFallbackCurrentFilerReportsExtractedPath(t *testing.T) {
	archivePath := archiveForIncludeTest(t, "fixture.zip")
	extractor, err := NewFallbackExtractor(archivePath, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer extractor.Close()

	current, ok := extractor.(CurrentFiler)
	if !ok {
		t.Fatalf("%T does not implement CurrentFiler", extractor)
	}
	if err := extractor.Extract(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := current.CurrentFile(); got == "" {
		t.Fatal("CurrentFile() after extraction is empty")
	}
}
