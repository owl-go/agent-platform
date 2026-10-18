package gormrepo

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	creditsapplication "agent-platform/backend/internal/biz/credits/application"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	creditsrepo "agent-platform/backend/internal/data/credits/gormrepo"
	"github.com/google/uuid"
)

func TestResponseCreditBudgetSerializesConcurrentAdmissionsAndActualOverrun(t *testing.T) {
	db := conversationTestDatabase(t)
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	service, err := creditsapplication.New(creditsrepo.New(db), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rate, err := creditsrepo.New(db).ResolveRate(ctx, creditsdomain.ModelRateKey{ProviderType: "openai", Protocol: "openai_responses", ModelID: "model"})
	if err != nil {
		t.Fatal(err)
	}
	rate.Fallback = 50
	responseID := uuid.NewString()
	request := func() creditsapplication.AdmissionRequest {
		return creditsapplication.AdmissionRequest{UserID: owner, ExecutionID: uuid.NewString(), ResponseID: responseID, ResponseBudget: 100, StagePosition: 1, Timezone: "Asia/Shanghai", ProviderType: "openai", Protocol: "openai_responses", ModelID: "model", FrozenRate: &rate}
	}
	var mutex sync.Mutex
	var admitted []creditsdomain.Admission
	denied := 0
	var workers sync.WaitGroup
	for i := 0; i < 3; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			value, err := service.Admit(ctx, request())
			mutex.Lock()
			defer mutex.Unlock()
			if err == nil {
				admitted = append(admitted, value)
			} else if errors.Is(err, creditsdomain.ErrResponseBudgetExhausted) {
				denied++
			} else {
				t.Error(err)
			}
		}()
	}
	workers.Wait()
	if len(admitted) != 2 || denied != 1 {
		t.Fatalf("admitted=%d denied=%d", len(admitted), denied)
	}
	settlement := creditsapplication.SettlementRequest{Admission: admitted[0], Usage: creditsdomain.Usage{InputTokens: 20000, Known: true}}
	actual, err := service.Settle(ctx, settlement)
	if err != nil || actual.Amount != 200 {
		t.Fatalf("measured settlement=%+v %v", actual, err)
	}
	if replay, err := service.Settle(ctx, settlement); err != nil || replay.Amount != actual.Amount {
		t.Fatalf("settlement replay=%+v %v", replay, err)
	}
	if _, err := service.Admit(ctx, request()); !errors.Is(err, creditsdomain.ErrResponseBudgetExhausted) {
		t.Fatalf("overrun allowed next model call: %v", err)
	}
	if err := service.Abort(ctx, admitted[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Admit(ctx, request()); !errors.Is(err, creditsdomain.ErrResponseBudgetExhausted) {
		t.Fatalf("aborting unused admission reset consumed budget: %v", err)
	}
	var count int64
	db.Table("credit_ledger").Where("user_id = ? AND entry_type = 'consumption'", owner).Count(&count)
	if count != 1 {
		t.Fatal("actual invocation charged more than once")
	}
	fresh := request()
	fresh.ResponseID = uuid.NewString()
	if _, err := service.Admit(ctx, fresh); err != nil {
		t.Fatalf("manual retry did not receive independent admission budget: %v", err)
	}
}
