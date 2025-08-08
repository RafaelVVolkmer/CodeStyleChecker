package main

/** ===============================================================
 *                          I M P O R T S
 * ================================================================ */
import (
    "bufio"
    "bytes"
    "errors"
    "flag"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strings"
    "unicode"
    "unicode/utf8"
)

/** ===============================================================
 *              T Y P E S  D E F I N I T I O N S
 * ================================================================ */
type ErrorInfo struct {
    Level   string
    Message string
}

type StyleError struct {
    LineNum int
    Start   int
    Length  int
    Message string
    Level   string
}

type typeCtx struct {
    isDataStructure bool
    isTypedef       bool
    dataType        string
    tagName         string
    tagLine         int
    tagPos          int
}

type FileContext struct {
    Filename string
    Lines    []string
    Raw      []byte
    Style    StyleMode
    Errors   []StyleError
}

type ErrorCode int

type StyleMode int

/** ===============================================================
 *              C O N S T  D E F I N I T I O N S
 * ================================================================ */
const (
    LevelError   = "ERROR"
    LevelWarning = "WARNING"
)

const (
    maxLineLength = 80
)

const (
    StyleKR StyleMode = iota
    StyleAllman
)

/** ===============================================================
 *                  E R R O R  M A P P I N G
 * ================================================================ */
const (
    ErrRecursiveInclusion = iota
    ErrSysBeforeProjIncludesOrder
    ErrSysIncludesNotSorted
    ErrProjIncludesNotSorted
    ErrFileMustEndWithNewline
    ErrLineLengthExceeded
    WarnTooManyBlankLinesConsecutively
    ErrNoSpaceBeforeSemicolon
    WarnNonASCIICharacter
    WarnFileEndsWithExtraBlankLines
    WarnFoundTODOOrFIXME
    ErrPragmaOnceAndIncludeGuard
    WarnUseOfInsecureFunction
    WarnPointerNotModifiedMustBeConst
    ErrBlankLineWithIndentation
    ErrTrailingWhitespace
    ErrElseMustBeOnSameLineAsClosingBrace
    ErrIncludeDirectiveIndentation
    ErrNoSpaceAllowedInsideParentheses
    ErrNoSpaceAllowedAroundBrackets
    ErrCommaMustBeSurroundedBySingleSpace
    ErrMultipleConsecutiveSpaces
    ErrPointerFormattingRules
    ErrPointerCastMustBeAttached
    ErrMacroBodyMustHaveSpaceAfterParams
    ErrMacroParamMustBeSnakeCase
    ErrMacroBodyIdentifierMustBeSnakeCase
    ErrOperatorMustHaveSpaceBefore
    ErrOperatorMustHaveSpaceAfter
    ErrKeywordMustHaveSpaceBeforeParen
    WarnMagicNumberDetected
    ErrFuncNameNoSpaceBeforeParen
    ErrFunctionNameMustBeModuleCamelCase
    ErrParameterLineWrongIndent
    ErrParameterLineMustEndWithComma
    ErrLabelMustHaveNoIndentation
    ErrLabelMustBeSnakeLowerCase
    ErrColonMustBeAttachedToToken
    ErrReturnTypeMustBeOnSameLineAsName
    ErrSpaceBeforeFuncCallParen
    ErrFunctionOpeningBraceMustBeOnOwnLine
    ErrMissingBlankLineAfterFunction
    WarnTooManyBlankLinesBetweenFunctions
    ErrAllmanOpeningBraceMustBeOwnLine
    ErrKRMissingSpaceBeforeBrace
    ErrKROpeningBraceMustBeSameLineAsControl
    WarnCaseBlocksMustNotUseBraces
    WarnCaseBlockMissingBreakOrFallthrough
    ErrExpectedSpaceAfterClosingBrace
    ErrInstanceMustBeSnakeLowerCase
    ErrInstanceMustNotEndWithT
    ErrTypeTagMustBeCamelCase
    WarnDeclaredWithoutInitialization
    ErrVariableNameMustNotEndWithT
    ErrMultipleVariableDeclarationsNotAllowed
    WarnTypedefFuncPtrNameMustBeSnakeLowerCaseAndEndWithT
    WarnTypedefGenericNameMustBeSnakeLowerCaseAndEndWithT
    ErrMacroNameMustBeScreamingSnakeCase
    ErrFunctionLikeMacroBodyMustBeParenthesized
    ErrParameterNameMustBeSnakeLowerCase
    ErrTernaryQuestionMarkMustHaveSpaceBefore
    ErrTernaryQuestionMarkMustHaveSpaceAfter
    ErrTernaryColonMustHaveSpaceBefore
    ErrTernaryColonMustHaveSpaceAfter
    ErrInlineEmptyBraceMustHaveSpaces
    ErrInlineBlockMustNotContainNestedBraces
    ErrInlineBlockMustContainOneStatement
    ErrInlineBlockMustNotContainControlStatements
    ErrClosingBraceMustBeOwnLine
    ErrAllocCallMustBeCast
    ErrExpectedSpaceAfterOpeningBrace
    ErrEnumElementMustBeScreamingSnakeCase
    ErrStructFieldMustBeSnakeLowerCase

    NumErrorMessages
)

/** ===============================================================
 *                  E R R O R  M E S S A G E S
 * ================================================================ */
var errorInfos = [NumErrorMessages]ErrorInfo{
    ErrRecursiveInclusion: {
        Level:   LevelError,
        Message: "recursive inclusion of '%s' detected",
    },
    ErrSysBeforeProjIncludesOrder: {
        Level:   LevelError,
        Message: "system includes (<...>) should come before project includes (\"%s\")",
    },
    ErrSysIncludesNotSorted: {
        Level:   LevelError,
        Message: "system includes (<...>) are not in alphabetical order",
    },
    ErrProjIncludesNotSorted: {
        Level:   LevelError,
        Message: "project includes (\"%s\") are not in alphabetical order",
    },
    ErrFileMustEndWithNewline: {
        Level:   LevelError,
        Message: "file must end with a newline",
    },
    ErrLineLengthExceeded: {
        Level:   LevelError,
        Message: "line length must not exceed %d characters, found %d",
    },
    WarnTooManyBlankLinesConsecutively: {
        Level:   LevelWarning,
        Message: "more than 1 blank line consecutively (%d)",
    },
    ErrNoSpaceBeforeSemicolon: {
        Level:   LevelError,
        Message: "no space before ';'",
    },
    WarnNonASCIICharacter: {
        Level:   LevelWarning,
        Message: "unexpected non-ASCII character: 0x%X",
    },
    WarnFileEndsWithExtraBlankLines: {
        Level:   LevelWarning,
        Message: "file ends with %d blank lines; remove excess",
    },
    WarnFoundTODOOrFIXME: {
        Level:   LevelWarning,
        Message: "found TODO/FIXME comment – check pending tasks before submitting",
    },
    ErrPragmaOnceAndIncludeGuard: {
        Level:   LevelError,
        Message: "do not use #pragma once and include-guard simultaneously; choose one",
    },
    WarnUseOfInsecureFunction: {
        Level:   LevelWarning,
        Message: "use of insecure function '%s'; consider using %s",
    },
    WarnPointerNotModifiedMustBeConst: {
        Level:   LevelWarning,
        Message: "pointer '%s' is not modified; consider declaring it 'const %s'",
    },
    ErrBlankLineWithIndentation: {
        Level:   LevelError,
        Message: "blank line must have no indentation",
    },
    ErrTrailingWhitespace: {
        Level:   LevelError,
        Message: "whitespace at the end of the line",
    },
    ErrElseMustBeOnSameLineAsClosingBrace: {
        Level:   LevelError,
        Message: `"else" must be on the same line as the closing '}' (K&R style)`,
    },
    ErrIncludeDirectiveIndentation: {
        Level:   LevelError,
        Message: "include directive must have no indentation",
    },
    ErrNoSpaceAllowedInsideParentheses: {
        Level:   LevelError,
        Message: "no space allowed inside parentheses",
    },
    ErrNoSpaceAllowedAroundBrackets: {
        Level:   LevelError,
        Message: "no space allowed after '[' or before ']'",
    },
    ErrCommaMustBeSurroundedBySingleSpace: {
        Level:   LevelError,
        Message: "comma must be followed by a space and preceded by a space",
    },
    ErrMultipleConsecutiveSpaces: {
        Level:   LevelError,
        Message: "multiple consecutive spaces between tokens",
    },
    ErrPointerFormattingRules: {
        Level: LevelError,
        Message: "pointer must be formatted as:\n" +
            "- 'type *ptr' for declarations\n" +
            "- '*ptr' or '*type' for dereferences\n" +
            "- 'type *' for casting",
    },
    ErrPointerCastMustBeAttached: {
        Level: LevelError,
        Message: "pointer cast must be attached to the operand:\n" +
            "- use '(t *)x' or '(t *)(x)', not '(t *) x'",
    },
    ErrMacroBodyMustHaveSpaceAfterParams: {
        Level:   LevelError,
        Message: "macro body must be preceded by a space after parameter list",
    },
    ErrMacroParamMustBeSnakeCase: {
        Level:   LevelError,
        Message: "macro parameter '%s' must be snake_lower_case",
    },
    ErrMacroBodyIdentifierMustBeSnakeCase: {
        Level:   LevelError,
        Message: "identifier '%s' in macro body must be snake_lower_case",
    },
    ErrOperatorMustHaveSpaceBefore: {
        Level:   LevelError,
        Message: "operator '%s' must have space before it",
    },
    ErrOperatorMustHaveSpaceAfter: {
        Level:   LevelError,
        Message: "operator '%s' must have space after it",
    },
    ErrKeywordMustHaveSpaceBeforeParen: {
        Level:   LevelError,
        Message: "keyword must have a space before '('",
    },
    WarnMagicNumberDetected: {
        Level:   LevelWarning,
        Message: "magic number '%s' detected; extract to constant",
    },
    ErrFuncNameNoSpaceBeforeParen: {
        Level:   LevelError,
        Message: "no space allowed between function name and '('",
    },
    ErrFunctionNameMustBeModuleCamelCase: {
        Level:   LevelError,
        Message: "function name '%s' must follow MODULE_camelCase",
    },
    ErrParameterLineWrongIndent: {
        Level:   LevelError,
        Message: "parameter line should be indented to %d spaces (found %d)",
    },
    ErrParameterLineMustEndWithComma: {
        Level:   LevelError,
        Message: "parameter line must end with ','",
    },
    ErrLabelMustHaveNoIndentation: {
        Level:   LevelError,
        Message: "label must have no indentation",
    },
    ErrLabelMustBeSnakeLowerCase: {
        Level:   LevelError,
        Message: "label '%s' must be snake_lower_case",
    },
    ErrColonMustBeAttachedToToken: {
        Level:   LevelError,
        Message: "':' must be attached without space to preceding token",
    },
    ErrReturnTypeMustBeOnSameLineAsName: {
        Level:   LevelError,
        Message: "return type must be on the same line as the function name",
    },
    ErrSpaceBeforeFuncCallParen: {
        Level:   LevelError,
        Message: "space before '(' in function call is not allowed",
    },
    ErrFunctionOpeningBraceMustBeOnOwnLine: {
        Level:   LevelError,
        Message: "function opening must be on its own line",
    },
    ErrMissingBlankLineAfterFunction: {
        Level:   LevelError,
        Message: "missing blank line after function definition",
    },
    WarnTooManyBlankLinesBetweenFunctions: {
        Level:   LevelWarning,
        Message: "more than one blank line (%d) between functions",
    },
    ErrAllmanOpeningBraceMustBeOwnLine: {
        Level:   LevelError,
        Message: "opening brace must be on its own line (%s)",
    },
    ErrKRMissingSpaceBeforeBrace: {
        Level:   LevelError,
        Message: "missing space before '{' in control statement",
    },
    ErrKROpeningBraceMustBeSameLineAsControl: {
        Level:   LevelError,
        Message: "opening brace must be on the same line as %s",
    },
    WarnCaseBlocksMustNotUseBraces: {
        Level:   LevelWarning,
        Message: "case blocks must not use '{ }'",
    },
    WarnCaseBlockMissingBreakOrFallthrough: {
        Level:   LevelWarning,
        Message: "'%s' block must end with a break; or have a '// fall-through' comment",
    },
    ErrExpectedSpaceAfterClosingBrace: {
        Level:   LevelError,
        Message: "expected space after '}'",
    },
    ErrInstanceMustBeSnakeLowerCase: {
        Level:   LevelError,
        Message: "%s instance '%s' must be snake_lower_case",
    },
    ErrInstanceMustNotEndWithT: {
        Level:   LevelError,
        Message: "%s instance '%s' must not end with '_t'",
    },
    ErrTypeTagMustBeCamelCase: {
        Level:   LevelError,
        Message: "%s tag '%s' must be camelCase",
    },
    WarnDeclaredWithoutInitialization: {
        Level:   LevelWarning,
        Message: "'%s' declared without initialization",
    },
    ErrVariableNameMustNotEndWithT: {
        Level:   LevelError,
        Message: "variable name '%s' must not end with '_t'",
    },
    ErrMultipleVariableDeclarationsNotAllowed: {
        Level:   LevelError,
        Message: "multiple variable declarations not allowed; use one line per variable",
    },
    WarnTypedefFuncPtrNameMustBeSnakeLowerCaseAndEndWithT: {
        Level:   LevelWarning,
        Message: "typedef name '%s' must be snake_lower_case and end with '_t'",
    },
    WarnTypedefGenericNameMustBeSnakeLowerCaseAndEndWithT: {
        Level:   LevelWarning,
        Message: "typedef name '%s' must be snake_lower_case and end with '_t'",
    },
    ErrMacroNameMustBeScreamingSnakeCase: {
        Level:   LevelError,
        Message: "macro name '%s' must be SCREAMING_SNAKE_CASE",
    },
    ErrFunctionLikeMacroBodyMustBeParenthesized: {
        Level:   LevelError,
        Message: "function-like macro body must be parenthesized, e.g. ((x)*(x))",
    },
    ErrParameterNameMustBeSnakeLowerCase: {
        Level:   LevelError,
        Message: "parameter name '%s' must be snake_lower_case",
    },
    ErrTernaryQuestionMarkMustHaveSpaceBefore: {
        Level:   LevelError,
        Message: "operator '?' must have space before it",
    },
    ErrTernaryQuestionMarkMustHaveSpaceAfter: {
        Level:   LevelError,
        Message: "operator '?' must have space after it",
    },
    ErrTernaryColonMustHaveSpaceBefore: {
        Level:   LevelError,
        Message: "operator ':' must have space before it",
    },
    ErrTernaryColonMustHaveSpaceAfter: {
        Level:   LevelError,
        Message: "operator ':' must have space after it",
    },
    ErrInlineEmptyBraceMustHaveSpaces: {
        Level:   LevelError,
        Message: "{} must have a space: use { }",
    },
    ErrInlineBlockMustNotContainNestedBraces: {
        Level:   LevelError,
        Message: "inline block must not contain nested braces",
    },
    ErrInlineBlockMustContainOneStatement: {
        Level:   LevelError,
        Message: "inline block must contain exactly one statement",
    },
    ErrInlineBlockMustNotContainControlStatements: {
        Level:   LevelError,
        Message: "inline block must not contain control statements",
    },
    ErrClosingBraceMustBeOwnLine: {
        Level:   LevelError,
        Message: "closing brace must be on its own line",
    },
    ErrAllocCallMustBeCast: {
        Level:   LevelError,
        Message: "allocation via %s must be cast to the target pointer type",
    },
    ErrExpectedSpaceAfterOpeningBrace: {
        Level:   LevelError,
        Message: "expected space after '{'",
    },
    ErrEnumElementMustBeScreamingSnakeCase: {
        Level:   LevelError,
        Message: "enum element '%s' must be SCREAMING_SNAKE_CASE",
    },
    ErrStructFieldMustBeSnakeLowerCase: {
        Level:   LevelError,
        Message: "%s field name '%s' must be snake_lower_case",
    },
}

