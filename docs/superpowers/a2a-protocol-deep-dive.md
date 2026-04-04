# A2A Protocol Deep Dive - Technical Reference for Go Implementation

> Source: A2A Protocol v1.0 Specification (a2a-protocol.org) + Official Go SDK v2 (github.com/a2aproject/a2a-go)

---

## 1. AgentCard

### Discovery

Agents publish their AgentCard at a well-known URL:

```
GET /.well-known/agent-card.json
```

The Go SDK defines this as:
```go
const WellKnownAgentCardPath = "/.well-known/agent-card.json"
```

Servers SHOULD provide cache headers (ETag, Cache-Control). Clients SHOULD cache and respect HTTP caching directives.

### Complete AgentCard Structure

**Proto definition** (source of truth from `specification/a2a.proto`):

Required fields: `name`, `description`, `supported_interfaces`, `version`, `capabilities`, `default_input_modes`, `default_output_modes`, `skills`

**Go SDK struct** (`a2a/agent.go`):

```go
type AgentCard struct {
    SupportedInterfaces  []*AgentInterface           `json:"supportedInterfaces"`
    Capabilities         AgentCapabilities            `json:"capabilities"`
    DefaultInputModes    []string                     `json:"defaultInputModes"`
    DefaultOutputModes   []string                     `json:"defaultOutputModes"`
    Description          string                       `json:"description"`
    DocumentationURL     string                       `json:"documentationUrl,omitempty"`
    IconURL              string                       `json:"iconUrl,omitempty"`
    Name                 string                       `json:"name"`
    Provider             *AgentProvider               `json:"provider,omitempty"`
    SecurityRequirements SecurityRequirementsOptions   `json:"securityRequirements,omitempty"`
    SecuritySchemes      NamedSecuritySchemes          `json:"securitySchemes,omitempty"`
    Signatures           []AgentCardSignature          `json:"signatures,omitempty"`
    Skills               []AgentSkill                  `json:"skills"`
    Version              string                       `json:"version"`
}
```

### JSON Example

```json
{
  "name": "Recipe Agent",
  "description": "Agent that helps users with recipes and cooking.",
  "version": "1.0.0",
  "provider": {
    "organization": "Acme Corp",
    "url": "https://acme.example.com"
  },
  "supportedInterfaces": [
    {
      "url": "https://api.example.com/a2a",
      "protocolBinding": "HTTP+JSON",
      "protocolVersion": "1.0"
    },
    {
      "url": "https://grpc.example.com/a2a",
      "protocolBinding": "GRPC",
      "protocolVersion": "1.0"
    }
  ],
  "capabilities": {
    "streaming": true,
    "pushNotifications": true,
    "extendedAgentCard": false
  },
  "defaultInputModes": ["text/plain", "application/json"],
  "defaultOutputModes": ["text/plain", "application/json"],
  "skills": [
    {
      "id": "recipe_search",
      "name": "Recipe Search",
      "description": "Search for recipes by ingredients or cuisine",
      "tags": ["recipes", "cooking", "search"],
      "examples": ["Find me a pasta recipe", "What can I make with chicken?"],
      "inputModes": ["text/plain"],
      "outputModes": ["text/plain", "application/json"]
    }
  ],
  "securitySchemes": {
    "bearerAuth": {
      "http": {
        "scheme": "bearer",
        "bearerFormat": "JWT"
      }
    }
  },
  "securityRequirements": [
    {
      "schemes": {
        "bearerAuth": []
      }
    }
  ]
}
```

### Supporting Types

```go
type AgentInterface struct {
    URL             string            `json:"url"`
    ProtocolBinding TransportProtocol `json:"protocolBinding"`
    Tenant          string            `json:"tenant,omitempty"`
    ProtocolVersion ProtocolVersion   `json:"protocolVersion"`
}

type TransportProtocol string
const (
    TransportProtocolJSONRPC  TransportProtocol = "JSONRPC"
    TransportProtocolGRPC     TransportProtocol = "GRPC"
    TransportProtocolHTTPJSON TransportProtocol = "HTTP+JSON"
)

type AgentCapabilities struct {
    Extensions        []AgentExtension `json:"extensions,omitempty"`
    PushNotifications bool             `json:"pushNotifications,omitempty"`
    Streaming         bool             `json:"streaming,omitempty"`
    ExtendedAgentCard bool             `json:"extendedAgentCard,omitempty"`
}

type AgentSkill struct {
    Description          string                      `json:"description"`
    Examples             []string                    `json:"examples,omitempty"`
    ID                   string                      `json:"id"`
    InputModes           []string                    `json:"inputModes,omitempty"`
    Name                 string                      `json:"name"`
    OutputModes          []string                    `json:"outputModes,omitempty"`
    SecurityRequirements SecurityRequirementsOptions  `json:"securityRequirements,omitempty"`
    Tags                 []string                    `json:"tags"`
}

type AgentProvider struct {
    Org string `json:"organization"`
    URL string `json:"url"`
}
```

