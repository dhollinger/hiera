package internal

import (
	"fmt"
	"sync"

	"github.com/lyraproj/hiera/hieraapi"
	"github.com/lyraproj/hiera/provider"

	"github.com/lyraproj/pcore/px"
	"github.com/lyraproj/pcore/types"
)

type DataHashProvider struct {
	hierarchyEntry hieraapi.Entry
	providerFunc   hieraapi.DataHash
	hashes         map[string]px.OrderedMap
	hashesLock     sync.RWMutex
}

func (dh *DataHashProvider) Lookup(key hieraapi.Key, invocation hieraapi.Invocation, merge hieraapi.MergeStrategy) px.Value {
	return invocation.WithDataProvider(dh, func() px.Value {
		locations := dh.hierarchyEntry.Locations()
		switch len(locations) {
		case 0:
			return dh.invokeWithLocation(invocation, nil, key.Root())
		case 1:
			return dh.invokeWithLocation(invocation, locations[0], key.Root())
		default:
			return merge.Lookup(locations, invocation, func(location interface{}) px.Value {
				return dh.invokeWithLocation(invocation, location.(hieraapi.Location), key.Root())
			})
		}
	})
}

func (dh *DataHashProvider) invokeWithLocation(invocation hieraapi.Invocation, location hieraapi.Location, root string) px.Value {
	if location == nil {
		return dh.lookupKey(invocation, nil, root)
	}
	return invocation.WithLocation(location, func() px.Value {
		if location.Exists() {
			return dh.lookupKey(invocation, location, root)
		}
		invocation.ReportLocationNotFound()
		return nil
	})
}

func (dh *DataHashProvider) lookupKey(invocation hieraapi.Invocation, location hieraapi.Location, root string) px.Value {
	if value := dh.dataValue(invocation, location, root); value != nil {
		invocation.ReportFound(root, value)
		return value
	}
	invocation.ReportNotFound(root)
	return nil
}

func (dh *DataHashProvider) dataValue(ic hieraapi.Invocation, location hieraapi.Location, root string) px.Value {
	hash := dh.dataHash(ic, location)
	value, found := hash.Get4(root)
	if !found {
		return nil
	}

	pfx := func() string {
		msg := fmt.Sprintf(`Value for key '%s' in hash returned from %s`, root, dh.FullName())
		if location != nil {
			msg = fmt.Sprintf(`%s, when using location '%s'`, msg, location)
		}
		return msg
	}

	value = px.AssertInstance(pfx, types.DefaultRichDataType(), value)
	return Interpolate(ic, value, true)
}

func (dh *DataHashProvider) providerFunction(ic hieraapi.Invocation) (pf hieraapi.DataHash) {
	if dh.providerFunc == nil {
		n := dh.hierarchyEntry.Function().Name()
		if n == `yaml_data` {
			// Shortcut. No need to go through pcore calling mechanism
			dh.providerFunc = provider.YamlData
			return dh.providerFunc
		}
		if n == `json_data` {
			// Shortcut. No need to go through pcore calling mechanism
			dh.providerFunc = provider.JsonData
			return dh.providerFunc
		}

		// Load lookup provider function using the standard loader
		if f, ok := px.Load(ic, px.NewTypedName(px.NsFunction, n)); ok {
			dh.providerFunc = func(pc hieraapi.ProviderContext, options map[string]px.Value) (value px.OrderedMap) {
				value = px.EmptyMap
				defer catchNotFound()
				v := f.(px.Function).Call(ic, nil, []px.Value{pc, px.Wrap(ic, options)}...)
				if dv, ok := v.(px.OrderedMap); ok {
					value = dv
				}
				return
			}
		} else {
			ic.ReportText(func() string {
				return fmt.Sprintf(`unresolved function '%s'`, n)
			})
			dh.providerFunc = func(pc hieraapi.ProviderContext, options map[string]px.Value) px.OrderedMap {
				return px.EmptyMap
			}
		}
	}
	return dh.providerFunc
}

func (dh *DataHashProvider) dataHash(ic hieraapi.Invocation, location hieraapi.Location) (hash px.OrderedMap) {
	key := ``
	opts := dh.hierarchyEntry.OptionsMap()
	if location != nil {
		key = location.Resolved()
		opts = optionsWithLocation(opts, key)
	}

	var ok bool
	dh.hashesLock.RLock()
	hash, ok = dh.hashes[key]
	dh.hashesLock.RUnlock()
	if ok {
		return
	}

	dh.hashesLock.Lock()
	defer dh.hashesLock.Unlock()

	if hash, ok = dh.hashes[key]; ok {
		return hash
	}
	hash = dh.providerFunction(ic)(newProviderContext(ic, &sync.Map{}), opts)
	dh.hashes[key] = hash
	return
}

func (dh *DataHashProvider) FullName() string {
	return fmt.Sprintf(`data_hash function '%s'`, dh.hierarchyEntry.Function().Name())
}

func newDataHashProvider(he hieraapi.Entry) hieraapi.DataProvider {
	ls := he.Locations()
	return &DataHashProvider{hierarchyEntry: he, hashes: make(map[string]px.OrderedMap, len(ls))}
}

func optionsWithLocation(options map[string]px.Value, loc string) map[string]px.Value {
	ov := types.WrapString(loc)
	if len(options) == 0 {
		return map[string]px.Value{`path`: ov}
	}
	newOpts := make(map[string]px.Value, len(options)+1)
	for k, v := range options {
		newOpts[k] = v
	}
	newOpts[`path`] = ov
	return newOpts
}
