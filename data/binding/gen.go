// +build ignore

package main

import (
	"os"
	"path"
	"runtime"
	"text/template"

	"fyne.io/fyne/v2"
)

const itemBindTemplate = `
// {{ .Name }} supports binding a {{ .Type }} value.
//
// Since: {{ .Since }}
type {{ .Name }} interface {
	DataItem
	Get() ({{ .Type }}, error)
	Set({{ .Type }}) error
}

// External{{ .Name }} supports binding a {{ .Type }} value to an external value.
//
// Since: {{ .Since }}
type External{{ .Name }} interface {
	{{ .Name }}
	Reload() error
}

// New{{ .Name }} returns a bindable {{ .Type }} value that is managed internally.
//
// Since: {{ .Since }}
func New{{ .Name }}() {{ .Name }} {
	blank := {{ .Default }}
	return &bound{{ .Name }}{val: &blank}
}

// Bind{{ .Name }} returns a new bindable value that controls the contents of the provided {{ .Type }} variable.
// If your code changes the content of the variable this refers to you should call Reload() to inform the bindings.
//
// Since: {{ .Since }}
func Bind{{ .Name }}(v *{{ .Type }}) External{{ .Name }} {
	if v == nil {
		return New{{ .Name }}().(External{{ .Name }}) // never allow a nil value pointer
	}

	return &bound{{ .Name }}{val: v}
}

type bound{{ .Name }} struct {
	base

	val *{{ .Type }}
}

func (b *bound{{ .Name }}) Get() ({{ .Type }}, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	if b.val == nil {
		return {{ .Default }}, nil
	}
	return *b.val, nil
}

func (b *bound{{ .Name }}) Reload() error {
	return b.Set(*b.val)
}

func (b *bound{{ .Name }}) Set(val {{ .Type }}) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	*b.val = val

	b.trigger()
	return nil
}
`

const prefTemplate = `
type prefBound{{ .Name }} struct {
	base
	key   string
	p     fyne.Preferences
	cache {{ .Type }}
}

// BindPreference{{ .Name }} returns a bindable {{ .Type }} value that is managed by the application preferences.
// Changes to this value will be saved to application storage and when the app starts the previous values will be read.
//
// Since: {{ .Since }}
func BindPreference{{ .Name }}(key string, p fyne.Preferences) {{ .Name }} {
	if prefBinds[p] != nil {
		if listen, ok := prefBinds[p][key]; ok {
			if l, ok := listen.({{ .Name }}); ok {
				return l
			}
			fyne.LogError(keyTypeMismatchError+key, nil)
		}
	}

	listen := &prefBound{{ .Name }}{key: key, p: p}
	ensurePreferencesAttached(p)
	prefBinds[p][key] = listen
	return listen
}

func (b *prefBound{{ .Name }}) Get() ({{ .Type }}, error) {
	b.cache = b.p.{{ .Name }}(b.key)
	return b.cache, nil
}

func (b *prefBound{{ .Name }}) Set(v {{ .Type }}) error {
	b.p.Set{{ .Name }}(b.key, v)

	b.lock.RLock()
	defer b.lock.RUnlock()
	b.trigger()
	return nil
}

func (b *prefBound{{ .Name }}) checkForChange() {
	if b.p.{{ .Name }}(b.key) == b.cache {
		return
	}

	b.trigger()
}
`

