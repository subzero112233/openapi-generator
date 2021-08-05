package traverser

import (
	"regexp"
	"sort"
	"strconv"
)

var idxRegex = regexp.MustCompile(`^\[(\d+)\]$`)

type Map map[interface{}]interface{}

type Any struct {
	data interface{}
}

func (doc Map) Get(path ...string) (final Any, ok bool) {
	if len(path) == 0 {
		return final, false
	}

	inext, ok := doc[path[0]]
	if !ok {
		return final, false
	}

	next := Any{inext}

	if len(path) > 1 {
		return next.Get(path[1:]...)
	}

	return next, true
}

func (any Any) Get(path ...string) (final Any, ok bool) {
	if len(path) == 0 {
		return any, true
	}

	var next Any

	matches := idxRegex.FindStringSubmatch(path[0])
	if len(matches) == 2 {
		// we expect the next part to be a slice
		s, ok := any.data.([]interface{})
		if !ok {
			return final, ok
		}

		idx, err := strconv.Atoi(matches[1])
		if err != nil {
			return final, false
		}

		if idx > len(s)-1 {
			return final, false
		}

		next = Any{s[idx]}
	} else {
		// we expect the next part to be another map (or the target field)
		m, ok := any.data.(Map)
		if !ok {
			return final, ok
		}

		mp, ok := m[path[0]]
		if !ok {
			return final, false
		}

		next = Any{mp}
	}

	if len(path) > 1 {
		return next.Get(path[1:]...)
	}

	return next, true
}

func (doc Map) Str(path ...string) (str string) {
	sub, ok := doc.Get(path...)
	if !ok {
		return str
	}

	str, _ = sub.data.(string)
	return str
}

func (any Any) Str(path ...string) (str string) {
	sub, ok := any.Get(path...)
	if !ok {
		return str
	}

	str, _ = sub.data.(string)
	return str
}

func (any Any) Bool(path ...string) (b bool) {
	sub, ok := any.Get(path...)
	if !ok {
		return b
	}

	b, _ = sub.data.(bool)
	return b
}

func (any Any) Int64(path ...string) (num int64) {
	sub, ok := any.Get(path...)
	if !ok {
		return num
	}

	switch v := sub.data.(type) {
	case int64:
		num = v
	case int32:
		num = int64(v)
	case int:
		num = int64(v)
	}

	return num
}

func (any Any) Uint64(path ...string) (num uint64) {
	sub, ok := any.Get(path...)
	if !ok {
		return num
	}

	switch v := sub.data.(type) {
	case int64:
		num = uint64(v)
	case int32:
		num = uint64(v)
	case int:
		num = uint64(v)
	case uint64:
		num = v
	case uint32:
		num = uint64(v)
	}

	return num
}

func (doc Map) Slice(path ...string) (slice []Any) {
	sub, ok := doc.Get(path...)
	if !ok {
		return slice
	}

	items, ok := sub.data.([]interface{})
	if !ok {
		return slice
	}

	slice = make([]Any, len(items))
	for i := range items {
		slice[i] = Any{items[i]}
	}

	return slice
}

func (any Any) Slice(path ...string) (slice []Any) {
	sub, ok := any.Get(path...)
	if !ok {
		return slice
	}

	items, ok := sub.data.([]interface{})
	if !ok {
		return slice
	}

	slice = make([]Any, len(items))
	for i := range items {
		slice[i] = Any{items[i]}
	}

	return slice
}

func (doc Map) Keys(path ...string) []string {
	sub, ok := doc.Get(path...)
	if !ok {
		return nil
	}

	m, ok := sub.data.(Map)
	if !ok {
		return nil
	}

	strs := make([]string, len(m))
	var i int
	for key := range m {
		strs[i] = key.(string)
		i++
	}

	sort.Strings(strs)

	return strs
}

func (any Any) Keys(path ...string) []string {
	sub, ok := any.Get(path...)
	if !ok {
		return nil
	}

	m, ok := sub.data.(Map)
	if !ok {
		return nil
	}

	strs := make([]string, len(m))
	var i int
	for key := range m {
		strs[i] = key.(string)
		i++
	}

	sort.Strings(strs)

	return strs
}
