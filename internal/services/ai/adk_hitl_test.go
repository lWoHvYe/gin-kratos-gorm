package ai

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

type memCheckPointStore struct {
	values map[string][]byte
}

func (m *memCheckPointStore) Get(_ context.Context, id string) ([]byte, bool, error) {
	v, ok := m.values[id]
	return v, ok, nil
}
func (m *memCheckPointStore) Set(_ context.Context, id string, v []byte) error {
	m.values[id] = v
	return nil
}

func TestDraftReviewAgentInterruptsAndResumes(t *testing.T) {
	store := &memCheckPointStore{values: map[string][]byte{}}
	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent: &draftReviewAgent{payload: []byte(`{"draft":true}`)}, CheckPointStore: store,
	})
	iter := runner.Run(context.Background(), []adk.Message{schema.UserMessage("review")}, adk.WithCheckPointID("cp-test"))
	interrupted := false
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			t.Fatal(event.Err)
		}
		if event.Action != nil && event.Action.Interrupted != nil {
			interrupted = true
			break
		}
	}
	if !interrupted {
		t.Fatal("expected interrupt")
	}
	if len(store.values) == 0 {
		t.Fatal("checkpoint was not saved")
	}

	resumeRunner := adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent: &draftReviewAgent{}, CheckPointStore: store,
	})
	resumed, err := resumeRunner.Resume(context.Background(), "cp-test")
	if err != nil {
		t.Fatal(err)
	}
	finished := false
	for {
		event, ok := resumed.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			t.Fatal(event.Err)
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			finished = true
		}
	}
	if !finished {
		t.Fatal("resume produced no output")
	}
}
