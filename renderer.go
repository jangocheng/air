package air

import (
	"errors"
	"html/template"
	"io"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// Renderer is used to provide a `Render()` method for an `Air` instance
// for renders a "text/html" response by using `template.Template`.
type Renderer struct {
	goTemplate *template.Template
	air        *Air

	TemplateFuncMap template.FuncMap
}

// defaultTemplateFuncMap is a default template func map of `Renderer`
var defaultTemplateFuncMap template.FuncMap

func init() {
	tfm := make(template.FuncMap)

	tfm["strlen"] = strlen
	tfm["substr"] = substr
	tfm["str2html"] = str2html
	tfm["html2str"] = html2str
	tfm["datefmt"] = datefmt
	tfm["eq"] = eq
	tfm["ne"] = ne
	tfm["lt"] = lt
	tfm["le"] = le
	tfm["gt"] = gt
	tfm["ge"] = ge

	defaultTemplateFuncMap = tfm
}

// NewRenderer returns a new instance of `Renderer`.
func NewRenderer(a *Air) *Renderer {
	return &Renderer{
		air:             a,
		TemplateFuncMap: defaultTemplateFuncMap,
	}
}

// parseTemplates parses files into templates.
//
// e.g. Config#TemplatesPath == "templates"
//
// templates/
//   index.html
//   login.html
//   register.html
//
// templates/parts/
//   header.html
//   footer.html
//
// will be parsed into:
//
// "index.html", "login.html", "register.html",
// "parts/header.html", "parts/footer.html".
func (r *Renderer) parseTemplates() {
	replace := strings.NewReplacer("\\", "/")
	tp := strings.TrimRight(replace.Replace(r.air.Config.TemplatesPath), "/")
	filenames, err := filepath.Glob(tp + "/*/*.html")
	if err != nil {
		panic(err)
	}
	for _, filename := range filenames {
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			panic(err)
		}
		s := string(b)
		name := filename[len(tp)+1:]
		var tmpl *template.Template
		if r.goTemplate == nil {
			r.goTemplate = template.New(name).Funcs(r.TemplateFuncMap)
		}
		if name == r.goTemplate.Name() {
			tmpl = r.goTemplate
		} else {
			tmpl = r.goTemplate.New(name)
		}
		_, err = tmpl.Parse(s)
		if err != nil {
			panic(err)
		}
	}
}

// Render renders a "text/html" response by using `template.Template`
func (r *Renderer) Render(wr io.Writer, tplName string, c *Context) error {
	return r.goTemplate.ExecuteTemplate(wr, tplName, c.Data)
}

// Basic type kind
type typeKind int

const (
	invalidKind typeKind = iota
	intKind
	uintKind
	floatKind
	complexKind
	boolKind
	stringKind
)

// Template func error
var (
	errBadComparisonType = errors.New("Invalid Type For Comparison")
	errBadComparison     = errors.New("Incompatible Types For Comparison")
	errNoComparison      = errors.New("Missing Argument For Comparison")
)

// strlen returns the number of characters in s.
func strlen(s string) int {
	return len([]rune(s))
}

// substr returns the substring from start to length.
func substr(s string, start, length int) string {
	bt := []rune(s)
	if start < 0 {
		start = 0
	}
	if start > len(bt) {
		start = start % len(bt)
	}
	var end int
	if (start + length) > (len(bt) - 1) {
		end = len(bt)
	} else {
		end = start + length
	}
	return string(bt[start:end])
}

// html2str returns escaping text convert from html.
func html2str(html string) string {
	src := string(html)

	re, _ := regexp.Compile("\\<[\\S\\s]+?\\>")
	src = re.ReplaceAllStringFunc(src, strings.ToLower)

	//remove STYLE
	re, _ = regexp.Compile("\\<style[\\S\\s]+?\\</style\\>")
	src = re.ReplaceAllString(src, "")

	//remove SCRIPT
	re, _ = regexp.Compile("\\<script[\\S\\s]+?\\</script\\>")
	src = re.ReplaceAllString(src, "")

	re, _ = regexp.Compile("\\<[\\S\\s]+?\\>")
	src = re.ReplaceAllString(src, "\n")

	re, _ = regexp.Compile("\\s{2,}")
	src = re.ReplaceAllString(src, "\n")

	return strings.TrimSpace(src)
}