/** ===============================================================
 *              R E G E X  D E F I N I T I O N S
 * ================================================================ */
var (
    typeNames      = []string{"int", "char", "float", "double", "long", "short", "bool", "void"}
    typePattern    = "(" + strings.Join(typeNames, "|") + ")"
    typedefPattern = "[A-Za-z_][A-Za-z0-9_]*_t"
    structPattern  = `(?:[A-Z][A-Za-z0-9]*|[a-z][A-Za-z0-9]*[A-Z][A-Za-z0-9]*)`
    typeOrTypedef  = "(?:" + typePattern + "|" + typedefPattern + ")"
    ptrPattern     = `\b(?:` + typePattern + `|[A-Za-z_][A-Za-z0-9_]*_t)\*`
    typeGroup      = "(?:" +
        "\\b" + typePattern + "\\b" + "|" +
        "\\b" + typedefPattern + "\\b" + "|" +
        "\\b" + structPattern + "\\b" +
        ")"
)

var (
    reControlStmt   = regexp.MustCompile(`^\s*(?:typedef\s+)?(if|else|for|while|switch|struct|union|enum)\b`)
    reBraceOnlyLine = regexp.MustCompile(`^\s*\{\s*$`)
    reTodo          = regexp.MustCompile(`\b(?:TODO|FIXME)\b`)
    reClosingAll    = regexp.MustCompile(
        `^\s*\}` +
            `\s*` +
            `([A-Za-z_][A-Za-z0-9_]*)?` +
            `\s*;?\s*$`,
    )
    reSplitFuncName = regexp.MustCompile(
        `^\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(`,
    )
    snakePattern          = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
    snakeTypedefPattern   = regexp.MustCompile(`^[a-z][a-z0-9_]*_t$`)
    screamingSnakePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
    reMacroDefLine        = regexp.MustCompile(
        `^\s*` +
            `(#\s*define)` +
            `\s+([A-Za-z_][A-Za-z0-9_]*)` +
            `(?:\(([^\)]*)\))?`,
    )
    reLabelDecl       = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)(\s*):$`)
    reTypedefFuncPtr  = regexp.MustCompile(`^\s*typedef\b.*\(\s*\*\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)`)
    reTypedefGeneric  = regexp.MustCompile(`^\s*typedef\b.*\b([A-Za-z_][A-Za-z0-9_]*)\s*;`)
    reDefine          = regexp.MustCompile(`^\s*#\s*define\s+([A-Za-z_][A-Za-z0-9_]*)`)
    reEnumElement     = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*(?:=|,)\s*`)
    reMultiVarDecl    = regexp.MustCompile(`^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+(?:\*\s*)?[A-Za-z_][A-Za-z0-9_]*\s*,`)
    reStructFieldName = regexp.MustCompile(`^\s*[A-Za-z_][A-Za-z0-9_]*\s*(?:\*\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*;`)
    reBadBracketSpace = regexp.MustCompile(`\[\s+|[ \t]+\]`)
    reVarDeclName     = regexp.MustCompile(
        `^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+(?:\*\s*)?` +
            `([A-Za-z_][A-Za-z0-9_]*)\s*(?:=|;).*`,
    )
    reMacroDef = regexp.MustCompile(
        `^\s*#\s*define\s+` +
            `([A-Za-z_][A-Za-z0-9_]*)\s*` +
            `\(\s*([^)]*)\)\s*` +
            `(.*)$`,
    )
    reIdent          = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
    reKeywordNoSpace = regexp.MustCompile(
        `\b(if|else|for|while|return|break|continue|switch|case|default|static|` +
            `const|extern|unsigned|signed|typedef|struct|union|enum|void|sizeof)\(`,
    )
    reMagicNumber = regexp.MustCompile(`\b([2-9][0-9]*)\b`)
    reFuncMacro   = regexp.MustCompile(
        `^\s*#\s*define\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)\s+(.+)$`,
    )
    reFuncHeader = regexp.MustCompile(
        `^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+([A-Za-z_][A-Za-z0-9_]*)\s*\(`,
    )
    reIdentSpaceParen = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s+\(`)
    reUninitDecl      = regexp.MustCompile(
        `^\s*` + typePattern + `(?:\s*\*\s*|\s+)` +
            `([A-Za-z_][A-Za-z0-9_]*)\s*;\s*$`,
    )
    reTypeDeclNoBrace = regexp.MustCompile(`^\s*(typedef\s+)?(struct|enum)\b.*[^{;]\s*$`)
    reStructEnd       = regexp.MustCompile(`^\s*\}\s*;?\s*$`)
    rePtrDecl         = regexp.MustCompile(`\b` + typePattern + `\s*\*\s*[A-Za-z_][A-Za-z0-9_]*`)
    reCorrectPtr      = regexp.MustCompile(`\b` + typePattern + ` \*[A-Za-z_][A-Za-z0-9_]*\b`)
    reTrailing        = regexp.MustCompile(`[ \t]+$`)
    reMultiSpace      = regexp.MustCompile(`\S( {2,})\S`)
    reFuncDecl        = regexp.MustCompile(
        `^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+` +
            `([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)\s*(?:;|$)`,
    )
    reFuncSignature = regexp.MustCompile(
        `^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+[A-Za-z_][A-Za-z0-9_]*\s*\(([^)]*)\)\s*$`,
    )
    reParamName                 = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)$`)
    reTernaryQNoSpaceBefore     = regexp.MustCompile(`\S\?`)
    reTernaryQNoSpaceAfter      = regexp.MustCompile(`\?\S`)
    reTernaryColonNoSpaceBefore = regexp.MustCompile(`\S:`)
    reTernaryColonNoSpaceAfter  = regexp.MustCompile(`:\S`)
    reCastPtr                   = regexp.MustCompile(`\(\s*(?:int|char|float|double|long|short|bool)\s*\*\s*\)`)
    reDeref                     = regexp.MustCompile(`(?:` + typeGroup + `|\))\s*\*\s*[A-Za-z_][A-Za-z0-9_]*`)
    reGenericPtr                = regexp.MustCompile(`\b(?:void|int|char|float|double|long|short|bool)\s*\*\b`)
    reGenericPtrComma           = regexp.MustCompile(
        `\b(?:void|int|char|float|double|long|short|bool|[A-Za-z_][A-Za-z0-9_]*_t)\s*\*\s*,`,
    )
    reGenericPtrParen = regexp.MustCompile(
        `\b(?:void|int|char|float|double|long|short|bool|[A-Za-z_][A-Za-z0-9_]*_t)\s*\*\s*\)`,
    )
    reBadDeref        = regexp.MustCompile(`\*\s+[A-Za-z_][A-Za-z0-9_]*`)
    reBadCastLeading  = regexp.MustCompile(`\(\s*(?:void|int|char|float|double|long|short|bool)\*\)`)
    reBadParenSpace   = regexp.MustCompile(`\(\s+|[ \t]+\)`)
    reBadComma        = regexp.MustCompile(`\s+,|,\S|, {2,}`)
    reTypeStarNoSpace = regexp.MustCompile(ptrPattern)
    reBadPtrCast      = regexp.MustCompile(
        `\(\s*` + typeOrTypedef + `\s*\*\s*\)\s+[A-Za-z_\(]`,
    )
    reMacroNoSpace = regexp.MustCompile(`^\s*#\s*define\s+[A-Za-z_][A-Za-z0-9_]*\([^)]*\)\S`)
    reOpenBrace    = regexp.MustCompile(`\{\s*$`)
    reCloseBrace   = regexp.MustCompile(`^\}\s*$`)
    reFuncNameOnly = regexp.MustCompile(
        `^\s*[A-Za-z_][A-Za-z0-9_]*\s*\(`,
    )
    reOnlyType = regexp.MustCompile(
        `^\s*(?:` +
            `(?:static|const|unsigned|signed|short|long)\s+` +
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
    reTypeStartBrace = regexp.MustCompile(
        `^\s*(typedef\s+)?(struct|enum|union)\b\s*` +
            `([A-Za-z_][A-Za-z0-9_]*)?\s*\{`,
    )
    reInlineBlock = regexp.MustCompile(`^\s*[^{]*\{[^}]*\}.*$`)
    reInlineStmt  = regexp.MustCompile(
        `^\s*(?:if|else if|for|while|switch)\s*\([^)]*\)\s*[^\{\}]+;?\s*$`,
    )
    reInnerControl        = regexp.MustCompile(`\b(?:if|for|while|switch)\b`)
    reNoSpaceAfterParen   = regexp.MustCompile(`\)\S`)
    reControlParenNoSpace = regexp.MustCompile(`\b(?:if|else|for|while|switch)\(`)
    reInclude             = regexp.MustCompile(`^\s*#\s*include\s+([<"].+[>"])`)
    reIncludeFile         = regexp.MustCompile(`^\s*#\s*include\s+"([^"]+)"`)
    reFuncSigSingle       = regexp.MustCompile(
        `^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+` +
            `([A-Za-z_][A-Za-z0-9_]*)\s*` +
            `\(([^)]*)\)`,
    )
    reFuncSigStart = regexp.MustCompile(
        `^\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)+` +
            `([A-Za-z_][A-Za-z0-9_]*)\s*\(`,
    )
    reInner          = regexp.MustCompile(`\((.*)\)`)
    caseBraceRe      = regexp.MustCompile(`^(\s*case\s+[^:]+:)\s*\{\s*$`)
    reSemicolonSpace = regexp.MustCompile(`\s+;`)
    reStr            = regexp.MustCompile(`"((?:\\.|[^"\\])*)"`)
    reChar           = regexp.MustCompile(`'((?:\\.|[^'\\])*)'`)
    reOpPattern      = regexp.MustCompile(
        `>=|<=|==|!=|\+=|-=|\*=|/=|%=` +
            `|\+\+|--` +
            `|[=+\-*/%<>?:]`,
    )
    reIncludeStyle = regexp.MustCompile(`^\s*(#\s*include)\s+([<"].+[>"])`)
    reCamel        = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
    reFunctionName = regexp.MustCompile(`^[A-Z]+_[a-z][A-Za-z0-9]*$`)
    reBecaketCase  = regexp.MustCompile(`^(\s*)(case\s+[^:]+)\s*\{\s*$`)
    reFuncSigEnd   = regexp.MustCompile(`\)`)
    reCombinedPtr  = regexp.MustCompile(
        fmt.Sprintf("(%s)|(%s)|(%s)",
            reBadDeref.String(),
            reTypeStarNoSpace.String(),
            reBadCastLeading.String(),
        ),
    )
)

