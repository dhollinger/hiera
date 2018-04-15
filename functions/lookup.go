package functions

import (
	"github.com/puppetlabs/go-evaluator/eval"
	"github.com/puppetlabs/go-evaluator/types"
	"github.com/puppetlabs/go-hiera/lookup"
)

func doLookup(c eval.Context, name eval.PValue, vtype eval.PType, dflt eval.PValue, options eval.KeyedValue) (found eval.PValue, err error) {
	names := []string{}
	if nameArr, cok := name.(*types.ArrayValue); cok {
		nameArr.Each(func(n eval.PValue) {
			names = append(names, n.String())
		})
	} else {
		names = append(names, name.String())
	}
	return lookup.Lookup(c, names, dflt, options)
}

func init() {
	eval.NewGoFunction2(`lookup`,
		func(l eval.LocalTypes) {
			l.Type(`NameType`, `Variant[String, Array[String]]`)
			l.Type(`MergeType`, `Variant[String[1], Hash[String,ScalarData]]`)
		},

		func(d eval.Dispatch) {
			d.Param(`NameType`)
			d.OptionalParam(`Type`)
			d.OptionalParam(`MergeType`)
			d.Function(func(c eval.Context, args []eval.PValue) eval.PValue {
				var vtype eval.PType = types.DefaultAnyType()
				options := eval.EMPTY_MAP
				nargs := len(args)
				if nargs > 1 {
					vtype = args[1].(eval.PType)
					if nargs > 2 {
						options = types.SingletonHash2(`merge`, args[2])
					}
				}
				found, err := doLookup(c, args[0], vtype, nil, options)
				if err != nil {
					panic(err)
				}
				return found
			})
		},

		// TODO: Add other variants
	)
}