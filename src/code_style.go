package main

import (
	"bufio"
	"fmt"
	"os"
	"bytes"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
	"path/filepath"
	"sort"
)

var styleMode = "kr" // default for everything except functions

const (
	banner = `  _____        __        ______       __    _______           __  
 / ___/__  ___/ /__ ____/ __/ /___ __/ /__ / ___/ /  ___ ____/ /__
/ /__/ _ \/ _  / -_)___/\ \/ __/ // / / -_) /__/ _ \/ -_) __/  '_/
\___/\___/\_,_/\__/   /___/\__/\_, /_/\__/\___/_//_/\__/\__/_/\_\ 
                              /___/            `
)

const (
	Reset       = "\x1b[0m"
	Keyword     = "\x1b[31m"        // light red for keywords
	Type        = "\x1b[0;33m"      // dark yellow for types
	Function    = "\x1b[32m"        // dark green for functions
	Variable    = "\x1b[0m"         // default for variables
	Number      = "\x1b[35m"        // magenta for numbers
	StringC     = "\x1b[32m"        // green for strings
	Comment     = "\x1b[37m"        // gray for comments
	Operator    = "\x1b[38;5;166m"  // orange for operators
	Brackets    = "\x1b[38;5;172m"  // parentheses and braces
	DefineCol   = "\x1b[36m"        // cyan for preprocessor directives

	ErrorBg     = "\x1b[41m"        // red background for error
	ErrorFg     = "\x1b[31m"        // red text for ERROR
	WarningFg   = "\x1b[33m"        // yellow text for WARNING

	LineNumCol  = "\033[38;5;245m"  // light gray (line number)
	PipeCol     = "\033[38;5;241m"  // dark gray (pipe)
	ErrorNumber = "\033[38;5;39m"   // blue for error number
	LetterCol	= "\x1b[94m"
	TitleCol	= "\033[34m"
)

var rainbowColors = []string{
	"\x1b[38;5;172m", //  0 → red
	"\x1b[32m",       //  1 → green
	"\x1b[33m",       //  2 → yellow
	"\x1b[34m",       //  3 → blue
	"\x1b[35m",       //  4 → magenta
	"\x1b[36m",       //  5 → cyan
	"\x1b[91m",       //  6 → light red
	"\x1b[92m",       //  7 → light green
	"\x1b[31m",       //  8 → light yellow
	"\x1b[94m",       //  9 → light blue
}

// Type patterns used in regex construction
var (
	typeNames = []string{"int", "char", "float", "double", "long", "short", "bool", "void"}
	typePattern = "(" + strings.Join(typeNames, "|") + ")"
	typedefPattern = "[A-Za-z_][A-Za-z0-9_]*_t"
	typeOrTypedef = "(?:" + typePattern + "|" + typedefPattern + ")"
	ptrPattern = `\b(?:` + typePattern + `|[A-Za-z_][A-Za-z0-9_]*_t)\*`
)

var (
	// Allman/K&R brace format
	reControlStmt    = regexp.MustCompile(`^\s*(?:typedef\s+)?(if|else|for|while|switch|struct|union|enum)\b`)
	reBraceOnlyLine  = regexp.MustCompile(`^\s*\{\s*$`)
	reTodo = regexp.MustCompile(`\b(?:TODO|FIXME)\b`)
	// struct/enum closing
	reClosingAll = regexp.MustCompile(
		`^\s*\}` +                     // closing brace
		`\s*` +                        // optional spaces
		`([A-Za-z_][A-Za-z0-9_]*)?` + // optional instance name
		`\s*;?\s*$`,                  // optional ; and end of line
	)
	reSplitFuncName = regexp.MustCompile(
		`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(`,
	)
	// snake_lower_case and *_t names
	snakePattern         = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	snakeTypedefPattern  = regexp.MustCompile(`^[a-z][a-z0-9_]*_t$`)
	// macros in SCREAMING_SNAKE_CASE
	screamingSnakePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

	reMacroDefLine = regexp.MustCompile(
		`^\s*` +                         // optional indentation
		`(#\s*define)` +                // 1: directive
		`\s+([A-Za-z_][A-Za-z0-9_]*)` + // 2: macro name
		`(?:\(([^\)]*)\))?`,            // 3: parameters (without parentheses)
	)
	reLabelDecl = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)(\s*):$`)
	// typedefs
	reTypedefFuncPtr = regexp.MustCompile(`^\s*typedef\b.*\(\s*\*\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)`)
	reTypedefGeneric = regexp.MustCompile(`^\s*typedef\b.*\b([A-Za-z_][A-Za-z0-9_]*)\s*;`)
	// defines
	reDefine = regexp.MustCompile(`^\s*#\s*define\s+([A-Za-z_][A-Za-z0-9_]*)`)

	// inside enum { … }
	reEnumElement = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*(?:=|,)\s*`)
	reMultiVarDecl = regexp.MustCompile(`^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+(?:\*\s*)?[A-Za-z_][A-Za-z0-9_]*\s*,`)
	// struct fields
	reStructFieldName = regexp.MustCompile(`^\s*[A-Za-z_][A-Za-z0-9_]*\s*(?:\*\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*;`)
	reBadBracketSpace = regexp.MustCompile(`\[\s+|[ \t]+\]`)
	// variable declarations (without functions)
	reVarDeclName = regexp.MustCompile(
		`^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+(?:\*\s*)?` +
			`([A-Za-z_][A-Za-z0-9_]*)\s*(?:=|;).*`,
	)

	reMacroDef = regexp.MustCompile(
		`^\s*#\s*define\s+` +                             // "# define <NAME>"
		`([A-Za-z_][A-Za-z0-9_]*)\s*` +                  // 1: macro name
		`\(\s*([^)]*)\)\s*` +                            // 2: parameter list
		`(.*)$`,                                         // 3: rest of line = macro body
	)

	reIdent = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

	// keywords attached to "("
	reKeywordNoSpace = regexp.MustCompile(
		`\b(if|else|for|while|return|break|continue|switch|case|default|static|` +
			`const|extern|unsigned|signed|typedef|struct|union|enum|void|sizeof)\(`,
	)
	reMagicNumber = regexp.MustCompile(`\b([2-9][0-9]*)\b`)
	reFuncMacro = regexp.MustCompile(
		`^\s*#\s*define\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)\s+(.+)$`,
	)
	reFuncHeader = regexp.MustCompile(
		`^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+([A-Za-z_][A-Za-z0-9_]*)\s*\(`,
	)
	// identifier + space + "(" → function calls
	reIdentSpaceParen = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s+\(`)

	// internal helpers
	reUninitDecl    = regexp.MustCompile(`^\s*` + typePattern + `(?:\s*\*\s*|\s+)` +
		`([A-Za-z_][A-Za-z0-9_]*)\s*;\s*$`)
	reTypeDeclNoBrace = regexp.MustCompile(`^\s*(typedef\s+)?(struct|enum)\b.*[^{;]\s*$`)
	reStructEnd     = regexp.MustCompile(`^\s*\}\s*;?\s*$`)
	rePtrDecl       = regexp.MustCompile(`\b` + typePattern + `\s*\*\s*[A-Za-z_][A-Za-z0-9_]*`)
	reCorrectPtr    = regexp.MustCompile(`\b` + typePattern + ` \*[A-Za-z_][A-Za-z0-9_]*\b`)
	reTrailing      = regexp.MustCompile(`[ \t]+$`)
	reMultiSpace    = regexp.MustCompile(`\S( {2,})\S`)
	reFuncDecl      = regexp.MustCompile(
		`^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+` +
			`([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)\s*(?:;|$)`,
	)
	reFuncSignature = regexp.MustCompile(
		`^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+[A-Za-z_][A-Za-z0-9_]*\s*\(([^)]*)\)\s*$`,
	)
	reParamName     = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)$`)

	reTernaryQNoSpaceBefore     = regexp.MustCompile(`\S\?`)
	reTernaryQNoSpaceAfter      = regexp.MustCompile(`\?\S`)
	reTernaryColonNoSpaceBefore = regexp.MustCompile(`\S:`)
	reTernaryColonNoSpaceAfter  = regexp.MustCompile(`:\S`)

	reCastPtr = regexp.MustCompile(`\(\s*(?:int|char|float|double|long|short|bool)\s*\*\s*\)`)
	// new: capture use of "*ptr" when dereferencing
	reDeref = regexp.MustCompile(`\*\s*[A-Za-z_][A-Za-z0-9_]*`)

	// catch "void *", "int *", etc. in parameters and generic declarations
	reGenericPtr = regexp.MustCompile(`\b(?:void|int|char|float|double|long|short|bool)\s*\*\b`)
	// catch "void *" (and other types or typedefs *_t) when followed by comma
	reGenericPtrComma = regexp.MustCompile(
		`\b(?:void|int|char|float|double|long|short|bool|[A-Za-z_][A-Za-z0-9_]*_t)\s*\*\s*,`,
	)
	// catch "void *" (and other types or typedefs *_t) when followed by ')'
	reGenericPtrParen = regexp.MustCompile(
		`\b(?:void|int|char|float|double|long|short|bool|[A-Za-z_][A-Za-z0-9_]*_t)\s*\*\s*\)`,
	)
	// error in "* ptr"
	reBadDeref        = regexp.MustCompile(`\*\s+[A-Za-z_][A-Za-z0-9_]*`)
	// error in "(int*)"
	reBadCastLeading  = regexp.MustCompile(`\(\s*(?:void|int|char|float|double|long|short|bool)\*\)`)
	// error in "(int * )"
	reBadParenSpace = regexp.MustCompile(`\(\s+|[ \t]+\)`)
	reBadComma      = regexp.MustCompile(`\s+,|,\S|, {2,}`)

	reTypeStarNoSpace = regexp.MustCompile(ptrPattern)

	reBadPtrCast = regexp.MustCompile(
		`\(\s*` + typeOrTypedef + `\s*\*\s*\)\s+[A-Za-z_\(]`,
	)
	reMacroNoSpace = regexp.MustCompile(`^\s*#\s*define\s+[A-Za-z_][A-Za-z0-9_]*\([^)]*\)\S`)
	reOpenBrace  = regexp.MustCompile(`\{\s*$`)
	reCloseBrace = regexp.MustCompile(`^\}\s*$`)
	reFuncNameOnly = regexp.MustCompile(
		`^\s*[A-Za-z_][A-Za-z0-9_]*\s*\(`,
	)
	reOnlyType = regexp.MustCompile(
		`^\s*(?:` +
			`(?:static|const|unsigned|signed|short|long)\s+` + // qualifiers
			`)*` +
			`(?:` + typePattern + `|` + typedefPattern + `)(?:\s*\*+)?\s*$`,
	)

	reAllocCall = regexp.MustCompile(`\b(malloc|realloc|calloc)\s*\(`)
	reAllocCast = regexp.MustCompile(
		`\(\s*[A-Za-z_][A-Za-z0-9_]*\s*\*\)\s*(?:malloc|realloc|calloc)\s*\(`,
	)

	reTypeStart = regexp.MustCompile(
		`^\s*(typedef\s+)?(struct|enum|union)\b\s*` +
		`([A-Za-z_][A-Za-z0-9_]*)?\s*$`,
	)
	// type already opening brace
	reTypeStartBrace = regexp.MustCompile(
		`^\s*(typedef\s+)?(struct|enum|union)\b\s*` +
		`([A-Za-z_][A-Za-z0-9_]*)?\s*\{`,
	)
	reInlineBlock = regexp.MustCompile(`^\s*[^{]*\{[^}]*\}.*$`)
	reInlineStmt = regexp.MustCompile(
		`^\s*(?:if|else if|for|while|switch)\s*\([^)]*\)\s*[^\{\}]+;?\s*$`,
	)
	reInnerControl = regexp.MustCompile(`\b(?:if|for|while|switch)\b`)
	reNoSpaceAfterParen = regexp.MustCompile(`\)\S`)
	reControlParenNoSpace = regexp.MustCompile(`\b(?:if|else|for|while|switch)\(`)
)