---

## 2. Task Lifecycle

### TaskState Enum (Proto)

```protobuf
enum TaskState {
    TASK_STATE_UNSPECIFIED    = 0;  // Unknown/indeterminate
    TASK_STATE_SUBMITTED     = 1;  // Accepted, waiting to start
    TASK_STATE_WORKING       = 2;  // Actively processing
    TASK_STATE_COMPLETED     = 3;  // Finished successfully (TERMINAL)
    TASK_STATE_FAILED        = 4;  // Finished with error (TERMINAL)
    TASK_STATE_CANCELED      = 5;  // Client canceled (TERMINAL)
    TASK_STATE_INPUT_REQUIRED = 6; // Awaiting client input (INTERRUPTED)
    TASK_STATE_REJECTED      = 7;  // Agent refused task (TERMINAL)
    TASK_STATE_AUTH_REQUIRED  = 8;  // Auth needed to proceed (INTERRUPTED)
}
```

### Go SDK Constants

```go
type TaskState string
const (
    TaskStateUnspecified   TaskState = ""
    TaskStateAuthRequired  TaskState = "TASK_STATE_AUTH_REQUIRED"
    TaskStateCanceled      TaskState = "TASK_STATE_CANCELED"
    TaskStateCompleted     TaskState = "TASK_STATE_COMPLETED"
    TaskStateFailed        TaskState = "TASK_STATE_FAILED"
    TaskStateInputRequired TaskState = "TASK_STATE_INPUT_REQUIRED"
    TaskStateRejected      TaskState = "TASK_STATE_REJECTED"
    TaskStateSubmitted     TaskState = "TASK_STATE_SUBMITTED"
    TaskStateWorking       TaskState = "TASK_STATE_WORKING"
)

// Terminal() returns true for completed, canceled, failed, rejected
func (ts TaskState) Terminal() bool {
    return ts == TaskStateCompleted ||
        ts == TaskStateCanceled ||
        ts == TaskStateFailed ||
        ts == TaskStateRejected
}
```

### State Machine

```
                          +-> COMPLETED (terminal)
                          |
SUBMITTED -> WORKING -----+-> FAILED (terminal)
   |            |    ^    |
   |            |    |    +-> CANCELED (terminal)
   |            v    |    |
   |     INPUT_REQUIRED   +-> REJECTED (terminal)
   |            |
   |            v
   |     AUTH_REQUIRED
   |
   +-> REJECTED (terminal, can happen from submitted directly)
   +-> FAILED (terminal, can happen from any non-terminal state)
```

**Transition rules:**
- `SUBMITTED` -> `WORKING`, `REJECTED`, `FAILED`
- `WORKING` -> `COMPLETED`, `FAILED`, `CANCELED`, `REJECTED`, `INPUT_REQUIRED`, `AUTH_REQUIRED`
- `INPUT_REQUIRED` -> `WORKING` (after client sends input), `CANCELED`, `FAILED`
- `AUTH_REQUIRED` -> `WORKING` (after auth provided), `CANCELED`, `FAILED`
- Terminal states (`COMPLETED`, `FAILED`, `CANCELED`, `REJECTED`) -> no further transitions

### Task Object

```go
type Task struct {
    ID        TaskID         `json:"id"`
    Artifacts []*Artifact    `json:"artifacts,omitempty"`
    ContextID string         `json:"contextId"`
    History   []*Message     `json:"history,omitempty"`
    Metadata  map[string]any `json:"metadata,omitempty"`
    Status    TaskStatus     `json:"status"`
}

type TaskStatus struct {
    Message   *Message   `json:"message,omitempty"`
    State     TaskState  `json:"state"`
    Timestamp *time.Time `json:"timestamp,omitempty"`
}
```

