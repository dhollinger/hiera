package provider

import (
	"github.com/lyraproj/hiera/hieraapi"
	"github.com/lyraproj/pcore/px"
	"github.com/lyraproj/pcore/types"
)

// ScopeLookupKey is a function that performs a lookup in the current scope.
func ScopeLookupKey(c hieraapi.ProviderContext, key string, _ map[string]px.Value) px.Value {
	if v, ok := c.Invocation().Scope().Get(types.WrapString(key)); ok {
		return v
	}
	return nil
}
