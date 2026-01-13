# CIE Tools Reference

Complete reference for all CIE MCP tools. Each tool is documented with parameters, examples, output format, usage tips, and common mistakes.

> **Target Audience:** Developers integrating CIE with Claude Code, Cursor, or other MCP clients.
>
> **Related Documentation:**
> - [Getting Started](./getting-started.md) - Quick start guide
> - [MCP Integration](./mcp-integration.md) - Setup with AI tools
> - [Architecture](./architecture.md) - How CIE works internally

---

## Quick Reference

| Task | Best Tool | Example Parameter |
|------|-----------|-------------------|
| Find exact text like `.GET(`, `->` | `cie_grep` | `text=".GET("` |
| List HTTP/REST endpoints | `cie_list_endpoints` | `path_pattern="apps/gateway"` |
| Trace call path to function | `cie_trace_path` | `target="RegisterRoutes"` |
| Search by meaning/concept | `cie_semantic_search` | `query="authentication logic"` |
| Answer architectural questions | `cie_analyze` | `question="What are entry points?"` |
| Find function by name | `cie_find_function` | `name="BuildRouter"` |
| What calls this function? | `cie_find_callers` | `function_name="HandleAuth"` |
| What does this function call? | `cie_find_callees` | `function_name="HandleAuth"` |
| Get function source code | `cie_get_function_code` | `function_name="BuildRouter"` |
| Find interface implementations | `cie_find_implementations` | `interface_name="Repository"` |
| Find type/interface/struct | `cie_find_type` | `name="UserService"` |
| Explore directory structure | `cie_directory_summary` | `path="internal/cie"` |
| Check index health | `cie_index_status` | `path_pattern="internal/cie"` |
| Verify patterns absent (security) | `cie_verify_absence` | `patterns=["apiKey", "password"]` |

---

## Tool Categories

