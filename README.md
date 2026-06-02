# zipper

A high-fidelity cross-platform console archiver built on top of high-performance pure-Go compression libraries. It is designed to act as a demonstration app for `unxed/zip` and `unxed/tar` while providing binary mimicry for seamless integration into legacy scripts.

## Key Features
*   **Dual Nature:** Acts as a modern CLI (`zipper`) but changes its behavior dynamically to match `zip`, `unzip`, or `tar` depending on the name of the executable being invoked (e.g., via symlinks).
*   **Highly Concurrent:** Parallel compression and extraction out-of-the-box using GOMAXPROCS concurrency limits.
*   **Strict Metadata Preservation:** Saves and restores platform-specific attributes including Extended Attributes (xattrs), POSIX ACLs, NTFS Security Descriptors, UID/GID, owner strings, symlinks, and hardlinks.
*   **WinZip AES & Central Directory Encryption:** Full support for AES-256 encrypted archives and invisible file lists (CDE) without disclosing filenames.
*   **Solid ZIP-in-ZIP Mode:** Transparently wraps uncompressed file tables into a single highly compressed `Solid.zip` block to maximize compression ratios while preserving metadata.
*   **Seekable Solid Access:** Uses seek index maps (`-seek-chunk`, `-seek-continuous`) to jump directly to any block within compressed streams in O(1) time without full decompression.
*   **Incremental Backups:** Synchronizes changes against a `.zip_dumpdir` manifest, automatically deleting removed files on incremental restore.
*   **Resilient Extraction:** Safe atomic writes, sparse blocks (skipping over zero blocks), and tolerant extraction of partially corrupted archives.

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
```