var unsafeFuncSuggestions = map[string]string{
	// String reading without limit
	"gets":    "fgets(buffer, size, stdin)",

	// String copy/concatenation without limit
	"strcpy":  "strlcpy(dest, src, dest_size) // or strncpy(dest, src, n)",
	"strcat":  "strlcat(dest, src, dest_size) // or strncat(dest, src, n)",

	// String formatting without limit
	"sprintf":  "snprintf(buffer, size, ...)",
	"vsprintf": "vsnprintf(buffer, size, ap)",

	// Scanf without specifying max width
	"scanf":   "fgets(line, size, stdin) and then sscanf(line, \"%…\", &…)",
	"fscanf":  "fgets(line, size, file) and then sscanf(line, \"%…\", &…)",
	"sscanf":  "sscanf(line, \"%width…\", &…) // use width specifiers",

	// Temporary filename generation
	"tmpnam":  "mkstemp(template) // or tmpfile()",

	// Working directory lookup — getwd is obsolete
	"getwd":   "getcwd(buffer, size)",
}

var reUnsafeFunc = regexp.MustCompile(
	`\b(` + strings.Join(func() []string {
		keys := make([]string, 0, len(unsafeFuncSuggestions))
		for k := range unsafeFuncSuggestions {
			keys = append(keys, k)
		}
		return keys
	}(), "|") + `)\s*\(`,
)

var keywords = map[string]bool{
	"auto": true, "break": true, "case": true, "char": true,
	"const": true, "continue": true, "default": true, "do": true,
	"double": true, "else": true, "enum": true, "extern": true,
	"float": true, "for": true, "goto": true, "if": true,
	"inline": true, "int": true, "long": true, "register": true,
	"restrict": true, "return": true, "short": true, "signed": true,
	"sizeof": true, "static": true, "struct": true, "switch": true,
	"typedef": true, "union": true, "unsigned": true, "void": true,
	"volatile": true, "while": true,
	// C11/C18 extras:
	"_Alignas": true, "_Alignof": true, "_Atomic": true, "_Bool": true,
	"_Complex": true, "_Generic": true, "_Imaginary": true, "_Noreturn": true,
	"_Static_assert": true, "_Thread_local": true,
}

var typesMap = map[string]bool{
	"int": true, "char": true, "float": true, "double": true, "long": true, "short": true,
	"bool": true,
}

var operatorRunes = map[rune]bool{
	'+': true, '-': true, '*': true, '/': true, '%': true,
	'=': true, '<': true, '>': true, '!': true, '&': true, '|': true,
	';': true, ',': true, '.': true, ':': true, '?': true,
}

type StyleError struct {
	LineNum int
	Start   int
	Length  int
	Message string
	Level   string // "ERROR" or "WARNING"
}

func main() {
	// usage: go run check_style.go [--style=<allman|k&r>] <file.c/h>
	args := os.Args[1:]
	if len(args) == 2 && strings.HasPrefix(args[0], "--style=") {
		styleMode = strings.TrimPrefix(args[0], "--style=")
		if styleMode != "allman" && styleMode != "kr" {
			fmt.Fprintf(os.Stderr, "Invalid style: %s\n", styleMode)
			os.Exit(1)
		}
		args = args[1:]
	}
	if len(args) != 1 {
		fmt.Println("Usage: go run check_style.go [--style=<allman|k&r>] <file.c/h>")
		return
	}
	filename := args[0]

	// 1) read the entire file in raw
	raw, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	// 2) split into lines
	lines := strings.Split(string(raw), "\n")

	// 3) execute global checks
	var errs []StyleError
	errs = processIncludes(lines, filename)
	checkEOFNewline(raw, &errs)
	checkHeaderGuard(lines, filename, &errs)

	// 4) execute line-by-line style
	styleErrs := checkStyle(lines)
	errs = append(errs, styleErrs...)

	// --- printing ---
	seen := make(map[string]bool)
	totalErrors, totalWarnings := 0, 0
	// header
	fmt.Printf("\n\n")
	fmt.Println(TitleCol + banner + Reset)
	fmt.Printf("\n")
	
	// print each unique error/warning, counting them
	for _, e := range errs {
		key := fmt.Sprintf("%d:%d:%s", e.LineNum, e.Start, e.Message)
		if seen[key] {
			continue
		}
		seen[key] = true

		if e.Level == "ERROR" {
			totalErrors++
		} else if e.Level == "WARNING" {
			totalWarnings++
		}

		levelColor := ErrorFg
		if e.Level == "WARNING" {
			levelColor = WarningFg
		}
		
	checkEOFNewline(raw, &errs)
	checkHeaderGuard(lines, filename, &errs)
		fmt.Printf("%s------------------------------------------------------------------%s\n", LineNumCol, Reset)
		fmt.Printf("%s#%d%s %s[%s]: %s%s%s%s\n\n",
			TitleCol, totalErrors+totalWarnings, Reset,
			levelColor, e.Level, Reset,
			LetterCol, e.Message, Reset,
		)
		fmt.Printf("%s%s:%d:%d%s\n", LineNumCol, filename, e.LineNum, e.Start+1, Reset)
		printContext(lines, e)
		fmt.Println()
	}

	fmt.Printf("%s------------------------------------------------------------------%s\n", LineNumCol, Reset)
	fmt.Printf("%sTotal: %s%d error(s)%s & %s%d warning(s)%s\n",
		TitleCol,
		ErrorFg, totalErrors, Reset,
		WarningFg, totalWarnings, Reset,
	)
	fmt.Printf("%s------------------------------------------------------------------%s\n\n", LineNumCol, Reset)
}
func findFirstUnsorted(keys []string) int {
	for i := 0; i < len(keys)-1; i++ {
		if keys[i] > keys[i+1] {
			return i + 1
		}
	}
	return 0
}
func processIncludes(lines []string, filename string) []StyleError {
	var errs []StyleError

	reInclude     := regexp.MustCompile(`^\s*#\s*include\s+([<"].+[>"])`)
	reIncludeFile := regexp.MustCompile(`^\s*#\s*include\s+"([^"]+)"`)

	type includeEntry struct{ inc string; line int }
	var sysIncludes, projIncludes []includeEntry

	for idx, l := range lines {
		if m := reInclude.FindStringSubmatch(l); m != nil {
			entry := includeEntry{inc: m[1], line: idx + 1}
			if strings.HasPrefix(m[1], "<") {
				sysIncludes = append(sysIncludes, entry)
			} else {
				projIncludes = append(projIncludes, entry)
			}
		}
		// recursion in .h
		if strings.HasSuffix(strings.ToLower(filename), ".h") {
			if m := reIncludeFile.FindStringSubmatch(l); m != nil && filepath.Base(filename) == m[1] {
				// keep it as it is
				pos := strings.Index(l, m[1])
				errs = append(errs, StyleError{
					LineNum: idx + 1,
					Start:   pos,
					Length:  len(m[1]),
					Message: fmt.Sprintf("recursive inclusion of '%s' detected", m[1]),
					Level:   "ERROR",
				})
			}
		}
	}

	// <> should come before ""
	if len(sysIncludes) > 0 && len(projIncludes) > 0 {
		firstProj := projIncludes[0]
		lastSys := sysIncludes[len(sysIncludes)-1]
		if firstProj.line < lastSys.line {
			// now highlight from '#' to end of line
			full := lines[firstProj.line-1]
			start := strings.Index(full, "#")
			errs = append(errs, StyleError{
				LineNum: firstProj.line,
				Start:   start,
				Length:  utf8.RuneCountInString(full[start:]),
				Message: `system includes (<...>) should come before project includes ("...")`,
				Level:   "ERROR",
			})
		}
	}

	// helper to extract keys and lines
	sortKeys := func(arr []includeEntry) (keys []string, lines []int) {
		keys = make([]string, len(arr))
		lines = make([]int, len(arr))
		for i, e := range arr {
			keys[i] = e.inc
			lines[i] = e.line
		}
		return keys, lines
	}

	// alphabetical order in <...>
	if sysKeys, _ := sortKeys(sysIncludes); !sort.StringsAreSorted(sysKeys) {
		idx := findFirstUnsorted(sysKeys)
		bad := sysIncludes[idx-1] // the include that should come before
		full := lines[bad.line-1]
		start := strings.Index(full, "#")
		errs = append(errs, StyleError{
			LineNum: bad.line,
			Start:   start,
			Length:  utf8.RuneCountInString(full[start:]),
			Message: "system includes (<...>) are not in alphabetical order",
			Level:   "ERROR",
		})
	}

	// alphabetical order in "..."
	if projKeys, _ := sortKeys(projIncludes); !sort.StringsAreSorted(projKeys) {
		idx := findFirstUnsorted(projKeys)
		bad := projIncludes[idx-1]
		full := lines[bad.line-1]
		start := strings.Index(full, "#")
		errs = append(errs, StyleError{
			LineNum: bad.line,
			Start:   start,
			Length:  utf8.RuneCountInString(full[start:]),
			Message: "project includes (\"...\") are not in alphabetical order",
			Level:   "ERROR",
		})
	}

	return errs
}

