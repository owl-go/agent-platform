package platformconfig

import (
	"testing"
	"time"
)

func TestRAGFlowConfigurationFailsClosed(t *testing.T) {
	valid := RetrievalConfig{Provider: "ragflow", RAGFlow: RAGFlowConfig{Endpoint: "https://ragflow.example.test", APIKey: "secret", DeploymentID: "local-v024", EmbeddingModel: "bge-m3@Ollama", RequestTimeout: Duration(30 * time.Second), ParseTimeout: Duration(5 * time.Minute)}}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (RetrievalConfig{}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"http://remote.test", "https://user:pass@host", "https://host/api/v1", "https://host?key=secret", "https://host#secret"} {
		c := valid
		c.RAGFlow.Endpoint = endpoint
		if c.Validate() == nil {
			t.Errorf("unsafe endpoint %s", endpoint)
		}
	}
	c := valid
	c.RAGFlow.APIKey = ""
	if c.Validate() == nil {
		t.Fatal("missing credential accepted")
	}
	c = valid
	c.Provider = ""
	if c.Validate() == nil {
		t.Fatal("partial configuration accepted")
	}
	c = valid
	c.RAGFlow.ParseTimeout = Duration(11 * time.Minute)
	if c.Validate() == nil {
		t.Fatal("parse can exceed job lease")
	}
}
