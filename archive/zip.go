package archive

import (
	"context"
	"os"
    "io"

	"github.com/unxed/zip"
)

type zipArchiver struct {
	f        io.WriteCloser
	a        *zip.Archiver
	filename string
	opts     Options
}

func NewZipArchiver(filename, chroot string, opts Options) (Archiver, error) {
	var f interface {
		io.WriteCloser
		Name() string
	}
	var err error

	if opts.SplitSize > 0 {
		f, err = zip.NewMultiVolumeWriter(filename, opts.SplitSize)
	} else {
		f, err = os.Create(filename)
	}
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
	if opts.Incremental {
		zopts = append(zopts, zip.WithArchiverIncremental(true))
	}
	if opts.Password != "" {
		zopts = append(zopts, zip.WithArchiverPassword(opts.Password))
	}
	if opts.EncryptCD {
		zopts = append(zopts, zip.WithArchiverEncryptCD(true))
	}
	if opts.SeekChunkSize > 0 {
		zopts = append(zopts, zip.WithArchiverSeekIndex(opts.SeekChunkSize, opts.SeekContinuous))
	}
	if opts.TorrentZip {
		zopts = append(zopts, zip.WithArchiverTorrentZip(true))
	}
	if opts.NoPlatformMetadata {
		zopts = append(zopts, zip.WithArchiverPlatformMetadata(false))
	}

	if opts.Method == "zstd" {
		zopts = append(zopts, zip.WithArchiverMethod(zip.ZSTD))
	} else if opts.Method == "store" {
		zopts = append(zopts, zip.WithArchiverMethod(zip.Store))
	} else {
		zopts = append(zopts, zip.WithArchiverMethod(zip.Deflate))
	}

	if opts.RecoveryPct > 0 && !opts.RecoveryExternal {
		zopts = append(zopts, zip.WithArchiverRecovery(opts.RecoveryPct, f))
	}

	a, err := zip.NewArchiver(f, chroot, zopts...)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &zipArchiver{f: f, a: a, filename: filename, opts: opts}, nil
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
	if err2 != nil {
		return err2
	}
	if z.opts.RecoveryPct > 0 && z.opts.RecoveryExternal {
		err := GenerateExternalPar2(z.filename, z.opts.RecoveryPct)
		if err != nil {
			return err
		}
	}
	return nil
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
	if opts.KeepOldFiles {
		zopts = append(zopts, zip.WithExtractorKeepOldFiles(true))
	}
	if opts.KeepNewerFiles {
		zopts = append(zopts, zip.WithExtractorKeepNewerFiles(true))
	}
	if opts.KeepBroken {
		zopts = append(zopts, zip.WithExtractorKeepBroken(true))
	}
	if opts.Sparse {
		zopts = append(zopts, zip.WithExtractorSparse(true))
	}
	if opts.Tolerant {
		zopts = append(zopts, zip.WithExtractorTolerant(true))
	}
	if opts.SafeWrites {
		zopts = append(zopts, zip.WithExtractorSafeWrites(true))
	}
	if opts.UnlinkFirst {
		zopts = append(zopts, zip.WithExtractorUnlinkFirst(true))
	}
	if opts.NumericOwner {
		zopts = append(zopts, zip.WithExtractorNumericOwner(true))
	}
	if opts.Incremental {
		zopts = append(zopts, zip.WithExtractorIncremental(true))
	}
	if opts.Password != "" {
		zopts = append(zopts, zip.WithExtractorPassword(opts.Password))
	}

	if opts.NoTimes {
		zopts = append(zopts, zip.WithExtractorNoTimes(true))
	}
	if opts.StripComponents > 0 {
		zopts = append(zopts, zip.WithExtractorStripComponents(opts.StripComponents))
	}
	if opts.MaxFileSize > 0 {
		zopts = append(zopts, zip.WithExtractorMaxFileSize(opts.MaxFileSize))
	}
	if opts.MaxRatio > 0 {
		zopts = append(zopts, zip.WithExtractorMaxRatio(opts.MaxRatio))
	}
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