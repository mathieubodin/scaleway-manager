# Go interfaces between the stories of #42

The exact Go API each story provides to the others, and the cross-story rules. Every story reads this file; a story implements its own part exactly as written here and uses the others' parts as given. Where a story spec and this file differ on a signature, this file wins.

## internal/kube (story 1)

```go
type Token string // String, GoString, LogValue: "[REDACTED]"; MarshalJSON: "\"[REDACTED]\""

type Resource int

const (
    Instances Resource = iota + 1
    LoadBalancers
    ScalewaySecrets
    NamespaceRoles
    Namespaces
    Secrets
    CRDs
    Deployments
)

func (r Resource) String() string      // the plural, e.g. "instances"
func (r Resource) Namespaced() bool
func (r Resource) Kind() string         // e.g. "Instance", "Namespace"; List kind is Kind()+"List"
func Resources() []Resource             // all eight, in the order above

func Path(r Resource, namespace, name string, status bool) (string, error)
func ParsePath(p string) (r Resource, namespace, name string, status bool, err error) // inverse of Path, for kubetest

type Client struct{ /* unexported */ }

func NewInCluster(getenv func(string) string, caPEM []byte) (*Client, error)
func (c *Client) List(ctx context.Context, tok Token, r Resource, namespace string) ([]json.RawMessage, error)
func (c *Client) Get(ctx context.Context, tok Token, r Resource, namespace, name string) (json.RawMessage, error)
func (c *Client) MergePatch(ctx context.Context, tok Token, r Resource, namespace, name string, patch []byte) (json.RawMessage, error)
func (c *Client) PatchStatus(ctx context.Context, tok Token, r Resource, namespace, name string, patch []byte) (json.RawMessage, error)

type APIError struct {
    Method string
    Path   string
    Code   int
    Body   []byte
}

func (e *APIError) Error() string // exactly "kube: <METHOD> <path>: <code>"

var (
    ErrUnknownResource     = errors.New("kube: unknown resource")
    ErrOperationNotAllowed = errors.New("kube: operation not allowed")
    ErrInvalidName         = errors.New("kube: invalid name")
    ErrEmptyToken          = errors.New("kube: empty token")
    ErrNoAPIServer         = errors.New("kube: KUBERNETES_SERVICE_HOST or KUBERNETES_SERVICE_PORT not set")
    ErrNoCA                = errors.New("kube: no certificate in CA PEM")
    ErrBadResponse         = errors.New("kube: bad response")
)
```

### Path

- An empty name gives the collection path.
- **Namespaced kind:**
  - an empty namespace with an empty name gives the all-namespaces path;
  - an empty namespace with a non-empty name returns `ErrInvalidName`.
- **Cluster kind:** a non-empty namespace returns `ErrInvalidName`.
- **`status` true:**
  - allowed only for `Instances`, `LoadBalancers` and `ScalewaySecrets` with a name;
  - on another kind, it returns `ErrOperationNotAllowed`;
  - with an empty name, it returns `ErrInvalidName`.
- `ParsePath` accepts exactly the paths `Path` builds and returns `ErrUnknownResource` for any other.

### Client errors

- **Precedence**, when several errors apply:
  1. `ErrUnknownResource`;
  2. `ErrOperationNotAllowed`;
  3. `ErrInvalidName`;
  4. `ErrEmptyToken`.
- **`NewInCluster`** returns `ErrNoAPIServer` before it looks at the CA.
- **`List(Deployments, "")`** returns `ErrInvalidName`.
- **`ErrBadResponse`** is returned, wrapped, when a 2xx body is not JSON or a list has no `items` array.
- **Patches** are sent unchanged: the client does not inspect them.
- **HTTP limits:**
  - the `http.Client` has a 30 s `Timeout`;
  - response bodies are read up to 16 MiB, and a larger body returns `ErrBadResponse`.

## internal/kube/kubetest (story 2)

```go
type Server struct {
    Host  string
    Port  string
    CAPEM []byte
    // unexported
}

type Request struct {
    Method      string
    RequestURI  string
    ContentType string
    Token       string // "" when absent
    Body        []byte
}

func New(t testing.TB) *Server
func (s *Server) Put(r kube.Resource, obj json.RawMessage)
func (s *Server) Object(r kube.Resource, namespace, name string) (json.RawMessage, bool)
func (s *Server) Delete(r kube.Resource, namespace, name string)
func (s *Server) ExpectToken(tok string)
func (s *Server) Tokens() []string
func (s *Server) Requests() []Request
func (s *Server) Fail(method, pathPrefix string, code int) // method "" matches any
func (s *Server) ClearFaults()
```

### Routing and errors

