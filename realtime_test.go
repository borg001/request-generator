package module

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/icontext"
	"github.com/darkrain/request-generator/renderer"
	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/stretchr/testify/require"
)

type realtimeBrokerStub struct {
	publishCount int
	err          error
}

func (broker *realtimeBrokerStub) Publish(_ context.Context, event RealtimeEvent) (RealtimeEvent, error) {
	broker.publishCount++
	if broker.err != nil {
		return RealtimeEvent{}, broker.err
	}
	event.EventID = "broker-event"
	return event, nil
}

func (broker *realtimeBrokerStub) Replay(context.Context, string, int) ([]RealtimeEvent, bool, error) {
	return nil, false, nil
}

func TestMemoryBrokerReplay(t *testing.T) {
	broker := NewMemoryBroker(MemoryBrokerOptions{MaxEvents: 10})

	first, err := broker.Publish(context.Background(), RealtimeEvent{Module: "chat_messages", Action: "add"})
	if err != nil {
		t.Fatalf("publish first: %v", err)
	}
	second, err := broker.Publish(context.Background(), RealtimeEvent{Module: "chat_messages", Action: "add"})
	if err != nil {
		t.Fatalf("publish second: %v", err)
	}

	events, resync, err := broker.Replay(context.Background(), first.EventID, 10)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if resync {
		t.Fatal("replay should not require resync")
	}
	if len(events) != 1 || events[0].EventID != second.EventID {
		t.Fatalf("unexpected replay events: %#v", events)
	}
}

func TestRealtimeTopicsForUserRestrictsRoleTopics(t *testing.T) {
	topics := realtimeTopicsForUser(&icontext.UserInfo{ID: 42, Role: "model"})
	require.Contains(t, topics, "user:42")
	require.Contains(t, topics, "role:model")
	require.Contains(t, topics, "role:all")
	require.NotContains(t, topics, "role:agency")
}

func TestMemoryBrokerReplayOverflowRequiresResync(t *testing.T) {
	broker := NewMemoryBroker(MemoryBrokerOptions{MaxEvents: 2})

	first, err := broker.Publish(context.Background(), RealtimeEvent{Module: "chat_messages", Action: "add"})
	if err != nil {
		t.Fatalf("publish first: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := broker.Publish(context.Background(), RealtimeEvent{Module: "chat_messages", Action: "add"}); err != nil {
			t.Fatalf("publish overflow event %d: %v", i, err)
		}
	}

	_, resync, err := broker.Replay(context.Background(), first.EventID, 10)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !resync {
		t.Fatal("expected resync after ring buffer overflow")
	}
}

func TestRealtimeCorrelationIsTypedAndValidated(t *testing.T) {
	broker := NewMemoryBroker(MemoryBrokerOptions{})
	event, err := broker.Publish(context.Background(), RealtimeEvent{
		Module: "records",
		Action: "update",
		Correlation: &RealtimeCorrelation{
			Field: "parent_id",
			Value: renderer.TypedValue{Type: renderer.TypedValueNumber, Number: 0},
		},
	})
	if err != nil {
		t.Fatalf("publish typed correlation: %v", err)
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal typed correlation: %v", err)
	}
	if string(encoded) == "" {
		t.Fatal("expected serialized event")
	}
	event.Correlation.Field = "changed"
	replayed, resync, err := broker.Replay(context.Background(), "0", 1)
	if err != nil || resync || len(replayed) != 1 {
		t.Fatalf("replay typed correlation: events=%#v resync=%v err=%v", replayed, resync, err)
	}
	if replayed[0].Correlation.Field != "parent_id" {
		t.Fatalf("broker stored a mutable correlation: %#v", replayed[0].Correlation)
	}
	if _, err := broker.Publish(context.Background(), RealtimeEvent{
		Correlation: &RealtimeCorrelation{Field: "parent_id", Value: renderer.TypedValue{}},
	}); err == nil {
		t.Fatal("expected invalid correlation to be rejected")
	}
}

