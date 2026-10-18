package runtimeexecutor

import (
	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
	"agent-platform/backend/internal/skillstore"
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoordinatedMembersMaterializeOnlyTheirFrozenSkillsAndCleanThem(t *testing.T) {
	executor, job, _ := coordinatedFixture(t)
	objects := memory.New()
	executor.objects = objects
	var raw bytes.Buffer
	writer := zip.NewWriter(&raw)
	for name, content := range map[string]string{"SKILL.md": "---\ndisplay_name: Private Review\n---\n# Review\nReview evidence.\n", "scripts/check.sh": "#!/bin/sh\nexit 0\n"} {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0755)
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = file.Write([]byte(content))
	}
	_ = writer.Close()
	archive, digest, _, err := skillstore.ValidateUpload(context.Background(), raw.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive)
	if hex.EncodeToString(sum[:]) != digest {
		t.Fatal("invalid fixture")
	}
	key := "expert-packages/fixture/review.zip"
	if _, err := objects.Put(context.Background(), key, bytes.NewReader(archive), objectstore.PutOptions{Size: int64(len(archive)), SHA256: digest, ContentType: "application/zip"}); err != nil {
		t.Fatal(err)
	}
	job.Snapshot.Stages[1].Skills = []domain.SkillSnapshot{{ID: "review-bundle", Name: "Private Review", SHA256: digest, ObjectKey: key}}
	leadCalls := 0
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(ctx context.Context, request agentruntime.ExecuteRequest, sink agentruntime.EventSink) (agentruntime.Result, error) {
			isLead := strings.Contains(request.Instruction, "# Lead Guidance")
			found := false
			err := filepath.WalkDir(executor.materializer.Root, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if strings.HasSuffix(filepath.ToSlash(path), "skills/review-bundle/scripts/check.sh") {
					found = true
					info, err := entry.Info()
					if err != nil {
						return err
					}
					if info.Mode().Perm()&0111 == 0 {
						t.Fatal("frozen executable Skill lost its mode")
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if found == isLead {
				t.Fatal("Skill was absent for member or leaked to lead")
			}
			if isLead {
				leadCalls++
				if leadCalls == 1 {
					return coordinationRuntimeResult(t, sink, request.RunID, `{"action":"delegate","tasks":[{"id":"review","member_id":"reviewer","instruction":"Review","required":true}]}`), nil
				}
				return coordinationRuntimeResult(t, sink, request.RunID, `{"action":"complete","response":"Reviewed"}`), nil
			}
			return coordinationRuntimeResult(t, sink, request.RunID, "Review result"), nil
		}}, nil
	}
	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if result.SuccessCommit != nil {
		defer result.SuccessCommit.Cleanup()
	}
	files := 0
	_ = filepath.WalkDir(executor.materializer.Root, func(_ string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			files++
		}
		return err
	})
	if files != 0 {
		t.Fatal("Skill credentials remained after execution")
	}
	_ = os.Getuid()
}