### Task JSON Example

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "contextId": "660e8400-e29b-41d4-a716-446655440000",
  "status": {
    "state": "TASK_STATE_COMPLETED",
    "timestamp": "2026-03-23T10:05:00Z",
    "message": {
      "messageId": "msg-abc-123",
      "role": "ROLE_AGENT",
      "parts": [{"text": "Task completed successfully"}]
    }
  },
  "artifacts": [
    {
      "artifactId": "art-001",
      "name": "analysis-result",
      "description": "The analysis output",
      "parts": [
        {"text": "Here is the analysis..."},
        {"data": {"score": 95, "details": ["item1", "item2"]}, "mediaType": "application/json"}
      ]
    }
  ],
  "history": [
    {
      "messageId": "msg-001",
      "role": "ROLE_USER",
      "parts": [{"text": "Analyze this document"}]
    },
    {
      "messageId": "msg-002",
      "role": "ROLE_AGENT",
      "parts": [{"text": "I'll analyze that for you."}]
    }
  ]
}
```

### contextId vs taskId

| Concept | Description | Who generates | When provided by client |
|---------|-------------|---------------|------------------------|
| `taskId` | Unique ID for a single Task | Server (UUIDv7) | To continue an existing task (e.g., respond to INPUT_REQUIRED) |
| `contextId` | Groups related tasks/messages in a conversation | Server (UUIDv7), or client | To keep multiple tasks in the same conversation |

**Multi-turn pattern:**
```
Turn 1: Client sends message with no taskId, no contextId
  -> Server creates Task{id: "task-1", contextId: "ctx-A"}

Turn 2: Client sends message with taskId: "task-1" (continues same task)
  -> Server returns updated Task{id: "task-1", contextId: "ctx-A"}

Turn 3: Client sends message with contextId: "ctx-A", no taskId (new task, same context)
  -> Server creates Task{id: "task-2", contextId: "ctx-A"}
```

If client provides both `taskId` and `contextId`, the `contextId` must match the one on the existing task. If only `taskId` is provided, the server infers `contextId` from it.

---

## 3. Message and Parts

### Message Object

```go
type Message struct {
    ID             string         `json:"messageId"`
    ContextID      string         `json:"contextId,omitempty"`
    Extensions     []string       `json:"extensions,omitempty"`
    Metadata       map[string]any `json:"metadata,omitempty"`
    Parts          ContentParts   `json:"parts"`
    ReferenceTasks []TaskID       `json:"referenceTaskIds,omitempty"`
    Role           MessageRole    `json:"role"`
    TaskID         TaskID         `json:"taskId,omitempty"`
}

type MessageRole string
const (
    MessageRoleUnspecified MessageRole = ""           // JSON: "ROLE_UNSPECIFIED"
    MessageRoleAgent      MessageRole = "ROLE_AGENT"
    MessageRoleUser       MessageRole = "ROLE_USER"
)
```

### Message JSON Example

```json
{
  "messageId": "msg-550e8400",
  "role": "ROLE_USER",
  "contextId": "ctx-660e8400",
  "taskId": "task-770e8400",
  "parts": [
    {"text": "Please analyze this CSV"},
    {
      "raw": "bmFtZSxhZ2UKQWxpY2UsMzAKQm9iLDI1",  # pragma: allowlist secret
      "filename": "data.csv",
      "mediaType": "text/csv"
    }
  ],
  "referenceTaskIds": ["task-previous-001"],
  "metadata": {
    "priority": "high"
  }
}
```

### Part Types

The `Part` is a union type with four content variants plus optional metadata fields:

```go
type Part struct {
    Content   PartContent    `json:"content"`    // One of: Text, Raw, URL, Data
    Filename  string         `json:"filename,omitempty"`
    MediaType string         `json:"mediaType,omitempty"`
    Metadata  map[string]any `json:"metadata,omitempty"`
}

