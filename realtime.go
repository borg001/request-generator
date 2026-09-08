package module

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/icontext"
	"github.com/darkrain/request-generator/renderer"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	realtimeContextKey = "_rg_realtime_event"
)

type RealtimeConfig struct {
	Enabled bool
	Broker  RealtimeBroker
	Path    string
	SSEPath string
}

type RealtimeBroker interface {
	Publish(ctx context.Context, event RealtimeEvent) (RealtimeEvent, error)
	Replay(ctx context.Context, afterID string, limit int) ([]RealtimeEvent, bool, error)
}

type RealtimeEvent struct {
	EventID     string                 `json:"event_id"`
	Type        string                 `json:"type"`
	Module      string                 `json:"module"`
	Action      string                 `json:"action"`
	RecordID    interface{}            `json:"record_id,omitempty"`
	Correlation *RealtimeCorrelation   `json:"correlation,omitempty"`
	Topics      []string               `json:"topics,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
}

// RealtimeCorrelation carries the record relation used by generic consumers
// to match an event to their local selection. It is independent from RecordID.
type RealtimeCorrelation struct {
	Field string              `json:"field"`
	Value renderer.TypedValue `json:"value"`
}

func (correlation RealtimeCorrelation) Validate() error {
	if correlation.Field == "" {
		return fmt.Errorf("realtime correlation field is required")
	}
	if err := correlation.Value.Validate(); err != nil {
		return fmt.Errorf("realtime correlation value: %w", err)
	}
	return nil
}

type RealtimePublish struct {
	Topics      []string
	RecordID    interface{}
	Correlation *RealtimeCorrelation
	Payload     map[string]interface{}
}

func realtimeUserTopic(userID int64) string {
	return fmt.Sprintf("user:%d", userID)
}

// RealtimeUserTopic names the channel one person listens on. An action that
// publishes outside the atomic pipeline still has to address the same channel
// the subscriber is on.
func RealtimeUserTopic(userID int64) string {
	return realtimeUserTopic(userID)
}

func realtimeRoleTopic(role string) string {
	return "role:" + role
}

func realtimeTopicsForUser(user *icontext.UserInfo) map[string]struct{} {
	topics := map[string]struct{}{
		realtimeUserTopic(user.ID):                 {},
		realtimeRoleTopic(string(actions.RoleAll)): {},
	}
	if user.Role != "" {
		topics[realtimeRoleTopic(user.Role)] = struct{}{}
	}
	return topics
}

type MemoryBrokerOptions struct {
	MaxEvents int
}

type MemoryBroker struct {
	mu     sync.RWMutex
	seq    int64
	events []RealtimeEvent
	max    int
}

func NewMemoryBroker(options MemoryBrokerOptions) *MemoryBroker {
	max := options.MaxEvents
	if max <= 0 {
		max = 1000
	}
	return &MemoryBroker{max: max}
}

func (b *MemoryBroker) Publish(ctx context.Context, event RealtimeEvent) (RealtimeEvent, error) {
	if event.Correlation != nil {
		if err := event.Correlation.Validate(); err != nil {
			return RealtimeEvent{}, err
		}
	}
	event.Correlation = cloneRealtimeCorrelation(event.Correlation)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	event.EventID = strconv.FormatInt(b.seq, 10)
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	b.events = append(b.events, event)
	if len(b.events) > b.max {
		b.events = append([]RealtimeEvent(nil), b.events[len(b.events)-b.max:]...)
	}
	result := event
	result.Correlation = cloneRealtimeCorrelation(event.Correlation)
	return result, nil
}

func (b *MemoryBroker) Replay(ctx context.Context, afterID string, limit int) ([]RealtimeEvent, bool, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if afterID == "" {
		return nil, false, nil
	}
	after, err := strconv.ParseInt(afterID, 10, 64)
	if err != nil {
		return nil, true, nil
	}
	if len(b.events) == 0 {
		return nil, true, nil
	}
	first, _ := strconv.ParseInt(b.events[0].EventID, 10, 64)
	if after < first-1 {
		return nil, true, nil
	}
	if limit <= 0 {
		limit = 500
	}
	out := make([]RealtimeEvent, 0)
	for _, event := range b.events {
		id, _ := strconv.ParseInt(event.EventID, 10, 64)
		if id > after {
			event.Correlation = cloneRealtimeCorrelation(event.Correlation)
			out = append(out, event)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, false, nil
}

func cloneRealtimeCorrelation(value *RealtimeCorrelation) *RealtimeCorrelation {
	if value == nil {
		return nil
	}
	cloned := *value
	if value.Value.Bool != nil {
		boolValue := *value.Value.Bool
		cloned.Value.Bool = &boolValue
	}
	return &cloned
}

type realtimeHub struct {
	mu             sync.RWMutex
	connections    map[*realtimeConnection]struct{}
	sseConnections map[*sseConnection]struct{}
}

type realtimeConnection struct {
	hub    *realtimeHub
	conn   *websocket.Conn
	write  sync.Mutex
	send   chan RealtimeEvent
	topics map[string]struct{}
	userID int64
	role   string
}

type sseConnection struct {
	hub    *realtimeHub
	send   chan RealtimeEvent
	topics map[string]struct{}
	userID int64
	role   string
}

func newRealtimeHub() *realtimeHub {
	return &realtimeHub{
		connections:    make(map[*realtimeConnection]struct{}),
		sseConnections: make(map[*sseConnection]struct{}),
	}
}

func (h *realtimeHub) add(c *realtimeConnection) {
	h.mu.Lock()
	h.connections[c] = struct{}{}
	h.mu.Unlock()
}

func (h *realtimeHub) remove(c *realtimeConnection) {
	h.mu.Lock()
	delete(h.connections, c)
	h.mu.Unlock()
	close(c.send)
}

func (h *realtimeHub) addSSE(c *sseConnection) {
	h.mu.Lock()
	h.sseConnections[c] = struct{}{}
	h.mu.Unlock()
}

func (h *realtimeHub) removeSSE(c *sseConnection) {
	h.mu.Lock()
	delete(h.sseConnections, c)
	h.mu.Unlock()
	close(c.send)
}

func (h *realtimeHub) publish(event RealtimeEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.connections {
		if !c.matches(event.Topics) {
			continue
		}
		select {
		case c.send <- event:
		default:
		}
	}
	for c := range h.sseConnections {
		if !c.matches(event.Topics) {
			continue
		}
		select {
		case c.send <- event:
		default:
		}
	}
}

func (c *realtimeConnection) matches(topics []string) bool {
	if len(c.topics) == 0 {
		return false
	}
	for _, topic := range topics {
		if _, ok := c.topics[topic]; ok {
			return true
		}
	}
	return false
}

func (c *sseConnection) matches(topics []string) bool {
	if len(c.topics) == 0 {
		return false
	}
	for _, topic := range topics {
		if _, ok := c.topics[topic]; ok {
			return true
		}
	}
	return false
}

func (generator *Generator) initRealtime() {
	if !generator.Realtime.Enabled {
		return
	}
	if generator.Realtime.Broker == nil {
		panic("request-generator realtime enabled without broker")
	}
	if generator.realtimeHub == nil {
		generator.realtimeHub = newRealtimeHub()
	}
	path := generator.Realtime.Path
	if path == "" {
		path = "/api/ws"
	}
	group := generator.group.Group(path)
	dummyAction := actions.ListModuleAction{Auth: true, Permission: []actions.Role{}}
	if generator.AuthMiddleware != nil {
		group.Use(func(c *gin.Context) {
			if token := c.Query("token"); token != "" && c.GetHeader("Authorization") == "" {
				c.Request.Header.Set("Authorization", "Bearer "+token)
			}
			generator.AuthMiddleware(dummyAction)(c)
		})
	}
	group.GET("", generator.handleRealtimeWebSocket())

	ssePath := generator.Realtime.SSEPath
	if ssePath == "" {
		ssePath = "/api/sse"
	}
	sseGroup := generator.group.Group(ssePath)
	if generator.AuthMiddleware != nil {
		sseGroup.Use(func(c *gin.Context) {
			if token := c.Query("token"); token != "" && c.GetHeader("Authorization") == "" {
				c.Request.Header.Set("Authorization", "Bearer "+token)
			}
			generator.AuthMiddleware(dummyAction)(c)
		})
	}
	sseGroup.GET("", generator.handleRealtimeSSE())
}

func (generator *Generator) handleRealtimeWebSocket() gin.HandlerFunc {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	return func(c *gin.Context) {
		user, ok := icontext.GetUser(c.Request.Context())
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
			return
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		rc := &realtimeConnection{
			hub:    generator.realtimeHub,
			conn:   conn,
			send:   make(chan RealtimeEvent, 64),
			topics: realtimeTopicsForUser(user),
			userID: user.ID,
			role:   user.Role,
		}
		generator.realtimeHub.add(rc)
		defer func() {
			generator.realtimeHub.remove(rc)
			conn.Close()
		}()

		go rc.writeLoop()
		rc.readLoop(generator)
	}
}

func (generator *Generator) handleRealtimeSSE() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := icontext.GetUser(c.Request.Context())
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
			return
		}
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "streaming unsupported"})
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		sc := &sseConnection{
			hub:    generator.realtimeHub,
			send:   make(chan RealtimeEvent, 64),
			topics: realtimeTopicsForUser(user),
			userID: user.ID,
			role:   user.Role,
		}
		generator.realtimeHub.addSSE(sc)
		defer generator.realtimeHub.removeSSE(sc)

		lastEventID := c.GetHeader("Last-Event-ID")
		if queryLastEventID := c.Query("last_event_id"); queryLastEventID != "" {
			lastEventID = queryLastEventID
		}
		replay := "ok"
		if lastEventID != "" {
			events, resync, err := generator.Realtime.Broker.Replay(c.Request.Context(), lastEventID, 500)
			if err != nil || resync {
				replay = "resync_required"
			} else {
				for _, event := range events {
					if sc.matches(event.Topics) {
						writeSSE(c.Writer, event.EventID, "message", event)
					}
				}
			}
		}
		writeSSE(c.Writer, "", "ready", gin.H{"type": "ready", "replay": replay})
		flusher.Flush()

		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-c.Request.Context().Done():
				return
			case <-ticker.C:
				_, _ = fmt.Fprint(c.Writer, ": ping\n\n")
				flusher.Flush()
			case event := <-sc.send:
				writeSSE(c.Writer, event.EventID, "message", event)
				flusher.Flush()
			}
		}
	}
}

func writeSSE(w gin.ResponseWriter, id, eventName string, data interface{}) {
	if id != "" {
		_, _ = fmt.Fprintf(w, "id: %s\n", id)
	}
	if eventName != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", eventName)
	}
	body, err := json.Marshal(data)
	if err != nil {
		body = []byte(`{"type":"error"}`)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", body)
}

type realtimeClientMessage struct {
	Type        string   `json:"type"`
	RequestID   string   `json:"request_id,omitempty"`
	LastEventID string   `json:"last_event_id,omitempty"`
	Topics      []string `json:"topics,omitempty"`
}

func (c *realtimeConnection) readLoop(generator *Generator) {
	for {
		var msg realtimeClientMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}
		switch msg.Type {
		case "hello":
			c.handleHello(generator, msg)
		case "subscribe":
			c.handleSubscribe(msg)
		case "unsubscribe":
			c.handleUnsubscribe(msg)
		case "ping":
			_ = c.writeJSON(gin.H{"type": "pong", "request_id": msg.RequestID})
		}
	}
}

func (c *realtimeConnection) writeLoop() {
	for event := range c.send {
		if err := c.writeJSON(event); err != nil {
			return
		}
	}
}

func (c *realtimeConnection) writeJSON(v interface{}) error {
	c.write.Lock()
	defer c.write.Unlock()
	return c.conn.WriteJSON(v)
}

func (c *realtimeConnection) handleHello(generator *Generator, msg realtimeClientMessage) {
	replay := "ok"
	if msg.LastEventID != "" {
		events, resync, err := generator.Realtime.Broker.Replay(context.Background(), msg.LastEventID, 500)
		if err != nil || resync {
			replay = "resync_required"
		} else {
			for _, event := range events {
				if c.matches(event.Topics) {
					c.send <- event
				}
			}
		}
	}
	_ = c.writeJSON(gin.H{"type": "ready", "request_id": msg.RequestID, "replay": replay})
}

func (c *realtimeConnection) handleSubscribe(msg realtimeClientMessage) {
	accepted := []string{}
	for _, topic := range msg.Topics {
		if c.canSubscribe(topic) {
			c.topics[topic] = struct{}{}
			accepted = append(accepted, topic)
		}
	}
	_ = c.writeJSON(gin.H{"type": "subscribed", "request_id": msg.RequestID, "topics": accepted})
}

func (c *realtimeConnection) handleUnsubscribe(msg realtimeClientMessage) {
	for _, topic := range msg.Topics {
		if topic == realtimeUserTopic(c.userID) || topic == realtimeRoleTopic(c.role) || topic == realtimeRoleTopic(string(actions.RoleAll)) {
			continue
		}
		delete(c.topics, topic)
	}
	_ = c.writeJSON(gin.H{"type": "unsubscribed", "request_id": msg.RequestID})
}

func (c *realtimeConnection) canSubscribe(topic string) bool {
	if topic == realtimeUserTopic(c.userID) || topic == realtimeRoleTopic(c.role) || topic == realtimeRoleTopic(string(actions.RoleAll)) {
		return true
	}
	return false
}

func SetRealtimePublish(c *gin.Context, event RealtimePublish) {
	if len(event.Topics) == 0 {
		return
	}
	raw, ok := c.Get(realtimeContextKey)
	if !ok {
		c.Set(realtimeContextKey, []RealtimePublish{event})
		return
	}
	switch existing := raw.(type) {
	case []RealtimePublish:
		c.Set(realtimeContextKey, append(existing, event))
	case RealtimePublish:
		c.Set(realtimeContextKey, []RealtimePublish{existing, event})
	default:
		c.Set(realtimeContextKey, []RealtimePublish{event})
	}
}

func (generator *Generator) publishRealtime(c *gin.Context, module *BaseModule, action actions.ModuleActionName, output interface{}) {
	if !generator.Realtime.Enabled || generator.Realtime.Broker == nil || generator.realtimeHub == nil {
		return
	}
	raw, ok := c.Get(realtimeContextKey)
	if !ok {
		return
	}
	pubs := []RealtimePublish{}
	switch value := raw.(type) {
	case RealtimePublish:
		pubs = append(pubs, value)
	case []RealtimePublish:
		pubs = value
	default:
		return
	}
	for _, pub := range pubs {
		_, _ = generator.publishRealtimeEvent(c.Request.Context(), module, action, output, pub)
	}
}

// PublishCommittedRealtime publishes a typed event after the caller has
// committed its transaction. It uses the same module/action validation and
// broker-to-hub delivery path as request-bound actions.
func (generator *Generator) PublishCommittedRealtime(ctx context.Context, moduleName string, action actions.ModuleActionName, pub RealtimePublish) (RealtimeEvent, error) {
	if ctx == nil {
		return RealtimeEvent{}, fmt.Errorf("realtime context is required")
	}
	module, ok := generator.moduleByName(moduleName)
	if !ok {
		return RealtimeEvent{}, fmt.Errorf("realtime module %q is not registered", moduleName)
	}
	return generator.publishRealtimeEvent(ctx, module, action, nil, pub)
}

func (generator *Generator) publishRealtimeEvent(ctx context.Context, module *BaseModule, action actions.ModuleActionName, output interface{}, pub RealtimePublish) (RealtimeEvent, error) {
	if !generator.Realtime.Enabled {
		return RealtimeEvent{}, fmt.Errorf("realtime is disabled")
	}
	if generator.Realtime.Broker == nil {
		return RealtimeEvent{}, fmt.Errorf("realtime broker is not configured")
	}
	if generator.realtimeHub == nil {
		return RealtimeEvent{}, fmt.Errorf("realtime hub is not initialized")
	}
	if len(pub.Topics) == 0 {
		return RealtimeEvent{}, fmt.Errorf("realtime topics are required")
	}
	if err := generator.validateRealtimePublish(module, action, pub); err != nil {
		return RealtimeEvent{}, err
	}
	event := RealtimeEvent{
		Type:        "event",
		Module:      module.Name,
		Action:      string(action),
		RecordID:    pub.RecordID,
		Correlation: pub.Correlation,
		Topics:      pub.Topics,
		CreatedAt:   time.Now().UTC(),
		Payload:     pub.Payload,
	}
	event.Correlation = cloneRealtimeCorrelation(event.Correlation)
	if event.Payload == nil {
		event.Payload = map[string]interface{}{}
	}
	if pub.RecordID == nil {
		if mapped := outputRecordID(output); mapped != nil {
			event.RecordID = mapped
		}
	}
	published, err := generator.Realtime.Broker.Publish(ctx, event)
	if err != nil {
		return RealtimeEvent{}, err
	}
	generator.realtimeHub.publish(published)
	return published, nil
}

func (generator *Generator) validateRealtimePublish(module *BaseModule, actionName actions.ModuleActionName, pub RealtimePublish) error {
	action, ok := findModuleAction(module, string(actionName))
	if !ok {
		return fmt.Errorf("realtime action %q is not declared", actionName)
	}
	event := actions.RealtimeEvent(action)
	if event == nil {
		if pub.Correlation != nil {
			return fmt.Errorf("realtime action %q does not declare correlation", actionName)
		}
		return nil
	}
	if pub.Correlation == nil {
		return fmt.Errorf("realtime action %q requires correlation", actionName)
	}
	if err := pub.Correlation.Validate(); err != nil {
		return err
	}
	if pub.Correlation.Field != event.CorrelationField {
		return fmt.Errorf("realtime correlation field %q does not match declared field %q", pub.Correlation.Field, event.CorrelationField)
	}
	field := module.GetField(event.CorrelationField)
	if field == nil {
		return fmt.Errorf("realtime correlation field %q is not declared", event.CorrelationField)
	}
	expected, err := runtimeTypedValueType(*field)
	if err != nil {
		return fmt.Errorf("realtime correlation field %q: %w", event.CorrelationField, err)
	}
	if pub.Correlation.Value.Type != expected {
		return fmt.Errorf("realtime correlation field %q has type %q, expected %q", event.CorrelationField, pub.Correlation.Value.Type, expected)
	}
	return nil
}

func outputRecordID(output interface{}) interface{} {
	if output == nil {
		return nil
	}
	body, err := json.Marshal(output)
	if err != nil {
		return nil
	}
	var mapped map[string]interface{}
	if err := json.Unmarshal(body, &mapped); err != nil {
		return nil
	}
	if v, ok := mapped["value"]; ok {
		return v
	}
	if v, ok := mapped["id"]; ok {
		return v
	}
	return nil
}