// str2html returns the `template.HTML` convert from raw.
func str2html(raw string) template.HTML {
	return template.HTML(raw)
}

// datefmt takes a time and a layout string and returns a string with the formatted date.
func datefmt(t time.Time, layout string) string {
	return t.Format(layout)
}

// basicKind returns a basic type kind with a provided `reflect.Value`.
func basicKind(v reflect.Value) (typeKind, error) {
	switch v.Kind() {
	case reflect.Bool:
		return boolKind, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intKind, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintKind, nil
	case reflect.Float32, reflect.Float64:
		return floatKind, nil
	case reflect.Complex64, reflect.Complex128:
		return complexKind, nil
	case reflect.String:
		return stringKind, nil
	}
	return invalidKind, errBadComparisonType
}

// eq evaluates the comparison a == b || a == c || ...
func eq(arg1 interface{}, arg2 ...interface{}) (bool, error) {
	v1 := reflect.ValueOf(arg1)
	k1, err := basicKind(v1)
	if err != nil {
		return false, err
	}
	if len(arg2) == 0 {
		return false, errNoComparison
	}
	for _, arg := range arg2 {
		v2 := reflect.ValueOf(arg)
		k2, err := basicKind(v2)
		if err != nil {
			return false, err
		}
		if k1 != k2 {
			return false, errBadComparison
		}
		truth := false
		switch k1 {
		case boolKind:
			truth = v1.Bool() == v2.Bool()
		case complexKind:
			truth = v1.Complex() == v2.Complex()
		case floatKind:
			truth = v1.Float() == v2.Float()
		case intKind:
			truth = v1.Int() == v2.Int()
		case stringKind:
			truth = v1.String() == v2.String()
		case uintKind:
			truth = v1.Uint() == v2.Uint()
		default:
			panic("Invalid Kind")
		}
		if truth {
			return true, nil
		}
	}
	return false, nil
}

// ne evaluates the comparison a != b && a != c && ...
func ne(arg1 interface{}, arg2 ...interface{}) (bool, error) {
	// != is the inverse of ==.
	equal, err := eq(arg1, arg2)
	return !equal, err
}

// lt evaluates the comparison a < b.
func lt(arg1, arg2 interface{}) (bool, error) {
	v1 := reflect.ValueOf(arg1)
	k1, err := basicKind(v1)
	if err != nil {
		return false, err
	}
	v2 := reflect.ValueOf(arg2)
	k2, err := basicKind(v2)
	if err != nil {
		return false, err
	}
	if k1 != k2 {
		return false, errBadComparison
	}
	truth := false
	switch k1 {
	case boolKind, complexKind:
		return false, errBadComparisonType
	case floatKind:
		truth = v1.Float() < v2.Float()
	case intKind:
		truth = v1.Int() < v2.Int()
	case stringKind:
		truth = v1.String() < v2.String()
	case uintKind:
		truth = v1.Uint() < v2.Uint()
	default:
		panic("Invalid Kind")
	}
	return truth, nil
}

// le evaluates the comparison <= b.
func le(arg1, arg2 interface{}) (bool, error) {
	// <= is < or ==.
	lessThan, err := lt(arg1, arg2)
	if lessThan || err != nil {
		return lessThan, err
	}
	return eq(arg1, arg2)
}

// gt evaluates the comparison a > b.
func gt(arg1, arg2 interface{}) (bool, error) {
	// > is the inverse of <=.
	lessOrEqual, err := le(arg1, arg2)
	if err != nil {
		return false, err
	}
	return !lessOrEqual, nil
}

// ge evaluates the comparison a >= b.
func ge(arg1, arg2 interface{}) (bool, error) {
	// >= is the inverse of <.
	lessThan, err := lt(arg1, arg2)
	if err != nil {
		return false, err
	}
	return !lessThan, nil
}