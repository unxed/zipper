package archive

import (
	"context"
	"os"

	"github.com/unxed/tar"
)

type tarArchiver struct {
	a        *tar.Archiver
	filename string
	opts     Options
}

func NewTarArchiver(filename, chroot string, opts Options) (Archiver, error) {
	var topts []tar.ArchiverOption
	topts = append(topts, tar.WithArchiverXattrs(opts.Xattrs))

	// Прокидываем процент восстановления в опции через публичную функцию-опцию
	if opts.RecoveryPct > 0 && !opts.RecoveryExternal {
		topts = append(topts, tar.WithArchiverRecovery(opts.RecoveryPct))
	}

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
	if opts.SplitSize > 0 {
		topts = append(topts, tar.WithArchiverSplitSize(opts.SplitSize))
	}
	if opts.Lock {
		topts = append(topts, tar.WithArchiverLock(true))
	}
	if opts.Level != 0 {
		topts = append(topts, tar.WithArchiverLevel(opts.Level))
	}
	if opts.PathMapping != nil {
		topts = append(topts, tar.WithArchiverPathMapping(opts.PathMapping))
	}

	a, err := tar.NewArchiver(filename, chroot, topts...)
	if err != nil {
		return nil, err
	}
	return &tarArchiver{a: a, filename: filename, opts: opts}, nil
}

func (t *tarArchiver) Archive(ctx context.Context, files map[string]os.FileInfo) error {
	return t.a.Archive(ctx, files)
}

func (t *tarArchiver) Close() error {
	err := t.a.Close()
	if err == nil && t.opts.RecoveryPct > 0 {
		if t.opts.RecoveryExternal {
			err = GenerateExternalPar2(t.filename, t.opts.RecoveryPct)
		} else {
			err = tar.AppendTarRecoveryRecord(t.filename, t.opts.RecoveryPct)
		}
	}
	return err
}
func (t *tarArchiver) Written() (bytes, entries int64) {
	return t.a.Written()
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

	if opts.NoTimes {
		topts = append(topts, tar.WithExtractorNoTimes(true))
	}
	if opts.StripComponents > 0 {
		topts = append(topts, tar.WithExtractorStripComponents(opts.StripComponents))
	}
	if opts.MaxFileSize > 0 {
		topts = append(topts, tar.WithExtractorMaxFileSize(opts.MaxFileSize))
	}
	if opts.MaxRatio > 0 {
		topts = append(topts, tar.WithExtractorMaxRatio(opts.MaxRatio))
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

func (t *tarExtractor) Written() (bytes, entries int64) {
	return t.e.Written()
}
