package domain

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalid         = errors.New("AI Application value is invalid")
	ErrNotFound        = errors.New("AI Application resource not found")
	ErrConflict        = errors.New("AI Application state conflicts with current state")
	ErrVersionConflict = errors.New("AI Application version conflicts with current state")
)

const SafetyRefusal = "暂时无法回答此类问题"

type SafetyDecision string

const (
	SafetyAllow  SafetyDecision = "allow"
	SafetyRefuse SafetyDecision = "refuse"
)

type ApplicationState string

const (
	StateDraft    ApplicationState = "draft"
	StateEnabled  ApplicationState = "enabled"
	StateDisabled ApplicationState = "disabled"
)

const (
	ScenarioCustomerConsultation = "customer-consultation"
	ScenarioPreSalesAdvisor      = "pre-sales-advisor"
	ScenarioAfterSalesSupport    = "after-sales-support"
	ScenarioProductGuide         = "product-guide"
	ScenarioEnterpriseKnowledge  = "enterprise-knowledge"
	ScenarioRecruitment          = "recruitment"
	ScenarioTraining             = "training"
	ScenarioCustom               = "custom"
)

var assistantScenarios = map[string]struct{}{
	ScenarioCustomerConsultation: {},
	ScenarioPreSalesAdvisor:      {},
	ScenarioAfterSalesSupport:    {},
	ScenarioProductGuide:         {},
	ScenarioEnterpriseKnowledge:  {},
	ScenarioRecruitment:          {},
	ScenarioTraining:             {},
	ScenarioCustom:               {},
}

type SafetyPolicy struct{ blocked []string }

func DefaultSafetyPolicy() SafetyPolicy {
	return SafetyPolicy{blocked: []string{"政治", "军事", "武器", "炸弹", "暴力", "入侵", "绕过安全", "malware", "weapon", "military", "politics"}}
}

func (policy SafetyPolicy) ValidateAnswer(answer string) error {
	if policy.Decide(answer) == SafetyRefuse {
		return fmt.Errorf("%w: answer contains a protected topic", ErrInvalid)
	}
	return nil
}

func (policy SafetyPolicy) Decide(input string) SafetyDecision {
	text := strings.ToLower(strings.TrimSpace(input))
	for _, keyword := range policy.blocked {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return SafetyRefuse
		}
	}
	return SafetyAllow
}

type ShareConfiguration struct {
	Enabled         bool     `json:"enabled"`
	Token           string   `json:"token,omitempty"`
	TokenHash       string   `json:"-"`
	TokenRevision   int64    `json:"token_revision"`
	AllowedOrigins  []string `json:"allowed_origins,omitempty"`
	Width           string   `json:"width"`
	Height          int      `json:"height"`
	FreeTextEnabled bool     `json:"free_text_enabled"`
	DailyCallLimit  int      `json:"daily_call_limit"`
}