const toStringTemplate = `
type stringFrom{{ .Name }} struct {
	base
{{ if .Format }}
	format string
{{ end }}
	from {{ .Name }}
}

// {{ .Name }}ToString creates a binding that connects a {{ .Name }} data item to a String.
// Changes to the {{ .Name }} will be pushed to the String and setting the string will parse and set the
// {{ .Name }} if the parse was successful.
//
// Since: {{ .Since }}
func {{ .Name }}ToString(v {{ .Name }}) String {
{{- if .Format }}
	return {{ .Name }}ToStringWithFormat(v, "{{ .Format }}")
{{- else }}
	str := &stringFrom{{ .Name }}{from: v}
	v.AddListener(str)
	return str
{{- end }}
}
{{ if .Format }}
// {{ .Name }}ToStringWithFormat creates a binding that connects a {{ .Name }} data item to a String and is
// presented using the specified format. Changes to the {{ .Name }} will be pushed to the String and setting
// the string will parse and set the {{ .Name }} if the string matches the format and its parse was successful.
//
// Since: {{ .Since }}
func {{ .Name }}ToStringWithFormat(v {{ .Name }}, format string) String {
	str := &stringFrom{{ .Name }}{from: v, format: format}
	v.AddListener(str)
	return str
}
{{ end }}
func (s *stringFrom{{ .Name }}) Get() (string, error) {
	val, err := s.from.Get()
	if err != nil {
		return "", err
	}
{{ if .ToString }}
	return {{ .ToString }}(val)
{{- else }}
	return fmt.Sprintf(s.format, val), nil
{{- end }}
}

func (s *stringFrom{{ .Name }}) Set(str string) error {
{{- if .FromString }}
	val, err := {{ .FromString }}(str)
	if err != nil {
		return err
	}
{{ else }}
	var val {{ .Type }}
	n, err := fmt.Sscanf(str, s.format+" ", &val) // " " denotes match to end of string
	if err != nil {
		return err
	}
	if n != 1 {
		return errParseFailed
	}
{{ end }}
	old, err := s.from.Get()
	if err != nil {
		return err
	}
	if val == old {
		return nil
	}
	if err = s.from.Set(val); err != nil {
		return err
	}

	s.DataChanged()
	return nil
}

func (s *stringFrom{{ .Name }}) DataChanged() {
	s.lock.RLock()
	defer s.lock.RUnlock()
	s.trigger()
}
`

const fromStringTemplate = `
type stringTo{{ .Name }} struct {
	base
{{ if .Format }}
	format string
{{ end }}
	from String
}

// StringTo{{ .Name }} creates a binding that connects a String data item to a {{ .Name }}.
// Changes to the String will be parsed and pushed to the {{ .Name }} if the parse was successful, and setting
// the {{ .Name }} update the String binding.
//
// Since: {{ .Since }}
func StringTo{{ .Name }}(str String) {{ .Name }} {
{{- if .Format }}
	return StringTo{{ .Name }}WithFormat(str, "{{ .Format }}")
{{- else }}
	v := &stringTo{{ .Name }}{from: str}
	str.AddListener(v)
	return v
{{- end }}
}
{{ if .Format }}
// StringTo{{ .Name }}WithFormat creates a binding that connects a String data item to a {{ .Name }} and is
// presented using the specified format. Changes to the {{ .Name }} will be parsed and if the format matches and
// the parse is successful it will be pushed to the String. Setting the {{ .Name }} will push a formatted value
// into the String.
//
// Since: {{ .Since }}
func StringTo{{ .Name }}WithFormat(str String, format string) {{ .Name }} {
	v := &stringTo{{ .Name }}{from: str, format: format}
	str.AddListener(v)
	return v
}
{{ end }}
func (s *stringTo{{ .Name }}) Get() ({{ .Type }}, error) {
	str, err := s.from.Get()
	if str == "" || err != nil {
		return {{ .Default }}, err
	}
{{ if .FromString }}
	return {{ .FromString }}(str)
{{- else }}
	var val {{ .Type }}
	n, err := fmt.Sscanf(str, s.format+" ", &val) // " " denotes match to end of string
	if err != nil {
		return {{ .Default }}, err
	}
	if n != 1 {
		return {{ .Default }}, errParseFailed
	}

	return val, nil
{{- end }}
}

func (s *stringTo{{ .Name }}) Set(val {{ .Type }}) error {
{{- if .ToString }}
	str, err := {{ .ToString }}(val)
	if err != nil {
		return err
	}
{{- else }}
	str := fmt.Sprintf(s.format, val)
{{ end }}
	old, err := s.from.Get()
	if str == old {
		return err
	}

	if err = s.from.Set(str); err != nil {
		return err
	}

	s.DataChanged()
	return nil
}

func (s *stringTo{{ .Name }}) DataChanged() {
	s.lock.RLock()
	defer s.lock.RUnlock()
	s.trigger()
}
`

