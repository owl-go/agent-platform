package skillstore

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"io/fs"
	"reflect"
	"testing"

	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
)

type archiveEntry struct {
	name string
	body string
	mode fs.FileMode
}

func uploadArchive(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(file, entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestInstallUploadNormalizesSkillRoot(t *testing.T) {
	want := map[string]string{"SKILL.md": "---\ndisplay_name: PDF 文档处理\n---\n# PDF", "scripts/convert.py": "print('pdf')"}
	for _, test := range []struct {
		name     string
		prefix   string
		metadata bool
	}{
		{name: "root"},
		{name: "folder", prefix: "pdf/"},
		{name: "macOS folder", prefix: "pdf/", metadata: true},
		{name: "macOS root", metadata: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			entries := []archiveEntry{{name: test.prefix + "SKILL.md", body: want["SKILL.md"]}, {name: test.prefix + "scripts/convert.py", body: want["scripts/convert.py"], mode: 0755}}
			if test.prefix != "" {
				entries = append(entries, archiveEntry{name: test.prefix})
			}
			if test.metadata {
				entries = append(entries, archiveEntry{name: "__MACOSX/._pdf", body: "metadata"}, archiveEntry{name: "__MACOSX/" + test.prefix + "._SKILL.md", body: "metadata"}, archiveEntry{name: test.prefix + ".DS_Store", body: "metadata"}, archiveEntry{name: test.prefix + "._SKILL.md", body: "metadata"})
			}
			provider := memory.New()
			store, err := New(provider)
			if err != nil {
				t.Fatal(err)
			}
			key, digest, err := store.InstallUpload(context.Background(), "owner", uploadArchive(t, entries))
			if err != nil {
				t.Fatalf("Skill ZIP upload failed: %v", err)
			}
			body, object, err := provider.Get(context.Background(), key)
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()
			archive, err := io.ReadAll(body)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(archive)
			if digest != hex.EncodeToString(hash[:]) || object.SHA256 != digest || object.Size != int64(len(archive)) {
				t.Fatal("stored Skill digest or size does not match normalized archive")
			}
			reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, file := range reader.File {
				if file.Name == "scripts/convert.py" && file.Mode().Perm() != 0755 {
					t.Fatal("normalization changed script permissions")
				}
				content, err := file.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(content)
				content.Close()
				if err != nil {
					t.Fatal(err)
				}
				got[file.Name] = string(data)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("stored Skill files = %v, want %v", got, want)
			}
		})
	}
}

type recordingUploadProvider struct {
	objectstore.Provider
	puts int
}

func (provider *recordingUploadProvider) Put(ctx context.Context, key string, body io.Reader, options objectstore.PutOptions) (objectstore.Object, error) {
	provider.puts++
	return provider.Provider.Put(ctx, key, body, options)
}

func TestInstallUploadRejectsInvalidArchivesBeforeStorage(t *testing.T) {
	for _, test := range []struct {
		name    string
		entries []archiveEntry
	}{
		{name: "missing skill", entries: []archiveEntry{{name: "pdf/readme.md"}}},
		{name: "multiple skills", entries: []archiveEntry{{name: "pdf/SKILL.md"}, {name: "other/SKILL.md"}}},
		{name: "sibling file", entries: []archiveEntry{{name: "pdf/SKILL.md"}, {name: "readme.md"}}},
		{name: "nested skill", entries: []archiveEntry{{name: "repo/pdf/SKILL.md"}}},
		{name: "skill directory", entries: []archiveEntry{{name: "SKILL.md/"}}},
		{name: "skill symlink", entries: []archiveEntry{{name: "SKILL.md", body: "other.md", mode: fs.ModeSymlink | 0777}}},
		{name: "resource symlink", entries: []archiveEntry{{name: "pdf/SKILL.md"}, {name: "pdf/script", body: "/etc/passwd", mode: fs.ModeSymlink | 0777}}},
		{name: "duplicate", entries: []archiveEntry{{name: "SKILL.md"}, {name: "SKILL.md"}}},
		{name: "file directory conflict", entries: []archiveEntry{{name: "SKILL.md"}, {name: "scripts"}, {name: "scripts/convert.py"}}},
		{name: "traversal", entries: []archiveEntry{{name: "SKILL.md"}, {name: "../escape"}}},
		{name: "internal traversal", entries: []archiveEntry{{name: "pdf/SKILL.md"}, {name: "pdf/../escape"}}},
		{name: "absolute path", entries: []archiveEntry{{name: "SKILL.md"}, {name: "/escape"}}},
		{name: "backslash", entries: []archiveEntry{{name: "SKILL.md"}, {name: `..\escape`}}},
		{name: "drive path", entries: []archiveEntry{{name: "SKILL.md"}, {name: "C:/escape"}}},
		{name: "unsafe metadata", entries: []archiveEntry{{name: "SKILL.md"}, {name: "__MACOSX/../../escape"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := &recordingUploadProvider{Provider: memory.New()}
			store, err := New(provider)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := store.InstallUpload(context.Background(), "owner", uploadArchive(t, test.entries)); err == nil {
				t.Fatal("invalid archive accepted")
			}
			if provider.puts != 0 {
				t.Fatal("invalid archive persisted")
			}
		})
	}
}

func TestInstallUploadRejectsCorruptOrOversizedContents(t *testing.T) {
	valid := uploadArchive(t, []archiveEntry{{name: "SKILL.md", body: "# PDF"}})
	central := bytes.Index(valid, []byte("PK\x01\x02"))
	if central < 0 {
		t.Fatal("missing ZIP central directory")
	}
	corrupt := bytes.Clone(valid)
	corrupt[central+16] ^= 0xff
	expanded := bytes.Clone(valid)
	binary.LittleEndian.PutUint32(expanded[central+24:], maxArchiveSize+1)
	cumulative := uploadArchive(t, []archiveEntry{{name: "SKILL.md"}, {name: "resource"}})
	for offset := 0; ; {
		index := bytes.Index(cumulative[offset:], []byte("PK\x01\x02"))
		if index < 0 {
			break
		}
		index += offset
		binary.LittleEndian.PutUint32(cumulative[index+24:], maxArchiveSize/2+1)
		offset = index + 46
	}
	for name, archive := range map[string][]byte{"empty": nil, "not ZIP": []byte("not ZIP"), "CRC mismatch": corrupt, "expanded size": expanded, "cumulative size": cumulative, "compressed size": make([]byte, maxArchiveSize+1)} {
		t.Run(name, func(t *testing.T) {
			provider := &recordingUploadProvider{Provider: memory.New()}
			store, err := New(provider)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := store.InstallUpload(context.Background(), "owner", archive); err == nil {
				t.Fatal("invalid archive accepted")
			}
			if provider.puts != 0 {
				t.Fatal("invalid archive persisted")
			}
		})
	}
}

func TestInstallUploadHonorsCancellationBeforeStorage(t *testing.T) {
	provider := &recordingUploadProvider{Provider: memory.New()}
	store, err := New(provider)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = store.InstallUpload(ctx, "owner", uploadArchive(t, []archiveEntry{{name: "pdf/SKILL.md"}}))
	if err != context.Canceled || provider.puts != 0 {
		t.Fatalf("cancelled upload: error = %v, writes = %d", err, provider.puts)
	}
}