func readLines(filename string) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func processCommentRules(text string, lineNum int, errs *[]StyleError) {
	if m := reTodo.FindStringIndex(text); m != nil {
		*errs = append(*errs, StyleError{
			LineNum: lineNum,
			Start:   m[0],
			Length:  m[1] - m[0],
			Message: "found TODO/FIXME comment – check pending tasks before submitting",
			Level:   "WARNING",
		})
	}
	// (other comment checks...)
}

func checkPragmaOnce(lines []string, filename string, errs *[]StyleError) {
	hasPragmaOnce := false
	for _, l := range lines {
		if strings.TrimSpace(l) == "#pragma once" {
			hasPragmaOnce = true
			break
		}
	}
	if hasPragmaOnce && strings.HasSuffix(filename, ".h") {
		// reuse logic for detecting guarding
		base := strings.ToUpper(strings.TrimSuffix(filepath.Base(filename), ".h"))
		guard := base + "_H"
		hasIfndef, hasDefine, hasEndif := false, false, false
		for _, l := range lines {
			t := strings.TrimSpace(l)
			if t == "#ifndef "+guard { hasIfndef = true }
			if t == "#define "+guard { hasDefine = true }
			if strings.HasPrefix(t, "#endif") { hasEndif = true }
		}
		if hasIfndef && hasDefine && hasEndif {
			*errs = append(*errs, StyleError{
				LineNum: 1,
				Start:   0,
				Length:  len("#pragma once"),
				Message: "do not use #pragma once and include-guard simultaneously; choose one",
				Level:   "ERROR",
			})
		}
	}
}

func checkUnsafeFunctions(codeOnly string, lineNum int, errs *[]StyleError) {
	for _, loc := range reUnsafeFunc.FindAllStringSubmatchIndex(codeOnly, -1) {
		// loc[2]: start of name, loc[3]: end of name
		name := codeOnly[loc[2]:loc[3]]
		suggestion := unsafeFuncSuggestions[name]
		*errs = append(*errs, StyleError{
			LineNum: lineNum,
			Start:   loc[2],
			Length:  len(name),
			Message: fmt.Sprintf(
				"use of insecure function '%s'; consider using %s",
				name, suggestion,
			),
			Level: "WARNING",
		})
	}
}
func checkConstPointerParams(lines []string, errs *[]StyleError) {
	// Regex that detects the start of a function signature (return type + name + "(")
	// and captures everything inside the parentheses, but only if it's on a single line.
	reFuncSigSingle := regexp.MustCompile(
		`^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+` + // return type (can be multiple words)
			`([A-Za-z_][A-Za-z0-9_]*)\s*` + // function name
			`\(([^)]*)\)`) // group 2 = parameter list (everything up to ')')

	// Regex that detects the start of a multiline signature (line containing "("
	// but not ")"). We'll use this to enter "multiline mode".
	reFuncSigStart := regexp.MustCompile(
		`^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+` + // return type
			`([A-Za-z_][A-Za-z0-9_]*)\s*\(`) // group 1 = function name, and we've already seen "("

	// Regex to find the closing of the parameter block: line containing ")"
	reFuncSigEnd := regexp.MustCompile(`\)`)

	// Helper function (common logic) for, given the "rawParams" (string containing "p1, p2, p3"),
	// iterating over each parameter and generating a warning about "non-const pointer" if applicable.
	checkParams := func(rawParams string, functionLine int) {
		parts := strings.Split(rawParams, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			// If it starts with "*" (p.e., "*type_t"), ignore (there's no type before the pointer)
			if strings.HasPrefix(p, "*") {
				continue
			}
			// We only want pointer parameters without "const"
			if !strings.Contains(p, "*") || strings.HasPrefix(p, "const ") {
				continue
			}
			// Extract the real name of the parameter (last "word" after the asterisk)
			fields := strings.Fields(p)             // ex: ["int", "*ptr"]
			rawName := fields[len(fields)-1]        // ex: "*ptr"
			name := strings.TrimLeft(rawName, "*")  // ex: "ptr"

			// Now, traverse the function body looking for assignment to "name"
			braceDepth := 0
			modified := false
			for j := functionLine + 1; j < len(lines); j++ {
				l := lines[j]
				if strings.Contains(l, "{") {
					braceDepth++
				}
				if strings.Contains(l, "}") {
					if braceDepth == 0 {
						break
					}
					braceDepth--
				}
				// If we find "name =" or "*name =", mark as modified
				if strings.Contains(l, name+" =") || strings.Contains(l, "*"+name+" =") {
					modified = true
					break
				}
			}

			if !modified {
				// Position the start at the actual name within the signature
				pos := strings.Index(lines[functionLine], name)
				if pos < 0 {
					pos = 0
				}
				*errs = append(*errs, StyleError{
					LineNum: functionLine + 1,
					Start:   pos,
					Length:  len(name),
					Message: fmt.Sprintf(
						"pointer '%s' is not modified; consider declaring it 'const %s'",
						name, p,
					),
					Level: "WARNING",
				})
			}
		}
	}

	inMultiline := false         // flag: we're in the middle of a multiline signature
	multilineLines := []string{} // stores the lines of the current signature
	startLineIdx := 0            // index of the line where the multiline signature started

	for idx, line := range lines {


		if !inMultiline {
			// 1) Check "single-line signature"
			if m := reFuncSigSingle.FindStringSubmatch(line); m != nil {
				// m[1] = function name, m[2] = content inside parentheses
				rawParams := m[2]
				checkParams(rawParams, idx)
				continue
			}

			// 2) Check "start of multiline signature": has "(" but not ")" on the same line
			if m := reFuncSigStart.FindStringSubmatch(line); m != nil && !strings.Contains(line, ")") {
				// We're entering multiline mode
				inMultiline = true
				multilineLines = []string{line}
				startLineIdx = idx
				continue
			}

			// If neither of the above, just continue
		} else {
			// We were already in multiline mode: add another line
			multilineLines = append(multilineLines, line)

			// If this line contains ")", we've closed the parameter block
			if reFuncSigEnd.MatchString(line) {
				// Concatenate all lines of the signature into one, to extract "rawParams"
				fullSig := strings.Join(multilineLines, " ")
				// Now we try to extract the content inside the parentheses "(...)"
				// We can use a simplified regex for capturing
				reInner := regexp.MustCompile(`\((.*)\)`)
				if m2 := reInner.FindStringSubmatch(fullSig); m2 != nil {
					rawParams := m2[1]
					// The "line number" of the "function line" will be startLineIdx (where the name appeared)
					checkParams(rawParams, startLineIdx)
				}
				// Exit multiline mode
				inMultiline = false
				multilineLines = nil
				continue
			}
			// If we haven't found ")", keep accumulating
		}
	}
}


func checkHeaderGuard(lines []string, filename string, errs *[]StyleError) {
	if !strings.HasSuffix(filename, ".h") {
		return
	}
	base := strings.ToUpper(strings.TrimSuffix(filepath.Base(filename), ".h"))
	guard := base + "_H"
	hasIfndef, hasDefine, hasEndif := false, false, false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "#ifndef "+guard { hasIfndef = true }
		if t == "#define "+guard { hasDefine = true }
		if strings.HasPrefix(t, "#endif") { hasEndif = true }
	}
	if !hasIfndef || !hasDefine || !hasEndif {
		*errs = append(*errs, StyleError{
			LineNum: 1,
			Start:   0,
			Length:  0,
			Message: fmt.Sprintf("header '%s' missing include-guard (%s)", filename, guard),
			Level:   "ERROR",
		})
	}
}

func checkEOFNewline(raw []byte, errs *[]StyleError) {
	if len(raw) == 0 || raw[len(raw)-1] == '\n' {
		return
	}
	// split by lines
	lines := bytes.Split(raw, []byte("\n"))
	lastLine := lines[len(lines)-1]
	// count runes in the last line
	col := utf8.RuneCount(lastLine)
	// mark the last character (col-1) with length=1
	*errs = append(*errs, StyleError{
		LineNum: len(lines),
		Start:   col - 1,
		Length:  1,
		Message: "file must end with a newline",
		Level:   "ERROR",
	})
}