- Routing goes through `kube.ParsePath`. Served methods are `GET` (list or get) and `PATCH` on the object and on `/status`. HTTP `DELETE` is not served: tests delete with `Server.Delete`.
- **Error bodies** are a Kubernetes `Status` object: `{"kind":"Status","apiVersion":"v1","status":"Failure","code":<n>,"reason":"<Unauthorized|NotFound|Conflict|UnsupportedMediaType|InternalError>"}`.
- **Order of checks:**
  1. the request is recorded (every request, rejected ones included; an absent token is recorded as `""`);
  2. token check (401);
  3. faults;
  4. `415`;
  5. 404 and 409.
- **`Fail`** matches `pathPrefix` against the URL path.

### Store

- **`resourceVersion`** is one global decimal counter starting at 1. `Put`, `Delete` (when it sets `deletionTimestamp`) and every patch that changes the object take the next value.
- **`generation`:** `Put` keeps `metadata.generation` when given, else sets 1.
- **Patches:**
  - A patch on an absent object gets 404.
  - In a patch, `metadata.name` and `metadata.namespace` are ignored.
  - `metadata.resourceVersion` is used only for the 409 check and is never stored from the patch.
- **`Put`** calls `t.Fatalf` when the scope does not match: a namespaced kind without `metadata.namespace`, or a cluster kind with one.

## cmd/fake-operator engine (story 3)

```go
type Clock interface{ Now() time.Time }

type kind struct {
    resource  kube.Resource
    name      string // "Instance", "LoadBalancer", "ScalewaySecret": the <Kind> of messages
    finalizer string
}

var (
    instanceKind       = kind{kube.Instances, "Instance", "scaleway.mathieubodin.io/instance-finalizer"}
    loadBalancerKind   = kind{kube.LoadBalancers, "LoadBalancer", "scaleway.mathieubodin.io/loadbalancer-finalizer"}
    scalewaySecretKind = kind{kube.ScalewaySecrets, "ScalewaySecret", "scaleway.mathieubodin.io/secret-finalizer"}
    kinds              = []kind{instanceKind, loadBalancerKind, scalewaySecretKind}
)

type operator struct {
    kube      *kube.Client
    clock     Clock
    rand      *rand.Rand // math/rand/v2
    tokenPath string
    log       *slog.Logger
    token     kube.Token // read at the start of each pass
}

func newOperator(c *kube.Client, clock Clock, rnd *rand.Rand, tokenPath string, log *slog.Logger) *operator
func (o *operator) pass(ctx context.Context) error

// lifecycle.go helpers, used by stories 4 and 5:
func (o *operator) writeStatus(ctx context.Context, k kind, obj map[string]any, status map[string]any) error
func (o *operator) setFinalizer(ctx context.Context, k kind, obj map[string]any, present bool) error
func defaultStatus(k kind) map[string]any // the Rust defaults of the companion's "Status fields"
func currentStatus(k kind, obj map[string]any) map[string]any // obj's status, or defaultStatus(k) when absent
```

### Objects and helpers

- **`obj`** is decoded with `json.Decoder.UseNumber`, so numbers are `json.Number`. Status numbers (`current_version`, `observed_generation`) are written as JSON integers.
- **`writeStatus`** sends `{"status":{...}}` with every field of the kind, absent ones as `null`. It sends nothing when the result equals `currentStatus`, and updates `obj` in place after a write.
- **`setFinalizer`** sends the whole list with the kind's finalizer added or removed, keeps the others, and sends nothing when there is no change.

### `pass`

- **Token:** it reads `tokenPath` first. An unreadable or empty file makes `pass` return that error before any `List`.
- **Error handling:**
  - per-kind `List` errors are logged (`msg` `list failed`, attributes `kind`, `error`) and that kind is skipped;
  - per-object errors are logged (`msg` `object failed`, attributes `kind`, `namespace`, `name`, `error`) and the pass goes on.
