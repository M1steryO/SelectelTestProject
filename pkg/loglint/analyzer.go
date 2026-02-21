package loglint

import (
	"flag"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	diagLowercase = "log message should start with a lowercase letter"
	diagEnglish   = "log message should be English-only"
	diagSpecial   = "log message should not contain punctuation/symbols/emoji"
	diagSensitive = "log message construction may leak sensitive data"
)

func NewAnalyzer(cfg Settings) *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name:     "loglint",
		Doc:      "checks slog/zap log messages for style and safety rules",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}

	// Standalone mode supports an optional config file flag.
	// In golangci-lint plugin mode, settings are typically provided via .golangci.yml.
	a.Flags = *flag.NewFlagSet("loglint", flag.ContinueOnError)
	configPath := a.Flags.String("config", "", "path to YAML/JSON config file for loglint settings (optional)")

	a.Run = func(pass *analysis.Pass) (any, error) {
		cfgRun := cfg
		if configPath != nil && *configPath != "" {
			loaded, err := LoadSettingsFromFile(cfgRun, *configPath)
			if err != nil {
				return nil, err
			}
			cfgRun = loaded
		}

		insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
		nodeFilter := []ast.Node{(*ast.CallExpr)(nil)}
		insp.Preorder(nodeFilter, func(n ast.Node) {
			call := n.(*ast.CallExpr)
			msgExpr, ok := extractMessageExpr(pass, call)
			if !ok {
				return
			}
			analyzeMessage(pass, msgExpr, cfgRun)
		})
		return nil, nil
	}

	return a
}

type logCall struct {
	fullName string
	msgIndex int
}

func extractMessageExpr(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, bool) {
	lc, ok := classifyLogCall(pass, call)
	if !ok {
		return nil, false
	}
	if lc.msgIndex < 0 || lc.msgIndex >= len(call.Args) {
		return nil, false
	}
	return call.Args[lc.msgIndex], true
}

func classifyLogCall(pass *analysis.Pass, call *ast.CallExpr) (logCall, bool) {
	fullName, ok := calleeFullName(pass, call)
	if !ok {
		return logCall{}, false
	}

	// log/slog
	if strings.HasPrefix(fullName, "log/slog.") || strings.HasPrefix(fullName, "log/slog.(*Logger).") {
		method := shortMethodName(fullName)
		idx := 0
		if strings.HasSuffix(method, "Context") {
			idx = 1
			method = strings.TrimSuffix(method, "Context")
		}
		switch method {
		case "Debug", "Info", "Warn", "Error":
			return logCall{fullName: fullName, msgIndex: idx}, true
		}
	}

	// go.uber.org/zap
	if strings.HasPrefix(fullName, "go.uber.org/zap.(*Logger).") || strings.HasPrefix(fullName, "go.uber.org/zap.(*SugaredLogger).") {
		method := shortMethodName(fullName)
		switch method {
		case "Debug", "Info", "Warn", "Error", "DPanic", "Panic", "Fatal",
			"Debugf", "Infof", "Warnf", "Errorf",
			"Debugw", "Infow", "Warnw", "Errorw":
			return logCall{fullName: fullName, msgIndex: 0}, true
		}
	}

	return logCall{}, false
}

func calleeFullName(pass *analysis.Pass, call *ast.CallExpr) (string, bool) {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		if sel := pass.TypesInfo.Selections[fn]; sel != nil {
			if f, ok := sel.Obj().(*types.Func); ok {
				return f.FullName(), true
			}
		}
		if obj, ok := pass.TypesInfo.Uses[fn.Sel].(*types.Func); ok {
			return obj.FullName(), true
		}
	case *ast.Ident:
		if obj, ok := pass.TypesInfo.Uses[fn].(*types.Func); ok {
			return obj.FullName(), true
		}
	}
	return "", false
}

func shortMethodName(fullName string) string {
	// examples:
	// log/slog.(*Logger).InfoContext -> InfoContext
	// go.uber.org/zap.(*Logger).Info -> Info
	if i := strings.LastIndex(fullName, ")."); i >= 0 {
		return fullName[i+2:]
	}
	if i := strings.LastIndex(fullName, "."); i >= 0 {
		return fullName[i+1:]
	}
	return fullName
}