const listBindTemplate = `
// {{ .Name }}List supports binding a list of {{ .Type }} values.
//
// Since: {{ .Since }}
type {{ .Name }}List interface {
	DataList

	Append(val {{ .Type }}) error
	Get() ([]{{ .Type }}, error)
	GetValue(index int) ({{ .Type }}, error)
	Prepend(val {{ .Type }}) error
	Set(v []{{ .Type }}) error
	SetValue(index int, val {{ .Type }}) error
}

// External{{ .Name }}List supports binding a list of {{ .Type }} values from an external variable.
//
// Since: {{ .Since }}
type External{{ .Name }}List interface {
	{{ .Name }}List

	Reload() error
}

// New{{ .Name }}List returns a bindable list of {{ .Type }} values.
//
// Since: {{ .Since }}
func New{{ .Name }}List() {{ .Name }}List {
	return &bound{{ .Name }}List{val: &[]{{ .Type }}{}}
}

// Bind{{ .Name }}List returns a bound list of {{ .Type }} values, based on the contents of the passed slice.
// If your code changes the content of the slice this refers to you should call Reload() to inform the bindings.
//
// Since: {{ .Since }}
func Bind{{ .Name }}List(v *[]{{ .Type }}) External{{ .Name }}List {
	if v == nil {
		return New{{ .Name }}List().(External{{ .Name }}List)
	}

	b := &bound{{ .Name }}List{val: v, updateExternal: true}

	for i := range *v {
		b.appendItem(bind{{ .Name }}ListItem(v, i, b.updateExternal))
	}

	return b
}

type bound{{ .Name }}List struct {
	listBase

	updateExternal bool
	val            *[]{{ .Type }}
}

func (l *bound{{ .Name }}List) Append(val {{ .Type }}) error {
	l.lock.Lock()
	defer l.lock.Unlock()

	*l.val = append(*l.val, val)

	return l.doReload()
}

func (l *bound{{ .Name }}List) Get() ([]{{ .Type }}, error) {
	l.lock.RLock()
	defer l.lock.RUnlock()

	return *l.val, nil
}

func (l *bound{{ .Name }}List) GetValue(i int) ({{ .Type }}, error) {
	if i < 0 || i >= l.Length() {
		return {{ .Default }}, errOutOfBounds
	}
	l.lock.RLock()
	defer l.lock.RUnlock()

	return (*l.val)[i], nil
}

func (l *bound{{ .Name }}List) Prepend(val {{ .Type }}) error {
	l.lock.Lock()
	defer l.lock.Unlock()
	*l.val = append([]{{ .Type }}{val}, *l.val...)

	return l.doReload()
}

func (l *bound{{ .Name }}List) Reload() error {
	l.lock.Lock()
	defer l.lock.Unlock()

	return l.doReload()
}

func (l *bound{{ .Name }}List) Set(v []{{ .Type }}) error {
	l.lock.Lock()
	defer l.lock.Unlock()
	*l.val = v

	return l.doReload()
}

func (l *bound{{ .Name }}List) doReload() (retErr error) {
	oldLen := len(l.items)
	newLen := len(*l.val)
	if oldLen > newLen {
		for i := oldLen - 1; i >= newLen; i-- {
			l.deleteItem(i)
		}
		l.trigger()
	} else if oldLen < newLen {
		for i := oldLen; i < newLen; i++ {
			l.appendItem(bind{{ .Name }}ListItem(l.val, i, l.updateExternal))
		}
		l.trigger()
	}

	for i, item := range l.items {
		if i > oldLen || i > newLen {
			break
		}

		var err error
		if l.updateExternal {
			item.(*boundExternal{{ .Name }}ListItem).lock.Lock()
			err = item.(*boundExternal{{ .Name }}ListItem).setIfChanged((*l.val)[i])
			item.(*boundExternal{{ .Name }}ListItem).lock.Unlock()
		} else {
			item.(*bound{{ .Name }}ListItem).lock.Lock()
			err = item.(*bound{{ .Name }}ListItem).doSet((*l.val)[i])
			item.(*bound{{ .Name }}ListItem).lock.Unlock()
		}
		if err != nil {
			retErr = err
		}
	}
	return
}

func (l *bound{{ .Name }}List) SetValue(i int, v {{ .Type }}) error {
	if i < 0 || i >= l.Length() {
		return errOutOfBounds
	}

	l.lock.Lock()
	(*l.val)[i] = v
	l.lock.Unlock()

	item, err := l.GetItem(i)
	if err != nil {
		return err
	}
	return item.({{ .Name }}).Set(v)
}

func bind{{ .Name }}ListItem(v *[]{{ .Type }}, i int, external bool) {{ .Name }} {
	if external {
		ret := &boundExternal{{ .Name }}ListItem{old: (*v)[i]}
		ret.val = v
		ret.index = i
		return ret
	}

	return &bound{{ .Name }}ListItem{val: v, index: i}
}

type bound{{ .Name }}ListItem struct {
	base

	val   *[]{{ .Type }}
	index int
}

func (b *bound{{ .Name }}ListItem) Get() ({{ .Type }}, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	return (*b.val)[b.index], nil
}

func (b *bound{{ .Name }}ListItem) Set(val {{ .Type }}) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	return b.doSet(val)
}

func (b *bound{{ .Name }}ListItem) doSet(val {{ .Type }}) error {
	(*b.val)[b.index] = val

	b.trigger()
	return nil
}

type boundExternal{{ .Name }}ListItem struct {
	bound{{ .Name }}ListItem

	old {{ .Type }}
}

func (b *boundExternal{{ .Name }}ListItem) setIfChanged(val {{ .Type }}) error {
	if val == b.old {
		return nil
	}
	(*b.val)[b.index] = val
	b.old = val

	b.trigger()
	return nil
}
`

