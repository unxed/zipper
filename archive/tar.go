package archive

import (
	"context"
	"os"

	"github.com/unxed/tar"
)

type tarArchiver struct {
	a *tar.Archiver
}

func NewTarArchiver(filename, chroot string, opts Options) (Archiver, error) {
	var topts []tar.ArchiverOption
	topts = append(topts, tar.WithArchiverXattrs(opts.Xattrs))

	if opts.Method == "zstd" {
		topts = append(topts, tar.WithArchiverMethod(tar.ZSTD))
	} else if opts.Method == "gzip" {
		topts = append(topts, tar.WithArchiverMethod(tar.GZIP))
	} else if opts.Method == "xz" {
		topts = append(topts, tar.WithArchiverMethod(tar.XZ))
	} else if opts.Method == "bzip2" {
		topts = append(topts, tar.WithArchiverMethod(tar.BZIP2))
	} else if opts.Method == "store" {
		topts = append(topts, tar.WithArchiverMethod(tar.Store))
	} else {
		topts = append(topts, tar.WithArchiverMethod(tar.Store))
	}

	if opts.IndexPath != "" {
		topts = append(topts, tar.WithArchiverIndex(opts.IndexPath))
	}
	topts = append(topts, tar.WithArchiverEmbeddedIndex(opts.EmbeddedIdx))
	if opts.Password != "" {
		topts = append(topts, tar.WithArchiverPassword(opts.Password))
	}

	a, err := tar.NewArchiver(filename, chroot, topts...)
	if err != nil {
		return nil, err
	}
	return &tarArchiver{a: a}, nil
}

func (t *tarArchiver) Archive(ctx context.Context, files map[string]os.FileInfo) error {
	return t.a.Archive(ctx, files)
}

func (t *tarArchiver) Close() error {
	return t.a.Close()
}

type tarExtractor struct {
	e *tar.Extractor
}

func NewTarExtractor(filename, chroot string, opts Options) (Extractor, error) {
	var topts []tar.ExtractorOption
	if opts.Concurrency > 0 {
		topts = append(topts, tar.WithExtractorConcurrency(opts.Concurrency))
	}
	topts = append(topts, tar.WithExtractorXattrs(opts.Xattrs))
	if opts.KeepOldFiles {
		topts = append(topts, tar.WithExtractorKeepOldFiles(true))
	}
	if opts.KeepNewerFiles {
		topts = append(topts, tar.WithExtractorKeepNewerFiles(true))
	}
	if opts.KeepBroken {
		topts = append(topts, tar.WithExtractorKeepBroken(true))
	}
	if opts.Sparse {
		topts = append(topts, tar.WithExtractorSparse(true))
	}
	if opts.Tolerant {
		topts = append(topts, tar.WithExtractorTolerant(true))
	}
	if opts.SafeWrites {
		topts = append(topts, tar.WithExtractorSafeWrites(true))
	}
	if opts.UnlinkFirst {
		topts = append(topts, tar.WithExtractorUnlinkFirst(true))
	}
	if opts.NumericOwner {
		topts = append(topts, tar.WithExtractorNumericOwner(true))
	}
	if opts.Incremental {
		topts = append(topts, tar.WithExtractorIncremental(true))
	}
	if opts.Password != "" {
		topts = append(topts, tar.WithExtractorPassword(opts.Password))
	}

	e, err := tar.NewExtractor(filename, chroot, topts...)
	if err != nil {
		return nil, err
	}
	return &tarExtractor{e: e}, nil
}

func (t *tarExtractor) Extract(ctx context.Context) error {
	return t.e.Extract(ctx)
}

func (t *tarExtractor) Close() error {
	return t.e.Close()
}