- **Order for each object:**
  1. `o.deleting`;
  2. `o.checkBefore`;
  3. `revokedBeforeFinalizer` (story 5; when it returns true, the engine calls `o.applyCause`, which writes the `Error`, and ends the object's pass);
  4. `o.setFinalizer(present)`: if it wrote, end the object's pass;
  5. `o.checkAfter`;
  6. `specListsValid`;
  7. `o.applyCause`;
  8. the lifecycle step.

### Lifecycle step

- `status.scaleway_id` null or absent → creation (`Syncing`; for ScalewaySecret, `Syncing` then `Synced` in the same pass).
- `sync_state` other than `Synced` with a `scaleway_id` → `Synced`.
- `Synced`: for ScalewaySecret, a version push when `metadata.generation` ≠ `observed_generation`; otherwise nothing.

### ScalewaySecret creation patches

- **First patch:** `scaleway_id` set; `current_version`, `last_synced_resource_version`, `observed_generation` and `error_message` null; `sync_state: Syncing`.
- **Second patch:** `current_version: 1`, `observed_generation` = `metadata.generation`, `sync_state: Synced`, `error_message: null`.

### Generated values

- **`scaleway_id`:** two `o.rand.Uint64()` calls give 16 bytes (big-endian). Set the version (byte 6 = `0x40 | b&0x0f`) and the variant (byte 8 = `0x80 | b&0x3f`), then format lowercase 8-4-4-4-12.
- **IP last octet:** `o.rand.IntN(254) + 1`, drawn after the id, at the step that writes the IP.
- **`created_at`:** `o.clock.Now().UTC().Format(time.RFC3339)`.

### Other rules

- **Deletion stub (story 3):** with `deletionTimestamp`, it calls `setFinalizer(false)` and returns handled. When the finalizer is already absent, there is no patch.
- **`main.go`:**
  - `rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 1))`;
  - `signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)`;
  - `HEALTH_ADDR` is read inside `run`.

## Leaf methods

All of them are methods on `*operator`, so they can use `o.kube`, `o.token`, `writeStatus` and `setFinalizer`.

```go
func (o *operator) deleting(ctx context.Context, k kind, obj map[string]any) (handled bool, err error)         // story 4
func (o *operator) checkBefore(ctx context.Context, k kind, obj map[string]any) (failed bool, err error)       // story 4
func (o *operator) checkAfter(ctx context.Context, k kind, obj map[string]any) (failed bool, err error)        // story 4
func specListsValid(k kind, obj map[string]any) bool                                                            // story 4
func revokedBeforeFinalizer(k kind, obj map[string]any) bool                                                   // story 5 (story 3 stub: false)
func (o *operator) applyCause(ctx context.Context, k kind, obj map[string]any) (handled bool, err error)      // story 5
func run(getenv func(string) string, stdout io.Writer, sigs <-chan os.Signal, tokenPath, caPath string, clock Clock, rnd *rand.Rand) int // story 6
```

- A non-nil `err` is a failed API call. The engine logs it and ends the object's pass.
- Story 3 creates all of these with no-effect bodies, so story 5 does not edit `operator.go`: its "single call in `operator.go`" already exists in story 3's code.

### Story 4 details

- **Region check:** `deleting` runs the ScalewaySecret region check for every ScalewaySecret, deleted or not. An invalid region writes the region `Error` and returns handled; a new secret then gets no finalizer. A deleted secret with an invalid region keeps its finalizer.
- **`Kubernetes API error`** is that exact text, with nothing appended (the operator's `for_status`).
- **"At creation"** for the ScalewaySecret `project-id` check means `status.scaleway_id` is null or absent.
- **UUID:** the forms Rust's `Uuid::parse_str` accepts, in any case:
  - hyphenated 8-4-4-4-12;
  - 32 hex digits;
  - braced `{…}`;
  - `urn:uuid:` prefixed.
- **Role:** a NamespaceRole without `spec.scaleway_role` counts as the read-only role `''`.

### Story 5 details

- **Synced** means `sync_state` is `Synced`. The `Error` cause on a ScalewaySecret with a `scaleway_id` has no effect whatever its `sync_state`.
- **Invalid `source-version`:**
  - it is reported on a ScalewaySecret with no cause, whatever its state;
  - on Instance and LoadBalancer it is ignored;
  - a valid one waits for `Synced`;
  - a cause takes precedence over `source-version`, and `source-version` takes precedence over a generation change in the same pass.
- **`message`** also replaces `Source Secret unavailable (see operator logs)`.
- **Annotation keys** are constants in `causes.go`: `outcomeAnnotation`, `messageAnnotation`, `sourceVersionAnnotation`. Story 4 reads `outcome` through its own local constant with the same value.

### Story 6 details

- **Start:** the guard reads the token from `tokenPath` first. An unreadable token logs one ERROR and returns 1.
- **Start-up errors:** `msg` `start failed` with attribute `error`; the guard uses `msg` `real operator found` with attribute `deployment` (`<namespace>/<name>`).
- **Loop:**
  - the first pass starts at once;
  - each pass runs with a context that the signal does not cancel, bounded by a 30 s timeout;
  - a signal ends the loop after the in-flight pass;
  - a pass whose `pass` returned an error before listing does not count as ended.
- **Health server:**
  - `GET /healthz` returns 503 with an empty body;
  - other methods get 405 and other paths 404, through `ServeMux` patterns;
  - `ReadHeaderTimeout` 10 s, `Shutdown` timeout 5 s.
- **Signals:** SIGINT is handled like SIGTERM, and a second signal is ignored.

## Story 7 details

- **Names:** every object is named `fake-operator`, with label `app.kubernetes.io/name: fake-operator`, which the Deployment's selector matches.
- **CI:** the new step comes right after `make build-image`, before "Run the image tests".
- **Formatting:** the JSON files use a 2-space indent and must pass `biome ci .`, which `make check` runs through `lint-workflows`.
