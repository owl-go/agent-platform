package workspace

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/objectstore/memory"
	platformserver "agent-platform/backend/internal/server"
	"agent-platform/backend/internal/skillstore"
)

type skillUploadRepository struct {
	workspaceapplication.Repository
	item  domain.Skill
	owner string
}

func (repository *skillUploadRepository) CreateSkill(_ context.Context, owner string, item domain.Skill) (domain.Skill, error) {
	item.ID, item.Version = "skill-1", 1
	repository.item, repository.owner = item, owner
	return item, nil
}

func (repository *skillUploadRepository) ListSkills(_ context.Context, owner string) ([]domain.Skill, error) {
	if owner != repository.owner {
		return nil, domain.ErrNotFound
	}
	return []domain.Skill{repository.item}, nil
}

func (repository *skillUploadRepository) UpdateSkill(_ context.Context, owner, id, name string, ref *string, key, digest string, version int64) (domain.Skill, error) {
	if owner != repository.owner || id != repository.item.ID || version != repository.item.Version {
		return domain.Skill{}, domain.ErrNotFound
	}
	repository.item.Name, repository.item.ObjectKey, repository.item.SHA256 = name, key, digest
	repository.item.Version++
	return repository.item, nil
}

func TestSkillUploadHTTPNormalizesFolderForCreateAndUpdate(t *testing.T) {
	provider := memory.New()
	store, err := skillstore.New(provider)
	if err != nil {
		t.Fatal(err)
	}
	repository := &skillUploadRepository{}
	application, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: application, skills: store}
	server := platformserver.NewHTTPServer(platformserver.HTTPConfig{}, nil)
	workspacev1.RegisterAgentWorkspaceServiceHTTPServer(server, service)

	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			var archive bytes.Buffer
			writer := zip.NewWriter(&archive)
			for _, name := range []string{"pdf/SKILL.md", "pdf/scripts/convert.py", "__MACOSX/pdf/._SKILL.md", "pdf/.DS_Store"} {
				entry, err := writer.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				content := method
				if name == "pdf/SKILL.md" {
					content = "---\ndisplay_name: PDF 文档处理\n---\n" + method
				}
				if _, err := io.WriteString(entry, content); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			url := "/api/v1/skills"
			payload := map[string]any{"source": "upload", "archive": archive.Bytes()}
			if method == http.MethodPatch {
				url += "/skill-1"
				payload = map[string]any{"archive": archive.Bytes(), "expected_version": repository.item.Version}
			}
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(method, url, bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: "owner-1"}))
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("Skill upload status = %d, body = %s", response.Code, response.Body.String())
			}
			stored, _, err := provider.Get(context.Background(), repository.item.ObjectKey)
			if err != nil {
				t.Fatal(err)
			}
			defer stored.Close()
			data, err := io.ReadAll(stored)
			if err != nil {
				t.Fatal(err)
			}
			reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			if len(reader.File) != 2 {
				t.Fatalf("stored %d files, want 2", len(reader.File))
			}
			root, err := reader.Open("SKILL.md")
			if err != nil {
				t.Fatalf("Skill root unavailable to Runtime: %v", err)
			}
			defer root.Close()
			content, err := io.ReadAll(root)
			if err != nil || string(content) != "---\ndisplay_name: PDF 文档处理\n---\n"+method {
				t.Fatalf("stored Skill contents = %q, error = %v", content, err)
			}
			if repository.item.Name != "PDF 文档处理" {
				t.Fatalf("Skill name = %q", repository.item.Name)
			}
		})
	}
}