type bindValues struct {
	Name, Type, Default  string
	Format, Since        string
	SupportsPreferences  bool
	FromString, ToString string // function names...
}

func newFile(name string) (*os.File, error) {
	_, dirname, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(dirname), name+".go")
	os.Remove(filepath)
	f, err := os.Create(filepath)
	if err != nil {
		fyne.LogError("Unable to open file "+f.Name(), err)
		return nil, err
	}

	f.WriteString(`// auto-generated
// **** THIS FILE IS AUTO-GENERATED, PLEASE DO NOT EDIT IT **** //

package binding
`)
	return f, nil
}

func writeFile(f *os.File, t *template.Template, d interface{}) {
	if err := t.Execute(f, d); err != nil {
		fyne.LogError("Unable to write file "+f.Name(), err)
	}
}

func main() {
	itemFile, err := newFile("binditems")
	if err != nil {
		return
	}
	defer itemFile.Close()
	itemFile.WriteString(`
import "fyne.io/fyne/v2"
`)
	convertFile, err := newFile("convert")
	if err != nil {
		return
	}
	defer convertFile.Close()
	convertFile.WriteString(`
import (
	"fmt"

	"fyne.io/fyne/v2"
)
`)
	prefFile, err := newFile("preference")
	if err != nil {
		return
	}
	defer prefFile.Close()
	prefFile.WriteString(`
import "fyne.io/fyne/v2"

const keyTypeMismatchError = "A previous preference binding exists with different type for key: "
`)

	listFile, err := newFile("bindlists")
	if err != nil {
		return
	}
	defer listFile.Close()
	listFile.WriteString(`
import "fyne.io/fyne/v2"
`)

	item := template.Must(template.New("item").Parse(itemBindTemplate))
	fromString := template.Must(template.New("fromString").Parse(fromStringTemplate))
	toString := template.Must(template.New("toString").Parse(toStringTemplate))
	preference := template.Must(template.New("preference").Parse(prefTemplate))
	list := template.Must(template.New("list").Parse(listBindTemplate))
	binds := []bindValues{
		bindValues{Name: "Bool", Type: "bool", Default: "false", Format: "%t", SupportsPreferences: true},
		bindValues{Name: "Float", Type: "float64", Default: "0.0", Format: "%f", SupportsPreferences: true},
		bindValues{Name: "Int", Type: "int", Default: "0", Format: "%d", SupportsPreferences: true},
		bindValues{Name: "Rune", Type: "rune", Default: "rune(0)"},
		bindValues{Name: "String", Type: "string", Default: "\"\"", SupportsPreferences: true},
		bindValues{Name: "URI", Type: "fyne.URI", Default: "fyne.URI(nil)", Since: "2.1",
			FromString: "uriFromString", ToString: "uriToString"},
	}
	for _, b := range binds {
		if b.Since == "" {
			b.Since = "2.0"
		}

		writeFile(itemFile, item, b)
		if b.SupportsPreferences {
			writeFile(prefFile, preference, b)
		}
		if b.Format != "" || b.ToString != "" {
			writeFile(convertFile, toString, b)
		}
		writeFile(listFile, list, b)
	}
	// add StringTo... at the bottom of the convertFile for correct ordering
	for _, b := range binds {
		if b.Since == "" {
			b.Since = "2.0"
		}

		if b.Format != "" || b.FromString != "" {
			writeFile(convertFile, fromString, b)
		}
	}
}