func TestValidateRealtimePublishUsesDeclaredActionCorrelation(t *testing.T) {
	parentID := pg.IntegerColumn("parent_id")
	module := &BaseModule{
		Name:   "records",
		Fields: []fields.ModuleField{{Column: parentID, Type: fields.ModuleFieldTypeInt}},
		Actions: []actions.ModuleAction{actions.AddModuleAction{
			Realtime: &actions.RealtimeEventConfig{CorrelationField: "parent_id"},
		}},
	}
	generator := &Generator{Modules: []*BaseModule{module}}

	valid := RealtimePublish{Correlation: &RealtimeCorrelation{
		Field: "parent_id",
		Value: renderer.TypedValue{Type: renderer.TypedValueNumber, Number: 42},
	}}
	require.NoError(t, generator.validateRealtimePublish(module, actions.ModuleActionNameAdd, valid))

	wrongField := valid
	wrongField.Correlation = &RealtimeCorrelation{
		Field: "unknown",
		Value: renderer.TypedValue{Type: renderer.TypedValueNumber, Number: 42},
	}
	require.EqualError(t, generator.validateRealtimePublish(module, actions.ModuleActionNameAdd, wrongField), `realtime correlation field "unknown" does not match declared field "parent_id"`)

	wrongType := valid
	wrongType.Correlation = &RealtimeCorrelation{
		Field: "parent_id",
		Value: renderer.TypedValue{Type: renderer.TypedValueString, String: "42"},
	}
	require.EqualError(t, generator.validateRealtimePublish(module, actions.ModuleActionNameAdd, wrongType), `realtime correlation field "parent_id" has type "string", expected "number"`)
}

func TestValidateRealtimeEventsRejectsNonPublishingAction(t *testing.T) {
	parentID := pg.IntegerColumn("parent_id")
	generator := &Generator{Modules: []*BaseModule{{
		Name:   "records",
		Fields: []fields.ModuleField{{Column: parentID, Type: fields.ModuleFieldTypeInt}},
		Actions: []actions.ModuleAction{actions.ListModuleAction{
			Realtime: &actions.RealtimeEventConfig{CorrelationField: "parent_id"},
		}},
	}}}
	require.EqualError(t, generator.validateRealtimeEvents(), `module "records" action "list": realtime event is only supported by add, update, or delete actions`)
}