// Content types (only one is set per Part):
type Text string      // Plain text content
type Raw  []byte      // Binary data (base64 in JSON)
type URL  string      // URL pointing to file content
type Data struct {    // Arbitrary structured JSON data
    Value any
}
```

**JSON serialization** - each Part serializes as a flat object with one content key:

#### TextPart
```json
{
  "text": "Hello, world!"
}
```

#### FilePart (inline binary via Raw)
```json
{
  "raw": "aGVsbG8gd29ybGQ=",
  "filename": "hello.txt",
  "mediaType": "text/plain"
}
```

#### FilePart (URL reference)
```json
{
  "url": "https://storage.example.com/files/document.pdf",
  "filename": "document.pdf",
  "mediaType": "application/pdf"
}
```

#### DataPart (structured JSON)
```json
{
  "data": {
    "formFields": [
      {"name": "email", "type": "string", "required": true},
      {"name": "age", "type": "number", "required": false}
    ]
  },
  "mediaType": "application/json"
}
```

### Constructors

```go
a2a.NewTextPart("Hello")
a2a.NewRawPart([]byte{0x48, 0x65, 0x6c, 0x6c, 0x6f})
a2a.NewFileURLPart("https://example.com/file.pdf", "application/pdf")
a2a.NewDataPart(map[string]any{"key": "value"})
```

### Artifact

Artifacts are task outputs, distinct from messages in the history:

```go
type Artifact struct {
    ID          ArtifactID     `json:"artifactId"`
    Description string         `json:"description,omitempty"`
    Extensions  []string       `json:"extensions,omitempty"`
    Metadata    map[string]any `json:"metadata,omitempty"`
    Name        string         `json:"name,omitempty"`
    Parts       ContentParts   `json:"parts"`
}
```

---

## 4. SendMessage (Synchronous)

### Protocol Bindings

A2A supports three transport bindings. Method names differ by binding:

| Operation | JSON-RPC method | REST endpoint | gRPC method |
|-----------|----------------|---------------|-------------|
| Send message | `SendMessage` | `POST /message:send` | `SendMessage` |
| Stream message | `SendStreamingMessage` | `POST /message:stream` | `SendStreamingMessage` |
| Get task | `GetTask` | `GET /tasks/{id}` | `GetTask` |
| List tasks | `ListTasks` | `GET /tasks` | `ListTasks` |
| Cancel task | `CancelTask` | `POST /tasks/{id}:cancel` | `CancelTask` |
| Subscribe | `SubscribeToTask` | `GET /tasks/{id}:subscribe` | `SubscribeToTask` |
| Create push config | `CreateTaskPushNotificationConfig` | `POST /tasks/{task_id}/pushNotificationConfigs` | `CreateTaskPushNotificationConfig` |
| Get push config | `GetTaskPushNotificationConfig` | `GET /tasks/{task_id}/pushNotificationConfigs/{id}` | `GetTaskPushNotificationConfig` |
| List push configs | `ListTaskPushNotificationConfigs` | `GET /tasks/{task_id}/pushNotificationConfigs` | `ListTaskPushNotificationConfigs` |
| Delete push config | `DeleteTaskPushNotificationConfig` | `DELETE /tasks/{task_id}/pushNotificationConfigs/{id}` | `DeleteTaskPushNotificationConfig` |
| Extended card | `GetExtendedAgentCard` | `GET /extendedAgentCard` | `GetExtendedAgentCard` |

### JSON-RPC Request

```json
{
  "jsonrpc": "2.0",
  "id": "req-001",
  "method": "SendMessage",
  "params": {
    "message": {
      "messageId": "msg-001",
      "role": "ROLE_USER",
      "parts": [
        {"text": "What is the weather in San Francisco?"}
      ]
    },
    "configuration": {
      "acceptedOutputModes": ["text/plain"],
      "historyLength": 10,
      "returnImmediately": false
    }
  }
}
```

### Go SDK Request Type

```go
type SendMessageRequest struct {
    Tenant   string             `json:"tenant,omitempty"`
    Config   *SendMessageConfig `json:"configuration,omitempty"`
    Message  *Message           `json:"message"`
    Metadata map[string]any     `json:"metadata,omitempty"`
}

type SendMessageConfig struct {
    AcceptedOutputModes []string    `json:"acceptedOutputModes,omitempty"`
    ReturnImmediately   bool        `json:"returnImmediately,omitempty"`
    HistoryLength       *int        `json:"historyLength,omitempty"`
    PushConfig          *PushConfig `json:"pushNotificationConfig,omitempty"`
}
```

### Response: Task (default, blocking)

When `returnImmediately` is `false` (default), the server blocks until the task reaches a terminal or interrupted state:

```json
{
  "jsonrpc": "2.0",
  "id": "req-001",
  "result": {
    "task": {
      "id": "task-550e8400",
      "contextId": "ctx-660e8400",
      "status": {
        "state": "TASK_STATE_COMPLETED",
        "timestamp": "2026-03-23T10:05:00Z"
      },
      "artifacts": [
        {
          "artifactId": "art-001",
          "parts": [{"text": "The weather in San Francisco is 62F and sunny."}]
        }
      ]
    }
  }
}
```

### Response: Task (returnImmediately: true)

Returns immediately with in-progress state:

```json
{
  "jsonrpc": "2.0",
  "id": "req-001",
  "result": {
    "task": {
      "id": "task-550e8400",
      "contextId": "ctx-660e8400",
      "status": {
        "state": "TASK_STATE_WORKING",
        "timestamp": "2026-03-23T10:00:00Z"
      }
    }
  }
}
```

Client then polls with `GetTask` or subscribes with `SubscribeToTask`.

### Response: Direct Message (no task tracking)

Agent can respond with a Message directly instead of creating a Task:

```json
{
  "jsonrpc": "2.0",
  "id": "req-001",
  "result": {
    "message": {
      "messageId": "msg-resp-001",
      "role": "ROLE_AGENT",
      "parts": [{"text": "Hello! How can I help?"}]
    }
  }
}
```

### Go SDK Response Type

```go
// SendMessageResult is an interface implemented by both *Task and *Message
type SendMessageResult interface {
    Event
    isSendMessageResult()
}
func (*Task) isSendMessageResult()    {}
func (*Message) isSendMessageResult() {}
```

---

## 5. Streaming (SendStreamingMessage)

### JSON-RPC Request

Same request format as `SendMessage`, but method is `SendStreamingMessage`:

```json
{
  "jsonrpc": "2.0",
  "id": "req-002",
  "method": "SendStreamingMessage",
  "params": {
    "message": {
      "messageId": "msg-002",
      "role": "ROLE_USER",
      "parts": [{"text": "Write a long essay about AI"}]
    }
  }
}
```

### SSE Event Stream

For HTTP+JSON (REST) binding, streaming uses Server-Sent Events. Each event contains a `StreamResponse`:

```
POST /message:stream HTTP/1.1
Content-Type: application/json