type SmartAssistant struct {
	ID               string             `json:"id"`
	OwnerID          string             `json:"owner_id,omitempty"`
	Name             string             `json:"name"`
	Icon             string             `json:"icon"`
	Description      string             `json:"description"`
	Introduction     string             `json:"introduction"`
	Scenario         string             `json:"scenario"`
	Prompt           string             `json:"prompt"`
	PreprocessPrompt string             `json:"preprocess_prompt"`
	ServiceGoal      string             `json:"service_goal"`
	AnswerScope      string             `json:"answer_scope"`
	OperatingRules   string             `json:"operating_rules"`
	ResponseStyle    string             `json:"response_style"`
	KnowledgeBaseIDs []string           `json:"knowledge_base_ids"`
	ExpertID         *string            `json:"expert_id,omitempty"`
	ExpertTeamID     *string            `json:"expert_team_id,omitempty"`
	DigitalHumanID   *string            `json:"digital_human_id,omitempty"`
	Share            ShareConfiguration `json:"share"`
	State            ApplicationState   `json:"state"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	Version          int64              `json:"version"`
}

func (assistant SmartAssistant) Validate() error {
	if strings.TrimSpace(assistant.Name) == "" || len([]rune(assistant.Name)) > 100 {
		return fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if assistant.State != "" && assistant.State != StateDraft && assistant.State != StateEnabled && assistant.State != StateDisabled {
		return fmt.Errorf("%w: unsupported assistant state", ErrInvalid)
	}
	if assistant.Scenario != "" {
		if _, ok := assistantScenarios[assistant.Scenario]; !ok {
			return fmt.Errorf("%w: unsupported assistant scenario", ErrInvalid)
		}
	}
	if len([]rune(assistant.Description)) > 5000 || len([]rune(assistant.Prompt)) > 10000 || len([]rune(assistant.PreprocessPrompt)) > 10000 || len([]rune(assistant.ServiceGoal)) > 5000 || len([]rune(assistant.AnswerScope)) > 10000 || len([]rune(assistant.OperatingRules)) > 10000 || len([]rune(assistant.ResponseStyle)) > 2000 {
		return fmt.Errorf("%w: assistant instructions are too long", ErrInvalid)
	}
	if assistant.Share.Width != "" && assistant.Share.Width != "100%" {
		match := regexp.MustCompile(`^([0-9]+)px$`).FindStringSubmatch(assistant.Share.Width)
		if len(match) != 2 || atoi(match[1]) < 320 || atoi(match[1]) > 1920 {
			return fmt.Errorf("%w: share width must be 100%% or px", ErrInvalid)
		}
	}
	if assistant.Share.Height != 0 && (assistant.Share.Height < 400 || assistant.Share.Height > 1600) {
		return fmt.Errorf("%w: share height is outside 400-1600px", ErrInvalid)
	}
	if assistant.Share.DailyCallLimit < 0 {
		return fmt.Errorf("%w: daily call limit cannot be negative", ErrInvalid)
	}
	for _, origin := range assistant.Share.AllowedOrigins {
		parsed, err := url.Parse(strings.TrimSpace(origin))
		if err != nil || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "https" && !isLocalHTTPOrigin(parsed)) {
			return fmt.Errorf("%w: share origin must be HTTPS or local development HTTP", ErrInvalid)
		}
	}
	return nil
}

func isLocalHTTPOrigin(origin *url.URL) bool {
	if origin.Scheme != "http" {
		return false
	}
	host := strings.ToLower(origin.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "[::1]" || host == "::1"
}

// ValidateForEnable checks the minimum visible configuration required before
// an assistant can accept a new conversation. Drafts remain editable while
// incomplete, so callers should use Validate for persistence and this method
// only for the enable transition.
func (assistant SmartAssistant) ValidateForEnable() error {
	if err := assistant.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(assistant.Icon) == "" {
		return fmt.Errorf("%w: icon is required before enabling", ErrInvalid)
	}
	return nil
}

func atoi(value string) int {
	var result int
	for _, digit := range value {
		result = result*10 + int(digit-'0')
	}
	return result
}

type FAQ struct {
	ID             string    `json:"id"`
	AssistantID    string    `json:"assistant_id,omitempty"`
	Question       string    `json:"question"`
	AnswerMarkdown string    `json:"answer_markdown"`
	DisplayOrder   int       `json:"display_order"`
	Category       string    `json:"category"`
	Tag            string    `json:"tag"`
	Icon           string    `json:"icon"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Version        int64     `json:"version"`
}

func (faq FAQ) Validate() error {
	if strings.TrimSpace(faq.Question) == "" || strings.TrimSpace(faq.AnswerMarkdown) == "" {
		return fmt.Errorf("%w: FAQ question and answer are required", ErrInvalid)
	}
	if len([]rune(faq.Question)) > 500 || len([]rune(faq.AnswerMarkdown)) > 20000 || faq.DisplayOrder < 0 {
		return fmt.Errorf("%w: FAQ fields are outside allowed limits", ErrInvalid)
	}
	return nil
}

type DigitalHuman struct {
	ID               string           `json:"id"`
	OwnerID          string           `json:"owner_id,omitempty"`
	Name             string           `json:"name"`
	AvatarObjectKey  string           `json:"avatar_object_key"`
	Voice            string           `json:"voice"`
	Language         string           `json:"language"`
	ExpressionStyle  string           `json:"expression_style"`
	SceneDescription string           `json:"scene_description"`
	State            ApplicationState `json:"state"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	Version          int64            `json:"version"`
}

func (human DigitalHuman) Validate() error {
	if strings.TrimSpace(human.Name) == "" || len([]rune(human.Name)) > 100 {
		return fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if human.State != "" && human.State != StateEnabled && human.State != StateDisabled {
		return fmt.Errorf("%w: unsupported digital human state", ErrInvalid)
	}
	return nil
}

type DigitalHumanPreview struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	AvatarObjectKey  string           `json:"avatar_object_key"`
	Voice            string           `json:"voice"`
	Language         string           `json:"language"`
	ExpressionStyle  string           `json:"expression_style"`
	SceneDescription string           `json:"scene_description"`
	State            ApplicationState `json:"state"`
	PreviewText      string           `json:"preview_text"`
}