func checkStyle(lines []string) []StyleError {
	var errs []StyleError
	indentStack := []int{0}
	nextIndent := -1
	
    type typeCtx struct {
        isDataStructure bool   // true for struct/enum/union
        isTypedef       bool   // true if this block was introduced by typedef
        dataType        string // "struct", "enum" or "union"
        tagName         string // the tag after struct/enum/union, if any
		tagLine         int    // line where the tagName appeared
		tagPos          int    // column where the tagName starts
    }

	pendingTypeDecl := false
	pendingTypedef := false
	typeKind := "" // "struct" or "enum"
    
	var typeStack []typeCtx
	var pendingTagLine, pendingTagPos int
   	inBlockComment := false  // flag for /* */ multi-line]
    var caseEndLine = -1       // index of the line of the break; associated
    var caseIndentLevel = 0    // indent where the case appeared
	const maxLineLength = 80
	const maskRune = '\uFFFD'
	var typeTag string
	inParamBlock := false
    paramIndent := 0

	pointerRegexes := []*regexp.Regexp{
		rePtrDecl,
		reTypedefFuncPtr,
		reCastPtr,
		reDeref,
		reGenericPtr,
		reGenericPtrComma,
		reGenericPtrParen,
	}

	combinedPtrRE := regexp.MustCompile(
		fmt.Sprintf("(%s)|(%s)|(%s)",
			reBadDeref.String(),       // "* ptr"
			reTypeStarNoSpace.String(),// "type* ptr"
			reBadCastLeading.String(), // "(int*) X"
		),
	)
    caseBraceRe := regexp.MustCompile(`^(\s*case\s+[^:]+:)\s*\{\s*$`)
            var proc []string
            for _, l := range lines {
                if m := caseBraceRe.FindStringSubmatch(l); m != nil {
                    // 1) line "case X:"
                    proc = append(proc, m[1])
                    // 2) line with only "{" (same indentation)
                    indent := l[:strings.Index(l, strings.TrimSpace(l))]
                    proc = append(proc, indent+"{")
                } else {
                    proc = append(proc, l)
                }
            }
            lines = proc

	for i, line := range lines {

        
		trim := strings.TrimSpace(line)
		if l := utf8.RuneCountInString(line); l > maxLineLength {
        errs = append(errs, StyleError{
            LineNum: i + 1,
            Start:   maxLineLength,
            Length:  l - maxLineLength,
            Message: fmt.Sprintf("line length must not exceed %d characters, found %d", maxLineLength, l),
            Level:   "ERROR",
        })
    }

    blankCount := 0
for i, line := range lines {
    if strings.TrimSpace(line) == "" {
        blankCount++
    } else {
        if blankCount > 1 {
            errs = append(errs, StyleError{
                LineNum: i, // last empty line of block
                Start:   0,
                Length:  0,
                Message: fmt.Sprintf("more than 1 blank line consecutively (%d)", blankCount),
                Level:   "WARNING",
            })
        }
        blankCount = 0
    }
}

reSemicolonSpace := regexp.MustCompile(`\s+;`)
for i, line := range lines {
    if loc := reSemicolonSpace.FindStringIndex(line); loc != nil {
        start := loc[0]
        errs = append(errs, StyleError{
            LineNum: i + 1,
            Start:   start,
            Length:  loc[1] - loc[0],
            Message: "no space before ';'",
            Level:   "ERROR",
        })
    }
}
for i, line := range lines {
    for idx, ch := range line {
        if ch > 127 {
            errs = append(errs, StyleError{
                LineNum: i + 1,
                Start:   idx,
                Length:  1,
                Message: fmt.Sprintf("unexpected non-ASCII character: 0x%X", ch),
                Level:   "WARNING",
            })
        }
    }
}
// (Optional) check if file ends with more than 1 blank line
if blankCount > 1 {
    errs = append(errs, StyleError{
        LineNum: len(lines),
        Start:   0,
        Length:  0,
        Message: fmt.Sprintf("file ends with %d blank lines; remove excess", blankCount),
        Level:   "WARNING",
    })
}
		// === 1) FILTER COMMENTS AND LITERALS ===
       codeOnly := line

       // 1a) if we were already inside /* ... */
       if inBlockComment {
           if end := strings.Index(codeOnly, "*/"); end >= 0 {
              // remove up to closing and exit the block
               codeOnly = strings.Repeat(string(maskRune), end+2) + codeOnly[end+2:]
               inBlockComment = false
           } else {
               processCommentRules(codeOnly, i+1, &errs)
               continue
           }
       }

       // 1b) remove any /* ... */ (can be multiple)
        // === 1) SINGLE LINE COMMENT //… ===
        if strings.HasPrefix(trim, "//") {
            processCommentRules(line, i+1, &errs)
            continue
        }

        // === 2) START OF BLOCK /*…*/ ===
        if strings.HasPrefix(trim, "/*") {
            processCommentRules(line, i+1, &errs)
            // if not closed on the same line, mark inBlockComment
            if !strings.Contains(trim, "*/") {
                inBlockComment = true
            }
            continue
       }

       // 1c) remove comment line //
       if idx := strings.Index(codeOnly, "//"); idx >= 0 {
            commentPart := codeOnly[idx:]
            processCommentRules(commentPart, i+1, &errs)
            codeOnly = codeOnly[:idx] + strings.Repeat(string(maskRune), len(codeOnly)-idx)
       }
	   	
       reStr  := regexp.MustCompile(`"((?:\\.|[^"\\])*)"`)
		codeOnly = reStr.ReplaceAllStringFunc(codeOnly, func(s string) string {
			n := len(s) - 2
			if n < 0 { return s }
			return " + strings.Repeat(string(maskRune), n) + "
		})
        reChar := regexp.MustCompile(`'((?:\\.|[^'\\])*)'`)
		codeOnly = reChar.ReplaceAllStringFunc(codeOnly, func(s string) string {
			n := len(s) - 2
			if n < 0 { return s }
			return `'` + strings.Repeat(string(maskRune), n) + `'`
		})
		trimmed := strings.Trim(codeOnly, string(maskRune))
		// if leftover is just maskRune (and possibly spaces),
		// then there's no real code — we can skip
		if trimmed == "" {
			continue
		}
        		indent := 0
		for _, ch := range line {
			if ch == ' ' {
				indent++
			} else if ch == '\t' {
				indent += 2
			} else {
				break
			}
		}
                if strings.HasPrefix(trim, "} else {") {
                        // 1) close previous block (pop indentStack)
                       if len(indentStack) > 1 {
                           indentStack = indentStack[:len(indentStack)-1]
                        }
                        // 2) now "open" the else block, pushing inner indent (indentForStack + 2).
                        //    We need to know what indentForStack was after the pop. Since we just popped,
                        //    the top of indentStack is the level of the "if(...)", so we can use that value.
                        indentForStack := indentStack[len(indentStack)-1]
                        indentStack = append(indentStack, indentForStack+2)
                        // 3) skip the rest of this iteration, as we don't want to revalidate the indent of "} else {"
                        continue
                    }

                        if styleMode == "kr" && trim == "else {" && i > 0 {
                                    prevTrim := strings.TrimSpace(lines[i-1])
                                    if prevTrim == "}" {
                                        // We found a "}" on its own line, followed by "else {".
                                        errs = append(errs, StyleError{
                                            LineNum: i + 1,
                                            Start:   strings.Index(line, "else"),
                                            Length:  len("else"),
                                            Message: `"else" must be on the same line as the closing '}' (K&R style)`,
                                            Level:   "ERROR",
                                        })
                                   }
                                }

            if strings.HasPrefix(trim, "#include") {
            if indent != 0 {
                errs = append(errs, StyleError{
                    LineNum: i + 1,
                    Start:   0,
                    Length:  indent,
                    Message: "include directive must have no indentation",
                    Level:   "ERROR",
                })
            }
            continue
        }
        		// 6) trailing whitespace

    		// ** Parentheses Rule **: no space after '(' or before ')'
		if locs := reBadParenSpace.FindAllStringIndex(codeOnly, -1); locs != nil {
			for _, loc := range locs {
				errs = append(errs, StyleError{
					LineNum: i + 1,
					Start:   loc[0],
					Length:  loc[1] - loc[0],
					Message: "no space allowed inside parentheses",
					Level:   "ERROR",
				})
			}
		}

		if locs := reBadBracketSpace.FindAllStringIndex(codeOnly, -1); locs != nil {
		for _, loc := range locs {
			errs = append(errs, StyleError{
				LineNum: i + 1,
				Start:   loc[0],
				Length:  loc[1] - loc[0],
				Message: "no space allowed after '[' or before ']'",
				Level:   "ERROR",
			})
		}
	}
		
		// ** Comma Rule **: the character before must be attached and followed by exactly one space
		if locs := reBadComma.FindAllStringIndex(codeOnly, -1); locs != nil {
			for _, loc := range locs {
				errs = append(errs, StyleError{
					LineNum: i + 1,
					Start:   loc[0],
					Length:  loc[1] - loc[0],
					Message: "comma must be followed by a space and preceded by a space",
					Level:   "ERROR",
				})
			}
		}

		// 7) multiple spaces
		for _, loc := range reMultiSpace.FindAllStringIndex(codeOnly, -1) {
			start := loc[0] + 1
			length := loc[1] - loc[0] - 2
			if length < 1 {
				length = 1
			}
			errs = append(errs, StyleError{i + 1, start, length,
				"multiple consecutive spaces between tokens", "ERROR"})
		}

		// pointer errors
		if locs := combinedPtrRE.FindAllStringIndex(codeOnly, -1); locs != nil {
			for _, loc := range locs {
				errs = append(errs, StyleError{
					LineNum: i+1,
					Start:   loc[0],
					Length:  loc[1]-loc[0],
					Message: "pointer must be formatted as:\n" +
							"- 'type *ptr' for declarations\n" +
							"- '*ptr' or '*type' for dereferences\n" +
							"- 'type *' for casting",
					Level:   "ERROR",
				})
			}
		}

		if locs := reBadPtrCast.FindAllStringIndex(codeOnly, -1); locs != nil {
			for _, loc := range locs {
				errs = append(errs, StyleError{
					LineNum: i + 1,
					Start:   loc[0],
					Length:  loc[1] - loc[0],
					Message: "pointer cast must be attached to the operand:\n" +
							 "- use '(t *)x' or '(t *)(x)', not '(t *) x'",
					Level:   "ERROR",
				})
			}
		}
		if reMacroNoSpace.MatchString(line) {
				// position of ')'
				pos := strings.Index(line, ")")
				errs = append(errs, StyleError{
					LineNum: i + 1,
					Start:   pos,
					Length:  1,
					Message: "macro body must be preceded by a space after parameter list",
					Level:   "ERROR",
				})
			}
		if m := reMacroDef.FindStringSubmatchIndex(codeOnly); m != nil {
			// extract substrings
			macroName  := line[m[2]:m[3]]
			rawParams  := line[m[4]:m[5]]
			macroBody  := line[m[6]:m[7]]

			// list of individual parameters
			params := []string{}
			for _, p := range strings.Split(rawParams, ",") {
				name := strings.TrimSpace(p)
				// validate each parameter
				if !snakePattern.MatchString(name) {
					pos := strings.Index(line, name)
					errs = append(errs, StyleError{
							LineNum: i+1,
							Start:   pos,
							Length:  len(name),
							Message: fmt.Sprintf("macro parameter '%s' must be snake_lower_case", name),
							Level:   "ERROR",
						})
				}
				params = append(params, name)
			}

			// now validate each identifier in the macro body
			for _, loc := range reIdent.FindAllStringIndex(macroBody, -1) {
				ident := macroBody[loc[0]:loc[1]]
				// ignore use of the macro's own name and parameters
				skip := ident == macroName
				for _, p := range params {
					if ident == p {
						skip = true
						break
					}
				}
				if skip {
					continue
				}
				// trigger error if not snake_lower_case
				if !snakePattern.MatchString(ident) {
					pos := strings.Index(line, ident)
					errs = append(errs, StyleError{
						LineNum: i+1,
						Start:   pos,
						Length:  len(ident),
						Message: fmt.Sprintf("identifier '%s' in macro body must be snake_lower_case", ident),
						Level:   "ERROR",
					})
				}
			}
		}

		// Differentiate between pointers and operators
		var pointerRanges [][]int
		for _, rePtr := range pointerRegexes {
			if locs := rePtr.FindAllStringIndex(codeOnly, -1); locs != nil {
				pointerRanges = append(pointerRanges, locs...)
			}
		}

		inPtrRange := func(s, e int) bool {
			for _, r := range pointerRanges {
				if s >= r[0] && e <= r[1] {
					return true
				}
			}
			return false
		}
		opPattern := regexp.MustCompile(
            `>=|<=|==|!=|\+=|-=|\*=|/=|%=` + // binary operators of two chars
            `|\+\+|--` +                    // unary operators
            `|[=+\-*/%<>?:]`,                // rest of single-char operators
        )

// iterate over each operator found
for _, loc := range opPattern.FindAllStringIndex(codeOnly, -1) {
    op := codeOnly[loc[0]:loc[1]]
    startIdx := loc[0]           // start in masked code
    endIdx   := loc[1]           // end

    if op == ":" && strings.HasSuffix(trim, ":") &&
       (strings.HasPrefix(trim, "case ") ||
        strings.HasPrefix(trim, "default") ||
        reLabelDecl.MatchString(trim)) {
        continue
    }

    // 2) skip ++ and -- completely
    if op == "++" || op == "--" {
        continue
    }
    // 3) skip '*' that are part of a pointer
    if op == "*" && inPtrRange(startIdx, endIdx) {
        continue
    }

    // 4) check space BEFORE: previous character must not be \S
    if startIdx > 0 && !unicode.IsSpace(rune(codeOnly[startIdx-1])) {
        errs = append(errs, StyleError{
            LineNum: i + 1,
            Start:   startIdx,
            Length:  len(op),
            Message: fmt.Sprintf("operator '%s' must have space before it", op),
            Level:   "ERROR",
        })
    }

    // 5) check space AFTER: next character must not be \S
    if endIdx < len(codeOnly) && !unicode.IsSpace(rune(codeOnly[endIdx])) {
        errs = append(errs, StyleError{
            LineNum: i + 1,
            Start:   startIdx,
            Length:  len(op),
            Message: fmt.Sprintf("operator '%s' must have space after it", op),
            Level:   "ERROR",
        })
    }
}

		// 11) Space before KEYWORDS: there must be a space before '('
		if m := reKeywordNoSpace.FindStringSubmatchIndex(codeOnly); m != nil {
			kw := line[m[0] : m[1]-1]
			errs = append(errs, StyleError{i + 1, m[0], len(kw),
				fmt.Sprintf("keyword '%s' must have a space before '('", kw), "ERROR"})
		}
        for _, loc := range reMagicNumber.FindAllStringIndex(codeOnly, -1) {
    num := codeOnly[loc[0]:loc[1]]
    // ignore if inside enum or define
    if reEnumElement.MatchString(codeOnly) || strings.HasPrefix(trim, "#define") {
        continue
    }
    errs = append(errs, StyleError{
        LineNum: i+1, Start: loc[0], Length: loc[1]-loc[0],
        Message: fmt.Sprintf("magic number '%s' detected; extract to constant", num),
        Level:   "WARNING",
    })
}
// 1) detect start of multiline parameter list
if !inParamBlock && strings.Contains(line, "(") && strings.HasSuffix(trim, ",") {
    // extract function name and validate spacing/pattern
    var loc []int
    if loc = reSplitFuncName.FindStringSubmatchIndex(line); loc == nil {
        loc = reFuncHeader.FindStringSubmatchIndex(line)
    }

    if loc != nil {
        name := line[loc[2]:loc[3]]

        // 1a) check if there's space between "name" and "("
        if loc[3] < len(line) && line[loc[3]] == ' ' {
            errs = append(errs, StyleError{
                LineNum: i + 1,
                Start:   loc[3],
                Length:  1,
                Message: "no space allowed between function name and '('",
                Level:   "ERROR",
            })
        }

        // 1b) validate MODULO_camelCase pattern (except "main")
        if name != "main" && !regexp.MustCompile(`^[A-Z]+_[a-z][A-Za-z0-9]*$`).MatchString(name) {
            pos := strings.Index(line, name)
            errs = append(errs, StyleError{
                LineNum: i + 1,
                Start:   pos,
                Length:  len(name),
                Message: fmt.Sprintf("function name '%s' must follow MODULE_camelCase", name),
                Level:   "ERROR",
            })
        }
    }

    // 1c) now mark the start of the multiline block and skip the rest of indentation/etc. checks
    if pp := strings.Index(line, "("); pp >= 0 {
        paramIndent = ((pp + 2) / 2) * 2
        inParamBlock = true
        continue
    }
}

// 2) if already in a multiline parameter block, just validate indent
if inParamBlock {
    // 2a) validate indent of parameter line
    if indent != paramIndent {
        errs = append(errs, StyleError{
            LineNum: i + 1,
            Start:   0,
            Length:  indent,
            Message: fmt.Sprintf(
                "parameter line should be indented to %d spaces (found %d)",
                paramIndent, indent,
            ),
            Level: "ERROR",
        })
    }

    // 2b) check if parentheses closed (end of multiline block)
    if strings.Contains(trim, ")") {
        inParamBlock = false
    } else {
        // if not closed, validate that it ends with a comma
        if !strings.HasSuffix(trim, ",") {
            errs = append(errs, StyleError{
                LineNum: i + 1,
                Start:   len(line) - 1,
                Length:  1,
                Message: "parameter line must end with ','",
                Level:   "ERROR",
            })
        }
    }

    // no more normal indentation checks in this cycle
    continue
}

				isPureClose  := strings.HasPrefix(trim, "}") && !strings.Contains(trim, "{")
		isInlineClose := strings.Contains(trim, "}") && 
						!strings.Contains(trim, "{") && 
						!strings.HasPrefix(trim, "}") && 
						!reClosingAll.MatchString(codeOnly)
        
		// Count leading spaces (tab = 2 spaces)

        		if trim == "" {
			if indent != 0 {
				errs = append(errs, StyleError{i + 1, 0, indent,
					"blank line must have no indentation", "ERROR"})
			}
			continue
		}
        	if loc := reTrailing.FindStringIndex(line); loc != nil {
		// convert byte-offset to column (runes)
		startCol := utf8.RuneCountInString(line[:loc[0]])
		length := utf8.RuneCountInString(line[loc[0]:loc[1]])
		errs = append(errs, StyleError{
			LineNum: i + 1,
			Start:   startCol,
			Length:  length,
			Message: "whitespace at the end of the line",
			Level:   "ERROR",
		})
	}
        if m := reLabelDecl.FindStringSubmatch(trim); m != nil {
                label := m[1]
                ws    := m[2]
            // ignore case/default
            if label == "case" || label == "default" {
                // fall into normal case/default handling
            } else {
                // 1) must be at indent level 0
                if indent != 0 {
                    errs = append(errs, StyleError{
                        LineNum: i+1,
                        Start:   0,
                        Length:  indent,
                        Message: "label must have no indentation",
                        Level:   "ERROR",
                    })
                }
            if !snakePattern.MatchString(label) {
            // find the column where the name starts
            pos := strings.Index(line, label)
            errs = append(errs, StyleError{
                LineNum: i+1,
                Start:   pos,
                Length:  len(label),
                Message: fmt.Sprintf("label '%s' must be snake_lower_case", label),
                Level:   "ERROR",
            })
        }
            if ws != "" {
            col := strings.Index(line, ":")
            errs = append(errs, StyleError{
                LineNum: i+1,
                Start:   col-len(ws),
                Length:  len(ws)+1,
                Message: "':' must be attached without space to preceding token",
                Level:   "ERROR",
            })
        }
                continue
        }
        }
        
        // validate function name (prototype or definition)
        if m := reFuncDecl.FindStringSubmatchIndex(codeOnly) ; m != nil && !inParamBlock {
            name := line[m[2]:m[3]]
            if name != "main" {
                if !regexp.MustCompile(`^[A-Z]+_[a-z][a-zA-Z0-9]*$`).MatchString(name) {
                    errs = append(errs, StyleError{i + 1, m[2], len(name),
                        fmt.Sprintf("function name '%s' must follow MODULE_camelCase", name), "ERROR"})
                }
            }
        }
        
if i > 0 {
    prevLine := strings.TrimSpace(lines[i-1])
    if reOnlyType.MatchString(prevLine) {
        if m := reSplitFuncName.FindStringSubmatch(line); m != nil {
            name := m[1]
            // ignore main
            if name != "main" && !regexp.MustCompile(`^[A-Z]+_[a-z][A-Za-z0-9]*$`).MatchString(name) {
                pos := strings.Index(line, name)
                errs = append(errs, StyleError{
                    LineNum: i + 1,
                    Start:   pos,
                    Length:  len(name),
                    Message: fmt.Sprintf("function name '%s' must follow MODULE_camelCase", name),
                    Level:   "ERROR",
                })
            }
        }
    }
}

		// detect "type" alone followed by "name(parameters)" on the next line
		if reOnlyType.MatchString(line) && i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if reFuncNameOnly.MatchString(next) {
				errs = append(errs, StyleError{
					LineNum: i + 1,
					Start:   0,
					Length:  utf8.RuneCountInString(trim),
					Message: "return type must be on the same line as the function name",
					Level:   "ERROR",
				})
			}
		}

        		// parentheses attached to function calls
                		if matches := reIdentSpaceParen.FindAllStringSubmatchIndex(line, -1); matches != nil {
                    			for _, m := range matches {
                    				name := line[m[2]:m[3]]
                    				// Ignore pre-defined keywords
                    				if keywords[name] {
                    					continue
                    				}
                    
                    				// Define error message based on whether it's a function or macro
                    				msg := "space before '(' in function call is not allowed"
                    				if screamingSnakePattern.MatchString(name) {
                    					msg = "space before '(' in macro call is not allowed"
                    				}
                    
                    				errs = append(errs, StyleError{
                    					LineNum: i + 1,
                    					Start:   m[2],
                    					Length:  m[3] - m[2],
                    					Message: msg,
                    					Level:   "ERROR",
                    				})
                    			}
                    		}

		// ——— BLOCK CLOSURE ———
			if isPureClose && len(indentStack) > 1 {
				indentStack = indentStack[:len(indentStack)-1]
			}
			if isInlineClose && len(indentStack) > 1 {
    		indentStack = indentStack[:len(indentStack)-1]
			}

		// ——— calculating expected indent ———

        // ─── 1) DETECTING case/default ───
        if strings.HasPrefix(trim, "case ") || strings.HasPrefix(trim, "default") && strings.HasSuffix(trim, ":") {

            // 1a) signal error of space before ':'
            nextIdx := i + 1
            for nextIdx < len(lines) {
                nt := strings.TrimSpace(lines[nextIdx])
                if nt == "" || strings.HasPrefix(nt, "//") {
                    nextIdx++
                    continue
                }
                break
            }
            if nextIdx < len(lines) {
                nextTrim := strings.TrimSpace(lines[nextIdx])
                if strings.HasPrefix(nextTrim, "case ") || (strings.HasPrefix(nextTrim, "default") && strings.HasSuffix(nextTrim, ":")) {
                    continue
                }
            }
            
            hasBraceSame := strings.Contains(trim, "{")
            hasBraceNext := false
            for k := i+1; k < len(lines); k++ {
                nt := strings.TrimSpace(lines[k])
                if nt == "" || strings.HasPrefix(nt, "//") { continue }
                hasBraceNext = (nt == "{")
                break
            }

            if hasBraceSame || hasBraceNext {
                // warn and exit **before** any indentation stack push
                
                errs = append(errs, StyleError{
                    LineNum: i+1,
                    Start:   strings.Index(line, "{"),
                    Length:  1,
                    Message: "case blocks must not use '{ }'",
                    Level:   "WARNING",
                })

                continue
            }

            if strings.Contains(trim, " :") {
                col := strings.Index(line, " :")
                errs = append(errs, StyleError{
                    LineNum: i + 1, Start: col, Length: 2,
                    Message: "':' must be attached to 'case' without space",
                    Level:   "ERROR",
                })
            }
            // 1b) look for break; ahead
            found := -1
            for j := i + 1; j < len(lines); j++ {
                t := strings.TrimSpace(lines[j])
                if strings.HasPrefix(t, "case ") || t == "default:" {
                    break // other case → abort search
                }
                if t == "break;" {
                    found = j
                    break
                }
            }
if found == -1 {
        // search for fall-through comment in subsequent lines, until next case/default or code
        hasFallThrough := false
        for k := i + 1; k < len(lines); k++ {
            nextTrim := strings.TrimSpace(lines[k])
            // if we reach another case/default, stop searching
            if strings.HasPrefix(nextTrim, "case ") || nextTrim == "default:" {
                break
            }
// accept line comment or block containing "fall-through"
        if (strings.HasPrefix(nextTrim, "//") || strings.HasPrefix(nextTrim, "/*")) &&
           strings.Contains(nextTrim, "fall-through") {
            hasFallThrough = true
            break
        }
        // if we find code (neither comment nor blank), stop searching
        if nextTrim != "" && !strings.HasPrefix(nextTrim, "//") && !strings.HasPrefix(nextTrim, "/*") {
            break
        }
        }

        if !hasFallThrough {
            errs = append(errs, StyleError{
                LineNum: i+1,
                Start:   strings.Index(line, ":"),
                Length:  1,
                Message: fmt.Sprintf("'%s' block must end with a break; or have a '// fall-through' comment", strings.TrimRight(trim, ":")),
                Level:   "WARNING",
            })
        }
    } else {
        // if we found break;, push normal indent
        caseIndentLevel = indent
        indentStack = append(indentStack, caseIndentLevel+2)
        caseEndLine = found
    }

            continue
        }
		expected := indentStack[len(indentStack)-1]
		if nextIndent >= 0 && !strings.HasPrefix(trim, "{") && trim != "}" && !reInlineBlock.MatchString(trim) {
			expected = nextIndent
		}

				if nextIndent >= 0 &&
		( strings.HasPrefix(trim, "{") || reInlineBlock.MatchString(trim) ) {
		nextIndent = -1
		}

		// ——— signal error, but adjust for the rest of the flow ———
		indentForStack := indent
		if indent != expected && !isInlineClose {
			errs = append(errs, StyleError{
					LineNum: i + 1,
					Start:   0,
					Length:  indent,
					Message: fmt.Sprintf("indentation expected: %d spaces, found %d", expected, indent),
					Level:   "ERROR",
				})
			// here we force that, from then on, use the correct value
			indentForStack = expected
		}
                // ─── 4) CLOSE pseudo-block at break; ───
        
		// ——— reset temporary nextIndent ———
		if nextIndent >= 0 && !strings.HasPrefix(trim, "{") {
			nextIndent = -1
		}

        if i == caseEndLine {
            // pop
            if len(indentStack) > 1 {
                indentStack = indentStack[:len(indentStack)-1]
            }
            // clear case flag
            caseEndLine = -1
            // put nextIndent back to normal
            nextIndent = -1

            continue
        }

		// ——— OPEN BLOCK ———
		if strings.Contains(trim, "{") && !strings.Contains(trim, "}") && !reInlineBlock.MatchString(trim) {
			nextIdx := i + 1
			for nextIdx < len(lines) {
				nxt := strings.TrimSpace(lines[nextIdx])
				if nxt == "" || strings.HasPrefix(nxt, "//") || strings.HasPrefix(nxt, "/*") {
					nextIdx++
					continue
				}
				// skip comments
				if strings.HasPrefix(nxt, "//") || strings.HasPrefix(nxt, "/*") {
					nextIdx++
					continue
				}

			if reCloseBrace.MatchString(nxt) {
                indentStack = append(indentStack, indentForStack)
            } else {
                // normal block: increase +2
                indentStack = append(indentStack, indentForStack+2)
            }
            break
        }
        if nextIdx >= len(lines) {
            indentStack = append(indentStack, indentForStack+2)
        }
	}
		
		// ——— CONTROL WITHOUT BRACES (if/else/for/while/etc) ———
        if reControlStmt.MatchString(trim) && !strings.Contains(trim, "{") {
            // if there's inline statement (if(...) stmt;), skip the next line's indent:
            if !reInlineStmt.MatchString(trim) {
                nextIndent = indentForStack + 2
            }
		}

		// if the line is now only spaces/masked code, skip the rest
		if strings.TrimSpace(codeOnly) == "" {
			continue
		}

        // ——— UNIFIED HANDLING OF INLINE ———
        if reInlineBlock.MatchString(trim) || reInlineStmt.MatchString(trim) {
            // 0a) space between keyword (if/else/for/...) and '('
            if m := reControlParenNoSpace.FindStringIndex(trim); m != nil {
                stmtIdx := strings.Index(line, trim)
                pos := stmtIdx + (m[1] - 1)
                errs = append(errs, StyleError{
                    LineNum: i+1,
                    Start:   pos,
                    Length:  1,
                    Message: "expected space between keyword and '('",
                    Level:   "ERROR",
                })
            }
            // 0b) space after ')'
            if rels := reNoSpaceAfterParen.FindStringIndex(trim); rels != nil {
                stmtIdx := strings.Index(line, trim)
                pos := stmtIdx + (rels[0] + 1)
                errs = append(errs, StyleError{
                    LineNum: i+1,
                    Start:   pos,
                    Length:  1,
                    Message: "expected space after ')'",
                    Level:   "ERROR",
                })
            }

            // Extract inner/innerOffset
            var inner string
            var innerOffset int
            hasBrace := strings.Index(line, "{") >= 0
            if hasBrace {
                b := strings.Index(line, "{")
                e := strings.LastIndex(line, "}")
                innerOffset = b + 1
                inner       = line[innerOffset:e]
            } else {
                p := strings.Index(trim, ")")
                stmtIdx := strings.Index(line, trim)
                innerOffset = stmtIdx + p + 1
                inner       = trim[p+1:]
            }

            // If empty block "{ }", skip all inner checks
			if hasBrace && inner == "" {
				// "{" is at b, so Start = b, Length = 2 for "{}"
				errs = append(errs, StyleError{
					LineNum: i+1,
                    Start:   strings.Index(line, "{"),
                    Length:  2,
                    Message: "{} must have a space: use { }",
					Level:   "ERROR",
				})
				continue
			}

            // If it's an inline block, check spacing around the content
            if hasBrace {
                if inner[0] != ' ' {
                    errs = append(errs, StyleError{
                        LineNum: i+1,
                        Start:   innerOffset,
                        Length:  1,
	                    Message: "expected space after '{'",
                        Level:   "ERROR",
                    })
                }
                if inner[len(inner)-1] != ' ' {
                    errs = append(errs, StyleError{
                        LineNum: i+1,
                        Start:   innerOffset + len(inner) - 1,
                        Length:  1,
                        Message: "expected space before '}'",
                        Level:   "ERROR",
                    })
                }
            }

            // 1) prohibit nested blocks
            if hasBrace {
                if idx := strings.IndexAny(inner, "{}"); idx >= 0 {
                    errs = append(errs, StyleError{
                        LineNum: i+1,
                        Start:   innerOffset + idx,
                        Length:  1,
                        Message: "inline block must not contain nested braces",
                        Level:   "ERROR",
                    })
                }
            }

            // 2) exactly one statement (if there's content)
            if strings.TrimSpace(inner) != "" {
                parts := strings.Split(inner, ";")
                cnt := 0
                for _, s := range parts {
                    if strings.TrimSpace(s) != "" {
                        cnt++
                    }
                }
                if cnt != 1 {
                    errs = append(errs, StyleError{
                        LineNum: i+1,
                        Start:   innerOffset,
                        Length:  len(inner),
                        Message: "inline block must contain exactly one statement",
                        Level:   "ERROR",
                    })
                }
            }

            // 3) prohibit control statements inside

            if m2 := reInnerControl.FindStringIndex(inner); m2 != nil {
                errs = append(errs, StyleError{
                    LineNum: i+1,
                    Start:   innerOffset + m2[0],
                    Length:  m2[1] - m2[0],
                    Message: "inline block must not contain control statements",
                    Level:   "ERROR",
                })
            }

            // skip the rest of TODO (indent, blocks, etc)
            continue
        }
		// detect start of struct/enum
		var ctx *typeCtx
		
        if len(typeStack) > 0 {
            ctx = &typeStack[len(typeStack)-1]
        }

        // detect opening of a struct/enum/union block
		if m := reTypeStart.FindStringSubmatch(trim); m != nil {
			pendingTypeDecl = true
			pendingTypedef  = m[1] != ""
			typeKind        = m[2]              // "struct"|"enum"|"union"
			typeTag         = m[3]              // "" if anonymous
			pendingTagLine 	= i + 1
			pendingTagPos  	= strings.Index(lines[i], typeTag)
		} else if m := reTypeStartBrace.FindStringSubmatch(trim); m != nil {
			newCtx := typeCtx{
				isDataStructure: true,
				isTypedef:       m[1] != "",
				dataType:        m[2],
				tagName:         m[3],           // can be "" for anonymous
				tagLine:         i + 1,
				tagPos:          strings.Index(lines[i], m[3]),
			}
			typeStack = append(typeStack, newCtx)
			pendingTypeDecl = false
		} else if pendingTypeDecl && strings.Contains(trim, "{") {
			newCtx := typeCtx{
				isDataStructure: true,
				isTypedef:       pendingTypedef,
				dataType:        typeKind,
				tagName:         typeTag,        // still can be ""
				tagLine:         pendingTagLine,
	 			tagPos:          pendingTagPos,
			}
			typeStack = append(typeStack, newCtx)
			pendingTypeDecl = false
		}

        // handle end-of-block "}" possibly with a name
       // ——— closing of struct/enum/union ———
	if m := reClosingAll.FindStringSubmatchIndex(codeOnly); m != nil && ctx != nil {
    // m[2]/m[3] = span of instance name (if any)
    nameStart, nameEnd := m[2], m[3]
    instanceName := ""
    if nameEnd > nameStart {
        instanceName = line[nameStart:nameEnd]
    }
	bracePos := strings.Index(lines[i], "}")
    if bracePos >= 0 && bracePos+1 < len(lines[i]) && lines[i][bracePos+1] != ' ' {
        errs = append(errs, StyleError{
            LineNum: i+1,
            Start:   bracePos+1,
            Length:  1,
            Message: "expected space after '}'",
            Level:   "ERROR",
        })
    }
    // 1) if it was typedef, require name and suffix _t
    if ctx.isTypedef {
        // no name after brace
        if instanceName == "" {
            // position exactly above '}'
            bracePos := strings.Index(lines[i], "}")
            errs = append(errs, StyleError{
                LineNum: i + 1,
                Start:   bracePos,
                Length:  1,
                Message: fmt.Sprintf("typedef %s must declare a name ending in '_t'", ctx.dataType),
                Level:   "ERROR",
            })
        // name exists but doesn't end with "_t"
        } else if !strings.HasSuffix(instanceName, "_t") || !snakeTypedefPattern.MatchString(instanceName) {
            errs = append(errs, StyleError{
                LineNum: i+1,
                Start:   nameStart,
                Length:  nameEnd-nameStart,
                Message: fmt.Sprintf("typedef name '%s' must be snake_lower_case and end with '_t'", instanceName),
                Level:   "ERROR",
            })
        }
		
    // 2) if it wasn't typedef, but has instance name, validate snake_lower_case
    } else if instanceName != "" {
        if !snakePattern.MatchString(instanceName) {
            errs = append(errs, StyleError{
                LineNum: i+1,
                Start:   nameStart,
                Length:  nameEnd-nameStart,
                Message: fmt.Sprintf("%s instance '%s' must be snake_lower_case", ctx.dataType, instanceName),
                Level:   "ERROR",
            })
        }
	           if strings.HasSuffix(instanceName, "_t") {
                errs = append(errs, StyleError{
                    LineNum: i+1,
                   Start:   nameStart,
                    Length:  nameEnd-nameStart,
                    Message: fmt.Sprintf("%s instance '%s' must not end with '_t'", ctx.dataType, instanceName),
                    Level:   "ERROR",
                })
            }
    }

    // 3) validate tagName of type (always present)
    if ctx.tagName != "" {
        camel := regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
        if !camel.MatchString(ctx.tagName) {
            errs = append(errs, StyleError{
                LineNum: ctx.tagLine,
                Start:   ctx.tagPos,
                Length:  len(ctx.tagName),
                Message: fmt.Sprintf("%s tag '%s' must be camelCase", ctx.dataType, ctx.tagName),
                Level:   "ERROR",
            })
        }
    }

    // pop from stack and skip the rest of the style
    typeStack = typeStack[:len(typeStack)-1]
    continue
}

        // --- enforce internal member naming rules based on ctx ---

        if ctx != nil && ctx.isDataStructure {
            switch ctx.dataType {
            case "enum":
                // enum elements -> SCREAMING_SNAKE_CASE
                if m := reEnumElement.FindStringSubmatchIndex(trim); m != nil {
                    name := trim[m[2]:m[3]]
                    if !screamingSnakePattern.MatchString(name) {
                        start := strings.Index(line, name)
                        errs = append(errs, StyleError{i + 1, start, len(name),
                            fmt.Sprintf("enum element '%s' must be SCREAMING_SNAKE_CASE", name),
                            "ERROR"})
                    }
                }

            case "struct", "union":
                // struct/union fields -> snake_lower_case
                if m := reStructFieldName.FindStringSubmatchIndex(codeOnly); m != nil {
                    name := line[m[2]:m[3]]
                    if !snakePattern.MatchString(name) {
                        errs = append(errs, StyleError{i + 1, m[2], m[3] - m[2],
                            fmt.Sprintf("%s field name '%s' must be snake_lower_case", ctx.dataType, name),
                            "ERROR"})
                    }
                }

            }
        }

		// 4) declaration without initialization (outside of struct/enum)
		if ctx == nil || !ctx.isDataStructure {
			if m := reUninitDecl.FindStringSubmatchIndex(codeOnly); m != nil {
				decl := line[m[4]:m[5]]
				errs = append(errs, StyleError{i + 1, m[4], m[5] - m[4],
					fmt.Sprintf("'%s' declared without initialization", decl), "WARNING"})
			}
		}

   if strings.HasPrefix(trim, "typedef") {
       // skip typedefs — they're handled in another rule
   } else if m := reVarDeclName.FindStringSubmatchIndex(codeOnly); m != nil {
       nameStart, nameEnd := m[2], m[3]
       varName := line[nameStart:nameEnd]
       if strings.HasSuffix(varName, "_t") {
           errs = append(errs, StyleError{
               LineNum: i+1,
               Start:   nameStart,
               Length:  nameEnd - nameStart,
              Message: fmt.Sprintf("variable name '%s' must not end with '_t'", varName),
               Level:   "ERROR",
           })
       }
   }
   if !inParamBlock && reMultiVarDecl.MatchString(codeOnly) {
    pos := strings.Index(line, ",")
    errs = append(errs, StyleError{
        LineNum: i+1, Start: pos, Length: 1,
        Message: "multiple variable declarations not allowed; use one line per variable",
        Level:   "ERROR",
    })
}


		// 2) typedef function pointer outside of block
		if m := reTypedefFuncPtr.FindStringSubmatchIndex(codeOnly); m != nil {
			name := line[m[2]:m[3]]
			if !snakeTypedefPattern.MatchString(name) {
				errs = append(errs, StyleError{i + 1, m[2], m[3] - m[2],
					fmt.Sprintf("typedef name '%s' must be snake_lower_case and end with '_t'", name),
					"WARNING"})
			}
		}

		// 3) generic typedef outside of block
		if m := reTypedefGeneric.FindStringSubmatchIndex(codeOnly); m != nil {
			name := line[m[2]:m[3]]
			if !snakeTypedefPattern.MatchString(name) {
				errs = append(errs, StyleError{i + 1, m[2], m[3] - m[2],
					fmt.Sprintf("typedef name '%s' must be snake_lower_case and end with '_t'", name),
					"WARNING"})
			}
		}

		// 3.1) defines/macros in SCREAMING_SNAKE_CASE
		if m := reDefine.FindStringSubmatchIndex(codeOnly); m != nil {
			name := line[m[2]:m[3]]
			if !screamingSnakePattern.MatchString(name) {
				errs = append(errs, StyleError{i + 1, m[2], m[3] - m[2],
					fmt.Sprintf("macro name '%s' must be SCREAMING_SNAKE_CASE", name),
					"ERROR"})
			}
		}

        if m := reFuncMacro.FindStringSubmatch(line); m != nil {
            body := m[2]
            if !(strings.HasPrefix(body, "(") && strings.HasSuffix(body, ")")) {
                pos := strings.Index(line, body)
                errs = append(errs, StyleError{
                    LineNum: i+1,
                    Start:   pos,
                    Length:  len(body),
                    Message: "function-like macro body must be parenthesized, e.g. ((x)*(x))",
                    Level:   "ERROR",
                })
            }
        }

		// function parameters
		if m := reFuncSignature.FindStringSubmatch(line); m != nil {
			paramList := m[1]
			for _, p := range strings.Split(paramList, ",") {
				p = strings.TrimSpace(p)
				if p == "" || p == "void" {
					continue
				}
				if mn := reParamName.FindStringSubmatch(p); mn != nil {
					name := mn[1]
					idx := strings.Index(line, name)
					if !snakePattern.MatchString(name) {
						errs = append(errs, StyleError{i + 1, idx, len(name),
							fmt.Sprintf("parameter name '%s' must be snake_lower_case", name), "ERROR"})
					}
				}
			}
		}

		// X) missing space before '{' in control statements

		    if strings.Contains(line, "?") {
        // '?' without space before
        for _, loc := range reTernaryQNoSpaceBefore.FindAllStringIndex(codeOnly, -1) {
            errs = append(errs, StyleError{i + 1, loc[0]+1, 1,
                "operator '?' must have space before it", "ERROR"})
        }
        // '?' without space after
        for _, loc := range reTernaryQNoSpaceAfter.FindAllStringIndex(codeOnly, -1) {
            errs = append(errs, StyleError{i + 1, loc[0], 1,
                "operator '?' must have space after it", "ERROR"})
        }
        // ':' without space before (only in ternary)
        for _, loc := range reTernaryColonNoSpaceBefore.FindAllStringIndex(codeOnly, -1) {
            errs = append(errs, StyleError{i + 1, loc[0]+1, 1,
                "operator ':' must have space before it", "ERROR"})
        }
        // ':' without space after (only in ternary)
        for _, loc := range reTernaryColonNoSpaceAfter.FindAllStringIndex(codeOnly, -1) {
            errs = append(errs, StyleError{i + 1, loc[0], 1,
                "operator ':' must have space after it", "ERROR"})
        }
    }
		// 10a) Functions: always Allman (open-brace on their own line)
		if strings.Contains(line, "{") && reFuncDecl.MatchString(codeOnly) {
			pos := strings.Index(line, "{")
			errs = append(errs, StyleError{i + 1, pos, 1,
				"function opening must be on its own line", "ERROR"})
		}

        for i := 0; i < len(lines); i++ {
            if reFuncDecl.MatchString(codeOnly) {
                // we've reached the line that starts the function; now find closing "}"
                braceDepth := 0
                j := i
                for ; j < len(lines); j++ {
                    if strings.Contains(lines[j], "{") {
                        braceDepth++
                    }
                    if strings.Contains(lines[j], "}") {
                        braceDepth--
                        if braceDepth == 0 {
                            break
                        }
                    }
                }
                // j is now the line of the "}"
                // check for blank lines after j
                blankCount := 0
                k := j + 1
                for k < len(lines) && strings.TrimSpace(lines[k]) == "" {
                    blankCount++
                    k++
                }
                if k < len(lines) && reFuncDecl.MatchString(/*apenas checar se a próxima definição é func*/ lines[k]) {
                    if blankCount == 0 {
                        errs = append(errs, StyleError{
                            LineNum: j+1,
                            Start:   0,
                            Length:  0,
                            Message: "missing blank line after function definition",
                            Level:   "ERROR",
                        })
                    } else if blankCount > 1 {
                        errs = append(errs, StyleError{
                            LineNum: j+2,
                            Start:   0,
                            Length:  0,
                            Message: fmt.Sprintf("more than one blank line (%d) between functions", blankCount),
                            Level:   "WARNING",
                        })
                    }
                }
                // skip i to j to avoid reprocessing inner lines
                i = j
            }
        }

		// 10b) Other blocks controlled by styleMode
		if styleMode == "allman" {
			// Allman: no "{" on the same line as control/type
			if reControlStmt.MatchString(codeOnly) && strings.Contains(line, "{") {
				pos := strings.Index(line, "{")
				kind := reControlStmt.FindString(line)
				errs = append(errs, StyleError{i + 1, pos, 1,
					fmt.Sprintf("opening brace must be on its own line (%s)", kind), "ERROR"})
			}
		} else {
			if reControlStmt.MatchString(codeOnly) {
				if idx := strings.Index(line, "){"); idx != -1 {
					errs = append(errs, StyleError{i + 1, idx + 1, 1,
						"missing space before '{' in control statement", "ERROR"})
				}
			}
			// K&R: no "{" alone on line below control/type
			if reBraceOnlyLine.MatchString(codeOnly) && i > 0 &&
				reControlStmt.MatchString(lines[i-1]) {
				pos := strings.Index(line, "{")
				kind := reControlStmt.FindString(lines[i-1])
				errs = append(errs, StyleError{i + 1, pos, 1,
					fmt.Sprintf("opening brace must be on the same line as %s", kind), "ERROR"})
			}
		}
		    // 11) New rule: FORCE consistency of {} or absence of them
        // -- single inline: { stmt }  OK
        if reInlineBlock.MatchString(trim) {
            // all good
        } else if strings.Contains(trim, "{") {
            // there's "{" but no "}" on the same line
            // we'll look for the corresponding "}"
            braceDepth := 1
            stmtLines := 0
            for j := i+1; j < len(lines) && braceDepth > 0; j++ {
                t := strings.TrimSpace(lines[j])
                // ignore comments and blank lines
                if t == "" || strings.HasPrefix(t, "//") {
                    continue
                }
                if strings.Contains(t, "{") {
                    braceDepth++
                }
                if strings.Contains(t, "}") {
                    braceDepth--
                    // if it closes and has code on the same line → error
                                    if braceDepth == 0 &&
                   !reCloseBrace.MatchString(t) &&
                  !reClosingAll.MatchString(t) { 
                        pos := strings.Index(lines[j], "}")
                        errs = append(errs, StyleError{j+1, pos, 1,
                            "closing brace must be on its own line", "ERROR"})
                    }
                    break
                }
                // only count stmt lines if still inside the block
                if braceDepth > 0 {
                    stmtLines++
                }
            }
        } 
        if reAllocCall.MatchString(codeOnly) && !reAllocCast.MatchString(codeOnly) {
        loc := reAllocCall.FindStringIndex(codeOnly)[0]
        name := reAllocCall.FindString(codeOnly)
        errs = append(errs, StyleError{
            LineNum: i+1,
            Start:   loc,
            Length:  len(name),
            Message: fmt.Sprintf("allocation via %s must be cast to the target pointer type", name),
            Level:   "ERROR",
        })
    }
		checkUnsafeFunctions(codeOnly, i+1, &errs)
        checkConstPointerParams(lines, &errs)
	}

	return errs
}

