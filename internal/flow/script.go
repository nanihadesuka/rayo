package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/dop251/goja"
	"gopkg.in/yaml.v3"
)

// script is the JavaScript of one flow: ${...}, runScript and evalScript.
type script struct {
	ctx context.Context // of the flow's run
	vm  *goja.Runtime
}

func newScript(ctx context.Context, log func(text string)) *script {
	s := &script{ctx: ctx, vm: goja.New()}
	vm := s.vm
	// Setting a plain name on an ordinary object can't fail.
	set := func(o interface{ Set(string, any) error }, name string, value any) { _ = o.Set(name, value) }
	set(vm, "output", vm.NewObject())
	tool := vm.NewObject()
	set(tool, "platform", "android")
	set(tool, "copiedText", "")
	set(vm, "rayo", tool)
	console := vm.NewObject()
	set(console, "log", func(c goja.FunctionCall) goja.Value {
		var parts []string
		for _, a := range c.Arguments {
			parts = append(parts, a.String())
		}
		log(strings.Join(parts, " "))
		return goja.Undefined()
	})
	set(vm, "console", console)
	set(vm, "json", func(text string) goja.Value {
		var v any
		if err := json.Unmarshal([]byte(text), &v); err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(v)
	})

	client := vm.NewObject()
	for _, m := range []string{"get", "post", "put", "delete"} {
		method := strings.ToUpper(m)
		set(client, m, func(url string, options goja.Value) goja.Value { return s.http(method, url, options) })
	}
	set(client, "request", func(url string, options goja.Value) goja.Value { return s.http("", url, options) })
	set(vm, "http", client)
	// What ${...} and conditions run in: a name that isn't set is undefined there, as in Maestro, so
	// ${ X || "default" } works whoever set X, or nobody.
	if _, err := vm.RunString(`Object.defineProperty(globalThis, "` + scope + `", {value: new Proxy(globalThis, {has: (t, k) => typeof k === "string"})})`); err != nil {
		panic(err) // the same code each time
	}
	return s
}

func (s *script) http(method, url string, options goja.Value) goja.Value {
	headers := map[string]string{}
	body := ""
	if options != nil && !goja.IsUndefined(options) && !goja.IsNull(options) {
		o := options.ToObject(s.vm)
		if v := o.Get("headers"); v != nil && !goja.IsUndefined(v) {
			if err := s.vm.ExportTo(v, &headers); err != nil {
				panic(s.vm.NewGoError(err))
			}
		}
		if v := o.Get("body"); v != nil && !goja.IsUndefined(v) {
			body = v.String()
		}
		if v := o.Get("method"); v != nil && !goja.IsUndefined(v) && method == "" {
			method = v.String()
		}
	}
	status, got, err := fetch(s.ctx, method, url, headers, body, 30*time.Second)
	if err != nil {
		panic(s.vm.NewGoError(err))
	}
	r := s.vm.NewObject()
	_ = r.Set("ok", status >= 200 && status < 300)
	_ = r.Set("status", status)
	_ = r.Set("body", got)
	return r
}

func fetch(ctx context.Context, method, url string, headers map[string]string, body string, limit time.Duration) (int, string, error) {
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), url, strings.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := http.Client{Timeout: limit}
	res, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 10<<20))
	return res.StatusCode, string(data), err
}

// with sets variables while fn runs.
func (s *script) with(env map[string]string, fn func() error) (err error) {
	old := map[string]goja.Value{}
	defer func() {
		for k, v := range old {
			var e error
			if v == nil {
				e = s.vm.GlobalObject().Delete(k)
			} else {
				e = s.vm.Set(k, v)
			}
			err = errors.Join(err, e)
		}
	}()
	for k, v := range env {
		old[k] = s.vm.Get(k)
		if err := s.vm.Set(k, v); err != nil {
			return fmt.Errorf("variable %s: %w", k, err)
		}
	}
	return fn()
}

// header evaluates the values of a file's env, where it is included: each ${...} sees the variables
// set outside, and one that isn't set is undefined, so ${ X || "default" } works. A name that is set
// already is left out, as the header only gives defaults.
func (s *script) header(env map[string]string) (map[string]string, error) {
	declared := []string{}
	for k := range env {
		if !s.defined(k) {
			if err := s.vm.Set(k, goja.Undefined()); err != nil {
				return nil, err
			}
			declared = append(declared, k)
		}
	}
	defer func() {
		for _, k := range declared {
			_ = s.vm.GlobalObject().Delete(k)
		}
	}()
	values := map[string]string{}
	for _, k := range declared {
		v, err := s.fill(env[k])
		if err != nil {
			return nil, fmt.Errorf("env %s: %w", k, err)
		}
		values[k] = v
	}
	return values, nil
}

