package gormrepo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/secretcrypto"
	"github.com/google/uuid"
)

func TestClaimExpertTagProjectionLoadsVersionedCredential(t *testing.T) {
	db := conversationTestDatabase(t)
	ctx := context.Background()
	repository := New(db, nil)
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	owner, credentialOwner, connection, model := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{owner, credentialOwner} {
		exec(`INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)`, id, id, id, id+"@example.test", id)
	}
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt([]byte("test-model-key"), "model-provider:"+credentialOwner)
	if err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO model_provider_connections(id,credential_owner_user_id,name,provider_type,endpoint,protocols,api_key_ciphertext,version) VALUES(?,?,'Provider','openai','https://example.test','["openai_responses"]','not-the-versioned-key',2)`, connection, credentialOwner)
	exec(`INSERT INTO model_provider_credential_versions(connection_id,connection_version,api_key_ciphertext) VALUES(?,2,?)`, connection, ciphertext)
	exec(`INSERT INTO provider_models(id,connection_id,model_id,display_name) VALUES(?,?,'model','Model')`, model, connection)
	defaults, _ := json.Marshal(map[string]string{"codex": model})
	exec(`INSERT INTO personal_settings(user_id,runtime_model_defaults) VALUES(?,?::jsonb)`, owner, string(defaults))
	for _, missingCredential := range []bool{false, true} {
		t.Run(map[bool]string{false: "available", true: "missing"}[missingCredential], func(t *testing.T) {
			if missingCredential {
				exec(`DELETE FROM model_provider_credential_versions WHERE connection_id=?`, connection)
			}
			expert := uuid.NewString()
			exec(`INSERT INTO experts(id,owner_user_id,name,name_normalized,introduction,core_capability,operating_procedure,output_standard,expertise_tags,tag_projection_status,tag_projection_requested_at) VALUES(?, ?, ?, ?, 'Intro','Architecture','Steps','Report','["Previous"]','queued',now())`, expert, owner, expert, strings.ToLower(expert))
			job, err := repository.ClaimNext(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if missingCredential {
				if job != nil {
					t.Fatal("tag projection with a missing credential was sent to the Runtime")
				}
			} else {
				if job == nil || job.Kind != application.JobExpertTagProjection || job.ExpertID != expert {
					t.Fatalf("expected Expert tag projection job, got %v", job)
				}
				stages, err := job.Snapshot.OrderedStages()
				if err != nil {
					t.Fatal(err)
				}
				provider := stages[0].ProviderModel
				key, err := box.Decrypt(provider.APIKeyCiphertext, "model-provider:"+provider.CredentialOwnerID)
				if err != nil {
					t.Fatalf("decrypt Model Provider credential: %v", err)
				}
				if string(key) != "test-model-key" || provider.ConnectionVersion != 2 || provider.CredentialOwnerID != credentialOwner {
					t.Fatal("tag projection did not resolve the versioned credential and its owner")
				}
				if err := repository.FinishExpertTagProjection(ctx, *job, application.ExecutionResult{FinalMessage: `["Architecture"]`}, ""); err != nil {
					t.Fatal(err)
				}
			}
			var row expertRecord
			if err := db.Where("id = ?", expert).Take(&row).Error; err != nil {
				t.Fatal(err)
			}
			if missingCredential {
				if row.TagProjectionStatus != "failed" || row.TagProjectionError == nil || !strings.Contains(*row.TagProjectionError, "load versioned Model Provider credential") || string(row.ExpertiseTags) != `["Previous"]` {
					t.Fatal("missing credential did not record a useful failure while retaining previous tags")
				}
			} else if row.TagProjectionStatus != "succeeded" || row.TagProjectionError != nil || string(row.ExpertiseTags) != `["Architecture"]` {
				t.Fatal("tag projection did not persist generated tags successfully")
			}
		})
	}
}

func TestParseProjectedTagsNormalizesAndLimitsModelOutput(t *testing.T) {
	tags, err := parseProjectedTags("```json\n[\" Go \", \"Architecture\", \"go\", \"Testing\", \"Security\", \"Delivery\", \"Ignored\"]\n```")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Go", "Architecture", "Testing", "Security", "Delivery"}
	if len(tags) != len(want) {
		t.Fatalf("tags = %#v", tags)
	}
	for index := range want {
		if tags[index] != want[index] {
			t.Fatalf("tags = %#v", tags)
		}
	}
}