{"message": {"messageId": "msg-002", "role": "ROLE_USER", "parts": [{"text": "Write an essay"}]}}

---

HTTP/1.1 200 OK
Content-Type: text/event-stream

data: {"task":{"id":"task-123","contextId":"ctx-456","status":{"state":"TASK_STATE_SUBMITTED"}}}

data: {"statusUpdate":{"taskId":"task-123","contextId":"ctx-456","status":{"state":"TASK_STATE_WORKING","timestamp":"2026-03-23T10:00:01Z"}}}

data: {"artifactUpdate":{"taskId":"task-123","contextId":"ctx-456","artifact":{"artifactId":"art-001","parts":[{"text":"Artificial intelligence has"}]},"append":false,"lastChunk":false}}

data: {"artifactUpdate":{"taskId":"task-123","contextId":"ctx-456","artifact":{"artifactId":"art-001","parts":[{"text":" transformed the way we"}]},"append":true,"lastChunk":false}}

data: {"artifactUpdate":{"taskId":"task-123","contextId":"ctx-456","artifact":{"artifactId":"art-001","parts":[{"text":" think about computing."}]},"append":true,"lastChunk":true}}

data: {"statusUpdate":{"taskId":"task-123","contextId":"ctx-456","status":{"state":"TASK_STATE_COMPLETED","timestamp":"2026-03-23T10:00:05Z"}}}
```

### StreamResponse Type

```go
type StreamResponse struct {
    Event  // Interface: one of *Message, *Task, *TaskStatusUpdateEvent, *TaskArtifactUpdateEvent
}

// JSON serializes as exactly one of:
// {"task": {...}}
// {"message": {...}}
// {"statusUpdate": {...}}
// {"artifactUpdate": {...}}
```

### Artifact Streaming

```go
type TaskArtifactUpdateEvent struct {
    Append    bool           `json:"append,omitempty"`     // true = append to previous chunk with same artifact ID
    Artifact  *Artifact      `json:"artifact"`
    ContextID string         `json:"contextId"`
    LastChunk bool           `json:"lastChunk,omitempty"`  // true = final chunk of this artifact
    TaskID    TaskID         `json:"taskId"`
    Metadata  map[string]any `json:"metadata,omitempty"`
}
```

### Status Update Events

```go
type TaskStatusUpdateEvent struct {
    ContextID string         `json:"contextId"`
    Status    TaskStatus     `json:"status"`
    TaskID    TaskID         `json:"taskId"`
    Metadata  map[string]any `json:"metadata,omitempty"`
}
```

---

## 6. Human-in-the-Loop (INPUT_REQUIRED)

### How It Works

1. Agent transitions task to `TASK_STATE_INPUT_REQUIRED`
2. The status message describes what input is needed
3. Client sends another `SendMessage` with the same `taskId`
4. Agent resumes processing

### Agent signals input needed (via streaming or blocking response):

```json
{
  "task": {
    "id": "task-123",
    "contextId": "ctx-456",
    "status": {
      "state": "TASK_STATE_INPUT_REQUIRED",
      "timestamp": "2026-03-23T10:00:02Z",
      "message": {
        "messageId": "msg-agent-001",
        "role": "ROLE_AGENT",
        "parts": [
          {"text": "I need your email address to proceed."},
          {
            "data": {
              "formFields": [
                {"name": "email", "type": "string", "label": "Email Address", "required": true}
              ]
            },
            "mediaType": "application/json"
          }
        ]
      }
    }
  }
}
```

### Client provides input (references same taskId):

```json
{
  "jsonrpc": "2.0",
  "id": "req-003",
  "method": "SendMessage",
  "params": {
    "message": {
      "messageId": "msg-003",
      "role": "ROLE_USER",
      "taskId": "task-123",
      "parts": [
        {
          "data": {"email": "user@example.com"},
          "mediaType": "application/json"
        }
      ]
    }
  }
}
```

### AUTH_REQUIRED

Works similarly but signals that authentication/authorization is needed before the agent can proceed. The status message should contain details about what credentials are required.

---

## 7. Authentication

### Security Scheme Types

Five scheme types are supported, mirroring OpenAPI 3.2:

```go
type SecurityScheme interface {
    isSecurityScheme()
}