/** ===============================================================
 *              G L O B A L  V A R I A B L E S
 * ================================================================ */
var unsafeFuncSuggestions = map[string]string{
    "gets":     "fgets(buffer, size, stdin)",
    "strcpy":   "strlcpy(dest, src, dest_size) // or strncpy(dest, src, n)",
    "strcat":   "strlcat(dest, src, dest_size) // or strncat(dest, src, n)",
    "sprintf":  "snprintf(buffer, size, ...)",
    "vsprintf": "vsnprintf(buffer, size, ap)",
    "scanf":    "fgets(line, size, stdin) and then sscanf(line, \"%…\", &…)",
    "fscanf":   "fgets(line, size, file) and then sscanf(line, \"%…\", &…)",
    "sscanf":   "sscanf(line, \"%width…\", &…) // use width specifiers",
    "tmpnam":   "mkstemp(template) // or tmpfile()",
    "getwd":    "getcwd(buffer, size)",
}

var keywords = map[string]bool{
    "auto":           true,
    "break":          true,
    "case":           true,
    "char":           true,
    "const":          true,
    "continue":       true,
    "default":        true,
    "do":             true,
    "double":         true,
    "else":           true,
    "enum":           true,
    "extern":         true,
    "float":          true,
    "for":            true,
    "goto":           true,
    "if":             true,
    "inline":         true,
    "int":            true,
    "long":           true,
    "register":       true,
    "restrict":       true,
    "return":         true,
    "short":          true,
    "signed":         true,
    "sizeof":         true,
    "static":         true,
    "struct":         true,
    "switch":         true,
    "typedef":        true,
    "union":          true,
    "unsigned":       true,
    "void":           true,
    "volatile":       true,
    "while":          true,
    "_Alignas":       true,
    "_Alignof":       true,
    "_Atomic":        true,
    "_Bool":          true,
    "_Complex":       true,
    "_Generic":       true,
    "_Imaginary":     true,
    "_Noreturn":      true,
    "_Static_assert": true,
    "_Thread_local":  true,
}

var typesMap = map[string]bool{
    "int":    true,
    "char":   true,
    "float":  true,
    "double": true,
    "long":   true,
    "short":  true,
    "bool":   true,
}

var operatorRunes = map[rune]bool{
    '+': true,
    '-': true,
    '*': true,
    '/': true,
    '%': true,
    '=': true,
    '<': true,
    '>': true,
    '!': true,
    '&': true,
    '|': true,
    ';': true,
    ',': true,
    '.': true,
    ':': true,
    '?': true,
}

/** ===============================================================
 *                      C L I  T O O L S
 * ================================================================ */
const (
    Reset     = "\x1b[0m"
    Keyword   = "\x1b[31m"
    Type      = "\x1b[0;33m"
    Function  = "\x1b[32m"
    Variable  = "\x1b[0m"
    Number    = "\x1b[35m"
    StringC   = "\x1b[32m"
    Comment   = "\x1b[37m"
    Operator  = "\x1b[38;5;166m"
    Brackets  = "\x1b[38;5;172m"
    DefineCol = "\x1b[36m"

    ErrorBg   = "\x1b[41m"
    ErrorFg   = "\x1b[31m"
    WarningFg = "\x1b[33m"

    LineNumCol  = "\033[38;5;245m"
    PipeCol     = "\033[38;5;241m"
    ErrorNumber = "\033[38;5;39m"
    LetterCol   = "\x1b[94m"
    TitleCol    = "\033[34m"
)

var rainbowColors = []string{
    "\x1b[38;5;172m",
    "\x1b[32m",
    "\x1b[33m",
    "\x1b[34m",
    "\x1b[35m",
    "\x1b[36m",
    "\x1b[91m",
    "\x1b[92m",
    "\x1b[31m",
    "\x1b[94m",
}

const (
    banner = `  _____        __        ______       __    _______           __  
 / ___/__  ___/ /__ ____/ __/ /___ __/ /__ / ___/ /  ___ ____/ /__
/ /__/ _ \/ _  / -_)___/\ \/ __/ // / / -_) /__/ _ \/ -_) __/  '_/
\___/\___/\_,_/\__/   /___/\__/\_, /_/\__/\___/_//_/\__/\__/_/\_\ 
                              /___/            `
)

/** ===============================================================
 *                  M A I N  F U N C T I O N
 * ================================================================ */
func main() {
    styleFlag := flag.String("style", "kr", "style mode (\"kr\" or \"allman\")")
    flag.Parse()

    if flag.NArg() != 1 {
        fmt.Fprintf(os.Stderr, "Usage: %s [--style=kr|allman] <file.c/h>\n", os.Args[0])
        os.Exit(1)
    }
    filename := flag.Arg(0)

    styleMode, err := parseStyle(*styleFlag)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Erro: %v\n", err)
        os.Exit(1)
    }

    raw, err := os.ReadFile(filename)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filename, err)
        os.Exit(1)
    }
    lines := strings.Split(string(raw), "\n")

    errs := make([]StyleError, 0, 1000)

    styleErrs, err := LintFile(filename, styleMode)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            fmt.Fprintf(os.Stderr, "File not found: %s\n", filename)
        } else {
            fmt.Fprintf(os.Stderr, "Failed to process %s: %v\n", filename, err)
        }
        os.Exit(1)
    }
    errs = append(errs, styleErrs...)

    seen := make(map[string]bool, len(errs))
    uniqueErrs := make([]StyleError, 0, len(errs))
    for _, e := range errs {
        key := fmt.Sprintf("%d:%d:%s", e.LineNum, e.Start, e.Message)
        if !seen[key] {
            seen[key] = true
            uniqueErrs = append(uniqueErrs, e)
        }
    }

    if len(uniqueErrs) == 0 {
        fmt.Printf("No style issues found in %s\n", filename)
        os.Exit(0)
    }

    totalErrors, totalWarnings := 0, 0
    fmt.Printf("\n\n")
    fmt.Println(TitleCol + banner + Reset)
    fmt.Printf("\n")

    for _, e := range uniqueErrs {
        switch e.Level {
        case LevelError:
            totalErrors++
        case LevelWarning:
            totalWarnings++
        }

        levelColor := ErrorFg
        if e.Level == LevelWarning {
            levelColor = WarningFg
        }

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

    os.Exit(1)
}

/** ===============================================================
 *                   F I L E  F U N C T I O N
 * ================================================================ */
func parseStyle(s string) (StyleMode, error) {
    switch strings.ToLower(s) {
    case "kr":
        return StyleKR, nil
    case "allman":
        return StyleAllman, nil
    default:
        return 0, fmt.Errorf("invalid style: %q (use \"kr\" or \"allman\")", s)
    }
}

func (ctx *FileContext) ProcessIncludes() {
    errs := processIncludes(ctx.Lines, ctx.Filename)
    ctx.Errors = append(ctx.Errors, errs...)
}

func (ctx *FileContext) CheckEOFNewline() {
    checkEOFNewline(ctx.Raw, &ctx.Errors)
}

func (ctx *FileContext) CheckHeaderGuard() {
    checkHeaderGuard(ctx.Lines, ctx.Filename, &ctx.Errors)
}

func (ctx *FileContext) CheckStyle() {
    styleErrs := checkStyle(ctx.Lines, ctx.Style)
    ctx.Errors = append(ctx.Errors, styleErrs...)
}

func preprocessCaseBraces(lines []string) []string {
    var out []string
    for _, l := range lines {
        if m := caseBraceRe.FindStringSubmatch(l); m != nil {
            indent := l[:strings.Index(l, strings.TrimSpace(l))]
            out = append(out, m[1])
            out = append(out, indent+"{")
        } else {
            out = append(out, l)
        }
    }
    return out
}

func LintFile(filename string, style StyleMode) ([]StyleError, error) {
    raw, err := os.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    lines := strings.Split(string(raw), "\n")
    lines = preprocessCaseBraces(lines)

    ctx := &FileContext{
        Filename: filename,
        Lines:    lines,
        Raw:      raw,
        Style:    style,
        Errors:   nil,
    }

    ctx.ProcessIncludes()
    ctx.CheckEOFNewline()
    ctx.CheckHeaderGuard()
    ctx.CheckStyle()

    return ctx.Errors, nil
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

    errs := make([]StyleError, 0, 10000)

    type includeEntry struct {
        inc  string
        line int
    }
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
        if strings.HasSuffix(strings.ToLower(filename), ".h") {
            if m := reIncludeFile.FindStringSubmatch(l); m != nil && filepath.Base(filename) == m[1] {
                pos := strings.Index(l, m[1])
                errs = append(errs, StyleError{
                    LineNum: idx + 1,
                    Start:   pos,
                    Length:  len(m[1]),
                    Message: FormatMessage(ErrRecursiveInclusion, m[1]),
                    Level:   FormatErrorLevel(ErrRecursiveInclusion),
                })
            }
        }
    }

    if len(sysIncludes) > 0 && len(projIncludes) > 0 {
        firstProj := projIncludes[0]
        lastSys := sysIncludes[len(sysIncludes)-1]

        if firstProj.line < lastSys.line {
            full := lines[firstProj.line-1]
            start := strings.Index(full, "#")
            errs = append(errs, StyleError{
                LineNum: firstProj.line,
                Start:   start,
                Length:  utf8.RuneCountInString(full[start:]),
                Message: FormatMessage(ErrSysBeforeProjIncludesOrder),
                Level:   FormatErrorLevel(ErrSysBeforeProjIncludesOrder),
            })
        }
    }

    sortKeys := func(arr []includeEntry) (keys []string, lines []int) {
        keys = make([]string, len(arr))
        lines = make([]int, len(arr))
        for i, e := range arr {
            keys[i] = e.inc
            lines[i] = e.line
        }
        return keys, lines
    }

    if sysKeys, _ := sortKeys(sysIncludes); !sort.StringsAreSorted(sysKeys) {
        idx := findFirstUnsorted(sysKeys)
        bad := sysIncludes[idx-1]
        full := lines[bad.line-1]
        start := strings.Index(full, "#")
        errs = append(errs, StyleError{
            LineNum: bad.line,
            Start:   start,
            Length:  utf8.RuneCountInString(full[start:]),
            Message: FormatMessage(ErrSysIncludesNotSorted),
            Level:   FormatErrorLevel(ErrSysIncludesNotSorted),
        })
    }

    if projKeys, _ := sortKeys(projIncludes); !sort.StringsAreSorted(projKeys) {
        idx := findFirstUnsorted(projKeys)
        bad := projIncludes[idx-1]
        full := lines[bad.line-1]
        start := strings.Index(full, "#")
        errs = append(errs, StyleError{
            LineNum: bad.line,
            Start:   start,
            Length:  utf8.RuneCountInString(full[start:]),
            Message: FormatMessage(ErrProjIncludesNotSorted),
            Level:   FormatErrorLevel(ErrProjIncludesNotSorted),
        })
    }

    return errs
}

func checkPragmaOnce(
    lines []string,
    filename string,
    errs *[]StyleError,
) {
    hasPragmaOnce := false

    for _, l := range lines {
        if strings.TrimSpace(l) == "#pragma once" {
            hasPragmaOnce = true
            break
        }
    }
    if hasPragmaOnce && strings.HasSuffix(filename, ".h") {
        base := strings.ToUpper(strings.TrimSuffix(filepath.Base(filename), ".h"))
        guard := base + "_H"
        hasIfndef, hasDefine, hasEndif := false, false, false
        for _, l := range lines {
            t := strings.TrimSpace(l)
            if t == "#ifndef "+guard {
                hasIfndef = true
            }
            if t == "#define "+guard {
                hasDefine = true
            }
            if strings.HasPrefix(t, "#endif") {
                hasEndif = true
            }
        }
        if hasIfndef && hasDefine && hasEndif {
            *errs = append(*errs, StyleError{
                LineNum: 1,
                Start:   0,
                Length:  len("#pragma once"),
                Message: FormatMessage(ErrPragmaOnceAndIncludeGuard),
                Level:   FormatErrorLevel(ErrPragmaOnceAndIncludeGuard),
            })
        }
    }
}

