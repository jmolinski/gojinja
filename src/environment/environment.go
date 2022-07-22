package environment

import (
	"fmt"
	"github.com/gojinja/gojinja/src/defaults"
	"github.com/gojinja/gojinja/src/filters"
	"github.com/gojinja/gojinja/src/lexer"
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
	Undefined  UndefinedConstructor
	Finalize   func(...any) any
	AutoEscape any // bool or func(string)bool
	Loader     *Loader
	Cache      Cache
	AutoReload bool
	Filters    map[string]filters.Filter
	Tests      map[string]tests.Test
	Globals    map[string]any
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
	Undefined  UndefinedConstructor
	Finalize   func(...any) any
	AutoEscape any // bool or func(string)bool
	Loader     *Loader
	CacheSize  int
	AutoReload bool
}

type UndefinedConstructor func(hint *string, obj any, name *string, exc func(msg string) error) runtime.IUndefined

func DefaultEnvOpts() *EnvOpts {
	return &EnvOpts{
		Optimized:           true,
		Extensions:          nil,
		EnvLexerInformation: lexer.DefaultEnvLexerInformation(),
		Undefined: func(hint *string, obj any, name *string, exc func(msg string) error) runtime.IUndefined {
			return runtime.NewUndefined(hint, obj, name, exc)
		},
		Finalize:   nil,
		AutoEscape: false,
		Loader:     nil,
		CacheSize:  400,
		AutoReload: true,
	}
}

// GetTemplate loads a template by name with `Loader` and returns a `Template`.
// If the template does not exist a `TemplateNotFound` exception is raised.
func (env *Environment) GetTemplate(name any, parent *string, globals map[string]any) (ITemplate, error) {
	switch v := name.(type) {
	case ITemplate:
		return v, nil
	case string:
		if parent != nil {
			v = env.JoinPath(v, *parent)
		}
		return env.loadTemplate(v, globals)
	default:
		return nil, fmt.Errorf("unexpected type for `name`")
	}
}

// JoinPath joins a template with the parent. By default, all the lookups are
// relative to the loader root so this method returns the `template`
// parameter unchanged, but if the paths should be relative to the
// parent template, this function can be used to calculate the real
// template name.
//
// Subclasses may override this method and implement template path
// joining here.
func (env *Environment) JoinPath(v string, parent string) string {
	// TODO in jinja it may be overwritten by subclass
	return v
}

func (env *Environment) loadTemplate(name string, globals map[string]any) (ITemplate, error) {
	if env.Loader == nil {
		return nil, fmt.Errorf("no loader for this environment specified")
	}
	cacheKey := fmt.Sprint(env.Loader, name)

	if env.Cache != nil {
		template, ok := env.Cache.Get(cacheKey)
		if ok {
			tmpl := template.(ITemplate)
			if !env.AutoReload && tmpl.IsUpToDate() {
				maps.Update(tmpl.Globals(), globals)
			}
			return tmpl, nil
		}
	}
	template, err := (*env.Loader).Load(env, name, env.MakeGlobals(globals))
	if err != nil {
		return nil, err
	}
	if env.Cache != nil {
		env.Cache.Add(cacheKey, template)
	}
	return template, nil
}

func (env *Environment) MakeGlobals(globals map[string]any) map[string]any {
	return maps.Chain(globals, env.Globals)
}
