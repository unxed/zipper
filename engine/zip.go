package engine

import (
	"context"
	"os"

	"github.com/unxed/zip"
)

type zipArchiver struct {
	f *os.File
	a *zip.Archiver
}

func NewZipArchiver(filename, chroot string, opts Options) (Archiver, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	var zopts []zip.ArchiverOption
	if opts.Concurrency > 0 {
		zopts = append(zopts, zip.WithArchiverConcurrency(opts.Concurrency))
	}
	zopts = append(zopts, zip.WithArchiverXattrs(opts.Xattrs))
	if opts.Solid {
		zopts = append(zopts, zip.WithArchiverSolid(true))
	}

	if opts.Method == "zstd" {
		zopts = append(zopts, zip.WithArchiverMethod(zip.ZSTD))
	} else if opts.Method == "store" {
		zopts = append(zopts, zip.WithArchiverMethod(zip.Store))
	} else {
		zopts = append(zopts, zip.WithArchiverMethod(zip.Deflate))
	}

	a, err := zip.NewArchiver(f, chroot, zopts...)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &zipArchiver{f: f, a: a}, nil
}

func (z *zipArchiver) Archive(ctx context.Context, files map[string]os.FileInfo) error {
	return z.a.Archive(ctx, files)
}

func (z *zipArchiver) Close() error {
	err1 := z.a.Close()
	err2 := z.f.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

type zipExtractor struct {
	e *zip.Extractor
}

func NewZipExtractor(filename, chroot string, opts Options) (Extractor, error) {
	var zopts []zip.ExtractorOption
	if opts.Concurrency > 0 {
		zopts = append(zopts, zip.WithExtractorConcurrency(opts.Concurrency))
	}
	zopts = append(zopts, zip.WithExtractorXattrs(opts.Xattrs))

	e, err := zip.NewExtractor(filename, chroot, zopts...)
	if err != nil {
		return nil, err
	}
	return &zipExtractor{e: e}, nil
}

func (z *zipExtractor) Extract(ctx context.Context) error {
	return z.e.Extract(ctx)
}

func (z *zipExtractor) Close() error {
	return z.e.Close()
}