func readLines(filename string) ([]string, error) {
    f, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    reader := bufio.NewReader(f)
    var lines []string

    for {
        line, err := reader.ReadString('\n')
        if err == io.EOF {
            if len(line) > 0 {
                lines = append(lines, strings.TrimRight(line, "\n"))
            }
            break
        }
        if err != nil {
            return nil, err
        }
        lines = append(lines, strings.TrimRight(line, "\n"))
    }

    return lines, nil
}

func FormatMessage(code ErrorCode, args ...interface{}) string {

    if code < 0 || code >= ErrorCode(NumErrorMessages) {
        return fmt.Sprintf("unknown error code %d", code)
    }
    return fmt.Sprintf(errorInfos[code].Message, args...)
}

func FormatErrorLevel(code ErrorCode) string {
    if code < 0 || code >= NumErrorMessages {
        return "UNKNOWN"
    }
    return errorInfos[code].Level
}

/** ===============================================================
 *          C H E C K  -  S T Y L E  F U N C T I O N
 * ================================================================ */
func checkStyle(lines []string, style StyleMode) []StyleError {
    const maskRune = '\uFFFD'

    var errs []StyleError
    var typeStack []typeCtx
    var typeTag string
    var pendingTagLine int
    var pendingTagPos int
    var caseEndLine = -1
    var caseIndentLevel = 0

    indentStack := []int{0}
    pendingTypeDecl := false
    pendingTypedef := false
    inBlockComment := false
    inParamBlock := false
    nextIndent := -1
    indentForStack := 0
    paramIndent := 0
    typeKind := ""

    blankCountTracker := make([]int, len(lines))

    pointerRegexes := []*regexp.Regexp{
        rePtrDecl,
        reTypedefFuncPtr,
        reCastPtr,
        reDeref,
        reGenericPtr,
        reGenericPtrComma,
        reGenericPtrParen,
    }

    for i, line := range lines {
        trim := strings.TrimSpace(line)
        codeOnly := line

        indent := getIndent(line)

        checkLineLength(i, line, &errs)

        checkConsecutiveBlankLines(i, lines, &blankCountTracker, &errs)

        lastBlankCount := blankCountTracker[len(lines)-1]

        checkTrailingBlankLinesIfEOF(lines, lastBlankCount, &errs)

        checkBlankLinesAfterFunction(0, lines, &errs)

        if handleInBlockComment(&codeOnly, i, &inBlockComment, maskRune, &errs) {
            continue
        }

        if handleFullLineComment(trim, line, i, &errs) {
            continue
        }

        if handleBlockCommentStart(trim, line, i, &inBlockComment, &errs) {
            continue
        }

        handleInlineComment(&codeOnly, maskRune, i, &errs)

        maskStringLiterals(&codeOnly, maskRune)

        maskCharLiterals(&codeOnly, maskRune)

        if shouldContinueIfOnlyMask(&codeOnly, maskRune) {
            continue
        }

        checkSemicolonSpace(i, codeOnly, &errs)
        checkNonASCII(i, line, &errs)

        if handleClosingElse(trim, &indentStack) {
            continue
        }

        checkKRElse(style, trim, lines, i, line, &errs)

        if handleIncludeIndentation(trim, indent, i, &errs) {
            continue
        }

        checkBadParenSpace(codeOnly, i, &errs)
        checkBadBracketSpace(codeOnly, i, &errs)
        checkBadCommaSpace(codeOnly, i, &errs)
        checkMultipleSpaces(codeOnly, i, &errs)

        checkPointerFormatting(codeOnly, i, reCombinedPtr, reBadPtrCast, &errs)

        checkMacroBodyNoSpace(line, i, &errs)

        checkMacroDefIdentifiers(line, codeOnly, i, &errs)

        checkOperatorSpacing(codeOnly, trim, i, pointerRegexes, &errs)

        checkKeywordSpaceBeforeParen(codeOnly, line, i, &errs)

        checkMagicNumberUsage(codeOnly, trim, i, &errs)

        if checkParamBlock(line, trim, indent, i, &inParamBlock, &paramIndent, &errs) {
            continue
        }

        if checkBlankLine(trim, indent, i, &errs) {
            continue
        }

        checkTrailingWhitespace(line, i, &errs)

        if checkLabelDecl(trim, line, indent, i, &errs) {
            continue
        }

        if !inParamBlock {
            checkFuncDeclName(codeOnly, line, i, &errs)
        }

        checkPrevLineOnlyTypeFuncName(lines, line, i, &errs)

        checkReturnTypeSameLine(lines, line, trim, i, &errs)
        checkFuncCallSpace(line, i+1, &errs)

        checkCloseIndent(trim, codeOnly, &indentStack)

        if checkCaseBlock(trim, line, i, lines, indent, &indentStack, &caseIndentLevel, &caseEndLine, &errs) {
            continue
        }

        if checkIndentRules(i, trim, codeOnly, indent,
            &indentStack, &nextIndent, &caseEndLine, &indentForStack, &errs) {
            continue
        }

        checkOpenBrace(i, trim, lines, indentForStack, &indentStack)

        checkControlStmtIndent(trim, codeOnly, indentForStack, &nextIndent)

        if isOnlyWhitespace(codeOnly) {
            continue
        }

        if checkInlineBlockOrStmt(trim, line, i, &errs) {
            continue
        }

        ctx := getCurrentCtx(typeStack)

        checkTypeStart(
            trim,
            i,
            lines,
            &pendingTypeDecl,
            &pendingTypedef,
            &typeKind,
            &typeTag,
            &pendingTagLine,
            &pendingTagPos,
            &typeStack,
        )

        if checkTypeClosing(codeOnly, line, i, lines, &typeStack, &errs) {
            continue
        }

        checkDataStructureFields(ctx, trim, line, codeOnly, i, &errs)
        checkUninitializedDecls(ctx, codeOnly, line, i, &errs)
        checkVarNameNotEndWithT(trim, codeOnly, line, i, &errs)
        checkMultipleVarDecl(inParamBlock, codeOnly, line, i, &errs)
        checkTypedefFuncPtrName(codeOnly, line, i, &errs)
        checkTypedefGenericName(codeOnly, line, i, &errs)
        checkMacroNameScreamingSnake(codeOnly, line, i, &errs)
        checkFuncMacroBodyParenthesized(line, i, &errs)
        checkParamNamesSnakeCase(line, codeOnly, i, &errs)
        checkTernarySpacing(codeOnly, i, &errs)
        checkFuncOpeningBraceOwnLine(line, codeOnly, i, &errs)
        checkAllmanBrace(style, line, codeOnly, i, &errs)

        prevTrim := getPrevLine(lines, i)

        checkKRBrace(style, line, codeOnly, prevTrim, i, &errs)
        checkClosingBraceOwnLine(trim, lines, i, &errs)
        checkAllocCallMustBeCast(codeOnly, i, &errs)
        checkUnsafeFunctions(codeOnly, i+1, &errs)
        checkConstPointerParams(lines, &errs)
    }

    return errs
}

/** ===============================================================
 *               C H E C K I N G  F U N C T I O N S
 * ================================================================ */
func processCommentRules(
    text string,
    lineNum int,
    errs *[]StyleError,
) {
    if m := reTodo.FindStringIndex(text); m != nil {
        *errs = append(*errs, StyleError{
            LineNum: lineNum,
            Start:   m[0],
            Length:  m[1] - m[0],
            Message: FormatMessage(WarnFoundTODOOrFIXME),
            Level:   FormatErrorLevel(WarnFoundTODOOrFIXME),
        })
    }
}

func checkUnsafeFunctions(
    codeOnly string,
    lineNum int,
    errs *[]StyleError,
) {
    var reUnsafeFunc = regexp.MustCompile(

        `\b(` + strings.Join(func() []string {
            keys := make([]string, 0, len(unsafeFuncSuggestions))
            for k := range unsafeFuncSuggestions {
                keys = append(keys, k)
            }
            return keys
        }(), "|") + `)\s*\(`,
    )

    for _, loc := range reUnsafeFunc.FindAllStringSubmatchIndex(codeOnly, -1) {
        name := codeOnly[loc[2]:loc[3]]
        suggestion := unsafeFuncSuggestions[name]
        *errs = append(*errs, StyleError{
            LineNum: lineNum,
            Start:   loc[2],
            Length:  len(name),
            Message: FormatMessage(WarnUseOfInsecureFunction, name, suggestion),
            Level:   FormatErrorLevel(WarnUseOfInsecureFunction),
        })
    }
}

func checkConstPointerParams(
    lines []string,
    errs *[]StyleError,
) {
    checkParams := func(rawParams string, functionLine int) {
        parts := strings.Split(rawParams, ",")
        for _, p := range parts {
            p = strings.TrimSpace(p)

            if strings.HasPrefix(p, "*") {
                continue
            }

            if !strings.Contains(p, "*") || strings.HasPrefix(p, "const ") {
                continue
            }

            fields := strings.Fields(p)
            rawName := fields[len(fields)-1]
            name := strings.TrimLeft(rawName, "*")

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

                if strings.Contains(l, name+" =") || strings.Contains(l, "*"+name+" =") {
                    modified = true
                    break
                }
            }

            if !modified {
                pos := strings.Index(lines[functionLine], name)
                if pos < 0 {
                    pos = 0
                }
                *errs = append(*errs, StyleError{
                    LineNum: functionLine + 1,
                    Start:   pos,
                    Length:  len(name),
                    Message: FormatMessage(WarnPointerNotModifiedMustBeConst, name, p),
                    Level:   FormatErrorLevel(WarnPointerNotModifiedMustBeConst),
                })
            }
        }
    }

    inMultiline := false
    multilineLines := []string{}
    startLineIdx := 0

    for idx, line := range lines {
        if !inMultiline {
            if m := reFuncSigSingle.FindStringSubmatch(line); m != nil {
                rawParams := m[2]
                checkParams(rawParams, idx)
                continue
            }

            if m := reFuncSigStart.FindStringSubmatch(line); m != nil && !strings.Contains(line, ")") {
                inMultiline = true
                multilineLines = []string{line}
                startLineIdx = idx
                continue
            }
        } else {
            multilineLines = append(multilineLines, line)
            if reFuncSigEnd.MatchString(line) {
                fullSig := strings.Join(multilineLines, " ")
                if m2 := reInner.FindStringSubmatch(fullSig); m2 != nil {
                    rawParams := m2[1]
                    checkParams(rawParams, startLineIdx)
                }

                inMultiline = false
                multilineLines = nil
                continue
            }
        }
    }
}

func checkHeaderGuard(
    lines []string,
    filename string,
    errs *[]StyleError,
) {
    if !strings.HasSuffix(filename, ".h") {
        return
    }

    base := strings.ToUpper(strings.TrimSuffix(filepath.Base(filename), ".h"))
    guard := base + "_H"
    hasIfndef, hasDefine, hasEndif := false, false, false

    for _, l := range lines {
        t := strings.TrimSpace(l)
        if t == "#ifndef "+guard {
            hasIfndef = true
        }
        if t == "#define "+guard {
            hasDefine = true
        }
        if strings.HasPrefix(t, "#endif") {
            hasEndif = true
        }
    }
    if !hasIfndef || !hasDefine || !hasEndif {
        *errs = append(*errs, StyleError{
            LineNum: 1,
            Start:   0,
            Length:  0,
            Message: FormatMessage(ErrPragmaOnceAndIncludeGuard),
            Level:   FormatErrorLevel(ErrPragmaOnceAndIncludeGuard),
        })
    }
}

func checkEOFNewline(
    raw []byte,
    errs *[]StyleError,
) {

    if len(raw) == 0 || raw[len(raw)-1] == '\n' {
        return
    }

    lines := bytes.Split(raw, []byte("\n"))
    lastLine := lines[len(lines)-1]
    col := utf8.RuneCount(lastLine)

    *errs = append(*errs, StyleError{
        LineNum: len(lines),
        Start:   col - 1,
        Length:  1,
        Message: FormatMessage(ErrFileMustEndWithNewline),
        Level:   FormatErrorLevel(ErrFileMustEndWithNewline),
    })
}

func checkLineLength(
    i int,
    line string,
    errs *[]StyleError,
) {
    const maxLineLength = 80
    if l := utf8.RuneCountInString(line); l > maxLineLength {
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   maxLineLength,
            Length:  l - maxLineLength,
            Message: FormatMessage(ErrLineLengthExceeded, maxLineLength, l),
            Level:   FormatErrorLevel(ErrLineLengthExceeded),
        })
    }
}