// 1. API Key
type APIKeySecurityScheme struct {
    Description string                       `json:"description,omitempty"`
    Location    APIKeySecuritySchemeLocation  `json:"location"`  // "header", "query", "cookie"
    Name        string                       `json:"name"`      // header/query/cookie parameter name
}

// 2. HTTP Auth (Bearer, Basic, etc.)
type HTTPAuthSecurityScheme struct {
    BearerFormat string `json:"bearerFormat,omitempty"` // e.g., "JWT"
    Description  string `json:"description,omitempty"`
    Scheme       string `json:"scheme"`                 // e.g., "bearer", "basic"
}

// 3. OAuth2
type OAuth2SecurityScheme struct {
    Description       string     `json:"description,omitempty"`
    Flows             OAuthFlows `json:"flows"`
    Oauth2MetadataURL string     `json:"oauth2MetadataUrl,omitempty"`
}

// 4. OpenID Connect
type OpenIDConnectSecurityScheme struct {
    Description      string `json:"description,omitempty"`
    OpenIDConnectURL string `json:"openIdConnectUrl"`
}

// 5. Mutual TLS
type MutualTLSSecurityScheme struct {
    Description string `json:"description,omitempty"`
}
```

### OAuth2 Flows

```go
type AuthorizationCodeOAuthFlow struct {
    AuthorizationURL string            `json:"authorizationUrl"`
    RefreshURL       string            `json:"refreshUrl,omitempty"`
    Scopes           map[string]string `json:"scopes"`
    TokenURL         string            `json:"tokenUrl"`
    PKCERequired     bool              `json:"pkceRequired,omitempty"`
}

type ClientCredentialsOAuthFlow struct {
    RefreshURL string            `json:"refreshUrl,omitempty"`
    Scopes     map[string]string `json:"scopes"`
    TokenURL   string            `json:"tokenUrl"`
}

type DeviceCodeOAuthFlow struct {
    DeviceAuthorizationURL string            `json:"deviceAuthorizationUrl"`
    RefreshURL             string            `json:"refreshUrl,omitempty"`
    Scopes                 map[string]string `json:"scopes"`
    TokenURL               string            `json:"tokenUrl"`
}

// ImplicitOAuthFlow and PasswordOAuthFlow are DEPRECATED
```

### JSON Example in AgentCard

```json
{
  "securitySchemes": {
    "bearerAuth": {
      "http": {
        "scheme": "bearer",
        "bearerFormat": "JWT"
      }
    },
    "apiKeyAuth": {
      "apiKey": {
        "name": "X-API-Key",
        "location": "header"
      }
    },
    "oauth2Auth": {
      "oauth2": {
        "flows": {
          "clientCredentials": {
            "tokenUrl": "https://auth.example.com/token",
            "scopes": {
              "agent:read": "Read agent data",
              "agent:write": "Modify agent data"
            }
          }
        }
      }
    }
  },
  "securityRequirements": [
    {"schemes": {"bearerAuth": []}},
    {"schemes": {"oauth2Auth": ["agent:read"]}}
  ]
}
```

---

## 8. Push Notifications

### Configuration

```go
type PushConfig struct {
    ID    string        `json:"id,omitempty"`
    Auth  *PushAuthInfo `json:"authentication,omitempty"`
    Token string        `json:"token,omitempty"`
    URL   string        `json:"url"`
}

type PushAuthInfo struct {
    Credentials string `json:"credentials,omitempty"`
    Scheme      string `json:"scheme"`  // HTTP auth scheme, e.g., "Bearer"
}
```

### Create push notification config

```json
{
  "jsonrpc": "2.0",
  "id": "req-004",
  "method": "CreateTaskPushNotificationConfig",
  "params": {
    "taskId": "task-123",
    "config": {
      "url": "https://client.example.com/a2a/webhook",
      "token": "session-unique-token",
      "authentication": {
        "scheme": "Bearer",
        "credentials": "webhook-secret-token"
      }
    }
  }
}
```

### Webhook payload (sent by server to client's webhook URL)

The server sends `TaskStatusUpdateEvent` or `TaskArtifactUpdateEvent` as HTTP POST to the configured URL, using the authentication info provided.

---

## 9. Error Handling

### A2A Error Sentinel Values

```go
var (
    ErrParseError                   = errors.New("parse error")
    ErrInvalidRequest               = errors.New("invalid request")
    ErrMethodNotFound               = errors.New("method not found")
    ErrInvalidParams                = errors.New("invalid params")
    ErrInternalError                = errors.New("internal error")
    ErrServerError                  = errors.New("server error")
    ErrTaskNotFound                 = errors.New("task not found")
    ErrTaskNotCancelable            = errors.New("task cannot be canceled")
    ErrPushNotificationNotSupported = errors.New("push notification not supported")
    ErrUnsupportedOperation         = errors.New("this operation is not supported")
    ErrUnsupportedContentType       = errors.New("incompatible content types")
    ErrInvalidAgentResponse         = errors.New("invalid agent response")
    ErrExtendedCardNotConfigured    = errors.New("extended card not configured")
    ErrExtensionSupportRequired     = errors.New("extension support required")
    ErrVersionNotSupported          = errors.New("this version is not supported")
    ErrUnauthenticated              = errors.New("unauthenticated")
    ErrUnauthorized                 = errors.New("permission denied")
)
```

### Error Type

```go
type Error struct {
    Err     error
    Message string
    Details map[string]any
}