func analyzeMessage(pass *analysis.Pass, msgExpr ast.Expr, cfg Settings) {
	// We try to get:
	// 1) constant message string (literal/const) => apply style rules
	// 2) string literal parts + identifier/selector names => apply sensitive checks even for dynamic messages
	constMsg, isConst := constString(pass, msgExpr)
	parts, idents := collectMessagePartsAndIdents(msgExpr)

	if cfg.Rules.LowercaseStart {
		if s := firstNonEmpty(parts, constMsg, isConst); s != "" {
			trim := strings.TrimLeftFunc(s, unicode.IsSpace)
			if trim != "" {
				r, _ := firstRune(trim)
				if r >= 'A' && r <= 'Z' {
					d := analysis.Diagnostic{
						Pos:     msgExpr.Pos(),
						End:     msgExpr.End(),
						Message: diagLowercase,
					}
					if lit, ok := msgExpr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if fixed, okFix := fixLowercaseStart(lit); okFix {
							d.SuggestedFixes = []analysis.SuggestedFix{fixed}
						}
					}
					pass.Report(d)
				}
			}
		}
	}

	if cfg.Rules.EnglishOnly {
		if s := firstNonEmpty(parts, constMsg, isConst); s != "" {
			if containsNonASCII(s) {
				d := analysis.Diagnostic{Pos: msgExpr.Pos(), End: msgExpr.End(), Message: diagEnglish}
				if lit, ok := msgExpr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if fix, okFix := fixStripNonASCII(lit); okFix {
						d.SuggestedFixes = []analysis.SuggestedFix{fix}
					}
				}
				pass.Report(d)
			}
		}
	}

	if cfg.Rules.NoSpecial {
		if s := firstNonEmpty(parts, constMsg, isConst); s != "" {
			if containsDisallowedSymbols(s, cfg.Allowed.AllowPunct) {
				d := analysis.Diagnostic{Pos: msgExpr.Pos(), End: msgExpr.End(), Message: diagSpecial}
				if lit, ok := msgExpr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if fix, okFix := fixStripNonASCII(lit); okFix {
						d.SuggestedFixes = []analysis.SuggestedFix{fix}
					}
				}
				pass.Report(d)
			}
		}
	}

	if cfg.Rules.NoSensitive {
		if mayLeakSensitive(msgExpr, parts, idents, cfg.Sensitive) {
			pass.Reportf(msgExpr.Pos(), diagSensitive)
		}
	}
}

func constString(pass *analysis.Pass, expr ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[expr]
	if !ok || tv.Value == nil {
		return "", false
	}
	if tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

func collectMessagePartsAndIdents(expr ast.Expr) ([]string, []string) {
	var parts []string
	idents := make(map[string]struct{})

	var walk func(e ast.Expr)
	walk = func(e ast.Expr) {
		switch x := e.(type) {
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				if s, err := strconv.Unquote(x.Value); err == nil {
					parts = append(parts, s)
				}
			}
		case *ast.BinaryExpr:
			if x.Op == token.ADD {
				walk(x.X)
				walk(x.Y)
			}
		case *ast.CallExpr:
			// Handle fmt.Sprintf("...", ...)
			if isFmtSprintf(x) && len(x.Args) > 0 {
				walk(x.Args[0])
				for i := 1; i < len(x.Args); i++ {
					walk(x.Args[i])
				}
				return
			}
			for _, a := range x.Args {
				walk(a)
			}
		case *ast.SelectorExpr:
			// collect field name, e.g. user.Password
			if x.Sel != nil && x.Sel.Name != "" {
				idents[strings.ToLower(x.Sel.Name)] = struct{}{}
			}
			walk(x.X)
		case *ast.Ident:
			if x.Name != "" && x.Name != "_" {
				idents[strings.ToLower(x.Name)] = struct{}{}
			}
		case *ast.UnaryExpr:
			walk(x.X)
		case *ast.ParenExpr:
			walk(x.X)
		case *ast.IndexExpr:
			walk(x.X)
			walk(x.Index)
		case *ast.IndexListExpr:
			walk(x.X)
			for _, idx := range x.Indices {
				walk(idx)
			}
		case *ast.SliceExpr:
			walk(x.X)
			if x.Low != nil {
				walk(x.Low)
			}
			if x.High != nil {
				walk(x.High)
			}
			if x.Max != nil {
				walk(x.Max)
			}
		default:
			// ignore other nodes
		}
	}

	walk(expr)

	var idList []string
	for k := range idents {
		idList = append(idList, k)
	}
	return parts, idList
}

func isFmtSprintf(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != "Sprintf" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkg.Name == "fmt"
}

func firstNonEmpty(parts []string, constMsg string, isConst bool) string {
	if isConst && constMsg != "" {
		return constMsg
	}
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			return p
		}
	}
	return ""
}