func checkConsecutiveBlankLines(
    i int,
    lines []string,
    errCount *[]int,
    errs *[]StyleError,
) {
    if i < 0 || i >= len(lines) || errCount == nil || errs == nil || *errCount == nil {
        return
    }

    line := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
    isBlank := (line == "")

    if isBlank {
        prev := 0
        if i > 0 {
            prev = (*errCount)[i-1]
        }
        (*errCount)[i] = prev + 1

        if (*errCount)[i] >= 2 {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   0,
                Length:  0,
                Message: FormatMessage(WarnTooManyBlankLinesConsecutively, (*errCount)[i]),
                Level:   FormatErrorLevel(WarnTooManyBlankLinesConsecutively),
            })
        }
    } else {
        (*errCount)[i] = 0
    }
}

func checkTrailingBlankLinesIfEOF(
    lines []string,
    blankCount int,
    errs *[]StyleError,
) {
    if blankCount > 1 {
        *errs = append(*errs, StyleError{
            LineNum: len(lines),
            Start:   0,
            Length:  0,
            Message: FormatMessage(WarnFileEndsWithExtraBlankLines, blankCount),
            Level:   FormatErrorLevel(WarnFileEndsWithExtraBlankLines),
        })
    }
}

func checkBlankLinesAfterFunction(
    i int,
    lines []string,
    errs *[]StyleError,
) {
    for i < len(lines) {
        line := strings.TrimSpace(lines[i])
        if reFuncDecl.MatchString(line) {
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
            blankCount := 0
            k := j + 1
            for k < len(lines) && strings.TrimSpace(lines[k]) == "" {
                blankCount++
                k++
            }
            if k < len(lines) && reFuncDecl.MatchString(strings.TrimSpace(lines[k])) {
                if blankCount == 0 {
                    *errs = append(*errs, StyleError{
                        LineNum: j + 1,
                        Start:   0,
                        Length:  0,
                        Message: FormatMessage(ErrMissingBlankLineAfterFunction),
                        Level:   FormatErrorLevel(ErrMissingBlankLineAfterFunction),
                    })
                } else if blankCount > 1 {
                    *errs = append(*errs, StyleError{
                        LineNum: j + 2,
                        Start:   0,
                        Length:  0,
                        Message: FormatMessage(WarnTooManyBlankLinesBetweenFunctions, blankCount),
                        Level:   FormatErrorLevel(WarnTooManyBlankLinesBetweenFunctions),
                    })
                }
            }
            i = j
        }
        i++
    }
}

func checkSemicolonSpace(
    i int,
    codeOnly string,
    errs *[]StyleError,
) {
    if loc := reSemicolonSpace.FindStringIndex(codeOnly); loc != nil {
        start := loc[0]
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   start,
            Length:  loc[1] - loc[0],
            Message: FormatMessage(ErrNoSpaceBeforeSemicolon),
            Level:   FormatErrorLevel(ErrNoSpaceBeforeSemicolon),
        })
    }
}

func checkNonASCII(
    i int,
    line string,
    errs *[]StyleError,
) {
    for idx, ch := range line {
        if ch > 127 {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   idx,
                Length:  1,
                Message: FormatMessage(WarnNonASCIICharacter, ch),
                Level:   FormatErrorLevel(WarnNonASCIICharacter),
            })
        }
    }
}

func handleInBlockComment(
    codeOnly *string,
    i int,
    inBlockComment *bool,
    maskRune rune,
    errs *[]StyleError,
) bool {
    if !*inBlockComment {
        return false
    }
    if end := strings.Index(*codeOnly, "*/"); end >= 0 {
        *codeOnly = strings.Repeat(string(maskRune), end+2) + (*codeOnly)[end+2:]
        *inBlockComment = false
        return false
    }

    processCommentRules(*codeOnly, i+1, errs)
    return true
}

func handleFullLineComment(
    trim string,
    line string,
    i int,
    errs *[]StyleError,
) bool {
    if strings.HasPrefix(trim, "//") {
        processCommentRules(line, i+1, errs)
        return true
    }
    return false
}

func handleBlockCommentStart(
    trim string,
    line string,
    i int,
    inBlockComment *bool,
    errs *[]StyleError,
) bool {
    if !strings.HasPrefix(trim, "/*") {
        return false
    }
    processCommentRules(line, i+1, errs)
    if !strings.Contains(trim, "*/") {
        *inBlockComment = true
    }
    return true
}

func handleInlineComment(
    codeOnly *string,
    maskRune rune,
    i int,
    errs *[]StyleError,
) {
    if idx := strings.Index(*codeOnly, "//"); idx >= 0 {
        commentPart := (*codeOnly)[idx:]
        processCommentRules(commentPart, i+1, errs)
        *codeOnly = (*codeOnly)[:idx] + strings.Repeat(string(maskRune), len(*codeOnly)-idx)
    }
}

func maskStringLiterals(codeOnly *string, maskRune rune) {
    *codeOnly = reStr.ReplaceAllStringFunc(*codeOnly, func(s string) string {
        return strings.Repeat(string(maskRune), len(s))
    })
}

func maskCharLiterals(codeOnly *string, maskRune rune) {
    *codeOnly = reChar.ReplaceAllStringFunc(*codeOnly, func(s string) string {
        return strings.Repeat(string(maskRune), len(s))
    })
}

func shouldContinueIfOnlyMask(codeOnly *string, maskRune rune) bool {
    trimmed := strings.Trim(*codeOnly, string(maskRune))
    return trimmed == ""
}

func handleClosingElse(trim string, indentStack *[]int) bool {
    if !strings.HasPrefix(trim, "} else {") {
        return false
    }
    if len(*indentStack) > 1 {
        *indentStack = (*indentStack)[:len(*indentStack)-1]
    }

    indentForStack := (*indentStack)[len(*indentStack)-1]
    *indentStack = append(*indentStack, indentForStack+2)
    return true
}

func checkKRElse(
    style StyleMode,
    trim string,
    lines []string,
    i int,
    line string,
    errs *[]StyleError,
) {
    if style != StyleKR || trim != "else {" || i == 0 {
        return
    }
    if strings.TrimSpace(lines[i-1]) == "}" {
        pos := strings.Index(line, "else")
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   pos,
            Length:  len("else"),
            Message: FormatMessage(ErrElseMustBeOnSameLineAsClosingBrace),
            Level:   FormatErrorLevel(ErrElseMustBeOnSameLineAsClosingBrace),
        })
    }
}

func getIndent(line string) int {
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
    return indent
}

func handleIncludeIndentation(
    trim string,
    indent int,
    i int,
    errs *[]StyleError,
) bool {
    if !strings.HasPrefix(trim, "#include") {
        return false
    }
    if indent != 0 {
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   0,
            Length:  indent,
            Message: FormatMessage(ErrIncludeDirectiveIndentation),
            Level:   FormatErrorLevel(ErrIncludeDirectiveIndentation),
        })
    }
    return true
}

func checkBadParenSpace(
    codeOnly string,
    i int,
    errs *[]StyleError,
) {
    if locs := reBadParenSpace.FindAllStringIndex(codeOnly, -1); locs != nil {
        for _, loc := range locs {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   loc[0],
                Length:  loc[1] - loc[0],
                Message: FormatMessage(ErrNoSpaceAllowedInsideParentheses),
                Level:   FormatErrorLevel(ErrNoSpaceAllowedInsideParentheses),
            })
        }
    }
}

func checkBadBracketSpace(
    codeOnly string,
    i int,
    errs *[]StyleError,
) {
    if locs := reBadBracketSpace.FindAllStringIndex(codeOnly, -1); locs != nil {
        for _, loc := range locs {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   loc[0],
                Length:  loc[1] - loc[0],
                Message: FormatMessage(ErrNoSpaceAllowedAroundBrackets),
                Level:   FormatErrorLevel(ErrNoSpaceAllowedAroundBrackets),
            })
        }
    }
}

func checkBadCommaSpace(
    codeOnly string,
    lineIndex int,
    errs *[]StyleError,
) {
    if locs := reBadComma.FindAllStringIndex(codeOnly, -1); locs != nil {
        for _, loc := range locs {
            *errs = append(*errs, StyleError{
                LineNum: lineIndex + 1,
                Start:   loc[0],
                Length:  loc[1] - loc[0],
                Message: FormatMessage(ErrCommaMustBeSurroundedBySingleSpace),
                Level:   FormatErrorLevel(ErrCommaMustBeSurroundedBySingleSpace),
            })
        }
    }
}

func checkMultipleSpaces(
    codeOnly string,
    lineIndex int,
    errs *[]StyleError,
) {
    for _, loc := range reMultiSpace.FindAllStringIndex(codeOnly, -1) {
        start := loc[0] + 1
        length := loc[1] - loc[0] - 2
        if length < 1 {
            length = 1
        }
        *errs = append(*errs, StyleError{
            LineNum: lineIndex + 1,
            Start:   start,
            Length:  length,
            Message: FormatMessage(ErrMultipleConsecutiveSpaces),
            Level:   FormatErrorLevel(ErrMultipleConsecutiveSpaces),
        })
    }
}

func checkPointerFormatting(
    codeOnly string,
    lineNum int,
    reCombinedPtr, reBadPtrCast *regexp.Regexp,
    errs *[]StyleError,
) {
    if locs := reCombinedPtr.FindAllStringIndex(codeOnly, -1); locs != nil {
        for _, loc := range locs {
            *errs = append(*errs, StyleError{
                LineNum: lineNum + 1,
                Start:   loc[0],
                Length:  loc[1] - loc[0],
                Message: FormatMessage(ErrPointerFormattingRules),
                Level:   FormatErrorLevel(ErrPointerFormattingRules),
            })
        }
    }

    if locs := reBadPtrCast.FindAllStringIndex(codeOnly, -1); locs != nil {
        for _, loc := range locs {
            *errs = append(*errs, StyleError{
                LineNum: lineNum + 1,
                Start:   loc[0],
                Length:  loc[1] - loc[0],
                Message: FormatMessage(ErrPointerCastMustBeAttached),
                Level:   FormatErrorLevel(ErrPointerCastMustBeAttached),
            })
        }
    }
}

func checkMacroBodyNoSpace(
    line string,
    lineNum int,
    errs *[]StyleError,
) {
    if reMacroNoSpace.MatchString(line) {
        pos := strings.Index(line, ")")
        *errs = append(*errs, StyleError{
            LineNum: lineNum + 1,
            Start:   pos,
            Length:  1,
            Message: FormatMessage(ErrMacroBodyMustHaveSpaceAfterParams),
            Level:   FormatErrorLevel(ErrMacroBodyMustHaveSpaceAfterParams),
        })
    }
}

func checkMacroDefIdentifiers(
    line, codeOnly string,
    lineNum int,
    errs *[]StyleError,
) {
    if m := reMacroDef.FindStringSubmatchIndex(codeOnly); m != nil {
        macroName := line[m[2]:m[3]]
        rawParams := line[m[4]:m[5]]
        macroBody := line[m[6]:m[7]]

        params := []string{}
        for _, p := range strings.Split(rawParams, ",") {
            name := strings.TrimSpace(p)
            if !snakePattern.MatchString(name) {
                pos := strings.Index(line, name)
                *errs = append(*errs, StyleError{
                    LineNum: lineNum + 1,
                    Start:   pos,
                    Length:  len(name),
                    Message: FormatMessage(ErrMacroParamMustBeSnakeCase, name),
                    Level:   FormatErrorLevel(ErrMacroParamMustBeSnakeCase),
                })
            }
            params = append(params, name)
        }

        for _, loc := range reIdent.FindAllStringIndex(macroBody, -1) {
            ident := macroBody[loc[0]:loc[1]]
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
            if !snakePattern.MatchString(ident) {
                pos := strings.Index(line, ident)
                *errs = append(*errs, StyleError{
                    LineNum: lineNum + 1,
                    Start:   pos,
                    Length:  len(ident),
                    Message: FormatMessage(ErrMacroBodyIdentifierMustBeSnakeCase, ident),
                    Level:   FormatErrorLevel(ErrMacroBodyIdentifierMustBeSnakeCase),
                })
            }
        }
    }
}

func checkOperatorSpacing(
    codeOnly string,
    trim string,
    lineNum int,
    pointerRegexes []*regexp.Regexp,
    errs *[]StyleError,
) {
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

    for _, loc := range reOpPattern.FindAllStringIndex(codeOnly, -1) {
        op := codeOnly[loc[0]:loc[1]]
        startIdx := loc[0]
        endIdx := loc[1]

        if op == ":" &&
            strings.HasSuffix(trim, ":") &&
            (strings.HasPrefix(trim, "case ") ||
                strings.HasPrefix(trim, "default") ||
                reLabelDecl.MatchString(trim)) {
            continue
        }

        if op == "++" || op == "--" {
            continue
        }

        if op == "*" && inPtrRange(startIdx, endIdx) {
            continue
        }

        if startIdx > 0 &&
            !unicode.IsSpace(rune(codeOnly[startIdx-1])) {
            *errs = append(*errs, StyleError{
                LineNum: lineNum + 1,
                Start:   startIdx,
                Length:  len(op),
                Message: FormatMessage(ErrOperatorMustHaveSpaceBefore, op),
                Level:   FormatErrorLevel(ErrOperatorMustHaveSpaceBefore),
            })
        }

        if endIdx < len(codeOnly) &&
            !unicode.IsSpace(rune(codeOnly[endIdx])) {
            *errs = append(*errs, StyleError{
                LineNum: lineNum + 1,
                Start:   startIdx,
                Length:  len(op),
                Message: FormatMessage(ErrOperatorMustHaveSpaceAfter, op),
                Level:   FormatErrorLevel(ErrOperatorMustHaveSpaceAfter),
            })
        }
    }
}

