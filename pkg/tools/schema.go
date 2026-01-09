// Copyright 2025 KrakLabs
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
//
// For commercial licensing, contact: licensing@kraklabs.com
//
// SPDX-License-Identifier: AGPL-3.0-or-later
package tools

import "context"

// GetSchema returns the CIE database schema documentation.
func GetSchema(ctx context.Context) (*ToolResult, error) {
	return NewResult(SchemaDocumentation), nil
}

// SchemaDocumentation contains the CIE schema docs (Schema v3).
const SchemaDocumentation = `# CIE Database Schema (v3)

Schema v3 uses vertical partitioning for performance: heavy columns (code_text, embedding) are in separate tables.

## Core Tables

### cie_file
Stores indexed source files.
| Field    | Type   | Description |
|----------|--------|-------------|
| id       | string | Unique file ID (hash) |
| path     | string | File path relative to repo root |
| hash     | string | Content hash |
| language | string | Programming language (go, typescript, python, etc.) |
| size     | int    | File size in bytes |

### cie_function
Stores function/method metadata (lightweight, ~500 bytes/row).
| Field      | Type   | Description |
|------------|--------|-------------|
| id         | string | Unique function ID (hash) |
| name       | string | Function name (includes receiver for methods, e.g., "Batcher.Batch") |
| signature  | string | Full function signature |
| file_path  | string | Path to containing file |
| start_line | int    | Starting line number |
| end_line   | int    | Ending line number |
| start_col  | int    | Starting column |
| end_col    | int    | Ending column |

### cie_function_code
Stores function source code (JOIN with cie_function when needed).
| Field       | Type   | Description |
|-------------|--------|-------------|
| function_id | string | Function ID (foreign key) |
| code_text   | string | Function source code (may be truncated) |

### cie_function_embedding
Stores function embeddings for semantic search (HNSW index here).
| Field       | Type       | Description |
|-------------|------------|-------------|
| function_id | string     | Function ID (foreign key) |
| embedding   | <F32; 1536> | Vector embedding (1536 dimensions) |

### cie_type
Stores type/struct/interface metadata.
| Field      | Type   | Description |
|------------|--------|-------------|
| id         | string | Unique type ID (hash) |
| name       | string | Type name |
| kind       | string | Type kind (struct, interface, class, type_alias) |
| file_path  | string | Path to containing file |
| start_line | int    | Starting line number |
| end_line   | int    | Ending line number |
| start_col  | int    | Starting column |
| end_col    | int    | Ending column |

### cie_type_code
Stores type source code.
| Field    | Type   | Description |
|----------|--------|-------------|
| type_id  | string | Type ID (foreign key) |
| code_text| string | Type source code |

### cie_type_embedding
Stores type embeddings for semantic search.
| Field    | Type       | Description |
|----------|------------|-------------|
| type_id  | string     | Type ID (foreign key) |
| embedding| <F32; 1536> | Vector embedding |

## Edge Tables

### cie_defines
Links files to their functions.
| Field       | Type   | Description |
|-------------|--------|-------------|
| file_id     | string | File ID |
| function_id | string | Function ID |

### cie_defines_type
Links files to their types.
| Field   | Type   | Description |
|---------|--------|-------------|
| file_id | string | File ID |
| type_id | string | Type ID |

### cie_calls
Function call relationships.
| Field     | Type   | Description |
|-----------|--------|-------------|
| caller_id | string | ID of calling function |
| callee_id | string | ID of called function |

### cie_import
Import statements.
| Field       | Type   | Description |
|-------------|--------|-------------|
| id          | string | Import ID |
| file_path   | string | File containing import |
| import_path | string | Imported package/module |
| alias       | string | Import alias (if any) |
| start_line  | int    | Line number |

## CozoScript Operators

### String Operations
- ` + "`starts_with(str, prefix)`" + ` - Check if string starts with prefix
- ` + "`ends_with(str, suffix)`" + ` - Check if string ends with suffix
- ` + "`regex_matches(str, pattern)`" + ` - Regex match (use (?i) for case-insensitive)
- ` + "`length(str)`" + ` - String length

### Comparison
- ` + "`=`" + `, ` + "`!=`" + `, ` + "`<`" + `, ` + "`>`" + `, ` + "`<=`" + `, ` + "`>=`" + ` - Standard comparisons

### Aggregation
- ` + "`count(field)`" + ` - Count occurrences
- ` + "`min(field)`" + `, ` + "`max(field)`" + ` - Min/max values

## Example Queries

### Find functions by name pattern (metadata only - fast)
` + "```" + `
?[file_path, name, start_line] := *cie_function { file_path, name, start_line },
  regex_matches(name, "(?i)batch")
` + "```" + `

### Get function with code (JOIN required)
` + "```" + `
?[name, file_path, code_text] :=
  *cie_function { id, name, file_path },
  *cie_function_code { function_id: id, code_text },
  name = "BuildMutations"
` + "```" + `

### Search in code text
` + "```" + `
?[name, file_path] :=
  *cie_function { id, name, file_path },
  *cie_function_code { function_id: id, code_text },
  regex_matches(code_text, "(?i)http\\.Get")
` + "```" + `

### Semantic search (HNSW on embedding table)
` + "```" + `
?[name, file_path, distance] :=
  ~cie_function_embedding:embedding_idx { function_id | query: q, k: 10, ef: 50, bind_distance: distance },
  q = vec([...1536 floats...]),
  *cie_function { id: function_id, name, file_path }
` + "```" + `

### Find callers of a function
` + "```" + `
?[caller_file, caller_name, callee_name] :=
  *cie_calls { caller_id, callee_id },
  *cie_function { id: callee_id, name: callee_name },
  *cie_function { id: caller_id, file_path: caller_file, name: caller_name },
  ends_with(callee_name, "Batch")
` + "```" + `

### List files by language
` + "```" + `
?[path, size] := *cie_file { path, language, size }, language = "go" :limit 20
` + "```" + `

## Important Notes

1. **Schema v3 Performance**: Most queries only need cie_function (metadata). JOIN with cie_function_code only when you need code_text.
2. **Go methods include receiver**: Function named "Batch" on type "Batcher" is stored as "Batcher.Batch"
3. **IDs are hashes**: Use joins via cie_defines and cie_calls to connect entities
4. **No LIKE operator**: Use regex_matches() instead
5. **No CONTAINS**: Use regex_matches() with pattern
6. **Limit results**: Always use :limit N for large result sets
7. **HNSW indices**: Located on cie_function_embedding:embedding_idx and cie_type_embedding:embedding_idx

---

## CIE Tools Quick Reference (v1.4.0)

### Search Tools

| Tool | Use Case | Key Parameters |
|------|----------|----------------|
| ` + "`cie_grep`" + ` | Fast literal text search | ` + "`text`" + ` OR ` + "`texts[]`" + ` for multi-pattern |
| ` + "`cie_search_text`" + ` | Regex search in code | ` + "`pattern`" + `, ` + "`literal=true`" + ` for exact match |
| ` + "`cie_semantic_search`" + ` | Natural language search | ` + "`query`" + `, ` + "`min_similarity`" + ` |
| ` + "`cie_find_function`" + ` | Find by function name | ` + "`name`" + `, ` + "`include_code`" + ` |
| ` + "`cie_find_type`" + ` | Find structs/interfaces | ` + "`name`" + `, ` + "`kind`" + ` |

### Analysis Tools

| Tool | Use Case | Key Parameters |
|------|----------|----------------|
| ` + "`cie_analyze`" + ` | Architecture questions | ` + "`question`" + ` (natural language) |
| ` + "`cie_list_endpoints`" + ` | HTTP API routes | ` + "`path_pattern`" + `, ` + "`method`" + ` |
| ` + "`cie_find_callers`" + ` | Who calls this function? | ` + "`function_name`" + ` |
| ` + "`cie_find_callees`" + ` | What does this call? | ` + "`function_name`" + ` |
| ` + "`cie_trace_path`" + ` | Call path from A to B | ` + "`target`" + `, ` + "`source`" + ` |
| ` + "`cie_find_implementations`" + ` | Interface implementations | ` + "`interface_name`" + ` |

### Audit Tools

| Tool | Use Case | Key Parameters |
|------|----------|----------------|
| ` + "`cie_verify_absence`" + ` | Verify patterns DON'T exist | ` + "`patterns[]`" + `, ` + "`severity`" + ` |
| ` + "`cie_grep`" + ` (multi) | Batch pattern search | ` + "`texts[]`" + ` returns grouped counts |

### Exploration Tools

| Tool | Use Case | Key Parameters |
|------|----------|----------------|
| ` + "`cie_list_files`" + ` | Browse indexed files | ` + "`path_pattern`" + `, ` + "`language`" + ` |
| ` + "`cie_directory_summary`" + ` | Module overview | ` + "`path`" + ` |
| ` + "`cie_get_file_summary`" + ` | File contents summary | ` + "`file_path`" + ` |
| ` + "`cie_index_status`" + ` | Check indexing health | ` + "`path_pattern`" + ` |

### Tips

1. **Multi-pattern search**: Use ` + "`cie_grep texts=[\"a\",\"b\",\"c\"]`" + ` instead of 3 separate calls
2. **Security audits**: Use ` + "`cie_verify_absence`" + ` to check for secrets/tokens
3. **API discovery**: ` + "`cie_list_endpoints`" + ` shows summary by method, path, and file
4. **Always start with**: ` + "`cie_index_status`" + ` to verify the path is indexed`
