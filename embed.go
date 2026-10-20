// Package scalewaymanager holds the files embedded in the server binary.
package scalewaymanager

import _ "embed"

// IndexHTML is the page served at /.
//
//go:embed web/index.html
var IndexHTML []byte