func checkKeywordSpaceBeforeParen(
    codeOnly string,
    line string,
    lineNum int,
    errs *[]StyleError,
) {
    if m := reKeywordNoSpace.FindStringSubmatchIndex(codeOnly); m != nil {
        kw := line[m[0] : m[1]-1]
        *errs = append(*errs, StyleError{
            LineNum: lineNum + 1,
            Start:   m[0],
            Length:  len(kw),
            Message: FormatMessage(ErrKeywordMustHaveSpaceBeforeParen),
            Level:   FormatErrorLevel(ErrKeywordMustHaveSpaceBeforeParen),
        })
    }
}

func checkMagicNumberUsage(
    codeOnly string,
    trim string,
    lineNum int,
    errs *[]StyleError,
) {
    for _, loc := range reMagicNumber.FindAllStringIndex(codeOnly, -1) {
        num := codeOnly[loc[0]:loc[1]]

        if reEnumElement.MatchString(codeOnly) || strings.HasPrefix(trim, "#define") {
            continue
        }

        *errs = append(*errs, StyleError{
            LineNum: lineNum + 1,
            Start:   loc[0],
            Length:  loc[1] - loc[0],
            Message: FormatMessage(WarnMagicNumberDetected, num),
            Level:   FormatErrorLevel(WarnMagicNumberDetected),
        })
    }
}

func checkParamBlock(
    line, trim string,
    indent, lineNum int,
    inParamBlock *bool,
    paramIndent *int,
    errs *[]StyleError,
) bool {
    if !*inParamBlock && strings.Contains(line, "(") && strings.HasSuffix(trim, ",") {

        var loc []int
        if loc = reSplitFuncName.FindStringSubmatchIndex(line); loc == nil {
            loc = reFuncHeader.FindStringSubmatchIndex(line)
        }
        if loc != nil {
            name := line[loc[2]:loc[3]]

            if loc[3] < len(line) && line[loc[3]] == ' ' {
                *errs = append(*errs, StyleError{
                    LineNum: lineNum + 1,
                    Start:   loc[3],
                    Length:  1,
                    Message: FormatMessage(ErrFuncNameNoSpaceBeforeParen),
                    Level:   FormatErrorLevel(ErrFuncNameNoSpaceBeforeParen),
                })
            }

            if name != "main" && !reFunctionName.MatchString(name) {
                pos := strings.Index(line, name)
                *errs = append(*errs, StyleError{
                    LineNum: lineNum + 1,
                    Start:   pos,
                    Length:  len(name),
                    Message: FormatMessage(ErrFunctionNameMustBeModuleCamelCase, name),
                    Level:   FormatErrorLevel(ErrFunctionNameMustBeModuleCamelCase),
                })
            }
        }

        if pp := strings.Index(line, "("); pp >= 0 {
            *paramIndent = ((pp + 2) / 2) * 2
        }
        *inParamBlock = true
        return true
    }

    if *inParamBlock {
        if indent != *paramIndent {
            *errs = append(*errs, StyleError{
                LineNum: lineNum + 1,
                Start:   0,
                Length:  indent,
                Message: FormatMessage(ErrParameterLineWrongIndent, *paramIndent, indent),
                Level:   FormatErrorLevel(ErrParameterLineWrongIndent),
            })
        }

        if strings.Contains(trim, ")") {
            *inParamBlock = false
            return true
        }
        if !strings.HasSuffix(trim, ",") {
            *errs = append(*errs, StyleError{
                LineNum: lineNum + 1,
                Start:   len(line) - 1,
                Length:  1,
                Message: FormatMessage(ErrParameterLineMustEndWithComma),
                Level:   FormatErrorLevel(ErrParameterLineMustEndWithComma),
            })
        }
        return true
    }

    return false
}

func isPureClose(trim string) bool {
    return strings.HasPrefix(trim, "}") && !strings.Contains(trim, "{")
}

func isInlineClose(trim, codeOnly string) bool {
    return strings.Contains(trim, "}") &&
        !strings.Contains(trim, "{") &&
        !strings.HasPrefix(trim, "}") &&
        !reClosingAll.MatchString(codeOnly)
}

func checkBlankLine(
    trim string,
    indent,
    lineNum int,
    errs *[]StyleError,
) bool {
    if trim == "" {
        if indent != 0 {
            *errs = append(*errs, StyleError{
                LineNum: lineNum + 1,
                Start:   0,
                Length:  indent,
                Message: FormatMessage(ErrBlankLineWithIndentation),
                Level:   FormatErrorLevel(ErrBlankLineWithIndentation),
            })
        }
        return true
    }
    return false
}

func checkTrailingWhitespace(
    line string,
    lineNum int,
    errs *[]StyleError,
) {
    if loc := reTrailing.FindStringIndex(line); loc != nil {
        startCol := utf8.RuneCountInString(line[:loc[0]])
        length := utf8.RuneCountInString(line[loc[0]:loc[1]])
        *errs = append(*errs, StyleError{
            LineNum: lineNum + 1,
            Start:   startCol,
            Length:  length,
            Message: FormatMessage(ErrTrailingWhitespace),
            Level:   FormatErrorLevel(ErrTrailingWhitespace),
        })
    }
}

func checkLabelDecl(
    trim,
    line string,
    indent,
    i int,
    errs *[]StyleError,
) bool {
    if m := reLabelDecl.FindStringSubmatch(trim); m != nil {
        label := m[1]
        ws := m[2]

        if label != "case" && label != "default" {
            if indent != 0 {
                *errs = append(*errs, StyleError{
                    LineNum: i + 1,
                    Start:   0,
                    Length:  indent,
                    Message: FormatMessage(ErrLabelMustHaveNoIndentation),
                    Level:   FormatErrorLevel(ErrLabelMustHaveNoIndentation),
                })
            }

            if !snakePattern.MatchString(label) {
                pos := strings.Index(line, label)
                *errs = append(*errs, StyleError{
                    LineNum: i + 1,
                    Start:   pos,
                    Length:  len(label),
                    Message: FormatMessage(ErrLabelMustBeSnakeLowerCase, label),
                    Level:   FormatErrorLevel(ErrLabelMustBeSnakeLowerCase),
                })
            }

            if ws != "" {
                col := strings.Index(line, ":")
                *errs = append(*errs, StyleError{
                    LineNum: i + 1,
                    Start:   col - len(ws),
                    Length:  len(ws) + 1,
                    Message: FormatMessage(ErrColonMustBeAttachedToToken),
                    Level:   FormatErrorLevel(ErrColonMustBeAttachedToToken),
                })
            }

            return true
        }
    }
    return false
}

func checkFuncDeclName(
    codeOnly,
    line string,
    i int,
    errs *[]StyleError,
) {
    if m := reFuncDecl.FindStringSubmatchIndex(codeOnly); m != nil {
        name := line[m[2]:m[3]]
        if name != "main" && !reFunctionName.MatchString(name) {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   m[2],
                Length:  len(name),
                Message: FormatMessage(ErrFunctionNameMustBeModuleCamelCase, name),
                Level:   FormatErrorLevel(ErrFunctionNameMustBeModuleCamelCase),
            })
        }
    }
}

func checkPrevLineOnlyTypeFuncName(lines []string, line string, i int, errs *[]StyleError) {
    if i == 0 {
        return
    }
    prevTrim := strings.TrimSpace(lines[i-1])
    if reOnlyType.MatchString(prevTrim) {
        if m := reSplitFuncName.FindStringSubmatchIndex(line); m != nil {
            name := line[m[2]:m[3]]
            if name != "main" && !reFunctionName.MatchString(name) {
                pos := strings.Index(line, name)
                *errs = append(*errs, StyleError{
                    LineNum: i + 1,
                    Start:   pos,
                    Length:  len(name),
                    Message: FormatMessage(ErrLabelMustBeSnakeLowerCase, name),
                    Level:   FormatErrorLevel(ErrLabelMustBeSnakeLowerCase),
                })
            }
        }
    }
}

func checkReturnTypeSameLine(
    lines []string,
    line,
    trim string,
    i int,
    errs *[]StyleError,
) {
    if !reOnlyType.MatchString(strings.TrimSpace(line)) || i+1 >= len(lines) {
        return
    }
    nextTrim := strings.TrimSpace(lines[i+1])
    if reFuncNameOnly.MatchString(nextTrim) {
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   0,
            Length:  utf8.RuneCountInString(trim),
            Message: FormatMessage(ErrReturnTypeMustBeOnSameLineAsName),
            Level:   FormatErrorLevel(ErrReturnTypeMustBeOnSameLineAsName),
        })
    }
}

func checkFuncCallSpace(
    line string,
    lineNum int,
    errs *[]StyleError,
) {
    if matches := reIdentSpaceParen.FindAllStringSubmatchIndex(line, -1); matches != nil {
        for _, m := range matches {
            name := line[m[2]:m[3]]
            if keywords[name] {
                continue
            }
            *errs = append(*errs, StyleError{
                LineNum: lineNum,
                Start:   m[2],
                Length:  m[3] - m[2],
                Message: FormatMessage(ErrSpaceBeforeFuncCallParen),
                Level:   FormatErrorLevel(ErrSpaceBeforeFuncCallParen),
            })
        }
    }
}

func checkCloseIndent(
    trim,
    codeOnly string,
    indentStack *[]int,
) bool {
    switch {
    case isPureClose(trim) && len(*indentStack) > 1:
        *indentStack = (*indentStack)[:len(*indentStack)-1]
        return true
    case isInlineClose(trim, codeOnly) && len(*indentStack) > 1:
        *indentStack = (*indentStack)[:len(*indentStack)-1]
        return true
    default:
        return false
    }
}

func checkCaseBlock(
    trim string,
    line string,
    i int,
    lines []string,
    indent int,
    indentStack *[]int,
    caseIndentLevel *int,
    caseEndLine *int,
    errs *[]StyleError,
) bool {
    if !(strings.HasPrefix(trim, "case ") || (strings.HasPrefix(trim, "default") && strings.HasSuffix(trim, ":"))) {
        return false
    }

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
            return true
        }
    }

    hasBraceSame := strings.Contains(trim, "{")
    hasBraceNext := false
    for k := i + 1; k < len(lines); k++ {
        nt := strings.TrimSpace(lines[k])
        if nt == "" || strings.HasPrefix(nt, "//") {
            continue
        }
        hasBraceNext = (nt == "{")
        break
    }
    if hasBraceSame || hasBraceNext {
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   strings.Index(line, "{"),
            Length:  1,
            Message: FormatMessage(WarnCaseBlocksMustNotUseBraces),
            Level:   FormatErrorLevel(WarnCaseBlocksMustNotUseBraces),
        })
        return true
    }

    if strings.Contains(trim, " :") {
        col := strings.Index(line, " :")
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   col,
            Length:  2,
            Message: FormatMessage(ErrTernaryColonMustHaveSpaceAfter),
            Level:   FormatErrorLevel(ErrTernaryColonMustHaveSpaceAfter),
        })
    }

    found := -1
    for j := i + 1; j < len(lines); j++ {
        t := strings.TrimSpace(lines[j])
        if strings.HasPrefix(t, "case ") || t == "default:" {
            break
        }
        if t == "break;" {
            found = j
            break
        }
    }

    if found == -1 {
        hasFallThrough := false
        for k := i + 1; k < len(lines); k++ {
            nextTrim := strings.TrimSpace(lines[k])
            if strings.HasPrefix(nextTrim, "case ") || nextTrim == "default:" {
                break
            }
            if (strings.HasPrefix(nextTrim, "//") || strings.HasPrefix(nextTrim, "/*")) &&
                strings.Contains(nextTrim, "fall-through") {
                hasFallThrough = true
                break
            }
            if nextTrim != "" && !strings.HasPrefix(nextTrim, "//") && !strings.HasPrefix(nextTrim, "/*") {
                break
            }
        }
        if !hasFallThrough {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   strings.Index(line, ":"),
                Length:  1,
                Message: FormatMessage(WarnCaseBlockMissingBreakOrFallthrough, strings.TrimRight(trim, ":")),
                Level:   FormatErrorLevel(WarnCaseBlockMissingBreakOrFallthrough),
            })
        }
    } else {
        *caseIndentLevel = indent
        *indentStack = append(*indentStack, *caseIndentLevel+2)
        *caseEndLine = found
    }

    return true
}

