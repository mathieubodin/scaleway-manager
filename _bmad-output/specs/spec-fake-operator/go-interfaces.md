# Go interfaces between the stories of #42

The exact Go API each story provides to the others, and the cross-story rules. Every story reads this file; a story implements its own part exactly as written here and uses the others' parts as given. Where a story spec and this file differ on a signature, this file wins. The behavior of `cmd/fake-operator` is described only in `fake-operator-contract.md`; this file points to its sections.

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
    token     kube.Token      // read at the start of each pass
    stop      <-chan struct{} // story 6: closed on a signal; nil never stops
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
- **`writeStatus`** and **`setFinalizer`** follow the contract's "Status fields" and "Writes". Both update `obj` in place after a write.

### `pass`

- **Token and errors:** the contract's "Runtime", item "Pass".
- **Order for each object**, implementing the contract's "Pass order":
  1. `o.deleting`;
  2. `o.checkBefore`;
  3. `revokedBeforeFinalizer` (story 5; when it returns true, the engine calls `o.applyCause`, which writes the `Error`, and ends the object's pass);
  4. `o.setFinalizer(present)`: if it wrote, end the object's pass;
  5. `o.checkAfter`;
  6. `specListsValid`;
  7. `o.applyCause`;
  8. the lifecycle step.

### Lifecycle step

The contract's "Lifecycle values", ScalewaySecret creation patches included.

### Generated values

Their format is in the contract's "Lifecycle values"; they are drawn as follows.

- **`scaleway_id`:** two `o.rand.Uint64()` calls give 16 bytes (big-endian). Set the version (byte 6 = `0x40 | b&0x0f`) and the variant (byte 8 = `0x80 | b&0x3f`), then format lowercase 8-4-4-4-12.
- **IP last octet:** `o.rand.IntN(254) + 1`, drawn after the id, at the step that writes the IP.
- **`created_at`:** from `o.clock.Now()`.

### Other rules

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

- **Region check:** `deleting` runs the contract's ScalewaySecret region check ("Pass order", step 1) for every ScalewaySecret, deleted or not, and returns handled when it fails.
- Every other rule of story 4 is in the contract's "Pass order" and "Checks".

### Story 5 details

- **Annotation keys** are constants in `causes.go`: `outcomeAnnotation`, `messageAnnotation`, `sourceVersionAnnotation`. Story 4 reads `outcome` through its own local constant with the same value.
- **`source-version`** is parsed with `strconv.ParseUint(v, 10, 32)`.
- Every other rule of story 5 is in the contract's "Causes".

### Story 6 details

- `run` sets `op.stop` to a channel closed by the signal. `pass` checks it before each kind and each object.
- `/healthz` is routed through `ServeMux` patterns.
- Every other rule of story 6 is in the contract's "Runtime".

## Story 7 details

The contract's "Packaging and RBAC".

## Clarifications after cold-start round 2

### internal/kube

- **`Kind()` strings**, in the order of the constants:
  - `Instance`, `LoadBalancer`, `ScalewaySecret`, `NamespaceRole`;
  - `Namespace`, `Secret`, `CustomResourceDefinition`, `Deployment`.
- **Empty name:** `Get`, `MergePatch` and `PatchStatus` with an empty name return `ErrInvalidName`.
- **List items:** a list with `"items": null` or `[]` returns an empty, non-nil slice.
- **Names of `Namespaces` objects** follow the DNS-1123 label rule, like every namespace argument.
- **Large non-2xx body:** over 16 MiB, it gives an `*APIError` whose `Body` holds the first 16 MiB.

### kubetest

- **List order:** list responses return items sorted by namespace, then by name.
- **List body:** `{"apiVersion":"v1","kind":"<Kind>List","metadata":{"resourceVersion":"<current>"},"items":[...]}`. A namespace with no objects gives an empty list, not 404.
- **`Tokens()`:** one entry per received request, in order. An absent token is recorded as `""`.
- **Finalizers during deletion:** kubetest does not refuse a patch that adds a finalizer while `deletionTimestamp` is set (decision D10: no 422). The fake never sends one, because deletion comes first.
- **Status reasons:** a code outside the five listed reasons gets `http.StatusText(code)` with spaces removed. Unserved methods and collection `PATCH` get 405 `MethodNotAllowed`. An unparseable path gets 404 `NotFound`. A non-object patch body gets 400 `BadRequest`. These checks come after the token check.
- **Patch responses:** a successful `PATCH` returns the updated object, or the last state of an object it removed.
- **`deletionTimestamp`:** `time.Now().UTC().Format(time.RFC3339)`. `Delete` does not change `generation`. `Delete` on an absent object, or on one already deleting, does nothing.
- **`status` key:**
  - an object patch ignores it only for the three kinds with `/status`;
  - other kinds apply it.
- **409:** it applies to both endpoints, on any mismatch of a `metadata.resourceVersion` carried by the patch.

### Engine (story 3)

- **Reading the token:** `func readToken(path string) (kube.Token, error)`, in `operator.go`, reads the file and applies `strings.TrimSpace`. An empty result is an error. `pass` uses it, and so does story 6 for the start-up guard.
- **Dispatch note:** the "dispatch note" of a story spec is its `invoke_dev_with` in `stories.yaml`: read scaleway-operator only at tag `scaleway-operator-v0.1.15`, with `git show`.
- **Stub location:** the `revokedBeforeFinalizer` stub lives in `causes.go`.

## Clarifications after cold-start round 3

- **Precedence.** On behavior, `fake-operator-contract.md` wins over this file and over the story specs. On signatures, this file wins over a story spec: the call order of `pass` (8 steps, with `revokedBeforeFinalizer`), `signal.Notify` on SIGTERM and SIGINT, and the helpers listed here.
- **Annotation keys in story 4.** Story 4 declares its annotation keys as function-local constants, never at package level, so they cannot clash with story 5's `outcomeAnnotation` and `messageAnnotation` when the two leaves merge.
- **Migration note.** No story writes `docs/migrations/next/42.md`. The deliver step of the epic writes it ("Migration: no impact", D14).
