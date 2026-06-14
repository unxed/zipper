# zipper
### ⚡ Quick Download (Nightly Builds)

| Platform | Format | Link |
| :--- | :--- | :--- |
| **Windows** | .zip | [amd64](https://github.com/unxed/zipper/releases/download/nightly/zipper-windows-amd64.zip) / [arm64](https://github.com/unxed/zipper/releases/download/nightly/zipper-windows-arm64.zip) |
| **macOS** | .tar.gz | [amd64](https://github.com/unxed/zipper/releases/download/nightly/zipper-darwin-amd64.tar.gz) / [arm64](https://github.com/unxed/zipper/releases/download/nightly/zipper-darwin-arm64.tar.gz) |
| **Linux** | .tar.gz | [amd64](https://github.com/unxed/zipper/releases/download/nightly/zipper-linux-amd64.tar.gz) / [arm64](https://github.com/unxed/zipper/releases/download/nightly/zipper-linux-arm64.tar.gz) |
| **FreeBSD** | .tar.gz | [amd64](https://github.com/unxed/zipper/releases/download/nightly/zipper-freebsd-amd64.tar.gz) / [arm64](https://github.com/unxed/zipper/releases/download/nightly/zipper-freebsd-arm64.tar.gz) |
| **DragonflyBSD** | .tar.gz | [amd64](https://github.com/unxed/zipper/releases/download/nightly/zipper-dragonfly-amd64.tar.gz) |
| **OpenBSD** | .tar.gz | [amd64](https://github.com/unxed/zipper/releases/download/nightly/zipper-openbsd-amd64.tar.gz) / [arm64](https://github.com/unxed/zipper/releases/download/nightly/zipper-openbsd-arm64.tar.gz) |
| **NetBSD** | .tar.gz | [amd64](https://github.com/unxed/zipper/releases/download/nightly/zipper-netbsd-amd64.tar.gz) / [arm64](https://github.com/unxed/zipper/releases/download/nightly/zipper-netbsd-arm64.tar.gz) |
| **Illumos** (experimental) | .tar.gz | [amd64](https://github.com/unxed/zipper/releases/download/nightly/zipper-illumos-amd64.tar.gz) |
| **Solaris** (experimental) | .tar.gz | [amd64](https://github.com/unxed/zipper/releases/download/nightly/zipper-solaris-amd64.tar.gz) |

*These builds are automated and represent the current state of the `main` branch.*

A high-fidelity cross-platform console archiver built on top of high-performance pure-Go compression libraries. It is designed to act as a demonstration app for `unxed/zip` and `unxed/tar` while providing binary mimicry for seamless integration into legacy scripts.

## Key Features
*   **Highly Concurrent:** Parallel compression and extraction out-of-the-box using GOMAXPROCS concurrency limits.
*   **Strict Metadata Preservation:** Saves and restores platform-specific attributes including Extended Attributes (xattrs), POSIX ACLs, NTFS Security Descriptors, UID/GID, owner strings, symlinks, and hardlinks.
*   **WinZip AES & Central Directory Encryption:** Full support for AES-256 encrypted archives and invisible file lists (CDE) without disclosing filenames.
*   **Solid ZIP-in-ZIP Mode:** Transparently wraps uncompressed file tables into a single highly compressed `Solid.zip` block to maximize compression ratios while preserving metadata.
*   **Seekable Solid Access:** Uses seek index maps (`-seek-chunk`, `-seek-continuous`) to jump directly to any block within compressed streams in O(1) time without full decompression.
*   **Incremental Backups:** Synchronizes changes against a `.zip_dumpdir` manifest, automatically deleting removed files on incremental restore.
*   **Resilient Extraction:** Safe atomic writes, sparse blocks (skipping over zero blocks), and tolerant extraction of partially corrupted archives.
*   **Mimics anything:** Acts as a modern CLI (`zipper`) but changes its behavior dynamically to match `zip`, `unzip`, or `tar` depending on the name of the executable being invoked (e.g., via symlinks).

## Formats coverage / platform specific features

| Feature | `unxed/tar` | `unxed/zip` |
| :--- | :--- | :--- |
| **NTFS ACLs** (Win) | `Get/SetFileSecurityW` \| PAX `MSWINDOWS.raw_sd` | `Get/SetFileSecurityW` \| Extra Field `0x4453` |
| **Alternative Data Streams** (Win) | `Find*StreamW` \| Virtual files (`file:stream`) | `Find*StreamW` \| Virtual files (`file:stream`) |
| **High-precision Timestamps** (Win/*nix) | `unix.Lutimes` \| PAX float64 UnixNano | `unix.Lutimes` / `Chtimes` \| Extra Fields `0x000a` / `0x5455` |
| **Symlinks** (Win/*nix) | `os.Symlink` \| TAR Typeflag `2` | `os.Symlink` \| Mode flag + File payload |
| **Hardlinks** (*nix) | `os.Link` \| TAR Typeflag `1` | `os.Link` \| `Store` method + Extra Field `0x000d` |
| **Special Files (Devices/FIFOs)** (*nix) | `unix.Mknod` \| TAR Typeflag `3`/`4`/`6` | `unix.Mknod` \| Extra Field `0x000d` |
| **Owner UID/GID** (*nix) | `os.Lchown` \| TAR header fields | `os.Lchown` \| Extra Field `0x7875` |
| **Owner/Group Names** (*nix) | `os.Lchown` \| TAR header fields | `os.Lchown` \| Extra Field `0x7817` ([spec](https://github.com/unxed/zip/blob/main/f4zip.md)) |
| **xattrs / POSIX ACLs** (*nix) | `Lget/setxattr`, `Extattr*` \| PAX `SCHILY.xattr` | `Lget/setxattr`, `Extattr*` \| Extra Field `0x7811` ([spec](https://github.com/unxed/zip/blob/main/f4zip.md)) |

## Design Philosophy

Historically, developers of major archiving utilities (such as RAR or 7-Zip) have focused on creating proprietary formats and maximizing compression ratios—a priority inherited from the floppy disk era. The `zipper` ecosystem (including `unxed/zip` and `unxed/tar`) takes the opposite approach. Instead of inventing a new archive format, this project focuses on enhancing the features and fidelity of existing, widely-adopted standards (`ZIP` and `TAR`) in a backward-compatible manner.

By aggregating metadata standards from various platforms and tools (including UNIX xattrs, Windows NTFS ACLs, `ratarmount` metadata, and `SOZip` random-access indexes) and implementing them within existing format specifications, we preserve high-fidelity file attributes across different operating systems. For example, Windows ACLs are made functional in both ZIP (via extra field `0x4453`) and TAR (via PAX headers), while UNIX xattrs are brought natively to ZIP archives. Integrating these features required very few custom extensions, as the existing specifications and popular formats already possessed the necessary primitives.

Additionally, this project prioritizes execution speed and random-access capabilities over marginally better compression ratios. To that end, `zipper` defaults to `.tar.zst` (TAR with Zstandard compression) for packaging. In the era of high-speed networks and high-capacity storage, a minor difference in archive size is often negligible compared to the substantial performance advantages offered by Zstandard. Furthermore, `.tar.zst` provides native or easily accessible compatibility across modern platforms: it is supported natively in recent Windows updates and Ubuntu, and on macOS via popular third-party archivers like Keka.

Because the tools are written in Go, they compile to single, zero-dependency binaries. If a system's default extraction utility lacks support for advanced metadata or compression algorithms, a user can quickly obtain the portable `zipper` binary for their platform to extract the archive.

For more details, refer to the technical specifications of our format extensions:
*   [f4 ZIP Extensions Specification](https://github.com/unxed/zip/blob/main/f4zip.md)
*   [f4 TAR Extensions Specification](https://github.com/unxed/tar/blob/main/f4tar.md)

An important note regarding development of extension standards: they must ensure backward compatibility and, whenever possible, utilize the standard extension mechanisms of the respective formats—except in cases where those capabilities are exhausted or do not allow the task to be accomplished efficiently.

## Use as a Go Library

The core archiver engine is designed as a standalone, reusable Go package located under the `./archive` directory. You can import `github.com/unxed/zipper/archive` into your own Go projects to get high-performance, concurrent, and high-fidelity archiving.

For detailed API specifications, interfaces, and code examples, see the [Archive Library Documentation](./archive/README.md).

## Build and Installation

Ensure you have Go (1.25 or newer) installed:

```bash
git clone --recursive https://github.com/unxed/f4.git
cd f4/zipper
go build -o zipper .
```

### Activating Mimicry Mode
To make `zipper` behave like standard system tools, simply create symbolic links or copy the binary under the respective names:

```bash
# On Unix-like systems
ln -s zipper tar
ln -s zipper zip
ln -s zipper unzip

# On Windows
copy zipper.exe tar.exe
copy zipper.exe zip.exe
copy zipper.exe unzip.exe
```

## Usage

### 1. Native CLI Mode (`zipper`)
*   **Create a Solid Zstandard ZIP with fast seek index:**
    ```bash
    zipper c -solid -seek-chunk 1048576 -m zstd archive.zip path/to/files
    ```
*   **Create an encrypted ZIP with hidden file list (CDE):**
    ```bash
    zipper c -e -p "secret_password" hidden.zip file.txt
    ```
*   **Tolerant incremental extraction:**
    ```bash
    zipper x -incremental -tolerant archive.zip
    ```

### 2. Mimicry Mode (`tar`, `zip`, `unzip`)
*   **Emulating Tar:**
    ```bash
    ./tar -czf archive.tar.gz file1.txt file2.txt
    ./tar -xzf archive.tar.gz
    ```
*   **Emulating Zip:**
    ```bash
    ./zip -P "mypass" -0 secure_store.zip raw.dat
    ```
*   **Emulating Unzip:**
    ```bash
    ./unzip -P "mypass" secure_store.zip -d /tmp/output
    ```