func NewError(err error, message string) *Error
func (e *Error) WithDetails(details map[string]any) *Error
```

### JSON-RPC Error Response

```json
{
  "jsonrpc": "2.0",
  "id": "req-001",
  "error": {
    "code": -32000,
    "message": "task not found",
    "data": {
      "taskId": "nonexistent-id"
    }
  }
}
```

---

## 10. Go SDK - Complete Server Example

### Minimal REST Server

```go
package main

import (
    "context"
    "fmt"
    "iter"
    "log"
    "net"
    "net/http"

    "github.com/a2aproject/a2a-go/v2/a2a"
    "github.com/a2aproject/a2a-go/v2/a2asrv"
)

// Implement the AgentExecutor interface
type myAgent struct{}

func (*myAgent) Execute(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
    return func(yield func(a2a.Event, error) bool) {
        // Emit a status update
        yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, nil), nil)

        // Do work...
        response := a2a.NewMessageForTask(
            a2a.MessageRoleAgent,
            execCtx,
            a2a.NewTextPart("Here is your result!"),
        )
        yield(response, nil)

        // Emit an artifact
        yield(a2a.NewArtifactEvent(
            execCtx,
            a2a.NewTextPart("The detailed analysis..."),
        ), nil)

        // Mark completed
        yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCompleted, nil), nil)
    }
}

func (*myAgent) Cancel(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
    return func(yield func(a2a.Event, error) bool) {
        yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil), nil)
    }
}

func main() {
    card := &a2a.AgentCard{
        Name:        "My Agent",
        Description: "An example A2A agent",
        Version:     "1.0.0",
        SupportedInterfaces: []*a2a.AgentInterface{
            a2a.NewAgentInterface("http://localhost:8080", a2a.TransportProtocolHTTPJSON),
        },
        DefaultInputModes:  []string{"text/plain"},
        DefaultOutputModes: []string{"text/plain"},
        Capabilities:       a2a.AgentCapabilities{Streaming: true},
        Skills: []a2a.AgentSkill{
            {
                ID:          "analyze",
                Name:        "Analyze",
                Description: "Analyze input data",
                Tags:        []string{"analysis"},
            },
        },
    }

    handler := a2asrv.NewHandler(&myAgent{})

    mux := http.NewServeMux()
    mux.Handle("/", a2asrv.NewRESTHandler(handler))
    mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(card))

    listener, _ := net.Listen("tcp", ":8080")
    log.Println("A2A server listening on :8080")
    log.Fatal(http.Serve(listener, mux))
}
```

### ExecutorContext (available in Execute/Cancel)

```go
type ExecutorContext struct {
    Message      *a2a.Message   // The message that triggered execution (nil for cancel)
    TaskID       a2a.TaskID     // Task ID (new or existing)
    StoredTask   *a2a.Task      // Non-nil if continuing an existing task
    RelatedTasks []*a2a.Task    // Referenced tasks if configured
    ContextID    string         // Context ID for grouping
    Metadata     map[string]any // Request metadata
    User         *User          // Authenticated user info
    ServiceParams *ServiceParams // Protocol-level service parameters
    Tenant       string         // Multi-tenant ID
}
```

### Handler Options

```go
// Custom task store (default: in-memory)
a2asrv.WithTaskStore(store taskstore.Store)

// Enable push notifications
a2asrv.WithPushNotifications(store push.ConfigStore, sender push.Sender)

// Concurrency limits
a2asrv.WithConcurrencyConfig(config limiter.ConcurrencyConfig)

// Custom event queue (default: in-memory)
a2asrv.WithEventQueueManager(manager eventqueue.Manager)

// Capability checks (validates against AgentCard capabilities)
a2asrv.WithCapabilityChecks(capabilities *a2a.AgentCapabilities)

// Custom logger
a2asrv.WithLogger(logger *slog.Logger)