func checkIndentRules(
    i int,
    trim, codeOnly string,
    indent int,
    indentStack *[]int,
    nextIndent *int,
    caseEndLine *int,
    indentForStack *int,
    errs *[]StyleError,
) bool {
    expected := (*indentStack)[len(*indentStack)-1]
    if *nextIndent >= 0 && !strings.HasPrefix(trim, "{") && trim != "}" && !reInlineBlock.MatchString(trim) {
        expected = *nextIndent
    }

    if *nextIndent >= 0 && (strings.HasPrefix(trim, "{") || reInlineBlock.MatchString(trim)) {
        *nextIndent = -1
    }

    *indentForStack = indent

    if indent != expected && !isInlineClose(trim, codeOnly) {
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   0,
            Length:  indent,
            Message: FormatMessage(ErrParameterLineWrongIndent, expected, indent),
            Level:   FormatErrorLevel(ErrParameterLineWrongIndent),
        })
        *indentForStack = expected
    }

    if *nextIndent >= 0 && !strings.HasPrefix(trim, "{") {
        *nextIndent = -1
    }

    if i == *caseEndLine {
        if len(*indentStack) > 1 {
            *indentStack = (*indentStack)[:len(*indentStack)-1]
        }
        *caseEndLine = -1
        *nextIndent = -1
        return true
    }

    return false
}

func checkOpenBrace(
    i int,
    trim string,
    lines []string,
    indentForStack int,
    indentStack *[]int,
) {
    if strings.Contains(trim, "{") && !strings.Contains(trim, "}") && !reInlineBlock.MatchString(trim) {
        nextIdx := i + 1

        for nextIdx < len(lines) {
            nxt := strings.TrimSpace(lines[nextIdx])
            if nxt == "" || strings.HasPrefix(nxt, "//") || strings.HasPrefix(nxt, "/*") {
                nextIdx++
                continue
            }

            if reCloseBrace.MatchString(nxt) {
                *indentStack = append(*indentStack, indentForStack)
            } else {
                *indentStack = append(*indentStack, indentForStack+2)
            }
            break
        }

        if nextIdx >= len(lines) {
            *indentStack = append(*indentStack, indentForStack+2)
        }
    }
}

func checkControlStmtIndent(
    trim,
    codeOnly string,
    indentForStack int,
    nextIndent *int,
) bool {
    if reControlStmt.MatchString(trim) && !strings.Contains(trim, "{") {
        if !reInlineStmt.MatchString(trim) {
            *nextIndent = indentForStack + 2
        }
        return true
    }
    return false
}

func isOnlyWhitespace(codeOnly string) bool {
    return strings.TrimSpace(codeOnly) == ""
}

func checkInlineBlockOrStmt(
    trim,
    line string,
    i int,
    errs *[]StyleError,
) bool {
    if !(reInlineBlock.MatchString(trim) || reInlineStmt.MatchString(trim)) {
        return false
    }

    if m := reControlParenNoSpace.FindStringIndex(trim); m != nil {
        stmtIdx := strings.Index(line, trim)
        pos := stmtIdx + (m[1] - 1)
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   pos,
            Length:  1,
            Message: FormatMessage(ErrKeywordMustHaveSpaceBeforeParen),
            Level:   FormatErrorLevel(ErrKeywordMustHaveSpaceBeforeParen),
        })
    }

    if rels := reNoSpaceAfterParen.FindStringIndex(trim); rels != nil {
        stmtIdx := strings.Index(line, trim)
        pos := stmtIdx + (rels[0] + 1)
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   pos,
            Length:  1,
            Message: FormatMessage(ErrKeywordMustHaveSpaceBeforeParen),
            Level:   FormatErrorLevel(ErrKeywordMustHaveSpaceBeforeParen),
        })
    }

    var inner string
    var innerOffset int
    hasBrace := strings.Index(line, "{") >= 0

    if hasBrace {
        b := strings.Index(line, "{")
        e := strings.LastIndex(line, "}")
        innerOffset = b + 1
        inner = line[innerOffset:e]
    } else {
        p := strings.Index(trim, ")")
        stmtIdx := strings.Index(line, trim)
        innerOffset = stmtIdx + p + 1
        inner = trim[p+1:]
    }

    if hasBrace && inner == "" {
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   strings.Index(line, "{"),
            Length:  2,
            Message: FormatMessage(ErrInlineEmptyBraceMustHaveSpaces),
            Level:   FormatErrorLevel(ErrInlineEmptyBraceMustHaveSpaces),
        })
        return true
    }

    if hasBrace {
        if inner[0] != ' ' {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   innerOffset,
                Length:  1,
                Message: FormatMessage(ErrExpectedSpaceAfterOpeningBrace),
                Level:   FormatErrorLevel(ErrExpectedSpaceAfterOpeningBrace),
            })
        }
        if inner[len(inner)-1] != ' ' {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   innerOffset + len(inner) - 1,
                Length:  1,
                Message: FormatMessage(ErrExpectedSpaceAfterClosingBrace),
                Level:   FormatErrorLevel(ErrExpectedSpaceAfterClosingBrace),
            })
        }
    }

    if hasBrace {
        if idx := strings.IndexAny(inner, "{}"); idx >= 0 {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   innerOffset + idx,
                Length:  1,
                Message: FormatMessage(ErrInlineBlockMustNotContainNestedBraces),
                Level:   FormatErrorLevel(ErrInlineBlockMustNotContainNestedBraces),
            })
        }
    }

    if strings.TrimSpace(inner) != "" {
        parts := strings.Split(inner, ";")
        cnt := 0
        for _, s := range parts {
            if strings.TrimSpace(s) != "" {
                cnt++
            }
        }
        if cnt != 1 {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   innerOffset,
                Length:  len(inner),
                Message: FormatMessage(ErrInlineBlockMustContainOneStatement),
                Level:   FormatErrorLevel(ErrInlineBlockMustContainOneStatement),
            })
        }
    }

    if m2 := reInnerControl.FindStringIndex(inner); m2 != nil {
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   innerOffset + m2[0],
            Length:  m2[1] - m2[0],
            Message: FormatMessage(ErrInlineBlockMustNotContainControlStatements),
            Level:   FormatErrorLevel(ErrInlineBlockMustNotContainControlStatements),
        })
    }

    return true
}

func getCurrentCtx(typeStack []typeCtx) *typeCtx {
    if len(typeStack) > 0 {
        return &typeStack[len(typeStack)-1]
    }
    return nil
}

func checkTypeStart(
    trim string,
    i int,
    lines []string,
    pendingTypeDecl *bool,
    pendingTypedef *bool,
    typeKind *string,
    typeTag *string,
    pendingTagLine *int,
    pendingTagPos *int,
    typeStack *[]typeCtx,
) {
    if m := reTypeStart.FindStringSubmatch(trim); m != nil {
        *pendingTypeDecl = true
        *pendingTypedef = m[1] != ""
        *typeKind = m[2]
        *typeTag = m[3]
        *pendingTagLine = i + 1
        *pendingTagPos = strings.Index(lines[i], *typeTag)
    } else if m := reTypeStartBrace.FindStringSubmatch(trim); m != nil {
        newCtx := typeCtx{
            isDataStructure: true,
            isTypedef:       m[1] != "",
            dataType:        m[2],
            tagName:         m[3],
            tagLine:         i + 1,
            tagPos:          strings.Index(lines[i], m[3]),
        }
        *typeStack = append(*typeStack, newCtx)
        *pendingTypeDecl = false
    } else if *pendingTypeDecl && strings.Contains(trim, "{") {
        newCtx := typeCtx{
            isDataStructure: true,
            isTypedef:       *pendingTypedef,
            dataType:        *typeKind,
            tagName:         *typeTag,
            tagLine:         *pendingTagLine,
            tagPos:          *pendingTagPos,
        }
        *typeStack = append(*typeStack, newCtx)
        *pendingTypeDecl = false
    }
}

func checkTypeClosing(
    codeOnly string,
    line string,
    i int,
    lines []string,
    typeStack *[]typeCtx,
    errs *[]StyleError,
) bool {
    if len(*typeStack) == 0 {
        return false
    }

    ctx := &(*typeStack)[len(*typeStack)-1]

    if m := reClosingAll.FindStringSubmatchIndex(codeOnly); m != nil {
        nameStart, nameEnd := m[2], m[3]

        instanceName := ""
        if nameEnd > nameStart {
            instanceName = line[nameStart:nameEnd]
        }

        bracePos := strings.Index(lines[i], "}")
        if bracePos >= 0 && bracePos+1 < len(lines[i]) && lines[i][bracePos+1] != ' ' {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   bracePos + 1,
                Length:  1,
                Message: FormatMessage(ErrExpectedSpaceAfterClosingBrace),
                Level:   FormatErrorLevel(ErrExpectedSpaceAfterClosingBrace),
            })
        }

        if ctx.isTypedef {
            if instanceName == "" {
                bracePos := strings.Index(lines[i], "}")
                *errs = append(*errs, StyleError{
                    LineNum: i + 1,
                    Start:   bracePos,
                    Length:  1,
                    Message: FormatMessage(WarnTypedefGenericNameMustBeSnakeLowerCaseAndEndWithT, ctx.dataType),
                    Level:   FormatErrorLevel(WarnTypedefGenericNameMustBeSnakeLowerCaseAndEndWithT),
                })
            } else if !strings.HasSuffix(instanceName, "_t") || !snakeTypedefPattern.MatchString(instanceName) {
                *errs = append(*errs, StyleError{
                    LineNum: i + 1,
                    Start:   nameStart,
                    Length:  nameEnd - nameStart,
                    Message: FormatMessage(WarnTypedefGenericNameMustBeSnakeLowerCaseAndEndWithT, instanceName),
                    Level:   FormatErrorLevel(WarnTypedefGenericNameMustBeSnakeLowerCaseAndEndWithT),
                })
            }
        } else if instanceName != "" {
            if !snakePattern.MatchString(instanceName) {
                *errs = append(*errs, StyleError{
                    LineNum: i + 1,
                    Start:   nameStart,
                    Length:  nameEnd - nameStart,
                    Message: FormatMessage(ErrInstanceMustBeSnakeLowerCase, ctx.dataType, instanceName),
                    Level:   FormatErrorLevel(ErrInstanceMustBeSnakeLowerCase),
                })
            }
            if strings.HasSuffix(instanceName, "_t") {
                *errs = append(*errs, StyleError{
                    LineNum: i + 1,
                    Start:   nameStart,
                    Length:  nameEnd - nameStart,
                    Message: FormatMessage(ErrInstanceMustNotEndWithT, ctx.dataType, instanceName),
                    Level:   FormatErrorLevel(ErrInstanceMustNotEndWithT),
                })
            }
        }

        if ctx.tagName != "" {
            if !reCamel.MatchString(ctx.tagName) {
                *errs = append(*errs, StyleError{
                    LineNum: ctx.tagLine,
                    Start:   ctx.tagPos,
                    Length:  len(ctx.tagName),
                    Message: FormatMessage(ErrTypeTagMustBeCamelCase, ctx.dataType, ctx.tagName),
                    Level:   FormatErrorLevel(ErrTypeTagMustBeCamelCase),
                })
            }
        }

        *typeStack = (*typeStack)[:len(*typeStack)-1]
        return true
    }

    return false
}

func checkDataStructureFields(
    ctx *typeCtx,
    trim string,
    line string,
    codeOnly string,
    i int,
    errs *[]StyleError,
) {
    if ctx == nil || !ctx.isDataStructure {
        return
    }

    switch ctx.dataType {
    case "enum":
        if m := reEnumElement.FindStringSubmatchIndex(trim); m != nil {
            name := trim[m[2]:m[3]]
            if !screamingSnakePattern.MatchString(name) {
                start := strings.Index(line, name)
                *errs = append(*errs, StyleError{
                    LineNum: i + 1,
                    Start:   start,
                    Length:  len(name),
                    Message: FormatMessage(ErrEnumElementMustBeScreamingSnakeCase, name),
                    Level:   FormatErrorLevel(ErrEnumElementMustBeScreamingSnakeCase),
                })
            }
        }

    case "struct", "union":
        if m := reStructFieldName.FindStringSubmatchIndex(codeOnly); m != nil {
            name := line[m[2]:m[3]]
            if !snakePattern.MatchString(name) {
                *errs = append(*errs, StyleError{
                    LineNum: i + 1,
                    Start:   m[2],
                    Length:  m[3] - m[2],
                    Message: FormatMessage(ErrStructFieldMustBeSnakeLowerCase, ctx.dataType, name),
                    Level:   FormatErrorLevel(ErrStructFieldMustBeSnakeLowerCase),
                })
            }
        }
    }
}