func printContext(lines []string, err StyleError) {
	start := err.LineNum - 2
	if start < 0 {
		start = 0
	}
	end := err.LineNum
	if end >= len(lines) {
		end = len(lines) - 1
	}
	for i := start; i <= end; i++ {
		line := lines[i]
		fmt.Printf("%s%3d%s %s|%s ", LineNumCol, i+1, Reset, PipeCol, Reset)
		if i == err.LineNum-1 {
			fmt.Println(highlightError(line, err.Start, err.Length))
		} else {
			fmt.Println(highlightLine(line))
		}
	}
}

func highlightError(line string, start, length int) string {
	r := []rune(line)
	n := len(r)
    // clamp start
    if start < 0 {
        start = 0
    } else if start > n {
        start = n
    }

    // clamp length
    if length < 0 {
        length = 0
    }
    if start+length > n {
        length = n - start
    }
	before := string(r[:start])
	errPart := string(r[start : start+length])
	after := string(r[start+length:])
	return highlightLine(before) + ErrorBg + Operator + errPart + Reset + highlightLine(after)
}

// helper to know if we're closing the expected type
func matchingOpen(br, open rune) bool {
    switch br {
    case '}': return open == '{'
    case ')': return open == '('
    case ']': return open == '['
    }
    return false
}

