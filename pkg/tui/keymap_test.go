package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/stretchr/testify/assert"
)

// TestKeymapCompleteness validates that all key.Matches calls in focus mode handlers
// are represented in the corresponding keymap's help display.
// This ensures when new keybindings are added to handlers, they're also added to help.
func TestKeymapCompleteness(t *testing.T) {
	tests := []struct {
		name             string
		functionName     string
		keymap           interface{ ShortHelp() []key.Binding; FullHelp() [][]key.Binding }
		keymapSourceName string // The name of the keymap variable used in key.Matches calls
	}{
		{
			name:             "switchTableFocusMode uses defaultKeyMap",
			functionName:     "switchTableFocusMode",
			keymap:           defaultKeyMap,
			keymapSourceName: "defaultKeyMap",
		},
		{
			name:             "switchIncidentFocusMode uses defaultKeyMap",
			functionName:     "switchIncidentFocusMode",
			keymap:           defaultKeyMap,
			keymapSourceName: "defaultKeyMap",
		},
		{
			name:             "switchInputFocusMode uses inputModeKeyMap",
			functionName:     "switchInputFocusMode",
			keymap:           inputModeKeyMap,
			keymapSourceName: "defaultKeyMap", // Uses defaultKeyMap for key.Matches
		},
		{
			name:             "switchErrorFocusMode uses errorViewKeyMap",
			functionName:     "switchErrorFocusMode",
			keymap:           errorViewKeyMap,
			keymapSourceName: "defaultKeyMap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the msgHandlers.go file to extract key.Matches calls
			matchedKeys := extractKeyMatchesFromFunction("msgHandlers.go", tt.functionName, tt.keymapSourceName)

			if len(matchedKeys) == 0 {
				t.Logf("No key.Matches calls found in %s - skipping validation", tt.functionName)
				return
			}

			// Get all keys from help (both short and full)
			helpKeys := make(map[string]bool)

			// Collect from ShortHelp
			for _, binding := range tt.keymap.ShortHelp() {
				helpKeys[getBindingFieldName(binding)] = true
			}

			// Collect from FullHelp
			for _, column := range tt.keymap.FullHelp() {
				for _, binding := range column {
					helpKeys[getBindingFieldName(binding)] = true
				}
			}

			// Verify each matched key is in the help
			for _, keyField := range matchedKeys {
				assert.True(t, helpKeys[keyField],
					"Key binding '%s' is used in %s via key.Matches but not present in help display. "+
					"Please add it to the keymap's ShortHelp() or FullHelp() method.",
					keyField, tt.functionName)
			}
		})
	}
}

// extractKeyMatchesFromFunction parses a Go source file and extracts all field names
// used in key.Matches calls within the specified function
func extractKeyMatchesFromFunction(filename, functionName, keymapName string) []string {
	var matchedKeys []string

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return matchedKeys
	}

	// Find the function declaration
	var targetFunc *ast.FuncDecl
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Name.Name == functionName {
				targetFunc = fn
				break
			}
		}
	}

	if targetFunc == nil {
		return matchedKeys
	}

	// Walk the AST looking for key.Matches calls
	ast.Inspect(targetFunc, func(n ast.Node) bool {
		// Look for call expressions
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if it's a call to key.Matches
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "key" || sel.Sel.Name != "Matches" {
			return true
		}

		// Extract the second argument (the key binding)
		if len(call.Args) < 2 {
			return true
		}

		// Second argument should be something like defaultKeyMap.Enter
		if selExpr, ok := call.Args[1].(*ast.SelectorExpr); ok {
			if ident, ok := selExpr.X.(*ast.Ident); ok {
				if ident.Name == keymapName {
					// Extract the field name (e.g., "Enter" from "defaultKeyMap.Enter")
					matchedKeys = append(matchedKeys, selExpr.Sel.Name)
				}
			}
		}

		return true
	})

	return matchedKeys
}

// getBindingFieldName uses reflection to find the field name of a key.Binding
// within a keymap struct
func getBindingFieldName(binding key.Binding) string {
	// Compare the binding against all fields in defaultKeyMap
	val := reflect.ValueOf(defaultKeyMap)
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if field.Type() == reflect.TypeOf(binding) {
			// Compare the actual binding values
			fieldBinding := field.Interface().(key.Binding)
			if reflect.DeepEqual(fieldBinding.Keys(), binding.Keys()) {
				return typ.Field(i).Name
			}
		}
	}

	// Also check inputModeKeyMap
	val = reflect.ValueOf(inputModeKeyMap)
	typ = val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if field.Type() == reflect.TypeOf(binding) {
			fieldBinding := field.Interface().(key.Binding)
			if reflect.DeepEqual(fieldBinding.Keys(), binding.Keys()) {
				return typ.Field(i).Name
			}
		}
	}

	// Also check errorViewKeyMap
	val = reflect.ValueOf(errorViewKeyMap)
	typ = val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if field.Type() == reflect.TypeOf(binding) {
			fieldBinding := field.Interface().(key.Binding)
			if reflect.DeepEqual(fieldBinding.Keys(), binding.Keys()) {
				return typ.Field(i).Name
			}
		}
	}

	return ""
}
