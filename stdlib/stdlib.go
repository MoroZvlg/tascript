// Package stdlib holds the built-in tascript modules (math, time).
package stdlib

import "github.com/MoroZvlg/tascript/registry"

func Register(reg *registry.Registry) {
	RegisterMath(reg)
	RegisterTime(reg)
}