func firstRune(s string) (rune, int) {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && size == 1 {
		return 0, 0
	}
	return r, size
}

func containsNonASCII(s string) bool {
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' {
			continue
		}
		if r > 127 { // any non-ascii char => non-English in our strict interpretation
			return true
		}
	}
	return false
}

func containsDisallowedSymbols(s string, allowPunct bool) bool {
	_ = allowPunct // оставляем для совместимости с текущими вызовами
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' || unicode.IsSpace(r) {
			continue
		}
		// "спецсимволы/эмодзи" = любой non-ASCII
		if r > 127 {
			return true
		}
		// ASCII пунктуацию/символы НЕ баним
	}
	return false
}

func mayLeakSensitive(expr ast.Expr, parts []string, idents []string, cfg SensitiveSettings) bool {
	keywords := normalizeKeywords(cfg.Keywords)
	if len(keywords) == 0 {
		return false
	}

	// Is this message expression "dynamic" (i.e., likely to include runtime values)?
	dynamic := !isStaticString(expr)

	// Check identifiers/selector names always — logging `password` variable is suspicious.
	for _, id := range idents {
		if matchesKeyword(id, keywords) {
			return true
		}
	}

	// For static literals, we only check textual keywords if CheckLiterals is enabled.
	if !dynamic && !cfg.CheckLiterals {
		return false
	}

	for _, p := range parts {
		lp := strings.ToLower(p)
		if matchesKeyword(lp, keywords) {
			return true
		}
		// key-like patterns even without keyword list precision (e.g. "token:" "api_key=")
		if looksLikeSecretLabel(lp) {
			return true
		}
	}
	return false
}

func isStaticString(expr ast.Expr) bool {
	// Consider it static if it is a literal string.
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	default:
		return false
	}
}

func normalizeKeywords(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, k := range in {
		k = strings.TrimSpace(strings.ToLower(k))
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func matchesKeyword(text string, keywords []string) bool {
	for _, k := range keywords {
		if strings.Contains(text, k) {
			return true
		}
	}
	return false
}

func looksLikeSecretLabel(s string) bool {
	// Heuristic: "password:", "token=", "api_key:" etc.
	labels := []string{"password", "passwd", "pwd", "token", "api_key", "apikey", "secret", "private_key", "authorization", "bearer"}
	for _, l := range labels {
		if strings.Contains(s, l+":") || strings.Contains(s, l+"=") {
			return true
		}
	}
	return false
}

func fixStripNonASCII(lit *ast.BasicLit) (analysis.SuggestedFix, bool) {
	orig, err := strconv.Unquote(lit.Value)
	if err != nil {
		return analysis.SuggestedFix{}, false
	}

	var b strings.Builder
	b.Grow(len(orig))

	changed := false
	for _, r := range orig {
		// Preserve ASCII whitespace and printable ASCII.
		if r <= 127 {
			b.WriteRune(r)
			continue
		}
		changed = true
		// drop non-ASCII rune
	}
	if !changed {
		return analysis.SuggestedFix{}, false
	}
	newLit := strconv.Quote(b.String())
	return analysis.SuggestedFix{
		Message: "remove non-ASCII characters",
		TextEdits: []analysis.TextEdit{
			{Pos: lit.Pos(), End: lit.End(), NewText: []byte(newLit)},
		},
	}, true
}

func fixLowercaseStart(lit *ast.BasicLit) (analysis.SuggestedFix, bool) {
	orig, err := strconv.Unquote(lit.Value)
	if err != nil {
		return analysis.SuggestedFix{}, false
	}
	trimLeft := strings.TrimLeftFunc(orig, unicode.IsSpace)
	if trimLeft == "" {
		return analysis.SuggestedFix{}, false
	}

	// Find byte index of first non-space rune in original string.
	idx := 0
	found := false
	for i, r := range orig {
		if !unicode.IsSpace(r) {
			idx = i
			found = true
			break
		}
	}
	if !found {
		return analysis.SuggestedFix{}, false
	}

	r, size := firstRune(orig[idx:])
	if r < 'A' || r > 'Z' || size == 0 {
		return analysis.SuggestedFix{}, false
	}

	fixed := orig[:idx] + strings.ToLower(string(r)) + orig[idx+size:]
	newLit := strconv.Quote(fixed)

	return analysis.SuggestedFix{
		Message: "lowercase first letter",
		TextEdits: []analysis.TextEdit{
			{
				Pos:     lit.Pos(),
				End:     lit.End(),
				NewText: []byte(newLit),
			},
		},
	}, true
}
