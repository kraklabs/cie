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

// Tests for analyze.go - including stub detection, code line counting,
// and other helper functions used in the Analyze tool.

import (
	"testing"
)

func TestDetectStub(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		filePath string
		wantStub bool
		wantMsg  string // substring to check in reason
	}{
		// Go - Strong indicators
		{
			name:     "Go not implemented error",
			code:     `func (a *Adapter) PauseProduct() error { return fmt.Errorf("not implemented") }`,
			filePath: "adapter.go",
			wantStub: true,
			wantMsg:  "not implemented",
		},
		{
			name:     "Go errors.New not implemented",
			code:     `func Foo() error { return errors.New("not implemented yet") }`,
			filePath: "foo.go",
			wantStub: true,
			wantMsg:  "not implemented",
		},
		{
			name:     "Go panic not implemented",
			code:     `func Bar() { panic("not implemented") }`,
			filePath: "bar.go",
			wantStub: true,
			wantMsg:  "not implemented",
		},
		{
			name:     "Go ErrNotImplemented",
			code:     `func Baz() error { return ErrNotImplemented }`,
			filePath: "baz.go",
			wantStub: true,
			wantMsg:  "ErrNotImplemented",
		},

		// Go - Weak indicators (short functions)
		{
			name: "Go short function only returns nil",
			code: `func (a *Adapter) DoNothing() error {
	return nil
}`,
			filePath: "adapter.go",
			wantStub: true,
			wantMsg:  "returns nil",
		},

		// Go - Real implementation (should NOT be stub)
		{
			name: "Go real implementation",
			code: `func (a *Adapter) DoSomething(ctx context.Context) error {
	// TODO: add retry logic
	result, err := a.client.Call(ctx)
	if err != nil {
		return fmt.Errorf("call failed: %w", err)
	}
	a.cache.Set(result)
	return nil
}`,
			filePath: "adapter.go",
			wantStub: false,
		},
		{
			name: "Go TODO with implementation",
			code: `func Process(data []byte) error {
	// TODO: optimize this later
	for _, b := range data {
		if err := processItem(b); err != nil {
			return err
		}
	}
	return nil
}`,
			filePath: "process.go",
			wantStub: false,
		},

		// Python
		{
			name:     "Python NotImplementedError",
			code:     `def foo(): raise NotImplementedError("not done yet")`,
			filePath: "foo.py",
			wantStub: true,
			wantMsg:  "NotImplementedError",
		},
		{
			name: "Python pass only",
			code: `def bar():
    pass`,
			filePath: "bar.py",
			wantStub: true,
			wantMsg:  "pass",
		},
		{
			name: "Python ellipsis",
			code: `def baz():
    ...`,
			filePath: "baz.py",
			wantStub: true,
			wantMsg:  "ellipsis",
		},
		{
			name: "Python real implementation",
			code: `def process(data):
    # TODO: add validation
    result = transform(data)
    return result`,
			filePath: "process.py",
			wantStub: false,
		},

		// TypeScript/JavaScript
		{
			name:     "TS throw not implemented",
			code:     `function foo() { throw new Error("not implemented"); }`,
			filePath: "foo.ts",
			wantStub: true,
			wantMsg:  "not implemented",
		},
		{
			name: "TS return undefined",
			code: `function bar() {
	return undefined;
}`,
			filePath: "bar.ts",
			wantStub: true,
			wantMsg:  "returns undefined",
		},

		// Rust
		{
			name:     "Rust todo macro",
			code:     `fn foo() { todo!(); }`,
			filePath: "foo.rs",
			wantStub: true,
			wantMsg:  "todo!()",
		},
		{
			name:     "Rust unimplemented macro",
			code:     `fn bar() { unimplemented!(); }`,
			filePath: "bar.rs",
			wantStub: true,
			wantMsg:  "unimplemented!()",
		},

		// Java
		{
			name:     "Java UnsupportedOperationException",
			code:     `void foo() { throw new UnsupportedOperationException("not implemented"); }`,
			filePath: "Foo.java",
			wantStub: true,
			wantMsg:  "UnsupportedOperationException",
		},

		// Generic
		{
			name:     "Generic not implemented string",
			code:     `function unknown() { console.log("not implemented"); }`,
			filePath: "unknown.xyz",
			wantStub: true,
			wantMsg:  "not implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectStub(tt.code, tt.filePath)

			if tt.wantStub {
				if result == nil {
					t.Errorf("expected stub detection, got nil")
					return
				}
				if !result.IsStub {
					t.Errorf("expected IsStub=true, got false")
				}
				if tt.wantMsg != "" && !containsIgnoreCase(result.Reason, tt.wantMsg) {
					t.Errorf("expected reason to contain %q, got %q", tt.wantMsg, result.Reason)
				}
			} else {
				if result != nil && result.IsStub {
					t.Errorf("expected no stub detection, got: %s", result.Reason)
				}
			}
		})
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && containsIgnoreCaseHelper(s, substr)))
}

func containsIgnoreCaseHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestCountCodeLines(t *testing.T) {
	tests := []struct {
		name string
		code string
		want int
	}{
		{
			name: "simple function",
			code: `func foo() {
	return nil
}`,
			want: 1, // only "return nil" counts
		},
		{
			name: "function with comments",
			code: `func foo() {
	// this is a comment
	return nil
}`,
			want: 1,
		},
		{
			name: "function with multiple statements",
			code: `func foo() {
	x := 1
	y := 2
	return x + y
}`,
			want: 3,
		},
		{
			name: "empty function",
			code: `func foo() {
}`,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countCodeLines(tt.code)
			if got != tt.want {
				t.Errorf("countCodeLines() = %d, want %d", got, tt.want)
			}
		})
	}
}