// Panic recovery handler
a2asrv.WithExecutionPanicHandler(handler func(r any) error)

// Distributed/cluster mode
a2asrv.WithClusterMode(config ClusterConfig)
```

---

## 11. Go SDK - Complete Client Example

```go
package main

import (
    "context"
    "log"

    "github.com/a2aproject/a2a-go/v2/a2a"
    "github.com/a2aproject/a2a-go/v2/a2aclient"
    "github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
)

func main() {
    ctx := context.Background()

    // Step 1: Resolve agent card from well-known URL
    card, err := agentcard.DefaultResolver.Resolve(ctx, "http://localhost:8080")
    if err != nil {
        log.Fatalf("Failed to resolve agent card: %v", err)
    }

    // Step 2: Create client from card (auto-selects transport based on supportedInterfaces)
    client, err := a2aclient.NewFromCard(ctx, card)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    // Step 3: Send a message (blocking by default)
    msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("Analyze the Q1 report"))
    resp, err := client.SendMessage(ctx, &a2a.SendMessageRequest{Message: msg})
    if err != nil {
        log.Fatalf("SendMessage failed: %v", err)
    }

    // resp is a2a.SendMessageResult - can be *a2a.Task or *a2a.Message
    switch v := resp.(type) {
    case *a2a.Task:
        log.Printf("Task %s completed with state: %s", v.ID, v.Status.State)
        for _, art := range v.Artifacts {
            log.Printf("Artifact: %s", art.Name)
        }
    case *a2a.Message:
        log.Printf("Direct response: %s", v.Parts[0].Text())
    }
}
```

### Streaming Client Usage

```go
// SendStreamingMessage returns iter.Seq2[a2a.Event, error]
for event, err := range client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{Message: msg}) {
    if err != nil {
        log.Fatalf("Stream error: %v", err)
        break
    }
    switch v := event.(type) {
    case *a2a.TaskStatusUpdateEvent:
        log.Printf("Status: %s", v.Status.State)
    case *a2a.TaskArtifactUpdateEvent:
        log.Printf("Artifact chunk: %s (append=%v, last=%v)",
            v.Artifact.Parts[0].Text(), v.Append, v.LastChunk)
    case *a2a.Message:
        log.Printf("Message: %s", v.Parts[0].Text())
    case *a2a.Task:
        log.Printf("Task snapshot: %s state=%s", v.ID, v.Status.State)
    }
}
```

### Continuing a Task (INPUT_REQUIRED)

```go
// After receiving a task in INPUT_REQUIRED state:
followUp := a2a.NewMessage(a2a.MessageRoleUser,
    a2a.NewDataPart(map[string]any{"email": "user@example.com"}),
)
followUp.TaskID = existingTask.ID  // Reference the existing task

resp, err = client.SendMessage(ctx, &a2a.SendMessageRequest{Message: followUp})
```

---

## 12. Key SDK Import Paths

```go
import (
    "github.com/a2aproject/a2a-go/v2/a2a"            // Core types (Message, Task, Part, etc.)
    "github.com/a2aproject/a2a-go/v2/a2asrv"          // Server handler + transports
    "github.com/a2aproject/a2a-go/v2/a2aclient"       // Client
    "github.com/a2aproject/a2a-go/v2/a2aclient/agentcard" // Agent card resolver
    "github.com/a2aproject/a2a-go/v2/a2agrpc/v1"      // gRPC transport
    "github.com/a2aproject/a2a-go/v2/a2asrv/taskstore" // Task persistence interface
    "github.com/a2aproject/a2a-go/v2/a2asrv/eventqueue" // Event queue management
    "github.com/a2aproject/a2a-go/v2/a2asrv/push"     // Push notification support
    "github.com/a2aproject/a2a-go/v2/a2asrv/limiter"  // Concurrency control
)
```

Install: `go get github.com/a2aproject/a2a-go/v2`

Requires: Go 1.24.4+

---

## 13. Protocol Version

```go
const Version ProtocolVersion = "1.0"
```

The version is communicated via the `protocolVersion` field in `AgentInterface`. For JSON-RPC, it can also be sent as an `A2A-Version` header. Server rejects unsupported versions with `ErrVersionNotSupported`.

---

## 14. Quick Reference: JSON-RPC Envelope

```go
// Request
type ServerRequest struct {
    JSONRPC string          `json:"jsonrpc"` // Always "2.0"
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
    ID      any             `json:"id"`
}

// Response
type ServerResponse struct {
    JSONRPC string `json:"jsonrpc"` // Always "2.0"
    ID      any    `json:"id"`
    Result  any    `json:"result,omitempty"`
    Error   *Error `json:"error,omitempty"`
}
```
