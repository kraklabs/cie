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

package ingestion

import (
	"testing"

	"github.com/kraklabs/cie/pkg/sigparse"
)

func TestParseGoSignatureParams_Simple(t *testing.T) {
	sig := "func storeFact(client Querier, fact string) error"
	params := ParseGoSignatureParams(sig)

	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d: %+v", len(params), params)
	}
	if params[0].Name != "client" || params[0].Type != "Querier" {
		t.Errorf("param 0: got %+v, want {client Querier}", params[0])
	}
	if params[1].Name != "fact" || params[1].Type != "string" {
		t.Errorf("param 1: got %+v, want {fact string}", params[1])
	}
}

func TestParseGoSignatureParams_QualifiedType(t *testing.T) {
	sig := "func run(ctx context.Context, q tools.Querier) error"
	params := ParseGoSignatureParams(sig)

	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d: %+v", len(params), params)
	}
	if params[0].Name != "ctx" || params[0].Type != "Context" {
		t.Errorf("param 0: got %+v, want {ctx Context}", params[0])
	}
	if params[1].Name != "q" || params[1].Type != "Querier" {
		t.Errorf("param 1: got %+v, want {q Querier}", params[1])
	}
}

func TestParseGoSignatureParams_Pointer(t *testing.T) {
	sig := "func process(client *Querier) error"
	params := ParseGoSignatureParams(sig)

	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d: %+v", len(params), params)
	}
	if params[0].Name != "client" || params[0].Type != "Querier" {
		t.Errorf("param 0: got %+v, want {client Querier}", params[0])
	}
}

func TestParseGoSignatureParams_Grouped(t *testing.T) {
	sig := "func swap(a, b int) (int, int)"
	params := ParseGoSignatureParams(sig)

	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d: %+v", len(params), params)
	}
	if params[0].Name != "a" || params[0].Type != "int" {
		t.Errorf("param 0: got %+v, want {a int}", params[0])
	}
	if params[1].Name != "b" || params[1].Type != "int" {
		t.Errorf("param 1: got %+v, want {b int}", params[1])
	}
}

func TestParseGoSignatureParams_Variadic(t *testing.T) {
	sig := "func format(msg string, args ...interface{}) string"
	params := ParseGoSignatureParams(sig)

	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d: %+v", len(params), params)
	}
	if params[0].Name != "msg" || params[0].Type != "string" {
		t.Errorf("param 0: got %+v, want {msg string}", params[0])
	}
	if params[1].Name != "args" || params[1].Type != "interface{}" {
		t.Errorf("param 1: got %+v, want {args interface{}}", params[1])
	}
}

func TestParseGoSignatureParams_FuncParam(t *testing.T) {
	sig := "func apply(fn func(int) error, val int) error"
	params := ParseGoSignatureParams(sig)

	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d: %+v", len(params), params)
	}
	if params[0].Name != "fn" || params[0].Type != "func" {
		t.Errorf("param 0: got %+v, want {fn func}", params[0])
	}
	if params[1].Name != "val" || params[1].Type != "int" {
		t.Errorf("param 1: got %+v, want {val int}", params[1])
	}
}

func TestParseGoSignatureParams_MethodReceiver(t *testing.T) {
	sig := "func (s *Server) Run(ctx context.Context, q Querier) error"
	params := ParseGoSignatureParams(sig)

	if len(params) != 2 {
		t.Fatalf("expected 2 params (receiver excluded), got %d: %+v", len(params), params)
	}
	if params[0].Name != "ctx" || params[0].Type != "Context" {
		t.Errorf("param 0: got %+v, want {ctx Context}", params[0])
	}
	if params[1].Name != "q" || params[1].Type != "Querier" {
		t.Errorf("param 1: got %+v, want {q Querier}", params[1])
	}
}

func TestParseGoSignatureParams_Empty(t *testing.T) {
	params := ParseGoSignatureParams("")
	if len(params) != 0 {
		t.Errorf("expected 0 params for empty signature, got %d", len(params))
	}

	params = ParseGoSignatureParams("func noParams() error")
	if len(params) != 0 {
		t.Errorf("expected 0 params for no-arg function, got %d", len(params))
	}
}

func TestParseGoSignatureParams_Slice(t *testing.T) {
	sig := "func process(items []string, handlers []Handler) error"
	params := ParseGoSignatureParams(sig)

	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d: %+v", len(params), params)
	}
	if params[0].Name != "items" || params[0].Type != "string" {
		t.Errorf("param 0: got %+v, want {items string}", params[0])
	}
	if params[1].Name != "handlers" || params[1].Type != "Handler" {
		t.Errorf("param 1: got %+v, want {handlers Handler}", params[1])
	}
}

func TestParseGoSignatureParams_PointerQualified(t *testing.T) {
	sig := "func create(db *sql.DB) error"
	params := ParseGoSignatureParams(sig)

	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d: %+v", len(params), params)
	}
	if params[0].Name != "db" || params[0].Type != "DB" {
		t.Errorf("param 0: got %+v, want {db DB}", params[0])
	}
}

func TestParseGoSignatureParams_MultipleGrouped(t *testing.T) {
	sig := "func multi(a, b, c int, x, y string) error"
	params := ParseGoSignatureParams(sig)

	if len(params) != 5 {
		t.Fatalf("expected 5 params, got %d: %+v", len(params), params)
	}
	for _, p := range params[:3] {
		if p.Type != "int" {
			t.Errorf("first 3 params should be int, got %+v", p)
		}
	}
	for _, p := range params[3:] {
		if p.Type != "string" {
			t.Errorf("last 2 params should be string, got %+v", p)
		}
	}
}

func TestNormalizeType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Querier", "Querier"},
		{"*Querier", "Querier"},
		{"[]Querier", "Querier"},
		{"tools.Querier", "Querier"},
		{"*tools.Querier", "Querier"},
		{"...string", "string"},
		{"func(int) error", "func"},
		{"string", "string"},
		{"int", "int"},
		{"[]*Querier", "Querier"},
	}
	for _, tt := range tests {
		got := sigparse.NormalizeType(tt.input)
		if got != tt.want {
			t.Errorf("normalizeType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractParamString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"func foo(a int, b string) error", "a int, b string"},
		{"func (r *Type) Foo(a int) error", "a int"},
		{"func bar() error", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := sigparse.ExtractParamString(tt.input)
		if got != tt.want {
			t.Errorf("extractParamString(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