- [Search Tools](#search-tools) - Find code by pattern or meaning
- [Navigation Tools](#navigation-tools) - Move around codebase structure
- [Analysis Tools](#analysis-tools) - Understand architecture and relationships
- [Administrative Tools](#administrative-tools) - Index management and schema

---

## Search Tools

### cie_semantic_search

Search code by meaning using embeddings. Returns functions semantically similar to the query.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `query` | string | Yes | — | Natural language description of what you're looking for |
| `limit` | int | No | 10 | Maximum number of results to return |
| `min_similarity` | float | No | 0.0 | Minimum similarity threshold (0.0-1.0, e.g., 0.7 = 70%) |
| `path_pattern` | string | No | — | Filter by file path regex (e.g., "apps/gateway") |
| `role` | string | No | `source` | Filter by code role: `source`, `test`, `any`, `generated`, `entry_point`, `router`, `handler` |
| `exclude_paths` | string | No | — | Exclude paths regex (e.g., "metrics\|dlq\|telemetry") |
| `exclude_anonymous` | bool | No | true | Exclude anonymous/arrow functions ($anon_X, $arrow_X) |

**Example:**

```json
{
  "query": "authentication middleware that validates JWT tokens",
  "min_similarity": 0.7,
  "path_pattern": "internal/http",
  "role": "handler"
}
```

**Output:**

```markdown
🟢 **AuthMiddleware** (92% match)
File: internal/http/auth.go:42-67
Signature: func AuthMiddleware(next http.Handler) http.Handler

🟡 **ValidateToken** (68% match)
File: internal/auth/jwt.go:23-45
Signature: func ValidateToken(token string) (*Claims, error)
```

Confidence indicators: 🟢 High (≥75%), 🟡 Medium (50-75%), 🔴 Low (<50%)

**Tips:**

- ⚡ **Use English queries** - Keyword boosting matches query terms against English function names
- 🎯 **Set `min_similarity: 0.7+`** for high-confidence results only (reduces noise)
- 📁 **Combine with `path_pattern`** to narrow search scope (e.g., "internal/cie")
- 🔍 **Use `role="handler"`** to find specific function types (handlers, routers, entry points)
- 🧹 **Exclude noise** with `exclude_paths="metrics|telemetry|dlq"` for cleaner results

**Common Mistakes:**

- ❌ Using non-English queries (reduces keyword boost effectiveness)
- ❌ Not excluding test files (set `role="source"` or `exclude_paths="_test[.]go"`)
- ❌ Setting `min_similarity` too low (gets noisy results below 0.5)
- ❌ Not using `exclude_paths` when results contain noise from metrics/monitoring code
- ✅ Start broad, then narrow with filters if too many results

---

### cie_grep

Ultra-fast literal text search (like grep). Searches for EXACT text - no regex. Perfect for finding specific code patterns like `.GET(`, `->`, `::`, `import`.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `text` | string | No* | — | Single exact text to search for |
| `texts` | []string | No* | — | Multiple patterns to search in parallel (returns grouped results) |
| `path` | string | No | — | Filter by file path substring (e.g., "routes", "internal/cie") |
| `exclude_pattern` | string | No | — | Regex pattern to EXCLUDE files (e.g., "_test[.]go", "[.]pb[.]go") |
| `case_sensitive` | bool | No | false | If true, search is case-sensitive |
| `context_lines` | int | No | 0 | Number of lines to show before/after each match (like `grep -C`) |
| `limit` | int | No | 30 | Maximum results to return |

\* Either `text` or `texts` must be provided (not both).

**Example:**

```json
{
  "text": ".GET(",
  "path": "apps/gateway",
  "exclude_pattern": "_test[.]go",
  "context_lines": 2,
  "limit": 20
}
```

**Multi-pattern batch search:**

```json
{
  "texts": ["access_token", "refresh_token", "api_key"],
  "path": "ui/src",
  "limit": 50
}
```

**Output:**

```markdown
## Pattern: .GET(

Found 15 matches in 8 files

**RegisterRoutes** (apps/gateway/internal/http/routes.go:45)
```go
func RegisterRoutes(r *chi.Mux) {
    r.GET("/health", healthHandler)
    r.GET("/metrics", metricsHandler)
}
```

**BuildAPIRouter** (apps/gateway/internal/api/router.go:67)
```go
apiRouter := chi.NewRouter()
apiRouter.GET("/users", listUsers)
apiRouter.GET("/users/{id}", getUser)
```
```

**Tips:**

- ⚡ **Fastest search tool** - Use for literal text patterns (no regex overhead)
- 📦 **Batch search with `texts`** - Search multiple patterns in one call (reduces API overhead)
- 🧹 **Exclude test files** - Use `exclude_pattern="_test[.]go"` to focus on source code
- 🔍 **Context lines** - Set `context_lines=2` to see code around matches
- ⚠️ **Use `[.]` not `\.`** in `exclude_pattern` for literal dots (CozoDB regex syntax)

**Common Mistakes:**

- ❌ Using regex patterns (use `cie_search_text` for regex)
- ❌ Using `\.` in exclude_pattern (use `[.]` instead for literal dots)
- ❌ Not excluding generated files (use `exclude_pattern="[.]pb[.]go|_generated[.]go"`)
- ✅ For complex patterns, use `cie_search_text` with `literal=true`

---

### cie_search_text

Search for text patterns in function code, signatures, or names using regex. For exact literal text, use `cie_grep` instead (faster).

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `pattern` | string | Yes | — | Regex pattern to search for |
| `search_in` | string | No | `all` | Where to search: `code`, `signature`, `name`, or `all` |
| `file_pattern` | string | No | — | Filter by file path regex |
| `exclude_pattern` | string | No | — | Regex pattern to exclude files |
| `literal` | bool | No | false | If true, treat pattern as literal (escape regex chars) |
| `limit` | int | No | 20 | Maximum number of results to return |

**Example:**

```json
{
  "pattern": "func.*Handler.*error",
  "search_in": "signature",
  "file_pattern": "internal/http",
  "limit": 15
}
```

**Literal search example:**

```json
{
  "pattern": ".GET(",
  "literal": true,
  "search_in": "code",
  "limit": 20
}
```

**Output:**

```markdown
Found 12 functions matching pattern in signatures

**HandleCreateUser** (internal/http/users.go:45)
Signature: func HandleCreateUser(w http.ResponseWriter, r *http.Request) error
Match: returns error type

**HandleUpdateUser** (internal/http/users.go:78)
Signature: func HandleUpdateUser(w http.ResponseWriter, r *http.Request) error
Match: returns error type
```

**Tips:**

- 🎯 **Use `literal=true` for exact patterns** like `.GET(` or `::new` (auto-escapes regex chars)
- 📝 **Search in specific locations** - Use `search_in="signature"` to find function signatures only
- ⚡ **For literal text, use `cie_grep` instead** - It's faster and optimized for exact matches
- 🔍 **Combine filters** - Use both `file_pattern` and `exclude_pattern` to narrow scope

**Common Mistakes:**

- ❌ Not escaping regex special chars (use `literal=true` for exact patterns)
- ❌ Using for simple literal search (use `cie_grep` instead - it's much faster)
- ❌ Forgetting `(?i)` for case-insensitive regex (or use `literal=true` with lowercase query)
- ✅ Test regex patterns in a regex tester before using in queries

---

### cie_verify_absence

Verify that specific patterns do NOT exist in code. Returns PASS/FAIL with detailed violations. Perfect for security audits (no hardcoded secrets, tokens, credentials) and CI/CD checks.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `patterns` | []string | Yes | — | Patterns that should NOT exist (e.g., ["access_token", "api_key", "password"]) |
| `path` | string | No | — | Limit check to specific path (e.g., "ui/src", "frontend/") |
| `exclude_pattern` | string | No | — | Regex pattern to EXCLUDE files from check (e.g., "_test[.]go\|mock") |
| `case_sensitive` | bool | No | false | If true, pattern matching is case-sensitive |
| `severity` | string | No | `warning` | Severity level for violations: `critical`, `warning`, or `info` |

**Example:**

```json
{
  "patterns": ["access_token", "refresh_token", "api_key", "secret"],
  "path": "ui/src",
  "exclude_pattern": "_test[.]|mock|fixture",
  "severity": "critical"
}
```

**Output:**

```markdown
## ❌ SECURITY CHECK: FAILED

Found 3 violations in 2 files

### 🔴 CRITICAL: access_token
**File:** ui/src/api/client.ts:45
**Function:** authenticate
**Line:** const token = localStorage.getItem('access_token');

### 🔴 CRITICAL: api_key
**File:** ui/src/config.ts:12
**Function:** getConfig
**Line:** apiKey: 'hardcoded_api_key_12345',

### 🟡 WARNING: secret
**File:** ui/src/utils/crypto.ts:23
**Function:** encrypt
**Line:** // Use secret key from environment
```

**PASS output:**

```markdown
## ✅ SECURITY CHECK: PASSED

No violations found. All 4 patterns are absent from:
- Path: ui/src
- Files checked: 127
- Excluded: _test., mock, fixture
```

**Tips:**

- 🔒 **Security audits** - Check for hardcoded secrets before committing frontend code
- ✅ **CI/CD integration** - Add to pre-commit hooks or CI pipeline
- 📁 **Scope to sensitive areas** - Use `path="ui/src"` to check only frontend code
- 🧹 **Exclude test files** - Use `exclude_pattern="_test[.]|mock|fixture"` to avoid false positives
- 🎯 **Batch checks** - Pass multiple patterns in one call (efficient API usage)

**Common Mistakes:**

- ❌ Not excluding test/mock files (gets false positives from test fixtures)
- ❌ Using overly broad patterns (e.g., "key" matches "keyboard")
- ❌ Not setting severity appropriately (use `critical` for security checks)
- ✅ Combine with code review for context (some "secrets" might be example data)

---

## Navigation Tools

### cie_find_function

Find functions by name with partial or exact matching. Handles Go receiver syntax automatically (e.g., searching "Batch" finds "Batcher.Batch").

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | Yes | — | Function name to find (exact or partial, e.g., "NewBatcher" or "Batch") |
| `exact_match` | bool | No | false | If true, match exact name only; if false, also match methods containing the name |
| `include_code` | bool | No | false | If true, include full function code in results |

**Example:**

```json
{
  "name": "BuildRouter",
  "exact_match": false,
  "include_code": false
}
```

**Output:**

```markdown
Found 3 functions matching "BuildRouter"

**BuildRouter** (apps/gateway/internal/http/router.go:34-67)
Signature: func BuildRouter() *chi.Mux

**BuildAPIRouter** (apps/gateway/internal/api/router.go:23-89)
Signature: func BuildAPIRouter(cfg *Config) (*chi.Mux, error)

**Router.BuildRoutes** (apps/gateway/internal/http/router.go:145-203)
Signature: func (r *Router) BuildRoutes() error
```

**Tips:**

- 🎯 **Partial matching by default** - Searching "Router" finds "BuildRouter", "APIRouter", "Router.Build"
- 🔍 **Use exact_match for precision** - Set `exact_match=true` when you know the full function name
- 📝 **Include code when needed** - Set `include_code=true` to see function implementation inline
- 🧩 **Method search** - Searching "Batch" automatically finds "Batcher.Batch", "BatchProcessor.Batch"

**Common Mistakes:**

- ❌ Expecting exact match by default (partial matching is the default for flexibility)
- ❌ Not using exact_match when function name is common (e.g., "Get" matches too many)
- ✅ Start with partial match, then use exact_match if too many results

---

### cie_find_callers

Find all functions that call a specific function. Useful for understanding how a function is used throughout the codebase (impact analysis).

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `function_name` | string | Yes | — | Name of the function to find callers for (e.g., "Batch", "NewBatcher") |
| `include_indirect` | bool | No | false | If true, include indirect callers (callers of callers) - can be expensive |

**Example:**

```json
{
  "function_name": "HandleAuth",
  "include_indirect": false
}
```

**Output:**

```markdown
## Callers of HandleAuth

Found 8 direct callers:

**RegisterRoutes** (apps/gateway/internal/http/routes.go:45)
  → calls HandleAuth at line 52

**SetupAuthMiddleware** (apps/gateway/internal/middleware/auth.go:23)
  → calls HandleAuth at line 67

**TestAuthFlow** (apps/gateway/internal/http/routes_test.go:123)
  → calls HandleAuth at line 134
```

**Tips:**

- 🔍 **Impact analysis** - See where a function is used before refactoring or removing
- 🐛 **Debugging** - Trace back to see what's calling a problematic function
- ⚠️ **Avoid `include_indirect` on large codebases** - Can return hundreds of results and be slow
- 📊 **Combine with `cie_trace_path`** - Use trace_path to see full call chains from entry points

**Common Mistakes:**

- ❌ Using `include_indirect=true` on large codebases without filters (very slow)
- ❌ Not checking if function name is unique (use `cie_find_function` first to verify)
- ✅ Use with `cie_get_function_code` to see both callers and implementation

---

### cie_find_callees

Find all functions called by a specific function. Useful for understanding a function's dependencies.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `function_name` | string | Yes | — | Name of the function to find callees for |

**Example:**

```json
{
  "function_name": "BuildRouter"
}
```

**Output:**

```markdown
## Functions called by BuildRouter

**BuildRouter** calls 12 functions:

1. **chi.NewRouter** (external)
2. **RegisterHealthRoutes** (apps/gateway/internal/health/routes.go:15)
3. **RegisterAPIRoutes** (apps/gateway/internal/api/routes.go:23)
4. **UseMiddleware** (apps/gateway/internal/middleware/setup.go:34)
5. **LoggerMiddleware** (apps/gateway/internal/middleware/logger.go:45)
```

**Tips:**

- 🧩 **Dependency analysis** - See what a function depends on before refactoring
- 🔍 **Code review** - Understand what a complex function is doing by seeing its calls
- 📊 **Combine with `cie_get_function_code`** - See both implementation and call list together
- 🔗 **Use with `cie_get_call_graph`** for complete picture - See both callers and callees

**Common Mistakes:**

- ❌ Expecting external library calls to show file paths (they show as "external")
- ✅ Combine with `cie_trace_path` to see how this function fits in execution flow

---

### cie_get_function_code

Get the full source code of a function by name.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `function_name` | string | Yes | — | Name of the function to get code for (e.g., "NewBatcher", "Pipeline.Run") |
| `full_code` | bool | No | false | If true, return complete code without truncation (for long functions) |

**Example:**

```json
{
  "function_name": "BuildRouter",
  "full_code": true
}
```

**Output:**

```markdown
**Function**: BuildRouter
**File**: apps/gateway/internal/http/router.go:34-67
**Signature**: func BuildRouter() *chi.Mux

```go
func BuildRouter() *chi.Mux {
    r := chi.NewRouter()
    r.Use(LoggerMiddleware)
    r.Use(RecoveryMiddleware)

    r.Get("/health", healthHandler)
    r.Get("/metrics", metricsHandler)

    r.Route("/api", func(r chi.Router) {
        r.Use(AuthMiddleware)
        r.Get("/users", listUsers)
        r.Post("/users", createUser)
    })

    return r
}
```
```

**Truncated output (when code > 3000 chars and `full_code=false`):**

```markdown
⚠️ **Code truncated**. To view full code:
- Use `Read` tool: `apps/gateway/internal/http/router.go` (lines 34-67)
- Or call this tool with `full_code: true`
```

**Tips:**

- 📝 **Use `full_code=true` for long functions** - Default truncates at 3000 characters
- 🔍 **Combine with callers/callees** - See implementation along with usage
- ⚡ **Syntax highlighting** - Output includes language-specific code blocks
- 🎯 **Partial name matching** - Searching "Router" finds "BuildRouter" if no exact match

**Common Mistakes:**

- ❌ Not using `full_code=true` for long functions (gets truncated)
- ❌ Expecting to see imports or types (only shows function body)
- ✅ Use `Read` tool for full file context if needed

---

### cie_find_type

Find types, interfaces, classes, or structs by name or pattern. Works across all languages: Go (struct/interface), Python (class), TypeScript (interface/class).

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | Yes | — | Type name to search for (e.g., "UserService", "Handler", "Config") |
| `kind` | string | No | `any` | Filter by type kind: `struct`, `interface`, `class`, `type_alias`, or `any` |
| `path_pattern` | string | No | — | Optional regex to filter by file path |
| `limit` | int | No | 20 | Maximum number of results to return |

**Example:**

```json
{
  "name": "Repository",
  "kind": "interface",
  "path_pattern": "internal/",
  "limit": 10
}
```

**Output:**

```markdown
Found 4 types matching "Repository"

**UserRepository** (interface)
File: internal/users/repository.go:15-23

**OrderRepository** (interface)
File: internal/orders/repository.go:12-19

**PostgresRepository** (struct)
File: internal/db/postgres.go:45-52

**InMemoryRepository** (struct)
File: internal/db/memory.go:23-34
```

**Tips:**

- 🔍 **Filter by kind** - Use `kind="interface"` to find only interfaces, not implementations
- 📁 **Scope to package** - Use `path_pattern="internal/users"` to narrow results
- 🎯 **Partial matching** - Searching "Repository" finds "UserRepository", "DBRepository", etc.
- 🧩 **Use with `cie_find_implementations`** - First find interface, then find implementations

**Common Mistakes:**

- ❌ Using wrong kind name (use `interface` not `Interface`, `struct` not `Struct`)
- ❌ Not using kind filter when name is generic (e.g., "Config" returns many types)
- ✅ Combine with `cie_get_type_code` to see type definition

---

### cie_find_implementations

Find types that implement a given interface. For Go: finds structs with methods matching the interface. For TypeScript: finds classes with `implements InterfaceName`.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `interface_name` | string | Yes | — | Name of the interface to find implementations for |
| `path_pattern` | string | No | — | Optional regex to filter by file path |
| `limit` | int | No | 20 | Maximum number of results to return |

**Example:**

```json
{
  "interface_name": "Repository",
  "path_pattern": "internal/",
  "limit": 15
}
```

**Output:**

```markdown
## Implementations of Repository

Found 5 types implementing Repository interface:

**PostgresRepository** (struct)
File: internal/db/postgres.go:45-78
Implements: Get, List, Create, Update, Delete

**InMemoryRepository** (struct)
File: internal/db/memory.go:23-56
Implements: Get, List, Create, Update, Delete

**CachedRepository** (struct)
File: internal/cache/repository.go:34-89
Implements: Get, List, Create, Update, Delete
```

**Tips:**

- 🧩 **Architecture exploration** - See all implementations of an interface pattern
- 🔍 **Find concrete types** - After finding interface with `cie_find_type`, use this to find usages
- 📁 **Scope by path** - Use `path_pattern` to find implementations in specific package
- ⚡ **Works across languages** - Go structs, TypeScript classes, Python classes

**Common Mistakes:**

- ❌ Expecting to see partial implementations (only shows types with all methods)
- ❌ Using on concrete types (only works with interfaces/abstract classes)
- ✅ Use `cie_find_type` first to verify interface exists and see its methods

---

### cie_list_functions_in_file

List all functions defined in a specific file. Useful for understanding file structure.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `file_path` | string | Yes | — | Path to the file (exact or partial, e.g., "router.go" or "internal/http/router.go") |

**Example:**

```json
{
  "file_path": "apps/gateway/internal/http/router.go"
}
```

**Output:**

```markdown
## Functions in apps/gateway/internal/http/router.go

Found 8 functions:

1. **BuildRouter** (line 34)
   Signature: func BuildRouter() *chi.Mux

2. **RegisterHealthRoutes** (line 68)
   Signature: func RegisterHealthRoutes(r chi.Router)

3. **RegisterAPIRoutes** (line 89)
   Signature: func RegisterAPIRoutes(r chi.Router)

4. **healthHandler** (line 112)
   Signature: func healthHandler(w http.ResponseWriter, r *http.Request)
```

**Tips:**

- 📁 **File exploration** - Quick overview of what functions a file contains
- 🔍 **Partial path matching** - Can use just filename if unique (e.g., "router.go")
- 📊 **Ordered by line number** - Functions appear in file order (top to bottom)
- 🎯 **Use with `cie_get_function_code`** - List functions, then drill into specific ones

**Common Mistakes:**

- ❌ Using full absolute path (use relative path from project root)
- ❌ Not handling ambiguous filenames (multiple files with same name - use more specific path)
- ✅ Use `cie_directory_summary` for overview of multiple files

---

### cie_find_similar_functions

Find functions with similar names or patterns. Useful for discovering related functionality.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `pattern` | string | Yes | — | Name pattern to search for (e.g., "Handler", "New", "Parse") |

**Example:**

```json
{
  "pattern": "Handler"
}
```

**Output:**

```markdown
Found 23 functions matching pattern "Handler":

**HandleCreateUser** (internal/http/users.go:45)
**HandleUpdateUser** (internal/http/users.go:78)
**HandleDeleteUser** (internal/http/users.go:112)
**HandleListUsers** (internal/http/users.go:145)
**HandleAuth** (internal/http/auth.go:34)
**HandleLogout** (internal/http/auth.go:67)
**healthHandler** (internal/http/health.go:23)
```

**Tips:**

- 🔍 **Discover related code** - Find all functions following a naming pattern
- 🎯 **Partial matching** - "Handler" finds "HandleX", "XHandler", "handleX"
- 📊 **Pattern-based exploration** - Search "New" to find all constructors
- 🧩 **Use with `cie_get_function_code`** - Explore similar functions to understand patterns

**Common Mistakes:**

- ❌ Using too generic pattern (e.g., "Get" returns hundreds of matches)
- ✅ Combine with more specific queries after getting overview

---

### cie_get_file_summary

Get a summary of all entities (functions, types, constants) defined in a file.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `file_path` | string | Yes | — | Path to the file to summarize |

**Example:**

```json
{
  "file_path": "internal/http/router.go"
}
```

**Output:**

```markdown
# File Summary: internal/http/router.go

## Types (2)
- **Router** (struct, line 15): HTTP router with middleware
- **RouteConfig** (struct, line 23): Route configuration options

## Functions (8)
- **NewRouter** (line 34): Constructor for Router
- **BuildRouter** (line 45): Builds router with all routes
- **RegisterRoutes** (line 67): Registers API routes
[...]

## Constants (3)
- **DefaultTimeout** (line 8): time.Duration
- **MaxBodySize** (line 9): int64
```

**Tips:**

- 📋 **Complete file overview** - See all entities at a glance
- 🎯 **Architecture understanding** - Understand file structure and responsibilities
- 🔍 **Use before diving deep** - Get overview before reading individual functions
- ✅ **Combine with `cie_directory_summary`** for package-level overview

**Common Mistakes:**

- ❌ Expecting variable declarations (only shows types, functions, constants)
- ✅ Use `Read` tool for complete file content if needed

---

### cie_get_type_code

Get the full source code of a type, interface, or class definition.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `type_name` | string | Yes | — | Name of the type to get code for |
| `full_code` | bool | No | false | If true, return complete code without truncation |

**Example:**

```json
{
  "type_name": "UserRepository",
  "full_code": true
}
```

**Output:**

```markdown
**Type**: UserRepository (interface)
**File**: internal/users/repository.go:15-28

```go
type UserRepository interface {
    Get(ctx context.Context, id string) (*User, error)
    List(ctx context.Context, filter *Filter) ([]*User, error)
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
}
```
```

**Tips:**

- 🧩 **See interface definitions** - Understand contract before implementing
- 📝 **Use `full_code=true` for large types** - Default truncates at 3000 characters
- 🔍 **Combine with `cie_find_implementations`** - See definition and implementations
- ⚡ **Syntax highlighting included** - Language-specific code blocks

**Common Mistakes:**

- ❌ Not using `full_code=true` for large struct/interface definitions
- ✅ Use `cie_find_type` first to verify type exists and get file location

---

## Analysis Tools

### cie_analyze

Answer architectural questions about the codebase using LLM narrative generation. Uses hybrid search (localized + global semantic search) with keyword boosting (+15% for matching function names).

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `question` | string | Yes | — | Natural language question about codebase architecture or structure |
| `path_pattern` | string | No | — | Focus analysis on specific path (e.g., "apps/gateway", "internal/cie") |
| `role` | string | No | `source` | Filter results: `source` (default, excludes tests), `test`, or `any` |

**Example:**

```json
{
  "question": "What are the main entry points and how do they initialize the application?",
  "path_pattern": "apps/gateway",
  "role": "source"
}
```

**Output:**

```markdown
## Analysis: Entry Points and Initialization

### Overview
The application has 2 primary entry points: the main CLI and the HTTP server.

### Main Entry Point
**main** (cmd/gateway/main.go:23)
- Initializes configuration from environment
- Sets up database connections
- Starts HTTP server
- Registers shutdown handlers

### HTTP Server Initialization
**NewServer** (internal/server/server.go:45)
- Builds router with middleware chain
- Configures timeouts and limits
- Registers health and metrics endpoints

### Key Initialization Steps
1. Load configuration (config.Load)
2. Initialize database (db.Connect)
3. Build router (http.BuildRouter)
4. Start server (server.ListenAndServe)

### Related Functions
- **LoadConfig** (internal/config/config.go:34) - Configuration loading
- **ConnectDB** (internal/db/db.go:23) - Database initialization
- **BuildRouter** (internal/http/router.go:45) - Route setup
```

**Tips:**

- 🤖 **LLM-powered analysis** - Gets narrative explanation with code context
- 🎯 **Ask architectural questions** - "How does X work?", "What are the entry points?"
- 📁 **Scope to module** - Use `path_pattern` to focus on specific part of codebase
- 🧹 **Exclude tests by default** - Set `role="source"` (default) to ignore test files
- ⚡ **Keyword boosting** - Query terms matching function names get +15% similarity boost

**Common Mistakes:**

- ❌ Asking overly broad questions (e.g., "How does everything work?")
- ❌ Not using `path_pattern` to focus on specific area
- ❌ Including test files in analysis (set `role="source"` explicitly if getting test noise)
- ✅ Ask specific, focused questions for best results

---

### cie_trace_path

Trace call paths from source function(s) to a target function. Shows execution flow. If no source specified, auto-detects entry points based on language conventions.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `target` | string | Yes | — | Target function name to trace to (e.g., "RegisterRoutes", "SaveUser") |
| `source` | string | No | auto-detect | Source function to trace from (auto-detects main/entry points if empty) |
| `path_pattern` | string | No | — | Filter by file path to narrow search scope |
| `max_paths` | int | No | 3 | Maximum number of paths to return |
| `max_depth` | int | No | 10 | Maximum call depth to search |

**Example:**

```json
{
  "target": "RegisterRoutes",
  "max_paths": 5,
  "max_depth": 15
}
```

**With explicit source:**

```json
{
  "target": "SaveUser",
  "source": "HandleCreateUser",
  "path_pattern": "internal/"
}
```

**Output:**

```markdown
## Call Paths to RegisterRoutes

Found 2 paths from entry points:

### Path 1 (depth: 4)
1. **main** (cmd/gateway/main.go:23)
   ↓ calls at line 45
2. **NewServer** (internal/server/server.go:67)
   ↓ calls at line 89
3. **BuildRouter** (internal/http/router.go:34)
   ↓ calls at line 56
4. **RegisterRoutes** (internal/http/routes.go:23) ✓ TARGET

### Path 2 (depth: 5)
1. **main** (cmd/gateway/main.go:23)
   ↓ calls at line 48
2. **InitApp** (internal/app/app.go:34)
   ↓ calls at line 67
3. **SetupHTTP** (internal/app/http.go:23)
   ↓ calls at line 45
4. **BuildRouter** (internal/http/router.go:34)
   ↓ calls at line 56
5. **RegisterRoutes** (internal/http/routes.go:23) ✓ TARGET
```

**Tips:**

- 🎯 **Auto-detect entry points** - Leave `source` empty to trace from main/entry points
- 🔍 **Trace between arbitrary functions** - Set both `source` and `target` for specific flow
- 📊 **Increase `max_paths` for complex flows** - Default 3 may miss alternate paths
- 🧹 **Use `path_pattern` to focus** - Narrows search to specific module
- ⚡ **BFS search** - Returns shortest paths first

**Common Mistakes:**

- ❌ Setting `max_depth` too low for deeply nested calls (default 10 is usually enough)
- ❌ Not using `path_pattern` on large codebases (can be slow without filtering)
- ❌ Expecting all possible paths (set `max_paths` higher if needed)
- ✅ Start with defaults, adjust `max_paths` and `max_depth` if needed

---

### cie_get_call_graph

Get the complete call graph for a function - both who calls it (callers) and what it calls (callees). Combines `cie_find_callers` and `cie_find_callees` in one tool.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `function_name` | string | Yes | — | Name of the function to analyze |

**Example:**

```json
{
  "function_name": "BuildRouter"
}
```

**Output:**

```markdown
## Call Graph: BuildRouter

### Callers (3 functions call BuildRouter)
- **main** (cmd/gateway/main.go:45)
- **NewServer** (internal/server/server.go:89)
- **TestRouter** (internal/http/router_test.go:23)

### Callees (BuildRouter calls 8 functions)
- **chi.NewRouter** (external)
- **LoggerMiddleware** (internal/middleware/logger.go:23)
- **RecoveryMiddleware** (internal/middleware/recovery.go:34)
- **RegisterHealthRoutes** (internal/health/routes.go:15)
- **RegisterAPIRoutes** (internal/api/routes.go:23)
```

**Tips:**

- 📊 **Complete picture** - See both incoming and outgoing calls in one view
- 🧩 **Understand dependencies** - Quickly see what function depends on and what depends on it
- 🔍 **Impact analysis** - Before refactoring, see both usage and dependencies
- ⚡ **Single query** - More efficient than calling callers and callees separately

**Common Mistakes:**

- ❌ Expecting indirect callers/callees (only shows direct calls)
- ✅ Use `cie_trace_path` for full execution flow including indirect calls

---

### cie_directory_summary

Get a summary of files in a directory with their main exported functions. Perfect for understanding module architecture quickly.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | Yes | — | Directory path to summarize (e.g., "apps/gateway/internal/http", "internal/cie") |
| `max_functions_per_file` | int | No | 5 | Maximum number of functions to show per file |

**Example:**

```json
{
  "path": "apps/gateway/internal/http",
  "max_functions_per_file": 5
}
```

**Output:**

```markdown
# Directory Summary: `apps/gateway/internal/http`

Found **7 files**

## apps/gateway/internal/http/router.go
- **BuildRouter** (line 34): `func BuildRouter() *chi.Mux`
- **RegisterRoutes** (line 67): `func RegisterRoutes(r chi.Router)`
- **NewRouter** (line 23): `func NewRouter(cfg *Config) (*chi.Mux, error)`

## apps/gateway/internal/http/users.go
- **HandleCreateUser** (line 45): `func HandleCreateUser(w http.ResponseWriter, r *http.Request) error`
- **HandleUpdateUser** (line 78): `func HandleUpdateUser(w http.ResponseWriter, r *http.Request) error`
- **HandleDeleteUser** (line 112): `func HandleDeleteUser(w http.ResponseWriter, r *http.Request) error`
- **HandleListUsers** (line 145): `func HandleListUsers(w http.ResponseWriter, r *http.Request) error`

## apps/gateway/internal/http/auth.go
- **HandleAuth** (line 34): `func HandleAuth(w http.ResponseWriter, r *http.Request) error`
- **HandleLogout** (line 67): `func HandleLogout(w http.ResponseWriter, r *http.Request) error`
```

**Tips:**

- 📁 **Module exploration** - Quick overview of what a package/directory contains
- 🎯 **Prioritizes exported functions** - Shows public API first
- 🔍 **Architecture understanding** - See how code is organized across files
- ⚡ **Adjustable detail** - Increase `max_functions_per_file` for more complete view

**Common Mistakes:**

- ❌ Including trailing slash in path (use "internal/http" not "internal/http/")
- ❌ Setting `max_functions_per_file` too high for large directories (gets verbose)
- ✅ Start with default 5, increase if needed

---

### cie_list_endpoints

List HTTP/REST endpoints defined in the codebase. Detects route definitions from multiple popular Go web frameworks (Gin, Echo, Chi, Fiber, net/http).

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path_pattern` | string | No | — | Filter results by file path using regex (e.g., "apps/gateway") |
| `path_filter` | string | No | — | Filter results by endpoint path using substring (e.g., "/health", "connections") |
| `method` | string | No | — | Filter results by HTTP method (case-insensitive): "GET", "POST", "PUT", "DELETE", "PATCH" |
| `limit` | int | No | 100 | Maximum number of endpoints to return |

**Example:**

```json
{
  "path_pattern": "apps/gateway",
  "method": "GET",
  "limit": 50
}
```

**Filter by endpoint path:**

```json
{
  "path_filter": "/api/v1",
  "method": "POST"
}
```

**Output:**

```markdown
## HTTP Endpoints

Found 23 endpoints:

| Method | Path | Handler | File |
|--------|------|---------|------|
| GET | /health | healthHandler | apps/gateway/internal/http/routes.go:45 |
| GET | /metrics | metricsHandler | apps/gateway/internal/http/routes.go:46 |
| GET | /api/users | listUsers | apps/gateway/internal/api/users.go:23 |
| POST | /api/users | createUser | apps/gateway/internal/api/users.go:45 |
| GET | /api/users/{id} | getUser | apps/gateway/internal/api/users.go:67 |
| PUT | /api/users/{id} | updateUser | apps/gateway/internal/api/users.go:89 |
| DELETE | /api/users/{id} | deleteUser | apps/gateway/internal/api/users.go:112 |
```

**Tips:**

- 🌐 **API discovery** - See all REST endpoints at a glance
- 🎯 **Filter by method** - Use `method="POST"` to see all write endpoints
- 📁 **Scope to service** - Use `path_pattern="apps/gateway"` for specific service
- 🔍 **Endpoint path search** - Use `path_filter="/api"` to see only API routes
- ⚡ **Supports multiple frameworks** - Works with Gin, Echo, Chi, Fiber, net/http

**Common Mistakes:**

- ❌ Expecting dynamic routes to be expanded (shows "{id}" not actual IDs)
- ❌ Not filtering by path_pattern on large codebases (can return too many results)
- ✅ Use `path_filter` to find specific endpoint patterns (e.g., "/health")

---

## Administrative Tools

### cie_index_status

Check indexing status and health for a path. Shows how many files and functions are indexed, warns if index appears incomplete.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path_pattern` | string | No | — | Path pattern to check (e.g., "apps/gateway", "internal/") - leave empty for entire index |

**Example:**

```json
{
  "path_pattern": "apps/gateway"
}
```

**Full index status:**

```json
{}
```

**Output:**

```markdown
## Index Status

### Overall Statistics
- **Total Files**: 1,247
- **Total Functions**: 8,934
- **Total Types**: 1,456
- **Total Embeddings**: 8,723 (97.6% of functions have embeddings)

### By Language
| Language | Files | Functions | Types |
|----------|-------|-----------|-------|
| Go | 892 | 6,234 | 1,123 |
| TypeScript | 245 | 1,890 | 234 |
| Python | 110 | 810 | 99 |

### apps/gateway (filtered)
- **Files**: 156
- **Functions**: 1,234
- **Types**: 189
- **Embeddings**: 1,198 (97.1%)

### Health Checks
✅ All critical tables present
✅ Embedding coverage > 95%
✅ No orphaned function code
```

**Tips:**

- 🏥 **Health monitoring** - Check if index is complete and healthy
- 🔍 **Use FIRST when searches return no results** - Verify path is indexed before debugging queries
- 📊 **Language breakdown** - See distribution of code across languages
- ⚡ **Fast operation** - Uses aggregation queries (doesn't scan all data)

**Common Mistakes:**

- ❌ Not checking index status when getting unexpected search results
- ❌ Assuming all code is indexed (some files may be excluded by .cie/project.yaml rules)
- ✅ Run after `cie index` to verify indexing completed successfully

---

### cie_schema

Get the CIE database schema, available tables, fields, operators, and example queries. Call this first to understand what data is available and how to query it.

**Parameters:**

None

**Example:**

```json
{}
```

**Output:**

```markdown
## CIE Database Schema (v3)

### Available Tables

#### cie_file
Indexed files in the codebase.

**Fields:**
- `id` (string): Unique file identifier
- `path` (string): File path relative to project root
- `hash` (string): File content hash
- `language` (string): Programming language (go, typescript, python, etc.)
- `size` (int): File size in bytes
- `role` (string): File role (source, test, generated)

#### cie_function
Function metadata and signatures.

**Fields:**
- `id` (string): Unique function identifier
- `name` (string): Function name (includes receiver for methods)
- `signature` (string): Full function signature
- `file_path` (string): File containing the function
- `start_line`, `end_line` (int): Line range
- `start_col`, `end_col` (int): Column range
- `role` (string): Function role (entry_point, handler, router, etc.)

#### cie_function_code
Function source code (separate table for performance).

**Fields:**
- `function_id` (string): References cie_function.id
- `code_text` (string): Full function source code

#### cie_function_embedding
Function embeddings for semantic search.

**Fields:**
- `function_id` (string): References cie_function.id
- `embedding` (vector): Embedding vector (indexed with HNSW)

#### cie_type
Type, interface, class, and struct definitions.

**Fields:**
- `id` (string): Unique type identifier
- `name` (string): Type name
- `kind` (string): Type kind (struct, interface, class, type_alias)
- `file_path` (string): File containing the type
- `start_line`, `end_line` (int): Line range

#### cie_type_code
Type source code (separate table).

**Fields:**
- `type_id` (string): References cie_type.id
- `code_text` (string): Full type source code

#### cie_call
Call relationships between functions.

**Fields:**
- `caller_id` (string): Function making the call
- `callee_id` (string): Function being called
- `line_number` (int): Line where call occurs

### CozoScript Query Language

CIE uses CozoScript (Datalog-based) for queries.

**Basic query format:**
```
?[field1, field2] := *table_name { field1, field2, ... }, condition1, condition2 :limit N
```

**Common operators:**
- `regex_matches(field, "pattern")` - Regex matching
- `starts_with(field, "prefix")` - Prefix matching
- `ends_with(field, "suffix")` - Suffix matching
- `field == "value"` - Exact equality
- `!condition` - Negation

**Example queries:**
```
# Find functions by name
?[name, file_path, start_line] := *cie_function { name, file_path, start_line },
  regex_matches(name, "(?i)BuildRouter") :limit 10

# Find Go files
?[path] := *cie_file { path, language }, language == "go" :limit 100

# Find function calls
?[caller_name, callee_name, line] := *cie_call { caller_id, callee_id, line_number: line },
  *cie_function { id: caller_id, name: caller_name },
  *cie_function { id: callee_id, name: callee_name } :limit 50
```
```

**Tips:**

- 📖 **Start here** - Read schema before using `cie_raw_query`
- 🎯 **Understand data model** - See what data is available and how it's structured
- 🔍 **Learn query syntax** - Examples show how to write custom queries
- ⚡ **Schema versioning** - Note schema version (v3) for compatibility

**Common Mistakes:**

- ❌ Trying to write queries without reading schema first
- ❌ Using SQL syntax (CIE uses CozoScript/Datalog, not SQL)
- ✅ Study example queries before writing custom ones

---

### cie_list_files

List files in the indexed codebase. Can filter by language, path pattern, or role.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `language` | string | No | — | Filter by language (e.g., "go", "typescript", "python") |
| `path_pattern` | string | No | — | Regex pattern to filter file paths (e.g., ".*batcher.*", "internal/cie/.*") |
| `role` | string | No | `source` | Filter by file role: `source` (exclude tests/generated), `test`, `generated`, or `any` |
| `limit` | int | No | 50 | Maximum results (default: 50) |

**Example:**

```json
{
  "language": "go",
  "path_pattern": "internal/",
  "role": "source",
  "limit": 100
}
```

**Output:**

```markdown
## Indexed Files

Found 127 Go files in internal/ (source only):

1. internal/cie/client.go (23 functions)
2. internal/cie/ingestion/batcher.go (8 functions)
3. internal/cie/ingestion/indexer.go (12 functions)
4. internal/http/router.go (15 functions)
5. internal/http/users.go (18 functions)
[...]
```

**Tips:**

- 📁 **Explore codebase structure** - See what files are indexed
- 🔍 **Filter by language** - Focus on specific language files
- 🧹 **Exclude tests** - Use `role="source"` (default) to ignore test files
- 📊 **Check coverage** - See how many files are indexed in specific area

**Common Mistakes:**

- ❌ Expecting all project files (only indexed files are shown)
- ❌ Not using `role` filter (includes test files by default if `role="any"`)
- ✅ Use `cie_index_status` to understand overall index health first

---

### cie_list_services

List gRPC services and RPC methods from .proto files. Shows service definitions, RPC methods, and their request/response types.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path_pattern` | string | No | — | Filter by file path (e.g., "api/proto") |
| `service_name` | string | No | — | Filter by service name |

**Example:**

```json
{
  "path_pattern": "api/proto",
  "service_name": "CIE"
}
```

**Output:**

```markdown
## gRPC Services

### CIEService (api/proto/v1/cie.proto)

**ExecuteWrite** (line 45)
- Request: `WriteRequest`
- Response: `WriteResponse`
- Description: Execute write operation with replication log

**MountProject** (line 67)
- Request: `MountRequest`
- Response: stream `SnapshotChunk`
- Description: Stream snapshot for initial sync

**StreamReplicationLog** (line 89)
- Request: `ReplicationRequest`
- Response: stream `LogEntry`
- Description: Stream incremental updates
```

**Tips:**

- 🌐 **gRPC API discovery** - See all RPC methods at a glance
- 📁 **Filter by path** - Use `path_pattern="api/"` to focus on API definitions
- 🎯 **Service-specific** - Use `service_name` to see specific service methods
- ⚡ **Works with proto files** - Parses .proto files for service definitions

**Common Mistakes:**

- ❌ Expecting non-gRPC services (only works with .proto file definitions)
- ❌ Not indexing .proto files (check .cie/project.yaml includes proto files)
- ✅ Use `cie_list_files language="protobuf"` to verify proto files are indexed

---

### cie_raw_query

Execute a raw CozoScript query against the CIE database. Use `cie_schema` first to understand available tables and operators.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `script` | string | Yes | — | CozoScript query to execute |

**Example:**

```json
{
  "script": "?[name, file_path] := *cie_function { name, file_path }, regex_matches(name, \"(?i)^main$\") :limit 10"
}
```

**Advanced query:**

```json
{
  "script": "?[caller, callee, count(line)] := *cie_call { caller_id, callee_id, line_number: line }, *cie_function { id: caller_id, name: caller }, *cie_function { id: callee_id, name: callee } :order count(line) :desc :limit 20"
}
```

**Output:**

```markdown
## Query Results

| name | file_path |
|------|-----------|
| main | cmd/gateway/main.go |
| main | cmd/cie-agent/main.go |
```

**Tips:**

- 🔧 **Power user tool** - For custom queries not covered by other tools
- 📖 **Read schema first** - Use `cie_schema` to understand table structure
- 🎯 **Test in small steps** - Build complex queries incrementally
- ⚠️ **No SQL** - CIE uses CozoScript (Datalog), not SQL syntax

**Common Mistakes:**

- ❌ Using SQL syntax (e.g., `SELECT * FROM` - use CozoScript instead)
- ❌ Not escaping regex patterns (use `[.]` for literal dots, not `\.`)
- ❌ Forgetting `:limit` (queries without limit can be slow)
- ✅ Study example queries in `cie_schema` before writing custom queries

---

## Common Patterns

### Multi-Step Investigation

**Pattern:** Start broad, then narrow with filters

```
1. cie_semantic_search query="authentication" (broad)
2. cie_semantic_search query="authentication" path_pattern="internal/auth" (narrowed)
3. cie_get_function_code function_name="AuthMiddleware" (details)
4. cie_find_callers function_name="AuthMiddleware" (usage)
```

### Understanding Execution Flow

**Pattern:** Entry point → call chain → target

```
1. cie_trace_path target="SaveUser" (auto-detect entry points)
2. cie_get_call_graph function_name="SaveUser" (see dependencies)
3. cie_get_function_code function_name="SaveUser" full_code=true (implementation)
```

### Exploring New Codebase

**Pattern:** Top-down exploration

```
1. cie_analyze question="What are the main entry points?" (overview)
2. cie_directory_summary path="internal/" (module structure)
3. cie_list_endpoints (API surface)
4. cie_semantic_search query="core business logic" role="source" (key code)
```

### Security Audit

**Pattern:** Check for patterns that shouldn't exist

```
1. cie_verify_absence patterns=["password", "api_key", "secret"] path="ui/src"
2. cie_grep text="hardcoded" path="src/" exclude_pattern="_test[.]go"
3. cie_semantic_search query="credential handling" min_similarity=0.7
```

### Refactoring Preparation

**Pattern:** Understand impact before changes

```
1. cie_find_function name="OldFunction" (locate)
2. cie_find_callers function_name="OldFunction" (usage sites)
3. cie_get_call_graph function_name="OldFunction" (dependencies)
4. cie_trace_path target="OldFunction" (entry points)
```

---

## Best Practices

### Always Use English

**All queries MUST be in English.** The keyword boost algorithm matches query terms against English function names. Non-English terms won't activate the boost.

❌ Bad: `query="autenticación de usuario"`
✅ Good: `query="user authentication"`

### Start with High-Level Tools

1. **For exploration:** Use `cie_analyze` or `cie_directory_summary`
2. **For specific search:** Use `cie_semantic_search` or `cie_grep`
3. **For details:** Use `cie_get_function_code` or `cie_get_call_graph`

### Use Role Filters

Most tools support `role` parameter to filter results:

- `role="source"` - Regular source code (excludes tests/generated) [DEFAULT]
- `role="test"` - Test files only
- `role="entry_point"` - Main functions and entry points
- `role="handler"` - HTTP request handlers
- `role="router"` - Route definition functions
- `role="any"` - No filtering

**Example:** `cie_semantic_search query="error handling" role="source"`

### Combine Tools for Complete Picture

Don't rely on a single tool. Combine multiple tools:

```
cie_find_function name="BuildRouter"              # Find it
cie_get_function_code function_name="BuildRouter"  # Read it
cie_find_callers function_name="BuildRouter"       # Who uses it
cie_find_callees function_name="BuildRouter"       # What it uses
```

### Use Filters to Reduce Noise

- **Path filters:** `path_pattern="internal/http"` to focus on specific module
- **Exclude patterns:** `exclude_pattern="_test[.]go|mock"` to ignore test code
- **Min similarity:** `min_similarity=0.7` for high-confidence semantic search results

### Check Index Health First

If searches return no results:

1. Run `cie_index_status` to verify path is indexed
2. Check if files were excluded by `.cie/project.yaml` rules
3. Verify indexing completed successfully (`cie index` CLI command)

---

## Troubleshooting

### "No results found"

**Possible causes:**

1. **Path not indexed** - Run `cie_index_status path_pattern="your/path"` to check
2. **Query too specific** - Try broader query or remove filters
3. **Typo in function/type name** - Use partial matching (don't set `exact_match=true`)
4. **Files excluded** - Check `.cie/project.yaml` for exclusion rules

**Solutions:**

- Remove `path_pattern` filter temporarily to see if results exist elsewhere
- Try `cie_list_files` to see what files are indexed
- Use `cie_semantic_search` instead of exact name search
- Re-run `cie index` CLI command if files were recently added

### "Too many results"

**Solutions:**

- Add `path_pattern` to narrow scope: `path_pattern="internal/users"`
- Exclude test files: `role="source"` or `exclude_pattern="_test[.]go"`
- Increase `min_similarity` for semantic search: `min_similarity=0.7`
- Use more specific search terms
- Reduce `limit` parameter to see top results only

### "Unexpected results in semantic search"

**Possible causes:**

1. **Non-English query** - Keyword boosting only works with English
2. **Test files included** - Set `role="source"` to exclude tests
3. **Low similarity threshold** - Results below 50% similarity may not be relevant
4. **Noise from metrics/monitoring code** - Use `exclude_paths`

**Solutions:**

- Translate query to English
- Add `exclude_paths="metrics|telemetry|dlq"` to filter noise
- Set `min_similarity=0.7` to see only high-confidence results
- Add `role="source"` to exclude test files

### "Query too slow"

**Solutions:**

- Add `path_pattern` to narrow search scope
- Reduce `limit` parameter
- Avoid `include_indirect=true` in `cie_find_callers`
- Use `cie_grep` instead of `cie_search_text` for literal text

### "Code truncated" in function code

**Solution:**

Set `full_code=true` parameter:

```json
{
  "function_name": "LongFunction",
  "full_code": true
}
```

---

## Related Documentation

- **[Getting Started](./getting-started.md)** - Install and first steps
- **[Configuration Reference](./configuration.md)** - All config options
- **[Architecture](./architecture.md)** - How CIE works internally
- **[MCP Integration](./mcp-integration.md)** - Setup with Claude Code, Cursor
- **[Troubleshooting Guide](./troubleshooting.md)** - Query errors, performance, MCP issues, and more

---

**Last Updated:** 2026-01-13
**Schema Version:** v3
**CIE Version:** 0.1.0+
