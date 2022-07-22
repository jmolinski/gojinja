package environment

import (
	"fmt"
	"github.com/gojinja/gojinja/src/defaults"
	"github.com/gojinja/gojinja/src/filters"
	"github.com/gojinja/gojinja/src/lexer"
	"github.com/gojinja/gojinja/src/loaders"
	"github.com/gojinja/gojinja/src/runtime"
	"github.com/gojinja/gojinja/src/tests"
	"github.com/gojinja/gojinja/src/utils/maps"
	"github.com/gojinja/gojinja/src/utils/slices"
	lru "github.com/hashicorp/golang-lru"
	"strings"
)

// Environment is the core component of Jinja.  It contains
// important shared variables like configuration, filters, tests,
// globals and others.  Instances of this class may be modified if
// they are not shared and if no template was loaded so far.
// Modifications on environments after the first template was loaded
// will lead to surprising effects and undefined behavior.
type Environment struct {
	Sandboxed     bool
	Overlayed     bool
	LinkedTo      *Environment
	Shared        bool
	Concat        func([]string) string
	ContextClass  runtime.ContextClass
	TemplateClass Class
	*lexer.EnvLexerInformation
	Optimized  bool
	Extensions map[string]Extension // extension name or extension
	Undefined  runtime.UndefinedClass
	Finalize   func(...any) any
	AutoEscape any // bool or func(string)bool
	Loader     *loaders.Loader
	Cache      Cache
	AutoReload bool
	Filters    map[string]filters.Filter
	Tests      map[string]tests.Test
	Globals    map[string]func(...any) any
	Policies   map[string]any
}

type Cache interface {
	Add(key, value interface{}) (evicted bool)
	Get(key interface{}) (value interface{}, ok bool)
}

type mapCache map[interface{}]interface{}

func (m mapCache) Add(key, value interface{}) (evicted bool) {
	m[key] = value
	return false
}
func (m mapCache) Get(key interface{}) (value interface{}, ok bool) {
	v, ok := m[key]
	return v, ok
}

func New(opts *EnvOpts) (*Environment, error) {
	var err error
	env := &Environment{
		Sandboxed:           false,
		Overlayed:           false,
		LinkedTo:            nil,
		Shared:              false,
		Concat:              func(strs []string) string { return strings.Join(strs, "") },
		ContextClass:        runtime.ContextClass{},
		TemplateClass:       Class{},
		EnvLexerInformation: opts.EnvLexerInformation,
		Optimized:           opts.Optimized,
		Undefined:           opts.Undefined,
		Finalize:            opts.Finalize,
		AutoEscape:          opts.AutoEscape,
		Loader:              opts.Loader,
		AutoReload:          opts.AutoReload,
		Filters:             maps.Copy(filters.Default),
		Tests:               maps.Copy(tests.Default),
		Globals:             maps.Copy(defaults.DefaultNamespace),
		Policies:            maps.Copy(defaults.DefaultPolicies),
	}
	env.Cache, err = createCache(opts.CacheSize)
	if err != nil {
		return nil, err
	}

	env.Extensions = LoadExtensions(env, opts.Extensions)
	if err = configCheck(env); err != nil {
		return nil, err
	}
	return env, nil
}

func configCheck(env *Environment) error {
	if env.BlockStartString == env.VariableStartString || env.BlockStartString == env.CommentStartString || env.CommentStartString == env.VariableStartString {
		return fmt.Errorf("block, variable and comment start strings must be different")
	}
	if !slices.Contains([]string{"\n", "\r\n", "\r"}, env.NewlineSequence) {
		return fmt.Errorf("'NewlineSequence' must be one of '\\n', '\\r\\n', or '\\r'")
	}
	return nil
}

func LoadExtensions(env *Environment, extensions map[string]func(*Environment) Extension) map[string]Extension {
	ret := make(map[string]Extension)
	for k, v := range extensions {
		ret[k] = v(env)
	}
	return ret
}

func createCache(cacheSize int) (Cache, error) {
	if cacheSize > 0 {
		return lru.New(cacheSize)
	}
	if cacheSize < 0 {
		return make(mapCache), nil
	}
	return nil, nil
}

type Extension struct{} // TODO change to real extension

type EnvOpts struct {
	*lexer.EnvLexerInformation
	Optimized  bool
	Extensions map[string]func(*Environment) Extension // TODO jinja accepts also extensions names but it's python import magic I don't know how to do it in golang.
	Undefined  runtime.UndefinedClass
	Finalize   func(...any) any
	AutoEscape any // bool or func(string)bool
	Loader     *loaders.Loader
	CacheSize  int
	AutoReload bool
}

func DefaultEnvOpts() *EnvOpts {
	return &EnvOpts{
		Optimized:           true,
		Extensions:          nil,
		EnvLexerInformation: lexer.DefaultEnvLexerInformation(),
		Undefined:           runtime.UndefinedClass{},
		Finalize:            nil,
		AutoEscape:          false,
		Loader:              nil,
		CacheSize:           400,
		AutoReload:          true,
	}
}