func checkUninitializedDecls(
    ctx *typeCtx,
    codeOnly, line string,
    i int,
    errs *[]StyleError,
) {
    if ctx == nil || !ctx.isDataStructure {
        if m := reUninitDecl.FindStringSubmatchIndex(codeOnly); m != nil {
            decl := line[m[4]:m[5]]
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   m[4],
                Length:  m[5] - m[4],
                Message: FormatMessage(WarnDeclaredWithoutInitialization, decl),
                Level:   FormatErrorLevel(WarnDeclaredWithoutInitialization),
            })
        }
    }
}

func checkVarNameNotEndWithT(
    trim, codeOnly, line string,
    i int,
    errs *[]StyleError,
) {
    if strings.HasPrefix(trim, "typedef") {
        return
    }
    if m := reVarDeclName.FindStringSubmatchIndex(codeOnly); m != nil {
        nameStart, nameEnd := m[2], m[3]
        varName := line[nameStart:nameEnd]
        if strings.HasSuffix(varName, "_t") {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   nameStart,
                Length:  nameEnd - nameStart,
                Message: FormatMessage(ErrVariableNameMustNotEndWithT, varName),
                Level:   FormatErrorLevel(ErrVariableNameMustNotEndWithT),
            })
        }
    }
}

func checkMultipleVarDecl(
    inParamBlock bool,
    codeOnly, line string,
    i int,
    errs *[]StyleError,
) {
    if !inParamBlock && reMultiVarDecl.MatchString(codeOnly) {
        pos := strings.Index(line, ",")
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   pos,
            Length:  1,
            Message: FormatMessage(ErrMultipleVariableDeclarationsNotAllowed),
            Level:   FormatErrorLevel(ErrMultipleVariableDeclarationsNotAllowed),
        })
    }
}

func checkTypedefFuncPtrName(
    codeOnly, line string,
    i int,
    errs *[]StyleError,
) {
    if m := reTypedefFuncPtr.FindStringSubmatchIndex(codeOnly); m != nil {
        name := line[m[2]:m[3]]
        if !snakeTypedefPattern.MatchString(name) {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   m[2],
                Length:  m[3] - m[2],
                Message: FormatMessage(
                    WarnTypedefGenericNameMustBeSnakeLowerCaseAndEndWithT,
                    name,
                ),
                Level: FormatErrorLevel(WarnTypedefGenericNameMustBeSnakeLowerCaseAndEndWithT),
            })
        }
    }
}

func checkTypedefGenericName(
    codeOnly, line string,
    i int,
    errs *[]StyleError,
) {
    if m := reTypedefGeneric.FindStringSubmatchIndex(codeOnly); m != nil {
        name := line[m[2]:m[3]]
        if !snakeTypedefPattern.MatchString(name) {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   m[2],
                Length:  m[3] - m[2],
                Message: FormatMessage(
                    WarnTypedefGenericNameMustBeSnakeLowerCaseAndEndWithT,
                    name,
                ),
                Level: FormatErrorLevel(WarnTypedefGenericNameMustBeSnakeLowerCaseAndEndWithT),
            })
        }
    }
}

func checkMacroNameScreamingSnake(
    codeOnly, line string,
    i int,
    errs *[]StyleError,
) {
    if m := reDefine.FindStringSubmatchIndex(codeOnly); m != nil {
        name := line[m[2]:m[3]]
        if !screamingSnakePattern.MatchString(name) {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   m[2],
                Length:  m[3] - m[2],
                Message: FormatMessage(ErrMacroNameMustBeScreamingSnakeCase, name),
                Level:   FormatErrorLevel(ErrMacroNameMustBeScreamingSnakeCase),
            })
        }
    }
}

func checkFuncMacroBodyParenthesized(
    line string,
    i int,
    errs *[]StyleError,
) {
    if m := reFuncMacro.FindStringSubmatchIndex(line); m != nil {
        body := line[m[4]:m[5]]
        if !(strings.HasPrefix(body, "(") && strings.HasSuffix(body, ")")) {
            pos := strings.Index(line, body)
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   pos,
                Length:  len(body),
                Message: FormatMessage(ErrFunctionLikeMacroBodyMustBeParenthesized),
                Level:   FormatErrorLevel(ErrFunctionLikeMacroBodyMustBeParenthesized),
            })
        }
    }
}

func checkParamNamesSnakeCase(
    line, codeOnly string,
    i int,
    errs *[]StyleError,
) {
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
                    *errs = append(*errs, StyleError{
                        LineNum: i + 1,
                        Start:   idx,
                        Length:  len(name),
                        Message: FormatMessage(ErrParameterNameMustBeSnakeLowerCase, name),
                        Level:   FormatErrorLevel(ErrParameterNameMustBeSnakeLowerCase),
                    })
                }
            }
        }
    }
}

func checkTernarySpacing(
    codeOnly string,
    i int,
    errs *[]StyleError,
) {
    for _, loc := range reTernaryQNoSpaceBefore.FindAllStringIndex(codeOnly, -1) {
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   loc[0] + 1,
            Length:  1,
            Message: FormatMessage(ErrTernaryQuestionMarkMustHaveSpaceBefore),
            Level:   FormatErrorLevel(ErrTernaryQuestionMarkMustHaveSpaceBefore),
        })
    }
    for _, loc := range reTernaryQNoSpaceAfter.FindAllStringIndex(codeOnly, -1) {
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   loc[0],
            Length:  1,
            Message: FormatMessage(ErrTernaryQuestionMarkMustHaveSpaceAfter),
            Level:   FormatErrorLevel(ErrTernaryQuestionMarkMustHaveSpaceAfter),
        })
    }
    for _, loc := range reTernaryColonNoSpaceBefore.FindAllStringIndex(codeOnly, -1) {
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   loc[0] + 1,
            Length:  1,
            Message: FormatMessage(ErrTernaryColonMustHaveSpaceBefore),
            Level:   FormatErrorLevel(ErrTernaryColonMustHaveSpaceBefore),
        })
    }
    for _, loc := range reTernaryColonNoSpaceAfter.FindAllStringIndex(codeOnly, -1) {
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   loc[0],
            Length:  1,
            Message: FormatMessage(ErrTernaryColonMustHaveSpaceAfter),
            Level:   FormatErrorLevel(ErrTernaryColonMustHaveSpaceAfter),
        })
    }
}

func checkFuncOpeningBraceOwnLine(
    line, codeOnly string,
    i int,
    errs *[]StyleError,
) {
    if strings.Contains(line, "{") && reFuncDecl.MatchString(codeOnly) {
        pos := strings.Index(line, "{")
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   pos,
            Length:  1,
            Message: FormatMessage(ErrFunctionOpeningBraceMustBeOnOwnLine),
            Level:   FormatErrorLevel(ErrFunctionOpeningBraceMustBeOnOwnLine),
        })
    }
}

func checkAllmanBrace(
    style StyleMode,
    line, codeOnly string,
    i int,
    errs *[]StyleError,
) {
    if style != StyleAllman {
        return
    }
    if reControlStmt.MatchString(codeOnly) && strings.Contains(line, "{") {
        pos := strings.Index(line, "{")
        kind := reControlStmt.FindString(line)
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   pos,
            Length:  1,
            Message: FormatMessage(ErrAllmanOpeningBraceMustBeOwnLine, kind),
            Level:   FormatErrorLevel(ErrAllmanOpeningBraceMustBeOwnLine),
        })
    }
}

func checkKRBrace(
    style StyleMode,
    line, codeOnly, prevLine string,
    i int,
    errs *[]StyleError,
) {
    if style != StyleKR {
        return
    }

    if reControlStmt.MatchString(codeOnly) {
        if idx := strings.Index(line, "){"); idx != -1 {
            *errs = append(*errs, StyleError{
                LineNum: i + 1,
                Start:   idx + 1,
                Length:  1,
                Message: FormatMessage(ErrKRMissingSpaceBeforeBrace),
                Level:   FormatErrorLevel(ErrKRMissingSpaceBeforeBrace),
            })
        }
    }

    if reBraceOnlyLine.MatchString(codeOnly) && i > 0 && reControlStmt.MatchString(prevLine) {
        pos := strings.Index(line, "{")
        kind := reControlStmt.FindString(prevLine)
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   pos,
            Length:  1,
            Message: FormatMessage(ErrKROpeningBraceMustBeSameLineAsControl, kind),
            Level:   FormatErrorLevel(ErrKROpeningBraceMustBeSameLineAsControl),
        })
    }
}

func checkClosingBraceOwnLine(
    trim string,
    lines []string,
    i int,
    errs *[]StyleError,
) {
    if reInlineBlock.MatchString(trim) {
        return
    }
    if !strings.Contains(trim, "{") {
        return
    }
    braceDepth := 1
    for j := i + 1; j < len(lines) && braceDepth > 0; j++ {
        t := strings.TrimSpace(lines[j])
        if t == "" || strings.HasPrefix(t, "//") {
            continue
        }
        if strings.Contains(t, "{") {
            braceDepth++
        }
        if strings.Contains(t, "}") {
            braceDepth--
            if braceDepth == 0 &&
                !reCloseBrace.MatchString(t) &&
                !reClosingAll.MatchString(t) {
                pos := strings.Index(lines[j], "}")
                *errs = append(*errs, StyleError{
                    LineNum: j + 1,
                    Start:   pos,
                    Length:  1,
                    Message: FormatMessage(ErrClosingBraceMustBeOwnLine),
                    Level:   FormatErrorLevel(ErrClosingBraceMustBeOwnLine),
                })
            }
            break
        }
    }
}

func getPrevLine(lines []string, i int) string {
    if i > 0 {
        return strings.TrimSpace(lines[i-1])
    }
    return ""
}

func checkAllocCallMustBeCast(
    codeOnly string,
    i int,
    errs *[]StyleError,
) {
    if reAllocCall.MatchString(codeOnly) && !reAllocCast.MatchString(codeOnly) {
        loc := reAllocCall.FindStringIndex(codeOnly)[0]
        name := reAllocCall.FindString(codeOnly)
        *errs = append(*errs, StyleError{
            LineNum: i + 1,
            Start:   loc,
            Length:  len(name),
            Message: FormatMessage(ErrAllocCallMustBeCast, name),
            Level:   FormatErrorLevel(ErrAllocCallMustBeCast),
        })
    }
}

/** ===============================================================
 *                  C L I  F U N C T I O N S
 * ================================================================ */
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

    if start < 0 {
        start = 0
    } else if start > n {
        start = n
    }

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

func matchingOpen(br, open rune) bool {

    switch br {
    case '}':
        return open == '{'
    case ')':
        return open == '('
    case ']':
        return open == '['
    }

    return false
}

func highlightLine(line string) string {

    if loc := reMacroDefLine.FindStringSubmatchIndex(line); loc != nil {
        before := line[:loc[0]]
        directive := line[loc[2]:loc[3]]
        macroName := line[loc[4]:loc[5]]
        paramsInner := ""

        if loc[6] != -1 {
            paramsInner = line[loc[6]:loc[7]]
        }

        rest := line[loc[1]:]

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

    if m := reIncludeStyle.FindStringSubmatchIndex(line); m != nil {
        before := line[:m[0]]
        directive := line[m[2]:m[3]]
        path := line[m[4]:m[5]]
        after := line[m[5]:]

        return before +
            DefineCol + directive + Reset + " " +
            StringC + path + Reset +
            after
    }

    var sb strings.Builder
    var stack []rune
    r := []rune(line)

    for i := 0; i < len(r); {
        ch := r[i]

        if ch == '/' && i+1 < len(r) && r[i+1] == '/' {
            sb.WriteString(Comment + string(r[i:]) + Reset)
            break
        }

        if ch == '"' {
            start := i
            i++
            for i < len(r) && !(r[i] == '"' && r[i-1] != '\\') {
                i++
            }
            if i < len(r) {
                i++
            }
            sb.WriteString(StringC + string(r[start:i]) + Reset)
            continue
        }

        if unicode.IsDigit(ch) {
            start := i
            for i < len(r) && (unicode.IsDigit(r[i]) || r[i] == '.') {
                i++
            }
            sb.WriteString(Number + string(r[start:i]) + Reset)
            continue
        }

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

        if unicode.IsLetter(ch) || ch == '_' {
            start := i

            for i < len(r) && (unicode.IsLetter(r[i]) || unicode.IsDigit(r[i]) || r[i] == '_') {
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

        if unicode.IsSpace(ch) {
            sb.WriteRune(ch)
            i++
            continue
        }

        if operatorRunes[ch] {
            sb.WriteString(Operator + string(ch) + Reset)
            i++
            continue
        }

        sb.WriteRune(ch)
        i++
    }

    return sb.String()
}