func (s *script) defined(name string) bool {
	v := s.vm.Get(name)
	return v != nil && !goja.IsUndefined(v)
}

func (s *script) run(file string) error {
	code, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	// In a function, so that two scripts can use the same names.
	_, err = s.eval(file, "(function(){"+string(code)+"\n})()")
	return err
}

const scriptTimeout = 60 * time.Second

const scope = "__rayo_scope__"

// expression evaluates code where a name that isn't set is undefined.
func (s *script) expression(name, code string) (goja.Value, error) {
	return s.eval(name, "with ("+scope+") {\n"+code+"\n}")
}

func (s *script) eval(name, code string) (goja.Value, error) {
	ctx, cancel := context.WithTimeoutCause(s.ctx, scriptTimeout, errors.New("the script took over 60 s"))
	defer cancel()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		s.vm.Interrupt(context.Cause(ctx))
		close(interrupted)
	})
	v, err := s.vm.RunScript(name, code)
	if !stop() {
		<-interrupted // or it could hit the next script
	}
	s.vm.ClearInterrupt()
	if e, ok := err.(*goja.Exception); ok {
		err = fmt.Errorf("%s", e.Value())
	}
	return v, err
}

var expression = regexp.MustCompile(`\$\{([^{}]*(?:\{[^{}]*\}[^{}]*)*)\}`)

// fill replaces each ${...} with its value.
func (s *script) fill(text string) (string, error) {
	var failed error
	result := expression.ReplaceAllStringFunc(text, func(m string) string {
		code := expression.FindStringSubmatch(m)[1]
		v, err := s.expression("expression", code)
		if err == nil && goja.IsUndefined(v) {
			err = fmt.Errorf("it is undefined")
		}
		if err != nil {
			failed = fmt.Errorf("${%s}: %w", code, err)
			return m
		}
		return v.String()
	})
	return result, failed
}

// resolve copies a value with each ${...} replaced.
func (s *script) resolve(n *yaml.Node) (*yaml.Node, error) {
	if n == nil {
		return nil, nil
	}
	c := *n
	if n.Kind == yaml.ScalarNode {
		v, err := s.fill(n.Value)
		c.Value = v
		return &c, err
	}
	c.Content = make([]*yaml.Node, len(n.Content))
	for i, child := range n.Content {
		// Inner commands are resolved when they run, and conditions and scripts are code already.
		if n.Kind == yaml.MappingNode && i%2 == 1 &&
			lateKeys[n.Content[i-1].Value] {
			c.Content[i] = child
			continue
		}
		r, err := s.resolve(child)
		if err != nil {
			return nil, err
		}
		c.Content[i] = r
	}
	return &c, nil
}

func code(text string) string {
	text = strings.TrimSpace(text)
	if m := expression.FindStringSubmatch(text); m != nil && m[0] == text {
		return m[1]
	}
	return text
}

// truthy evaluates "${a == b}" or "a == b".
func (s *script) truthy(text string) (bool, error) {
	v, err := s.expression("condition", code(text))
	if err != nil {
		return false, err
	}
	if t, ok := v.Export().(string); ok {
		return t != "" && t != "false" && t != "0", nil
	}
	return v.ToBoolean(), nil
}

func runScript(c *call) error {
	if when := arg(c, keyWhen); when != nil {
		if ok, err := c.r.holds(c.ctx, when); err != nil || !ok {
			return err
		}
	}
	return c.r.script.with(arg(c, keyEnv), func() error {
		return c.r.script.run(relative(c.s.Path, arg(c, keyFile)))
	})
}

func httpStep(c *call) error {
	body, headers := "", map[string]string{}
	if n := arg(c, keyJSON); n != nil {
		var value any
		if err := n.Decode(&value); err != nil {
			return err
		}
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		body, headers["Content-Type"] = string(data), "application/json"
	}
	status, got, err := fetch(c.ctx, arg(c, keyMethod), arg(c, keyURL), headers, body, c.limit)
	if err != nil {
		return err
	}
	expect := arg(c, keyExpect)
	if (expect != 0 && status != expect) || (expect == 0 && status >= 300) {
		return fmt.Errorf("%d: %.300s", status, got)
	}
	return nil
}