func highlightLine(line string) string {
    // 1) Macro with parameters: handle and recurse on the rest
    if loc := reMacroDefLine.FindStringSubmatchIndex(line); loc != nil {
        before     := line[:loc[0]]             // indentation
        directive  := line[loc[2]:loc[3]]       // "#define"
        macroName  := line[loc[4]:loc[5]]       // name of the macro
        paramsInner := ""
        if loc[6] != -1 {
            paramsInner = line[loc[6]:loc[7]]   // text inside (...)
        }
        rest := line[loc[1]:]                   // everything after the signature

        var sb strings.Builder
        sb.WriteString(before)
        sb.WriteString(DefineCol + directive + Reset + " ")
        sb.WriteString(Function + macroName + Reset)
        if paramsInner != "" {
            sb.WriteString(Brackets + "(" + Reset)
            sb.WriteString(Variable + paramsInner + Reset)
            sb.WriteString(Brackets + ")" + Reset)
        }
        sb.WriteString(highlightLine(rest))
        return sb.String()
    }
    
    includeRE := regexp.MustCompile(`^\s*(#\s*include)\s+([<"].+[>"])`)
    if m := includeRE.FindStringSubmatchIndex(line); m != nil {
        before    := line[:m[0]]           // indentation
        directive := line[m[2]:m[3]]       // "# include"
        path      := line[m[4]:m[5]]       // "<stdio.h>" or "\"oi.h\""
        after     := line[m[5]:]           // rest of the line
        return before +
            DefineCol + directive + Reset + " " +
            StringC + path + Reset +
            after  // don't call highlightLine( after ) — keep the text raw
    }

    // 2) Normal highlight + rainbow brackets
    var sb    strings.Builder
    var stack []rune
    r := []rune(line)

    for i := 0; i < len(r); {
        ch := r[i]

        // comment lines "//..."
        if ch == '/' && i+1 < len(r) && r[i+1] == '/' {
            sb.WriteString(Comment + string(r[i:]) + Reset)
            break
        }

        // string literals "..."
        if ch == '"' {
            start := i; i++
            for i < len(r) && !(r[i] == '"' && r[i-1] != '\\') {
                i++
            }
            if i < len(r) { i++ }
            sb.WriteString(StringC + string(r[start:i]) + Reset)
            continue
        }

        // numbers
        if unicode.IsDigit(ch) {
            start := i
            for i < len(r) && (unicode.IsDigit(r[i]) || r[i] == '.') {
                i++
            }
            sb.WriteString(Number + string(r[start:i]) + Reset)
            continue
        }

        // rainbow brackets
        switch ch {
        case '(', '{', '[':
            stack = append(stack, ch)
            depth := len(stack) - 1
            color := rainbowColors[depth%len(rainbowColors)]
            sb.WriteString(color + string(ch) + Reset)
            i++
            continue

        case ')', '}', ']':
            if len(stack) > 0 && matchingOpen(ch, stack[len(stack)-1]) {
                depth := len(stack) - 1
                color := rainbowColors[depth%len(rainbowColors)]
                sb.WriteString(color + string(ch) + Reset)
                stack = stack[:len(stack)-1]
            } else {
                sb.WriteString(Brackets + string(ch) + Reset)
            }
            i++
            continue
        }

        // identifiers / keywords / types / function calls
        if unicode.IsLetter(ch) || ch == '_' {
            start := i
            for i < len(r) &&
                (unicode.IsLetter(r[i]) || unicode.IsDigit(r[i]) || r[i] == '_') {
                i++
            }
            word := string(r[start:i])
            switch {
            case keywords[word]:
                sb.WriteString(Keyword + word + Reset)
            case typesMap[word]:
                sb.WriteString(Type + word + Reset)
            case i < len(r) && r[i] == '(':
                sb.WriteString(Function + word + Reset)
            default:
                sb.WriteString(Variable + word + Reset)
            }
            continue
        }

        // spaces
        if unicode.IsSpace(ch) {
            sb.WriteRune(ch)
            i++
            continue
        }

        // operators
        if operatorRunes[ch] {
            sb.WriteString(Operator + string(ch) + Reset)
            i++
            continue
        }

        // any other character
        sb.WriteRune(ch)
        i++
    }

    return sb.String()
}
