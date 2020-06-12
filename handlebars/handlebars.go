package handlebars

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aymerick/raymond"
)

// Engine struct
type Engine struct {
	// views folder
	directory string
	// views extension
	extension string
	// layout variable name that incapsulates the template
	layout string
	// reload on each render
	reload bool
	// debug prints the parsed templates
	debug bool
	// lock for funcmap and templates
	mutex sync.RWMutex
	// template funcmap
	funcmap map[string]interface{}
	// templates
	Templates map[string]*raymond.Template
}

// New returns a Handlebar render engine for Fiber
func New(directory, extension string) *Engine {
	engine := &Engine{
		directory: directory,
		extension: extension,
		layout:    "embed",
		funcmap:   make(map[string]interface{}),
		Templates: make(map[string]*raymond.Template),
	}
	return engine
}

// Delims sets the action delimiters to the specified strings, to be used in
// templates. An empty delimiter stands for the
// corresponding default: {{ or }}.
func (e *Engine) Delims(left, right string) *Engine {
	fmt.Println("delims: this method is not supported for handlebars")
	return e
}

// Layout defines the variable name that will incapsulate the template
func (e *Engine) Layout(key string) *Engine {
	e.layout = key
	return e
}

// AddFunc adds the function to the template's function map.
// It is legal to overwrite elements of the default actions
func (e *Engine) AddFunc(name string, fn interface{}) *Engine {
	e.mutex.Lock()
	e.funcmap[name] = fn
	e.mutex.Unlock()
	return e
}

// Reload if set to true the templates are reloading on each render,
// use it when you're in development and you don't want to restart
// the application when you edit a template file.
func (e *Engine) Reload(enabled bool) *Engine {
	e.reload = enabled
	return e
}

// Debug will print the parsed templates when Load is triggered.
func (e *Engine) Debug(enabled bool) *Engine {
	e.debug = enabled
	return e
}

// Parse is deprecated, please use Load() instead
func (e *Engine) Parse() error {
	fmt.Println("Parse() is deprecated, please use Load() instead.")
	return e.Load()
}

// Parse parses the templates to the engine.
func (e *Engine) Load() error {
	// race safe
	e.mutex.Lock()
	defer e.mutex.Unlock()
	// Set template settings
	raymond.RegisterHelpers(e.funcmap)
	// Loop trough each directory and register template files
	err := filepath.Walk(e.directory, func(path string, info os.FileInfo, err error) error {
		// Return error if exist
		if err != nil {
			return err
		}
		// Skip file if it's a directory or has no file info
		if info == nil || info.IsDir() {
			return nil
		}
		// Get file extension of file
		ext := filepath.Ext(path)
		// Skip file if it does not equal the given template extension
		if ext != e.extension {
			return nil
		}
		// Get the relative file path
		// ./views/html/index.tmpl -> index.tmpl
		rel, err := filepath.Rel(e.directory, path)
		if err != nil {
			return err
		}
		// Reverse slashes '\' -> '/' and
		// partials\footer.tmpl -> partials/footer.tmpl
		name := filepath.ToSlash(rel)
		// Remove ext from name 'index.tmpl' -> 'index'
		name = strings.Replace(name, e.extension, "", -1)

		// Read the file
		// #gosec G304
		buf, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		// Create new template associated with the current one
		// This enable use to invoke other templates {{ template .. }}
		tmpl, err := raymond.Parse(string(buf))
		if err != nil {
			return err
		}
		// This will panic, see solution at the end of the function
		// raymond.RegisterPartialTemplate(name, tmpl)
		e.Templates[name] = tmpl

		// Debugging
		if e.debug {
			fmt.Printf("views: parsed template: %s\n", name)
		}
		return err
	})
	// Register all templates with each other
	for i := range e.Templates {
		for n, t := range e.Templates {
			e.Templates[i].RegisterPartialTemplate(n, t)
		}
	}
	return err
}

// Execute will render the template by name
func (e *Engine) Render(out io.Writer, template string, binding interface{}, layout ...string) error {
	if e.reload {
		if err := e.Load(); err != nil {
			return err
		}
	}
	tmpl, ok := e.Templates[template]
	if !ok {
		return fmt.Errorf("render: template %s does not exist", template)
	}
	parsed, err := tmpl.Exec(binding)
	if err != nil {
		return err
	}
	if len(layout) > 0 {
		lay := e.Templates[layout[0]]
		if lay == nil {
			return fmt.Errorf("render: layout %s does not exist", layout[0])
		}
		var bind map[string]interface{}
		if binding == nil {
			bind = make(map[string]interface{}, 1)
		} else if m, ok := binding.(map[string]interface{}); ok {
			bind = m
		}
		bind[e.layout] = raymond.SafeString(parsed)
		parsed, err := lay.Exec(bind)
		if err != nil {
			return err
		}
		_, err = out.Write([]byte(parsed))
		if err != nil {
			return err
		}
		return nil
	}
	_, err = out.Write([]byte(parsed))
	return err
}