// A hook that answers the request itself still commits domain work, and the
// events it queued travel with that work. The add handler returns as soon as
// the response is written, so the queued events are published on that path too.
func TestQueuedRealtimeSurvivesAResponseWrittenByAHook(t *testing.T) {
	module := &BaseModule{Name: "records", Actions: []actions.ModuleAction{actions.AddModuleAction{}}}
	broker := &realtimeBrokerStub{}
	hub := newRealtimeHub()
	subscriber := &sseConnection{send: make(chan RealtimeEvent, 2), topics: map[string]struct{}{"user:7": {}}}
	hub.addSSE(subscriber)
	generator := &Generator{
		Modules:     []*BaseModule{module},
		Realtime:    RealtimeConfig{Enabled: true, Broker: broker},
		realtimeHub: hub,
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/records", nil)
	SetRealtimePublish(context, RealtimePublish{Topics: []string{"user:7"}, Payload: map[string]interface{}{"toast": true}})
	context.JSON(http.StatusOK, gin.H{"value": 1})
	require.True(t, context.Writer.Written())

	generator.publishRealtime(context, module, actions.ModuleActionNameAdd, nil)
	require.Equal(t, 1, broker.publishCount)
	require.Equal(t, true, (<-subscriber.send).Payload["toast"])
}

func TestPublishCommittedRealtimeUsesValidatedBrokerAndHubPath(t *testing.T) {
	module := &BaseModule{
		Name:    "records",
		Actions: []actions.ModuleAction{actions.AddModuleAction{}},
	}
	broker := &realtimeBrokerStub{}
	hub := newRealtimeHub()
	subscriber := &sseConnection{
		send:   make(chan RealtimeEvent, 2),
		topics: map[string]struct{}{"actor:42": {}},
	}
	hub.addSSE(subscriber)
	generator := &Generator{
		Modules:     []*BaseModule{module},
		Realtime:    RealtimeConfig{Enabled: true, Broker: broker},
		realtimeHub: hub,
	}

	published, err := generator.PublishCommittedRealtime(context.Background(), "records", actions.ModuleActionNameAdd, RealtimePublish{
		Topics:   []string{"actor:42"},
		RecordID: 7,
		Payload:  map[string]interface{}{"status": "updated"},
	})
	require.NoError(t, err)
	require.Equal(t, 1, broker.publishCount)
	require.Equal(t, "broker-event", published.EventID)
	require.Equal(t, "records", published.Module)
	require.Equal(t, "add", published.Action)
	require.Equal(t, 7, published.RecordID)
	require.Equal(t, "updated", published.Payload["status"])
	require.Equal(t, published, <-subscriber.send)
	require.Empty(t, subscriber.send)
}

func TestPublishCommittedRealtimeDoesNotPublishHubEventOnBrokerError(t *testing.T) {
	module := &BaseModule{Name: "records", Actions: []actions.ModuleAction{actions.AddModuleAction{}}}
	broker := &realtimeBrokerStub{err: errors.New("broker unavailable")}
	hub := newRealtimeHub()
	subscriber := &sseConnection{send: make(chan RealtimeEvent, 1), topics: map[string]struct{}{"actor:42": {}}}
	hub.addSSE(subscriber)
	generator := &Generator{
		Modules:     []*BaseModule{module},
		Realtime:    RealtimeConfig{Enabled: true, Broker: broker},
		realtimeHub: hub,
	}

	_, err := generator.PublishCommittedRealtime(context.Background(), "records", actions.ModuleActionNameAdd, RealtimePublish{Topics: []string{"actor:42"}})
	require.EqualError(t, err, "broker unavailable")
	require.Equal(t, 1, broker.publishCount)
	require.Empty(t, subscriber.send)
}

func TestPublishCommittedRealtimeRejectsUnknownContract(t *testing.T) {
	generator := &Generator{
		Modules:     []*BaseModule{{Name: "records", Actions: []actions.ModuleAction{actions.AddModuleAction{}}}},
		Realtime:    RealtimeConfig{Enabled: true, Broker: &realtimeBrokerStub{}},
		realtimeHub: newRealtimeHub(),
	}

	_, err := generator.PublishCommittedRealtime(context.Background(), "unknown", actions.ModuleActionNameAdd, RealtimePublish{Topics: []string{"actor:42"}})
	require.EqualError(t, err, `realtime module "unknown" is not registered`)
	_, err = generator.PublishCommittedRealtime(context.Background(), "records", actions.ModuleActionNameUpdate, RealtimePublish{Topics: []string{"actor:42"}})
	require.EqualError(t, err, `realtime action "update" is not declared`)
	_, err = generator.PublishCommittedRealtime(context.Background(), "records", actions.ModuleActionNameAdd, RealtimePublish{})
	require.EqualError(t, err, "realtime topics are required")
}

func TestPublishCommittedRealtimeRejectsUnavailableTransport(t *testing.T) {
	module := &BaseModule{Name: "records", Actions: []actions.ModuleAction{actions.AddModuleAction{}}}
	pub := RealtimePublish{Topics: []string{"actor:42"}}

	tests := []struct {
		name     string
		realtime RealtimeConfig
		hub      *realtimeHub
		expected string
	}{
		{name: "disabled", expected: "realtime is disabled"},
		{name: "broker", realtime: RealtimeConfig{Enabled: true}, hub: newRealtimeHub(), expected: "realtime broker is not configured"},
		{name: "hub", realtime: RealtimeConfig{Enabled: true, Broker: &realtimeBrokerStub{}}, expected: "realtime hub is not initialized"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			generator := &Generator{Modules: []*BaseModule{module}, Realtime: test.realtime, realtimeHub: test.hub}
			_, err := generator.PublishCommittedRealtime(context.Background(), "records", actions.ModuleActionNameAdd, pub)
			require.EqualError(t, err, test.expected)
		})
	}
}
