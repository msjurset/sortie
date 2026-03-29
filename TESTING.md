# Test Coverage

## Running Tests

```bash
# All tests
go test -v ./...

# Single package
go test -v ./internal/dispatcher/...

# With race detector
go test -race ./...
```

## Coverage by Action Type

| Action | Dispatch | Undo | Error Paths | Notes |
|--------|----------|------|-------------|-------|
| **move** | Content verified, source removed | Restores source, removes dest | | |
| **copy** | Content verified, source preserved | Removes copy, source preserved | | |
| **rename** | Content verified at new path | Renames back | | |
| **delete** | Source removed, in trash | Restores from trash | | |
| **compress** | .gz created, source removed | Decompresses and removes .gz | | |
| **extract** (zip) | Nested dirs, content verified | Removes extracted dir | Unsupported format, zip slip | |
| **extract** (tar.gz) | Content verified | (shared with zip) | | |
| **symlink** | Target verified, content via link | Removes link, source preserved | | |
| **chmod** | Mode applied, old mode stored | Restores original mode | Invalid mode string | |
| **checksum** | SHA256 hash verified exactly | Removes sidecar | Bad algorithm, custom dest, MD5, default algo | |
| **exec** | Template expansion, output verified | Cannot undo (tested) | Command failure | |
| **notify** | Desktop notification, default title | Cannot undo (tested) | | Requires osascript (skipped if absent) |
| **convert** | -- | Removes output | Missing tool, empty tool field | Requires external tool |
| **resize** | sips with real PNG | Removes output | | Skipped if sips absent |
| **watermark** | -- | Removes output | Missing overlay, missing tool | Requires composite |
| **ocr** | -- | Removes output | Missing tool | Requires tesseract |
| **encrypt** | -- | Removes output | Missing recipient, missing tool | |
| **decrypt** | -- | Removes output | Missing tool | |
| **encrypt+decrypt** | Full round-trip with age | Via round-trip | | Skipped if age absent |
| **upload** | -- | Cannot undo (tested) | Missing remote | Requires aws/gsutil |
| **tag** | xattr verified on disk | Cannot undo (tested) | Missing tags | Skipped if xattr absent |
| **open** | Default + specific app | Cannot undo (tested) | | Skipped if open absent |
| **deduplicate** | No-dup move, skip, delete | Move reversed; skip no-op; delete error | Missing dest, different content | |
| **unquarantine** | Attr removed, no-op if absent | Cannot undo (tested) | | Skipped if xattr absent |

## Additional Unit Tests

- `resizeDimension` -- all 5 dimension format variants (percentage, width+height, width only, height only, none)
- `detectUploadTool` -- s3/gs/default URI scheme detection
- `buildTagsPlist` -- correct XML plist structure
- `buildTagsPlistXMLEscape` -- special characters properly escaped
- `ExpandString` -- Path variable, name/ext, date variables, empty string
- `ExpandTemplate` -- year/month paths, date in filename, name+ext, conflict resolution
- `extractZipSlip` -- path traversal prevention (rejects `../` entries)
- `DryRun` -- no side effects when dry-run is true
- `hashFile` -- same content produces same hash, different content produces different hash

## External Tool Dependencies

The following tests require external tools and are skipped when the tool is not installed:

| Test | Tool | Install |
|------|------|---------|
| `TestDispatchResizeSips` | `sips` | Built into macOS |
| `TestDispatchNotifyDesktop` | `osascript` | Built into macOS |
| `TestDispatchNotifyDefaultTitle` | `osascript` | Built into macOS |
| `TestDispatchTagWithXattr` | `xattr` | Built into macOS |
| `TestDispatchEncryptDecryptRoundTrip` | `age`, `age-keygen` | `brew install age` |
| `TestDispatchOpenDefault` | `open` | Built into macOS |
| `TestDispatchOpenWithApp` | `open` | Built into macOS |
| `TestDispatchUnquarantine` | `xattr` | Built into macOS |
| `TestDispatchUnquarantineNoAttr` | `xattr` | Built into macOS |

## Rule Validation Tests

The `sortie validate` command and underlying `rule.ValidateRules()` function are tested in `internal/rule/validate_test.go`:

### Required Field Validation
Tests that all 16 action types with required fields produce errors when those fields are missing:
- move/copy/rename/symlink/extract without dest
- chmod without mode
- exec without command
- convert without tool
- resize without dimensions or dest
- watermark without overlay or dest
- encrypt without recipient
- upload without remote
- tag without tags
- deduplicate without dest

### Invalid Value Validation
- chmod with non-octal mode (`abc`, `0999`)
- checksum with unsupported algorithm (`sha512`)
- deduplicate with invalid on_duplicate value
- Unknown action type
- Octal mode edge cases (too short, too long)

### Chain Combination Validation
- delete not last in chain (error)
- compress not last in chain (warning)
- deduplicate not last in chain (warning)
- open not last in chain (warning: async race)
- Consecutive move/rename (warning)
- Valid chain produces no findings

### Infinite Loop Detection
- move dest inside watched directory
- copy dest inside watched directory
- Template dest with static prefix inside watched directory
- rename dest in exact watched directory
- Chain action dest inside watched directory
- Dest in different directory produces no warning

### Helper Function Tests
- `templateStaticPrefix` — extracts directory prefix before template variables
- `isSubpath` — checks parent/child path relationships
- `isValidOctalMode` — validates octal permission strings

## What Is Not Tested with Real Dispatch

The `convert`, `watermark`, `ocr`, and `upload` dispatch happy paths require external tools (`ffmpeg`, `composite`, `tesseract`, `aws`/`gsutil`) that are not assumed to be installed. For these actions, tests cover:

1. Correct error when the tool binary is missing
2. Correct error when required config fields are absent (e.g., overlay for watermark, recipient for encrypt)
3. Undo correctly removes the output file

To run full integration tests for these, install the tools and the existing tests will exercise them (tool-missing errors will not trigger when the tool is found).
