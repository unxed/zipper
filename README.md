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
